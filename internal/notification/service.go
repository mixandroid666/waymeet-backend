package notification

import (
	"context"
	"fmt"
	"log/slog"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/api/option"

	"waymeet-backend/internal/platform/storage"
	"waymeet-backend/internal/platform/storage/dbgen"
)

// Service handles push notification delivery and device token management.
type Service struct {
	db     *storage.DB
	client *messaging.Client // nil when FCM is not configured
	log    *slog.Logger
}

// New creates the notification service. When credFile is empty FCM is disabled
// and all Send* calls are no-ops.
func New(credFile string, db *storage.DB, log *slog.Logger) *Service {
	svc := &Service{db: db, log: log}
	if credFile == "" {
		log.Info("notification: FCM disabled (FCM_CREDENTIALS_FILE not set)")
		return svc
	}
	app, err := firebase.NewApp(context.Background(), nil, option.WithCredentialsFile(credFile))
	if err != nil {
		log.Error("notification: failed to init Firebase app", "err", err)
		return svc
	}
	client, err := app.Messaging(context.Background())
	if err != nil {
		log.Error("notification: failed to get FCM client", "err", err)
		return svc
	}
	svc.client = client
	log.Info("notification: FCM enabled")
	return svc
}

// RegisterToken upserts an FCM device token for the given user.
func (s *Service) RegisterToken(ctx context.Context, userID, token, platform string) error {
	uid, err := parseUUID(userID)
	if err != nil {
		return err
	}
	return s.db.Queries.UpsertDeviceToken(ctx, dbgen.UpsertDeviceTokenParams{
		UserID:   uid,
		FcmToken: token,
		Platform: platform,
	})
}

// SendChat delivers a push notification for a new chat message.
// Stale tokens returned by FCM are pruned from the database.
// Runs in its own goroutine â€” callers should not wait for it.
func (s *Service) SendChat(ctx context.Context, toUserID, senderName, body, conversationID string) {
	if s.client == nil {
		return
	}
	uid, err := parseUUID(toUserID)
	if err != nil {
		return
	}
	tokens, err := s.db.Queries.GetDeviceTokensByUser(ctx, uid)
	if err != nil || len(tokens) == 0 {
		return
	}

	resp, err := s.client.SendEachForMulticast(ctx, &messaging.MulticastMessage{
		Tokens: tokens,
		Notification: &messaging.Notification{
			Title: senderName,
			Body:  body,
		},
		Data: map[string]string{
			"type":            "chat",
			"conversation_id": conversationID,
		},
		Android: &messaging.AndroidConfig{
			Priority: "high",
		},
		APNS: &messaging.APNSConfig{
			Payload: &messaging.APNSPayload{
				Aps: &messaging.Aps{Sound: "default"},
			},
		},
	})
	if err != nil {
		s.log.Error("notification: FCM multicast failed", "err", err)
		return
	}
	// Prune tokens that FCM reports as unregistered.
	for i, r := range resp.Responses {
		if !r.Success {
			_ = s.db.Queries.DeleteDeviceToken(ctx, tokens[i])
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
