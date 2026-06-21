package notification

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"waymeet-backend/internal/auth"
	"waymeet-backend/internal/platform/httpx"
)

// Handler exposes device-token management endpoints.
type Handler struct {
	svc  *Service
	auth *auth.Service
	log  *slog.Logger
}

func NewHandler(svc *Service, authSvc *auth.Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, auth: authSvc, log: log}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle("POST /api/v1/device-tokens",
		h.auth.Middleware(http.HandlerFunc(h.register)))
	mux.Handle("DELETE /api/v1/device-tokens",
		h.auth.Middleware(http.HandlerFunc(h.delete)))
}

type tokenRequest struct {
	Token    string `json:"token"`
	Platform string `json:"platform"` // "android" | "ios"
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	var req tokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Token == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "token is required")
		return
	}
	platform := req.Platform
	if platform == "" {
		platform = "android"
	}
	if err := h.svc.RegisterToken(r.Context(), userID, req.Token, platform); err != nil {
		h.log.Error("notification: register token", "err", err)
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	var req tokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Token == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "token is required")
		return
	}
	if err := h.svc.db.Queries.DeleteDeviceToken(r.Context(), req.Token); err != nil {
		h.log.Error("notification: delete token", "err", err)
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
