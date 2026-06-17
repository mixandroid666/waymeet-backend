package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// ErrInvalidToken is returned for malformed, expired, or unknown tokens.
var ErrInvalidToken = errors.New("invalid or expired token")

// TokenService mints and validates stateless JWT access tokens (HS256).
//
// Refresh tokens are NOT JWTs — they are opaque random strings stored hashed in
// the database (see service.go), so they can be revoked and rotated.
type TokenService struct {
	secret    []byte
	accessTTL time.Duration
}

// NewTokenService builds a TokenService.
func NewTokenService(secret string, accessTTL time.Duration) *TokenService {
	return &TokenService{secret: []byte(secret), accessTTL: accessTTL}
}

// MintAccess returns a signed access token for userID and its expiry.
func (ts *TokenService) MintAccess(userID string) (string, time.Time, error) {
	now := time.Now()
	exp := now.Add(ts.accessTTL)
	claims := jwt.RegisteredClaims{
		Subject:   userID,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(exp),
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(ts.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign token: %w", err)
	}
	return signed, exp, nil
}

// ParseAccess validates a token and returns its subject (the user id).
func (ts *TokenService) ParseAccess(tokenStr string) (string, error) {
	claims := &jwt.RegisteredClaims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return ts.secret, nil
	})
	if err != nil || claims.Subject == "" {
		return "", ErrInvalidToken
	}
	return claims.Subject, nil
}

// generateRefreshToken returns a 256-bit URL-safe random token.
func generateRefreshToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate refresh token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// hashToken returns the hex SHA-256 of a refresh token (what we store/look up).
// SHA-256 (not bcrypt) so the column stays indexable for fast lookup; refresh
// tokens are already high-entropy, so a fast hash is safe here.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// uuidToString renders a pgtype.UUID in canonical 8-4-4-4-12 form.
func uuidToString(u pgtype.UUID) string {
	b := u.Bytes
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
