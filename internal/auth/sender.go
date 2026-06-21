package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
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
	s.log.Info("otp issued (dev â€” not actually sent)", "contact", contact, "code", code)
	return nil
}

// ResendSender delivers OTP codes by email via the Resend API
// (https://resend.com). It only handles email contacts; phone contacts are
// passed to the fallback Sender (no SMS provider is wired up yet).
type ResendSender struct {
	apiKey   string
	from     string
	log      *slog.Logger
	client   *http.Client
	fallback Sender // used for non-email contacts (e.g. phone)
}

// NewResendSender builds an email Sender. from is the verified sender address
// (e.g. "noreply@yourdomain.com"); without a verified domain use Resend's
// sandbox address "onboarding@resend.dev". fallback handles phone contacts.
func NewResendSender(apiKey, from string, log *slog.Logger, fallback Sender) ResendSender {
	return ResendSender{
		apiKey:   apiKey,
		from:     from,
		log:      log,
		client:   &http.Client{Timeout: 10 * time.Second},
		fallback: fallback,
	}
}

func (s ResendSender) SendOTP(ctx context.Context, contact, code string) error {
	// Email addresses contain "@"; anything else (phone) goes to the fallback.
	if !strings.Contains(contact, "@") {
		return s.fallback.SendOTP(ctx, contact, code)
	}

	body, err := json.Marshal(map[string]any{
		"from":    s.from,
		"to":      []string{contact},
		"subject": "Your Waymeet verification code",
		"text":    fmt.Sprintf("Your Waymeet verification code is %s.\n\nIt expires in 10 minutes. If you didn't request this, you can ignore this email.", code),
	})
	if err != nil {
		return fmt.Errorf("resend: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("resend: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("resend: send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("resend: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	s.log.Info("otp email sent", "contact", contact)
	return nil
}
