package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

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
}

func NewUserHandlers(db storage.DB) *UserHandlers {
	return &UserHandlers{db: db}
}

func NewUserHandlersWithStorage(db storage.DB, fileStorage storage.FileStorage, cfg *config.Config) *UserHandlers {
	return &UserHandlers{db: db, fileStorage: fileStorage, config: cfg}
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

func (h *UserHandlers) DeleteAccountAPI(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == 0 {
		httputil.WriteJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	if err := h.deleteOwnedFiles(r, userID); err != nil {
		logger.GetDefaultLogger().
			WithRequestID(middleware.GetRequestID(r.Context())).
			WithUserID(userID).
			ErrorWithErr(err, "Failed to purge owned files during account deletion")
		httputil.WriteJSONError(w, http.StatusInternalServerError, "Failed to delete account data")
		return
	}
	if err := h.db.DeleteUser(r.Context(), userID); err != nil {
		httputil.WriteJSONError(w, http.StatusNotFound, "User not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *UserHandlers) deleteOwnedFiles(r *http.Request, userID int64) error {
	if h.fileStorage == nil || h.config == nil {
		return nil
	}

	const pageSize = 100
	var purgeErr error

	for page := 1; ; page++ {
		scans, err := h.db.GetScansByUserID(r.Context(), userID, page, pageSize)
		if err != nil {
			return fmt.Errorf("list scans: %w", err)
		}
		for _, scan := range scans {
			for _, path := range userScanImagePaths(h.config.UploadDir, scan) {
				if err := h.fileStorage.DeleteImage(path); err != nil {
					purgeErr = errors.Join(purgeErr, fmt.Errorf("delete scan image %q: %w", path, err))
				}
			}
		}
		if len(scans) < pageSize {
			break
		}
	}

	for page := 1; ; page++ {
		docs, err := h.db.GetDocumentsByUserID(r.Context(), userID, page, pageSize)
		if err != nil {
			return fmt.Errorf("list documents: %w", err)
		}
		for _, doc := range docs {
			if doc.FileURL == "" {
				continue
			}
			if err := h.fileStorage.DeletePDF(doc.FileURL); err != nil {
				purgeErr = errors.Join(purgeErr, fmt.Errorf("delete PDF %q: %w", doc.FileURL, err))
			}
		}
		if len(docs) < pageSize {
			break
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
