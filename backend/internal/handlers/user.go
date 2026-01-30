package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gemini-hackathon/app/internal/middleware"
	"github.com/gemini-hackathon/app/internal/storage"
)

type UserHandlers struct {
	db storage.DB
}

func NewUserHandlers(db storage.DB) *UserHandlers {
	return &UserHandlers{db: db}
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
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(GetLanguagesResponse{
		Languages: supportedLanguages,
	})
}

func (h *UserHandlers) GetUserProfileAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := middleware.GetUserID(r.Context())
	if userID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := h.db.GetUserByID(r.Context(), userID)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	if user == nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(GetUserProfileResponse{
		PreferredLanguage: user.PreferredLanguage,
	})
}

func (h *UserHandlers) UpdateUserPreferencesAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := middleware.GetUserID(r.Context())
	if userID == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req UpdateUserPreferencesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if !isValidLanguage(req.PreferredLanguage) {
		http.Error(w, "Invalid language", http.StatusBadRequest)
		return
	}

	if err := h.db.UpdateUserLanguage(r.Context(), userID, req.PreferredLanguage); err != nil {
		http.Error(w, "Failed to update user language", http.StatusInternalServerError)
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
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func isValidLanguage(lang string) bool {
	for _, l := range supportedLanguages {
		if l.Caption == lang {
			return true
		}
	}
	return false
}
