package chat

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"waymeet-backend/internal/auth"
	"waymeet-backend/internal/platform/httpx"
	"waymeet-backend/internal/platform/mediastore"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// Handler manages the WebSocket chat upgrade endpoint and REST chat endpoints.
type Handler struct {
	hub   *Hub
	svc   *Service
	auth  *auth.Service
	media mediastore.Store
	log   *slog.Logger
}

func NewHandler(hub *Hub, svc *Service, authSvc *auth.Service, media mediastore.Store, log *slog.Logger) *Handler {
	return &Handler{hub: hub, svc: svc, auth: authSvc, media: media, log: log}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/ws/chat", h.connect)
	mux.Handle("POST /api/v1/conversations", h.auth.Middleware(http.HandlerFunc(h.createConversation)))
	mux.Handle("GET /api/v1/conversations", h.auth.Middleware(http.HandlerFunc(h.listConversations)))
	mux.Handle("GET /api/v1/conversations/{id}/messages", h.auth.Middleware(http.HandlerFunc(h.listMessages)))
	mux.Handle("GET /api/v1/users/{id}/presence", h.auth.Middleware(http.HandlerFunc(h.getPresence)))
	mux.Handle("POST /api/v1/chat/upload-image", h.auth.Middleware(http.HandlerFunc(h.uploadImage)))
}

func (h *Handler) connect(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing token")
		return
	}
	userID, err := h.auth.ParseAccess(token)
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "invalid or expired token")
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.log.Warn("chat: ws upgrade failed", "user", userID, "err", err)
		return
	}

	client := newClient(h.hub, userID, conn)
	h.hub.register <- client

	go client.writePump()
	go client.readPump(h.svc)
}

// --- DTOs ------------------------------------------------------------------

type conversationDTO struct {
	ID               string  `json:"id"`
	PartnerID        string  `json:"partner_id"`
	PartnerName      string  `json:"partner_name"`
	PartnerAvatarURL string  `json:"partner_avatar_url"`
	LastMessage      string  `json:"last_message"`
	LastSenderID     string  `json:"last_sender_id"`
	LastMessageAt    *string `json:"last_message_at"`
}

type messageDTO struct {
	ID             string `json:"id"`
	ConversationID string `json:"conversation_id"`
	SenderID       string `json:"sender_id"`
	Body           string `json:"body"`
	MsgType        string `json:"msg_type"`
	MediaURL       string `json:"media_url,omitempty"`
	CreatedAt      string `json:"created_at"`
}

// --- REST handlers ---------------------------------------------------------

func (h *Handler) createConversation(w http.ResponseWriter, r *http.Request) {
	viewerID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	var body struct {
		PartnerID string `json:"partner_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.PartnerID == "" {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "partner_id required")
		return
	}
	convID, err := h.svc.GetOrCreateConversation(r.Context(), viewerID, body.PartnerID)
	if err != nil {
		h.log.Error("chat: GetOrCreateConversation failed", "err", err, "viewer", viewerID, "partner", body.PartnerID)
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"id": convID})
}

func (h *Handler) listConversations(w http.ResponseWriter, r *http.Request) {
	viewerID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	convs, err := h.svc.ListConversations(r.Context(), viewerID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	out := make([]conversationDTO, 0, len(convs))
	for _, c := range convs {
		dto := conversationDTO{
			ID:               c.ID,
			PartnerID:        c.PartnerID,
			PartnerName:      c.PartnerName,
			PartnerAvatarURL: c.PartnerAvatarURL,
			LastMessage:      c.LastMessage,
			LastSenderID:     c.LastSenderID,
		}
		if c.LastMessageAt != nil {
			s := c.LastMessageAt.UTC().Format(time.RFC3339)
			dto.LastMessageAt = &s
		}
		out = append(out, dto)
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"conversations": out})
}

func (h *Handler) getPresence(w http.ResponseWriter, r *http.Request) {
	targetID := r.PathValue("id")
	status := "offline"
	if h.hub.IsOnline(targetID) {
		status = "online"
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": status})
}

func (h *Handler) listMessages(w http.ResponseWriter, r *http.Request) {
	viewerID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	convID := r.PathValue("id")

	limit := int32(50)
	offset := int32(0)
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = int32(n)
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = int32(n)
		}
	}

	msgs, err := h.svc.ListMessages(r.Context(), viewerID, convID, limit, offset)
	if err != nil {
		if errors.Is(err, ErrForbidden) {
			httpx.Error(w, http.StatusForbidden, "forbidden", "")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	out := make([]messageDTO, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, messageDTO{
			ID:             m.ID,
			ConversationID: m.ConversationID,
			SenderID:       m.SenderID,
			Body:           m.Body,
			MsgType:        m.MsgType,
			MediaURL:       m.MediaURL,
			CreatedAt:      m.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"messages": out})
}

func (h *Handler) uploadImage(w http.ResponseWriter, r *http.Request) {
	viewerID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "request too large or malformed")
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "image field required")
		return
	}
	defer file.Close()

	ct := header.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "image/") {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "file must be an image")
		return
	}

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext == "" {
		ext = ".jpg"
	}

	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "")
		return
	}
	key := fmt.Sprintf("chat/%s/%x%s", viewerID, b, ext)

	url, err := h.media.Save(key, file)
	if err != nil {
		h.log.Error("chat: failed to save image", "err", err)
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "failed to save image")
		return
	}

	httpx.JSON(w, http.StatusOK, map[string]string{"url": url})
}
