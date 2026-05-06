package whip

import (
	"io"
	"sync"
	"sync/atomic"

	"github.com/google/uuid"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
	"github.com/rs/zerolog/log"
)

type TrackPair struct {
	SessionID uuid.UUID
	Video     *TrackReader
	Audio     *TrackReader
	mu        sync.Mutex
	ready     chan struct{}
	hasVideo  bool
	hasAudio  bool
}

type TrackReader struct {
	track  *webrtc.TrackRemote
	done   chan struct{}
	closed atomic.Bool
	once   sync.Once
}

func NewTrackPair(sessionID uuid.UUID) *TrackPair {
	return &TrackPair{
		SessionID: sessionID,
		ready:     make(chan struct{}),
	}
}

func (tp *TrackPair) AddTrack(track *webrtc.TrackRemote) {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	reader := &TrackReader{
		track: track,
		done:  make(chan struct{}),
	}

	switch track.Kind() {
	case webrtc.RTPCodecTypeVideo:
		tp.Video = reader
		tp.hasVideo = true
	case webrtc.RTPCodecTypeAudio:
		tp.Audio = reader
		tp.hasAudio = true
	}

	if tp.hasVideo && tp.hasAudio {
		select {
		case <-tp.ready:
		default:
			close(tp.ready)
		}
	}
}

func (tp *TrackPair) Ready() <-chan struct{} {
	return tp.ready
}

func (tr *TrackReader) ReadRTP() (*rtp.Packet, error) {
	if tr.closed.Load() {
		return nil, io.EOF
	}
	pkt, _, err := tr.track.ReadRTP()
	return pkt, err
}

func (tr *TrackReader) Close() {
	tr.once.Do(func() {
		tr.closed.Store(true)
		close(tr.done)
	})
}

func (tr *TrackReader) Done() <-chan struct{} {
	return tr.done
}

type TrackManager struct {
	mu     sync.RWMutex
	tracks map[uuid.UUID]*TrackPair
}

func NewTrackManager() *TrackManager {
	return &TrackManager{
		tracks: make(map[uuid.UUID]*TrackPair),
	}
}

func (tm *TrackManager) HandleTrack(sessionID uuid.UUID, track *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
	tm.mu.Lock()
	pair, exists := tm.tracks[sessionID]
	if !exists {
		pair = NewTrackPair(sessionID)
		tm.tracks[sessionID] = pair
	}
	tm.mu.Unlock()

	pair.AddTrack(track)

	log.Info().
		Str("session_id", sessionID.String()).
		Str("kind", track.Kind().String()).
		Str("codec", track.Codec().MimeType).
		Msg("track registered")
}

func (tm *TrackManager) GetTrackPair(sessionID uuid.UUID) *TrackPair {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.tracks[sessionID]
}

func (tm *TrackManager) Remove(sessionID uuid.UUID) {
	tm.mu.Lock()
	pair, exists := tm.tracks[sessionID]
	if exists {
		if pair.Video != nil {
			pair.Video.Close()
		}
		if pair.Audio != nil {
			pair.Audio.Close()
		}
		delete(tm.tracks, sessionID)
	}
	tm.mu.Unlock()
}
