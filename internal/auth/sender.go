package auth

import (
	"context"
	"log/slog"
)

// Sender delivers an OTP code to a contact (email or phone).
//
// The dev implementation (LogSender) just logs it. Swap in real
// implementations later: an email provider (SES/SendGrid) and an SMS provider
// (Twilio) selected by contact type.
type Sender interface {
	SendOTP(ctx context.Context, contact, code string) error
}

// LogSender writes the OTP to the application log instead of sending it.
// Used in development so the flow is testable without an email/SMS provider.
type LogSender struct {
	log *slog.Logger
}

// NewLogSender returns a Sender that logs codes.
func NewLogSender(log *slog.Logger) LogSender { return LogSender{log: log} }

func (s LogSender) SendOTP(_ context.Context, contact, code string) error {
	s.log.Info("otp issued (dev — not actually sent)", "contact", contact, "code", code)
	return nil
}
