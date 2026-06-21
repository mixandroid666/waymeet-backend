package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"waymeet-backend/internal/notification"
	"waymeet-backend/internal/platform/storage"
	"waymeet-backend/internal/platform/storage/dbgen"
)

var ErrNotFound = errors.New("not found")
var ErrForbidden = errors.New("forbidden")

type ConversationSummary struct {
	ID               string
	PartnerID        string
	PartnerName      string
	PartnerAvatarURL string
	LastMessage      string
	LastSenderID     string
	LastMessageAt    *time.Time
}

type Message struct {
	ID             string
	ConversationID string
	SenderID       string
	Body           string
	MsgType        string
	MediaURL       string
	CreatedAt      time.Time
}

type Service struct {
	hub    *Hub
	db     *storage.DB
	notify *notification.Service
	log    *slog.Logger
}

func NewService(hub *Hub, db *storage.DB, notify *notification.Service, log *slog.Logger) *Service {
	return &Service{hub: hub, db: db, notify: notify, log: log}
}

func (s *Service) GetOrCreateConversation(ctx context.Context, userA, userB string) (string, error) {
	a, err := parseUUID(userA)
	if err != nil {
		return "", ErrNotFound
	}
	b, err := parseUUID(userB)
	if err != nil {
		return "", ErrNotFound
	}
	row, err := s.db.Queries.GetOrCreateConversation(ctx, dbgen.GetOrCreateConversationParams{UserA: a, UserB: b})
	if err != nil {
		return "", err
	}
	convID := uuidString(row.ID)

	// If the partner is already online, push their presence to the creator immediately.
	if s.hub.IsOnline(userB) {
		if env, err := json.Marshal(presenceEnvelope{Type: "presence", UserID: userB, Status: "online"}); err == nil {
			s.hub.deliver(userA, env)
		}
	}
	return convID, nil
}

func (s *Service) ListConversations(ctx context.Context, viewerID string) ([]ConversationSummary, error) {
	vid, err := parseUUID(viewerID)
	if err != nil {
		return nil, ErrNotFound
	}
	rows, err := s.db.Queries.ListConversations(ctx, vid)
	if err != nil {
		return nil, err
	}
	out := make([]ConversationSummary, 0, len(rows))
	for _, r := range rows {
		var lastAt *time.Time
		if r.LastMessageAt.Valid {
			t := r.LastMessageAt.Time
			lastAt = &t
		}
		out = append(out, ConversationSummary{
			ID:               uuidString(r.ID),
			PartnerID:        uuidString(r.PartnerID),
			PartnerName:      deref(r.PartnerName),
			PartnerAvatarURL: deref(r.PartnerAvatarUrl),
			LastMessage:      deref(r.LastMessage),
			LastSenderID:     uuidString(r.LastSenderID),
			LastMessageAt:    lastAt,
		})
	}
	return out, nil
}

func (s *Service) ListMessages(ctx context.Context, viewerID, conversationID string, limit, offset int32) ([]Message, error) {
	vid, err := parseUUID(viewerID)
	if err != nil {
		return nil, ErrNotFound
	}
	cid, err := parseUUID(conversationID)
	if err != nil {
		return nil, ErrNotFound
	}

	_, err = s.db.Queries.GetConversationPartner(ctx, dbgen.GetConversationPartnerParams{ViewerID: vid, ConversationID: cid})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrForbidden
		}
		return nil, err
	}

	rows, err := s.db.Queries.ListMessages(ctx, dbgen.ListMessagesParams{ConversationID: cid, Lim: limit, Off: offset})
	if err != nil {
		return nil, err
	}
	out := make([]Message, 0, len(rows))
	for _, r := range rows {
		mediaURL := ""
		if r.MediaURL != nil {
			mediaURL = *r.MediaURL
		}
		msgType := r.MsgType
		if msgType == "" {
			msgType = "text"
		}
		out = append(out, Message{
			ID:             uuidString(r.ID),
			ConversationID: uuidString(r.ConversationID),
			SenderID:       uuidString(r.SenderID),
			Body:           r.Body,
			MsgType:        msgType,
			MediaURL:       mediaURL,
			CreatedAt:      r.CreatedAt.Time,
		})
	}
	return out, nil
}

// inboundMsg is the JSON payload a client sends over WebSocket.
type inboundMsg struct {
	ConversationID string `json:"conversation_id"`
	Body           string `json:"body"`
	MsgType        string `json:"msg_type"`  // "text" | "sticker" | "image"; defaults to "text"
	MediaURL       string `json:"media_url"` // required when msg_type = "image"
}

// Envelope is the JSON payload delivered to each party for chat messages.
type Envelope struct {
	Type           string `json:"type"`
	ID             string `json:"id"`
	ConversationID string `json:"conversation_id"`
	From           string `json:"from"`
	Body           string `json:"body"`
	MsgType        string `json:"msg_type"`
	MediaURL       string `json:"media_url,omitempty"`
	Ts             string `json:"ts"`
}

// presenceEnvelope is the JSON payload for online/offline presence events.
type presenceEnvelope struct {
	Type   string `json:"type"`
	UserID string `json:"user_id"`
	Status string `json:"status"` // "online" | "offline"
}

func (s *Service) handleInbound(sender *Client, raw []byte) {
	var msg inboundMsg
	if err := json.Unmarshal(raw, &msg); err != nil || msg.ConversationID == "" {
		return
	}

	msgType := msg.MsgType
	if msgType == "" {
		msgType = "text"
	}

	var mediaURL *string
	switch msgType {
	case "text", "sticker":
		if msg.Body == "" {
			return
		}
	case "image":
		if msg.MediaURL == "" {
			return
		}
		// Normalize body to a display-friendly preview used in conversation lists.
		msg.Body = "ðŸ“· Photo"
		mediaURL = &msg.MediaURL
	default:
		return
	}

	cid, err := parseUUID(msg.ConversationID)
	if err != nil {
		return
	}
	sid, err := parseUUID(sender.userID)
	if err != nil {
		return
	}

	partnerUUID, err := s.db.Queries.GetConversationPartner(context.Background(), dbgen.GetConversationPartnerParams{
		ViewerID:       sid,
		ConversationID: cid,
	})
	if err != nil {
		s.log.Debug("chat: sender not in conversation", "user", sender.userID, "conv", msg.ConversationID)
		return
	}

	saved, err := s.db.Queries.CreateMessage(context.Background(), dbgen.CreateMessageParams{
		ConversationID: cid,
		SenderID:       sid,
		Body:           msg.Body,
		MsgType:        msgType,
		MediaURL:       mediaURL,
	})
	if err != nil {
		s.log.Error("chat: failed to persist message", "err", err)
		return
	}

	env, err := json.Marshal(Envelope{
		Type:           "message",
		ID:             uuidString(saved.ID),
		ConversationID: msg.ConversationID,
		From:           sender.userID,
		Body:           msg.Body,
		MsgType:        msgType,
		MediaURL:       msg.MediaURL,
		Ts:             saved.CreatedAt.Time.UTC().Format(time.RFC3339),
	})
	if err != nil {
		return
	}

	partnerID := uuidString(partnerUUID)
	if !s.hub.deliver(partnerID, env) {
		s.log.Debug("chat: partner offline, sending push", "partner", partnerID)
		go s.pushChat(partnerID, sender.userID, msg.Body, msg.ConversationID)
	}
	_ = sender.tryDeliver(env)
}

// pushChat looks up the sender's display name and fires a push notification.
func (s *Service) pushChat(toUserID, senderID, body, conversationID string) {
	if s.notify == nil {
		return
	}
	senderName := "New message"
	if sid, err := parseUUID(senderID); err == nil {
		if u, err := s.db.Queries.GetUserByID(context.Background(), sid); err == nil && u.DisplayName != nil {
			senderName = *u.DisplayName
		}
	}
	s.notify.SendChat(context.Background(), toUserID, senderName, body, conversationID)
}

// StartPresenceWorker drains presenceC and fans out online/offline events to conversation partners.
func (s *Service) StartPresenceWorker(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case change, ok := <-s.hub.PresenceC():
				if !ok {
					return
				}
				s.broadcastPresence(change)
			}
		}
	}()
}

func (s *Service) broadcastPresence(change presenceChange) {
	uid, err := parseUUID(change.userID)
	if err != nil {
		return
	}
	ctx := context.Background()
	partners, err := s.db.Queries.GetConversationPartners(ctx, uid)
	if err != nil {
		s.log.Debug("presence: failed to get partners", "err", err)
		return
	}
	status := "offline"
	if change.online {
		status = "online"
	}
	env, err := json.Marshal(presenceEnvelope{Type: "presence", UserID: change.userID, Status: status})
	if err != nil {
		return
	}
	for _, p := range partners {
		if id := uuidString(p); id != "" {
			s.hub.deliver(id, env)
		}
	}
}

func parseUUID(s string) (pgtype.UUID, error) {
	var u pgtype.UUID
	err := u.Scan(s)
	return u, err
}

func uuidString(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	b := u.Bytes
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
