package session

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/asmwasim/webrtc-hls-pipeline/internal/auth"
)

type createRequest struct {
	Title string `json:"title"`
}

func HandleCreate(repo *Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := auth.GetClaims(r.Context())

		var req createRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
			return
		}

		if req.Title == "" {
			http.Error(w, `{"error":"title is required"}`, http.StatusBadRequest)
			return
		}

		tenantID, err := uuid.Parse(claims.TenantID)
		if err != nil {
			http.Error(w, `{"error":"invalid tenant_id in token"}`, http.StatusBadRequest)
			return
		}
		teacherID, err := uuid.Parse(claims.UserID)
		if err != nil {
			http.Error(w, `{"error":"invalid user_id in token"}`, http.StatusBadRequest)
			return
		}

		s, err := repo.Create(r.Context(), tenantID, teacherID, req.Title)
		if err != nil {
			http.Error(w, `{"error":"failed to create session"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(s)
	}
}

func HandleGet(repo *Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(chi.URLParam(r, "sessionID"))
		if err != nil {
			http.Error(w, `{"error":"invalid session id"}`, http.StatusBadRequest)
			return
		}

		s, err := repo.GetByID(r.Context(), id)
		if err != nil {
			http.Error(w, `{"error":"session not found"}`, http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(s)
	}
}

func HandleList(repo *Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := auth.GetClaims(r.Context())

		tenantID, err := uuid.Parse(claims.TenantID)
		if err != nil {
			http.Error(w, `{"error":"invalid tenant_id in token"}`, http.StatusBadRequest)
			return
		}

		sessions, err := repo.List(r.Context(), tenantID)
		if err != nil {
			http.Error(w, `{"error":"failed to list sessions"}`, http.StatusInternalServerError)
			return
		}

		if sessions == nil {
			sessions = []*Session{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sessions)
	}
}

func HandleEnd(repo *Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(chi.URLParam(r, "sessionID"))
		if err != nil {
			http.Error(w, `{"error":"invalid session id"}`, http.StatusBadRequest)
			return
		}

		if err := repo.UpdateStatus(r.Context(), id, "ended"); err != nil {
			http.Error(w, `{"error":"failed to end session"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ended"}`))
	}
}
