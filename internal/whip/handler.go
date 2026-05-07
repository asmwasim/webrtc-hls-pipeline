package whip

import (
	"context"
	"io"
	"net/http"
	"sync"

	"github.com/google/uuid"
	"github.com/pion/interceptor"
	"github.com/pion/interceptor/pkg/nack"
	"github.com/pion/webrtc/v4"
	"github.com/rs/zerolog/log"

	"github.com/asmwasim/webrtc-hls-pipeline/internal/events"
	"github.com/asmwasim/webrtc-hls-pipeline/internal/httputil"
	"github.com/asmwasim/webrtc-hls-pipeline/internal/metrics"
	"github.com/asmwasim/webrtc-hls-pipeline/internal/session"
)

type Handler struct {
	mu           sync.RWMutex
	sessions     map[uuid.UUID]*StreamSession
	sessionRepo  *session.Repository
	publisher    *events.Publisher
	trackCB      func(sessionID uuid.UUID, track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver)
	disconnectCB func(sessionID uuid.UUID, tenantID uuid.UUID)
}

type StreamSession struct {
	SessionID    uuid.UUID
	TenantID     uuid.UUID
	TeacherID    uuid.UUID
	PC           *webrtc.PeerConnection
	disconnected sync.Once
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
		sessionID, ok := httputil.ParseSessionID(r)
		if !ok {
			httputil.WriteError(w, http.StatusBadRequest, "invalid session id")
			return
		}

		sess, err := h.sessionRepo.GetByID(r.Context(), sessionID)
		if err != nil {
			httputil.WriteError(w, http.StatusNotFound, "session not found")
			return
		}

		if sess.Status == session.StatusLive {
			httputil.WriteError(w, http.StatusConflict, "session already live")
			return
		}

		if r.Header.Get("Content-Type") != "application/sdp" {
			httputil.WriteError(w, http.StatusUnsupportedMediaType, "content-type must be application/sdp")
			return
		}

		offer, err := io.ReadAll(r.Body)
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "failed to read offer")
			return
		}

		pc, err := h.createPeerConnection()
		if err != nil {
			log.Error().Err(err).Msg("failed to create peer connection")
			httputil.WriteError(w, http.StatusInternalServerError, "failed to create peer connection")
			return
		}

		ss := &StreamSession{
			SessionID: sessionID,
			TenantID:  sess.TenantID,
			TeacherID: sess.TeacherID,
			PC:        pc,
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
				ctx := context.Background()
				if err := h.sessionRepo.UpdateStatus(ctx, sessionID, session.StatusLive); err != nil {
					log.Error().Err(err).Msg("failed to update session status to live")
				}
				h.publisher.Publish(ctx, events.StreamStarted, map[string]string{
					"session_id": sessionID.String(),
					"tenant_id":  sess.TenantID.String(),
					"teacher_id": sess.TeacherID.String(),
				})

			case webrtc.PeerConnectionStateDisconnected,
				webrtc.PeerConnectionStateFailed,
				webrtc.PeerConnectionStateClosed:
				ss.disconnected.Do(func() {
					metrics.StreamsActive.Dec()
					h.removeSession(sessionID)
					ctx := context.Background()
					if err := h.sessionRepo.UpdateStatus(ctx, sessionID, session.StatusEnded); err != nil {
						log.Error().Err(err).Msg("failed to update session status to ended")
					}
					h.publisher.Publish(ctx, events.StreamEnded, map[string]string{
						"session_id": sessionID.String(),
						"tenant_id":  sess.TenantID.String(),
					})
					if h.disconnectCB != nil {
						h.disconnectCB(sessionID, sess.TenantID)
					}
				})
			}
		})

		if err := pc.SetRemoteDescription(webrtc.SessionDescription{
			Type: webrtc.SDPTypeOffer,
			SDP:  string(offer),
		}); err != nil {
			pc.Close()
			log.Error().Err(err).Msg("failed to set remote description")
			httputil.WriteError(w, http.StatusBadRequest, "failed to set remote description")
			return
		}

		answer, err := pc.CreateAnswer(nil)
		if err != nil {
			pc.Close()
			log.Error().Err(err).Msg("failed to create answer")
			httputil.WriteError(w, http.StatusInternalServerError, "failed to create answer")
			return
		}

		gatherComplete := webrtc.GatheringCompletePromise(pc)

		if err := pc.SetLocalDescription(answer); err != nil {
			pc.Close()
			log.Error().Err(err).Msg("failed to set local description")
			httputil.WriteError(w, http.StatusInternalServerError, "failed to set local description")
			return
		}

		<-gatherComplete

		h.mu.Lock()
		h.sessions[sessionID] = ss
		h.mu.Unlock()

		w.Header().Set("Content-Type", "application/sdp")
		w.Header().Set("Location", r.URL.String())
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(pc.LocalDescription().SDP))
	}
}

func (h *Handler) HandleDeleteResource() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID, ok := httputil.ParseSessionID(r)
		if !ok {
			httputil.WriteError(w, http.StatusBadRequest, "invalid session id")
			return
		}

		h.mu.RLock()
		ss := h.sessions[sessionID]
		h.mu.RUnlock()

		if ss == nil {
			httputil.WriteError(w, http.StatusNotFound, "no active stream")
			return
		}

		ss.PC.Close()

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
