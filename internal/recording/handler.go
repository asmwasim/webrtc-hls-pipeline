package recording

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RecordingInfo struct {
	ID          uuid.UUID `json:"id"`
	SessionID   uuid.UUID `json:"session_id"`
	Status      string    `json:"status"`
	MP4URL      string    `json:"mp4_url,omitempty"`
}

func HandleGetRecording(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID, err := uuid.Parse(chi.URLParam(r, "sessionID"))
		if err != nil {
			http.Error(w, `{"error":"invalid session id"}`, http.StatusBadRequest)
			return
		}

		var info RecordingInfo
		err = pool.QueryRow(r.Context(),
			`SELECT id, session_id, status, mp4_url FROM recordings WHERE session_id = $1 ORDER BY created_at DESC LIMIT 1`,
			sessionID,
		).Scan(&info.ID, &info.SessionID, &info.Status, &info.MP4URL)

		if err != nil {
			http.Error(w, `{"error":"recording not found"}`, http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(info)
	}
}
