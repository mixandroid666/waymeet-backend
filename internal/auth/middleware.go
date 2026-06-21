package auth

import (
	"context"
	"net/http"
	"strings"

	"waymeet-backend/internal/platform/httpx"
)

type ctxKey int

const userIDKey ctxKey = iota

// Middleware authenticates a request from its `Authorization: Bearer <jwt>`
// header, storing the user id in the request context. Other modules wrap their
// protected routes with this.
func (ts *TokenService) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || token == "" {
			httpx.Error(w, http.StatusUnauthorized, "unauthorized", "Missing bearer token")
			return
		}
		userID, err := ts.ParseAccess(token)
		if err != nil {
			httpx.Error(w, http.StatusUnauthorized, "unauthorized", "Invalid or expired token")
			return
		}
		ctx := context.WithValue(r.Context(), userIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// UserIDFromContext returns the authenticated user id set by Middleware.
func UserIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(userIDKey).(string)
	return id, ok
}
