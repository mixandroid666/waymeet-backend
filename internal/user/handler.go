package user

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"ruammit-backend/internal/auth"
	"ruammit-backend/internal/platform/httpx"
)

// Upload limits.
const (
	maxAvatarBytes     = 5 << 20  // 5 MB per avatar
	maxPhotoBytes      = 2 << 20  // 2 MB per highlight photo
	maxPhotos          = 6
	maxMultipartMemory = 16 << 20
	maxRequestBytes    = maxAvatarBytes + (maxPhotos * maxPhotoBytes) + (2 << 20)
)

// Handler exposes the profile HTTP endpoints (all authenticated).
type Handler struct {
	svc  *Service
	auth *auth.Service
	log  *slog.Logger
}

// NewHandler builds the user handler.
func NewHandler(svc *Service, authSvc *auth.Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, auth: authSvc, log: log}
}

// RegisterRoutes mounts the user routes on the given mux, behind auth.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle("GET /api/v1/users/me", h.auth.Middleware(http.HandlerFunc(h.getMe)))
	mux.Handle("PATCH /api/v1/users/me", h.auth.Middleware(http.HandlerFunc(h.updateMe)))
	mux.Handle("GET /api/v1/users/{id}", h.auth.Middleware(http.HandlerFunc(h.getUser)))
	mux.Handle("POST /api/v1/users/{id}/follow", h.auth.Middleware(http.HandlerFunc(h.follow)))
	mux.Handle("DELETE /api/v1/users/{id}/follow", h.auth.Middleware(http.HandlerFunc(h.unfollow)))
}

// --- response DTO ----------------------------------------------------------

type profileDTO struct {
	ID               string   `json:"id"`
	Email            string   `json:"email,omitempty"`
	Phone            string   `json:"phone,omitempty"`
	DisplayName      string   `json:"display_name"`
	AvatarURL        string   `json:"avatar_url"`
	Bio              string   `json:"bio"`
	Gender           string   `json:"gender"`
	BirthDate        *string  `json:"birth_date"` // YYYY-MM-DD or null
	Status           string   `json:"status"`
	ProfileCompleted bool     `json:"profile_completed"`
	ThumbnailURLs    []string `json:"thumbnail_urls"` // ordered, up to 6
	FollowerCount    int64    `json:"follower_count"`
	FollowingCount   int64    `json:"following_count"`
}

type publicProfileDTO struct {
	ID             string `json:"id"`
	DisplayName    string `json:"display_name"`
	AvatarURL      string `json:"avatar_url"`
	Bio            string `json:"bio"`
	FollowerCount  int64  `json:"follower_count"`
	FollowingCount int64  `json:"following_count"`
	IsFollowing    bool   `json:"is_following"`
}

func profileDTOOf(p *Profile) profileDTO {
	urls := p.Photos
	if urls == nil {
		urls = []string{}
	}
	dto := profileDTO{
		ID:               p.ID,
		Email:            p.Email,
		Phone:            p.Phone,
		DisplayName:      p.DisplayName,
		AvatarURL:        p.AvatarURL,
		Bio:              p.Bio,
		Gender:           p.Gender,
		Status:           p.Status,
		ProfileCompleted: p.ProfileCompleted,
		ThumbnailURLs:    urls,
		FollowerCount:    p.FollowerCount,
		FollowingCount:   p.FollowingCount,
	}
	if p.BirthDate != nil {
		s := p.BirthDate.Format("2006-01-02")
		dto.BirthDate = &s
	}
	return dto
}

// --- handlers --------------------------------------------------------------

func (h *Handler) getMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	profile, err := h.svc.Get(r.Context(), userID)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, profileDTOOf(profile))
}

// updateMe applies a profile update from a multipart/form-data request:
// display_name, birth_date (YYYY-MM-DD), gender, bio and an optional avatar.
func (h *Handler) updateMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
	if err := r.ParseMultipartForm(maxMultipartMemory); err != nil {
		if strings.Contains(err.Error(), "request body too large") {
			httpx.Error(w, http.StatusRequestEntityTooLarge, "payload_too_large", "The avatar is too large.")
			return
		}
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "Expected multipart/form-data.")
		return
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()

	in, ok := h.parseInput(w, r)
	if !ok {
		return
	}

	profile, err := h.svc.Update(r.Context(), userID, *in)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, profileDTOOf(profile))
}

// parseInput reads the (all-optional) profile fields. Only fields actually
// present in the form are set, so the update stays partial.
func (h *Handler) parseInput(w http.ResponseWriter, r *http.Request) (*UpdateProfileInput, bool) {
	in := &UpdateProfileInput{}

	if has(r, "display_name") {
		v := r.FormValue("display_name")
		in.DisplayName = &v
	}
	if has(r, "gender") {
		v := r.FormValue("gender")
		in.Gender = &v
	}
	if has(r, "bio") {
		v := r.FormValue("bio")
		in.Bio = &v
	}
	if raw := strings.TrimSpace(r.FormValue("birth_date")); raw != "" {
		t, err := time.Parse("2006-01-02", raw)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "invalid_birth_date", "Birth date must be YYYY-MM-DD.")
			return nil, false
		}
		in.BirthDate = &t
	}

	if fhs := r.MultipartForm.File["avatar"]; len(fhs) > 0 {
		avatar, ok := h.readAvatar(w, fhs[0])
		if !ok {
			return nil, false
		}
		in.Avatar = avatar
	}

	for i := 1; i <= maxPhotos; i++ {
		key := fmt.Sprintf("photo_%d", i)
		fhs := r.MultipartForm.File[key]
		if len(fhs) == 0 {
			continue
		}
		photo, ok := h.readPhoto(w, fhs[0], int16(i))
		if !ok {
			return nil, false
		}
		in.Photos = append(in.Photos, *photo)
	}

	if has(r, "replace_photos") {
		in.ReplacePhotos = true
		for i := 1; i <= maxPhotos; i++ {
			urlKey := fmt.Sprintf("photo_%d_url", i)
			if !has(r, urlKey) {
				continue
			}
			if u := r.FormValue(urlKey); u != "" {
				in.KeepPhotos = append(in.KeepPhotos, KeepPhoto{
					Order: int16(i),
					URL:   u,
				})
			}
		}
	}

	return in, true
}

func (h *Handler) readAvatar(w http.ResponseWriter, fh *multipart.FileHeader) (*NewAvatar, bool) {
	if fh.Size > maxAvatarBytes {
		httpx.Error(w, http.StatusRequestEntityTooLarge, "payload_too_large", "The avatar is too large.")
		return nil, false
	}
	f, err := fh.Open()
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "Could not read the avatar.")
		return nil, false
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxAvatarBytes+1))
	if err != nil || int64(len(data)) > maxAvatarBytes {
		httpx.Error(w, http.StatusRequestEntityTooLarge, "payload_too_large", "The avatar is too large.")
		return nil, false
	}
	ext, ok := imageExt(data, fh.Filename)
	if !ok {
		httpx.Error(w, http.StatusBadRequest, "unsupported_media_type", "Avatar must be JPG, PNG or WebP.")
		return nil, false
	}
	return &NewAvatar{Ext: ext, Data: data}, true
}

func (h *Handler) readPhoto(w http.ResponseWriter, fh *multipart.FileHeader, order int16) (*NewPhoto, bool) {
	if fh.Size > maxPhotoBytes {
		httpx.Error(w, http.StatusRequestEntityTooLarge, "payload_too_large", "A highlight photo is too large (2 MB limit).")
		return nil, false
	}
	f, err := fh.Open()
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", "Could not read a highlight photo.")
		return nil, false
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxPhotoBytes+1))
	if err != nil || int64(len(data)) > maxPhotoBytes {
		httpx.Error(w, http.StatusRequestEntityTooLarge, "payload_too_large", "A highlight photo is too large (2 MB limit).")
		return nil, false
	}
	ext, ok := imageExt(data, fh.Filename)
	if !ok {
		httpx.Error(w, http.StatusBadRequest, "unsupported_media_type", "Highlight photos must be JPG, PNG or WebP.")
		return nil, false
	}
	return &NewPhoto{Order: order, Ext: ext, Data: data}, true
}

// --- helpers ---------------------------------------------------------------

// has reports whether a (non-file) form field was actually submitted, so an
// absent field is "leave unchanged" while an empty one is an explicit clear.
func has(r *http.Request, key string) bool {
	if r.MultipartForm == nil {
		return false
	}
	_, ok := r.MultipartForm.Value[key]
	return ok
}

func imageExt(data []byte, filename string) (string, bool) {
	switch http.DetectContentType(data) {
	case "image/jpeg":
		return ".jpg", true
	case "image/png":
		return ".png", true
	case "image/webp":
		return ".webp", true
	}
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".jpg", ".jpeg":
		return ".jpg", true
	case ".png":
		return ".png", true
	case ".webp":
		return ".webp", true
	}
	return "", false
}

func (h *Handler) getUser(w http.ResponseWriter, r *http.Request) {
	viewerID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	targetID := r.PathValue("id")
	profile, err := h.svc.GetPublicProfile(r.Context(), viewerID, targetID)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, publicProfileDTO{
		ID:             profile.ID,
		DisplayName:    profile.DisplayName,
		AvatarURL:      profile.AvatarURL,
		Bio:            profile.Bio,
		FollowerCount:  profile.FollowerCount,
		FollowingCount: profile.FollowingCount,
		IsFollowing:    profile.IsFollowing,
	})
}

func (h *Handler) follow(w http.ResponseWriter, r *http.Request) {
	viewerID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	targetID := r.PathValue("id")
	if err := h.svc.Follow(r.Context(), viewerID, targetID); err != nil {
		h.writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) unfollow(w http.ResponseWriter, r *http.Request) {
	viewerID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "")
		return
	}
	targetID := r.PathValue("id")
	if err := h.svc.Unfollow(r.Context(), viewerID, targetID); err != nil {
		h.writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidName):
		httpx.Error(w, http.StatusBadRequest, "invalid_name", "Your name must be 2–50 characters.")
	case errors.Is(err, ErrInvalidGender):
		httpx.Error(w, http.StatusBadRequest, "invalid_gender", "Choose male, female or other.")
	case errors.Is(err, ErrInvalidBirth):
		httpx.Error(w, http.StatusBadRequest, "invalid_birth_date", "That birth date isn't valid.")
	case errors.Is(err, ErrUnderage):
		httpx.Error(w, http.StatusBadRequest, "underage", "You must be at least 13 to use Ruammit.")
	case errors.Is(err, ErrBioTooLong):
		httpx.Error(w, http.StatusBadRequest, "bio_too_long", "Your bio is too long.")
	case errors.Is(err, ErrNotFound):
		httpx.Error(w, http.StatusNotFound, "not_found", "Account not found.")
	case errors.Is(err, ErrCannotFollowSelf):
		httpx.Error(w, http.StatusBadRequest, "cannot_follow_self", "You cannot follow yourself.")
	default:
		h.log.Error("user service error", "err", err)
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "")
	}
}
