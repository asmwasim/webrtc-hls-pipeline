package hls

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/asmwasim/webrtc-hls-pipeline/internal/session"
)

type Handler struct {
	segmentDir  string
	sessionRepo *session.Repository
}

func NewHandler(segmentDir string, sessionRepo *session.Repository) *Handler {
	return &Handler{
		segmentDir:  segmentDir,
		sessionRepo: sessionRepo,
	}
}

func (h *Handler) HandleWatch() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID, err := uuid.Parse(chi.URLParam(r, "sessionID"))
		if err != nil {
			http.Error(w, `{"error":"invalid session id"}`, http.StatusBadRequest)
			return
		}

		sess, err := h.sessionRepo.GetByID(r.Context(), sessionID)
		if err != nil {
			http.Error(w, `{"error":"session not found"}`, http.StatusNotFound)
			return
		}

		if sess.Status != "live" && sess.Status != "ended" {
			http.Error(w, `{"error":"stream not available"}`, http.StatusNotFound)
			return
		}

		playlistURL := "/hls/" + sessionID.String() + "/master.m3u8"

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"hls_url":"` + playlistURL + `"}`))
	}
}

func (h *Handler) ServeSegments() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := chi.URLParam(r, "sessionID")
		filename := chi.URLParam(r, "*")

		if !isValidSegmentPath(filename) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		filePath := filepath.Join(h.segmentDir, sessionID, filename)

		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if strings.HasSuffix(filename, ".m3u8") {
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		} else if strings.HasSuffix(filename, ".ts") {
			w.Header().Set("Content-Type", "video/mp2t")
			w.Header().Set("Cache-Control", "public, max-age=3600")
		}

		http.ServeFile(w, r, filePath)
	}
}

func isValidSegmentPath(path string) bool {
	if strings.Contains(path, "..") {
		return false
	}
	return strings.HasSuffix(path, ".m3u8") || strings.HasSuffix(path, ".ts")
}
