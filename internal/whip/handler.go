package whip

import (
	"io"
	"net/http"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/pion/interceptor"
	"github.com/pion/interceptor/pkg/nack"
	"github.com/pion/webrtc/v4"
	"github.com/rs/zerolog/log"

	"github.com/asmwasim/webrtc-hls-pipeline/internal/events"
	"github.com/asmwasim/webrtc-hls-pipeline/internal/metrics"
	"github.com/asmwasim/webrtc-hls-pipeline/internal/session"
)

type Handler struct {
	mu          sync.RWMutex
	sessions    map[uuid.UUID]*StreamSession
	sessionRepo *session.Repository
	publisher   *events.Publisher
	trackCB     func(sessionID uuid.UUID, track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver)
	disconnectCB func(sessionID uuid.UUID, tenantID uuid.UUID)
}

type StreamSession struct {
	SessionID uuid.UUID
	PC        *webrtc.PeerConnection
}

func NewHandler(sessionRepo *session.Repository, publisher *events.Publisher) *Handler {
	return &Handler{
		sessions:    make(map[uuid.UUID]*StreamSession),
		sessionRepo: sessionRepo,
		publisher:   publisher,
	}
}

func (h *Handler) OnTrack(cb func(sessionID uuid.UUID, track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver)) {
	h.trackCB = cb
}

func (h *Handler) OnDisconnect(cb func(sessionID uuid.UUID, tenantID uuid.UUID)) {
	h.disconnectCB = cb
}

func (h *Handler) GetStreamSession(sessionID uuid.UUID) *StreamSession {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.sessions[sessionID]
}

func (h *Handler) HandleWHIP() http.HandlerFunc {
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

		if sess.Status == "live" {
			http.Error(w, `{"error":"session already live"}`, http.StatusConflict)
			return
		}

		contentType := r.Header.Get("Content-Type")
		if contentType != "application/sdp" {
			http.Error(w, `{"error":"content-type must be application/sdp"}`, http.StatusUnsupportedMediaType)
			return
		}

		offer, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, `{"error":"failed to read offer"}`, http.StatusBadRequest)
			return
		}

		pc, err := h.createPeerConnection()
		if err != nil {
			log.Error().Err(err).Msg("failed to create peer connection")
			http.Error(w, `{"error":"failed to create peer connection"}`, http.StatusInternalServerError)
			return
		}

		pc.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
			log.Info().
				Str("session_id", sessionID.String()).
				Str("codec", track.Codec().MimeType).
				Str("kind", track.Kind().String()).
				Msg("track received")

			if h.trackCB != nil {
				h.trackCB(sessionID, track, receiver)
			}
		})

		pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
			log.Info().
				Str("session_id", sessionID.String()).
				Str("state", state.String()).
				Msg("peer connection state changed")

			switch state {
			case webrtc.PeerConnectionStateConnected:
				metrics.StreamsActive.Inc()
				if err := h.sessionRepo.UpdateStatus(r.Context(), sessionID, "live"); err != nil {
					log.Error().Err(err).Msg("failed to update session status to live")
				}
				h.publisher.Publish(r.Context(), events.StreamStarted, map[string]string{
					"session_id": sessionID.String(),
					"tenant_id":  sess.TenantID.String(),
					"teacher_id": sess.TeacherID.String(),
				})

			case webrtc.PeerConnectionStateDisconnected,
				webrtc.PeerConnectionStateFailed,
				webrtc.PeerConnectionStateClosed:
				metrics.StreamsActive.Dec()
				h.removeSession(sessionID)
				if err := h.sessionRepo.UpdateStatus(r.Context(), sessionID, "ended"); err != nil {
					log.Error().Err(err).Msg("failed to update session status to ended")
				}
				h.publisher.Publish(r.Context(), events.StreamEnded, map[string]string{
					"session_id": sessionID.String(),
					"tenant_id":  sess.TenantID.String(),
				})
				if h.disconnectCB != nil {
					h.disconnectCB(sessionID, sess.TenantID)
				}
			}
		})

		if err := pc.SetRemoteDescription(webrtc.SessionDescription{
			Type: webrtc.SDPTypeOffer,
			SDP:  string(offer),
		}); err != nil {
			pc.Close()
			log.Error().Err(err).Msg("failed to set remote description")
			http.Error(w, `{"error":"failed to set remote description"}`, http.StatusBadRequest)
			return
		}

		answer, err := pc.CreateAnswer(nil)
		if err != nil {
			pc.Close()
			log.Error().Err(err).Msg("failed to create answer")
			http.Error(w, `{"error":"failed to create answer"}`, http.StatusInternalServerError)
			return
		}

		gatherComplete := webrtc.GatheringCompletePromise(pc)

		if err := pc.SetLocalDescription(answer); err != nil {
			pc.Close()
			log.Error().Err(err).Msg("failed to set local description")
			http.Error(w, `{"error":"failed to set local description"}`, http.StatusInternalServerError)
			return
		}

		<-gatherComplete

		h.mu.Lock()
		h.sessions[sessionID] = &StreamSession{
			SessionID: sessionID,
			PC:        pc,
		}
		h.mu.Unlock()

		w.Header().Set("Content-Type", "application/sdp")
		w.Header().Set("Location", r.URL.String())
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(pc.LocalDescription().SDP))
	}
}

func (h *Handler) HandleDeleteResource() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID, err := uuid.Parse(chi.URLParam(r, "sessionID"))
		if err != nil {
			http.Error(w, `{"error":"invalid session id"}`, http.StatusBadRequest)
			return
		}

		h.mu.RLock()
		ss := h.sessions[sessionID]
		h.mu.RUnlock()

		if ss == nil {
			http.Error(w, `{"error":"no active stream"}`, http.StatusNotFound)
			return
		}

		ss.PC.Close()
		h.removeSession(sessionID)

		w.WriteHeader(http.StatusOK)
	}
}

func (h *Handler) removeSession(sessionID uuid.UUID) {
	h.mu.Lock()
	delete(h.sessions, sessionID)
	h.mu.Unlock()
}

func (h *Handler) createPeerConnection() (*webrtc.PeerConnection, error) {
	m := &webrtc.MediaEngine{}

	if err := m.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType:    webrtc.MimeTypeH264,
			ClockRate:   90000,
			SDPFmtpLine: "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42001f",
		},
		PayloadType: 96,
	}, webrtc.RTPCodecTypeVideo); err != nil {
		return nil, err
	}

	if err := m.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType:  webrtc.MimeTypeOpus,
			ClockRate: 48000,
			Channels:  2,
		},
		PayloadType: 111,
	}, webrtc.RTPCodecTypeAudio); err != nil {
		return nil, err
	}

	i := &interceptor.Registry{}

	responder, err := nack.NewResponderInterceptor()
	if err != nil {
		return nil, err
	}
	i.Add(responder)

	api := webrtc.NewAPI(
		webrtc.WithMediaEngine(m),
		webrtc.WithInterceptorRegistry(i),
	)

	return api.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	})
}
