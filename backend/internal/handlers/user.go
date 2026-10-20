package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gemini-hackathon/app/internal/auth"
	"github.com/gemini-hackathon/app/internal/config"
	"github.com/gemini-hackathon/app/internal/httputil"
	"github.com/gemini-hackathon/app/internal/logger"
	"github.com/gemini-hackathon/app/internal/middleware"
	"github.com/gemini-hackathon/app/internal/models"
	"github.com/gemini-hackathon/app/internal/storage"
)

type UserHandlers struct {
	db          storage.DB
	fileStorage storage.FileStorage
	config      *config.Config
	identities  auth.IdentityDirectory
}

func NewUserHandlers(db storage.DB) *UserHandlers {
	return &UserHandlers{db: db}
}

func NewUserHandlersWithStorage(db storage.DB, fileStorage storage.FileStorage, cfg *config.Config, identities auth.IdentityDirectory) *UserHandlers {
	return &UserHandlers{db: db, fileStorage: fileStorage, config: cfg, identities: identities}
}

type Language struct {
	Caption  string `json:"caption"`
	ImageURL string `json:"imageUrl"`
}

type GetLanguagesResponse struct {
	Languages []Language `json:"languages"`
}

type GetUserProfileResponse struct {
	PreferredLanguage string `json:"preferredLanguage"`
}

type UpdateUserPreferencesRequest struct {
	PreferredLanguage string `json:"preferredLanguage"`
}

type UpdateUserPreferencesResponse struct {
	PreferredLanguage string `json:"preferredLanguage"`
}

var supportedLanguages = []Language{
	{Caption: "ID", ImageURL: "https://flagcdn.com/w40/id.png"},
	{Caption: "JP", ImageURL: "https://flagcdn.com/w40/jp.png"},
	{Caption: "EN", ImageURL: "https://flagcdn.com/w40/gb.png"},
}

func (h *UserHandlers) GetLanguagesAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httputil.WriteJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(GetLanguagesResponse{
		Languages: supportedLanguages,
	})
}

func (h *UserHandlers) GetUserProfileAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httputil.WriteJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	userID := middleware.GetUserID(r.Context())
	if userID == 0 {
		httputil.WriteJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	user, err := h.db.GetUserByID(r.Context(), userID)
	if err != nil {
		httputil.WriteJSONError(w, http.StatusInternalServerError, "Database error")
		return
	}

	if user == nil {
		httputil.WriteJSONError(w, http.StatusNotFound, "User not found")
		return
	}

	// The web app loads the profile once per sign-in or account switch. Clear
	// the browser HTTP cache for this origin then, so responses cached by
	// earlier builds (scan images were "private, max-age=300") can't be opened
	// by the next account through a copied URL.
	w.Header().Set("Clear-Site-Data", `"cache"`)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(GetUserProfileResponse{
		PreferredLanguage: user.PreferredLanguage,
	})
}

func (h *UserHandlers) UpdateUserPreferencesAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		httputil.WriteJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	userID := middleware.GetUserID(r.Context())
	if userID == 0 {
		httputil.WriteJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req UpdateUserPreferencesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteJSONError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if !isValidLanguage(req.PreferredLanguage) {
		httputil.WriteJSONError(w, http.StatusBadRequest, "Invalid language")
		return
	}

	if err := h.db.UpdateUserLanguage(r.Context(), userID, req.PreferredLanguage); err != nil {
		httputil.WriteJSONError(w, http.StatusInternalServerError, "Failed to update user language")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(UpdateUserPreferencesResponse{
		PreferredLanguage: req.PreferredLanguage,
	})
}

func (h *UserHandlers) UsersMeAPI(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.GetUserProfileAPI(w, r)
	case http.MethodPatch:
		h.UpdateUserPreferencesAPI(w, r)
	case http.MethodDelete:
		h.DeleteAccountAPI(w, r)
	default:
		httputil.WriteJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// DeleteAccountAPI deletes owned files, then the local profile together with a
// permanent Clerk tombstone, then the Clerk user. If the Clerk step fails the
// caller may retry: the tombstoned subject can still reach this endpoint,
// which then only repeats the Clerk deletion.
func (h *UserHandlers) DeleteAccountAPI(w http.ResponseWriter, r *http.Request) {
	log := logger.GetDefaultLogger().WithRequestID(middleware.GetRequestID(r.Context()))
	userID := middleware.GetUserID(r.Context())
	clerkUserID := middleware.GetClerkUserID(r.Context())
	if userID == 0 && clerkUserID == "" {
		httputil.WriteJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	if userID != 0 {
		// Exclusive account lock: an upload either finishes before its file is
		// listed here or starts after the account is gone and fails.
		deleted := runWithAccountLock(w, r, h.db, log.WithUserID(userID), userID, true, "Failed to delete account data", func(ctx context.Context) error {
			if err := h.deleteOwnedFiles(ctx, userID); err != nil {
				log.WithUserID(userID).ErrorWithErr(err, "Failed to purge owned files during account deletion")
				return &lockedStepFailure{http.StatusInternalServerError, "Failed to delete account data"}
			}
			if err := h.db.DeleteUserAndTombstoneClerk(ctx, userID, clerkUserID); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return &lockedStepFailure{http.StatusNotFound, "User not found"}
				}
				log.WithUserID(userID).ErrorWithErr(err, "Failed to delete account records")
				return &lockedStepFailure{http.StatusInternalServerError, "Failed to delete account data"}
			}
			return nil
		})
		if !deleted {
			return
		}
	}

	if clerkUserID != "" && h.identities != nil {
		if err := h.identities.DeleteUser(r.Context(), clerkUserID); err != nil {
			log.ErrorWithErr(err, "Account data deleted but Clerk user deletion failed")
			httputil.WriteJSONError(w, http.StatusBadGateway, "Account data deleted; sign-in removal failed, please retry")
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *UserHandlers) deleteOwnedFiles(ctx context.Context, userID int64) error {
	if h.fileStorage == nil || h.config == nil {
		return nil
	}

	scans, docs, err := h.db.GetUserUploads(ctx, userID)
	if err != nil {
		return fmt.Errorf("list uploads: %w", err)
	}

	var purgeErr error
	for _, scan := range scans {
		for _, path := range userScanImagePaths(h.config.UploadDir, scan) {
			if err := h.fileStorage.DeleteImage(path); err != nil {
				purgeErr = errors.Join(purgeErr, fmt.Errorf("delete scan image %q: %w", path, err))
			}
		}
	}
	for _, doc := range docs {
		path := documentPDFPath(h.config.UploadDir, doc)
		if err := h.fileStorage.DeletePDF(path); err != nil {
			purgeErr = errors.Join(purgeErr, fmt.Errorf("delete PDF %q: %w", path, err))
		}
	}

	return purgeErr
}

func userScanImagePaths(uploadDir string, scan *models.Scan) []string {
	if scan.ImageURL != "" && strings.HasPrefix(scan.ImageURL, "/uploads/") {
		return []string{filepath.Join(uploadDir, filepath.Base(scan.ImageURL))}
	}
	base := filepath.Join(uploadDir, strconv.FormatInt(scan.ID, 10))
	return []string{base + ".jpg", base + ".png", base + ".webp"}
}

func isValidLanguage(lang string) bool {
	for _, l := range supportedLanguages {
		if l.Caption == lang {
			return true
		}
	}
	return false
}
