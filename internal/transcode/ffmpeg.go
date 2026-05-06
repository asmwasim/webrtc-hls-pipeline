package transcode

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pion/webrtc/v4/pkg/media/h264writer"
	"github.com/rs/zerolog/log"

	"github.com/asmwasim/webrtc-hls-pipeline/internal/whip"
)

type Pipeline struct {
	sessionID  uuid.UUID
	segmentDir string
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	cancel     context.CancelFunc
	done       chan struct{}
}

type Manager struct {
	mu         sync.RWMutex
	pipelines  map[uuid.UUID]*Pipeline
	segmentDir string
}

func NewManager(segmentDir string) *Manager {
	return &Manager{
		pipelines:  make(map[uuid.UUID]*Pipeline),
		segmentDir: segmentDir,
	}
}

func (m *Manager) Start(ctx context.Context, sessionID uuid.UUID, tracks *whip.TrackPair) error {
	outDir := filepath.Join(m.segmentDir, sessionID.String())
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("mkdir segments: %w", err)
	}

	pipeCtx, cancel := context.WithCancel(ctx)

	args := buildFFmpegArgs(outDir)
	cmd := exec.CommandContext(pipeCtx, "ffmpeg", args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("stdin pipe: %w", err)
	}

	cmd.Stderr = &ffmpegLogger{sessionID: sessionID}

	p := &Pipeline{
		sessionID:  sessionID,
		segmentDir: outDir,
		cmd:        cmd,
		stdin:      stdin,
		cancel:     cancel,
		done:       make(chan struct{}),
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("start ffmpeg: %w", err)
	}

	log.Info().Str("session_id", sessionID.String()).Msg("ffmpeg started")

	m.mu.Lock()
	m.pipelines[sessionID] = p
	m.mu.Unlock()

	go p.pumpTracks(tracks)
	go p.wait()

	return nil
}

func (m *Manager) Stop(sessionID uuid.UUID) {
	m.mu.Lock()
	p, exists := m.pipelines[sessionID]
	if exists {
		delete(m.pipelines, sessionID)
	}
	m.mu.Unlock()

	if !exists {
		return
	}

	p.stdin.Close()

	select {
	case <-p.done:
	case <-time.After(10 * time.Second):
		p.cancel()
		<-p.done
	}

	log.Info().Str("session_id", sessionID.String()).Msg("ffmpeg stopped")
}

func (m *Manager) IsRunning(sessionID uuid.UUID) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.pipelines[sessionID]
	return exists
}

func (m *Manager) GetSegmentDir(sessionID uuid.UUID) string {
	return filepath.Join(m.segmentDir, sessionID.String())
}

func (p *Pipeline) pumpTracks(tracks *whip.TrackPair) {
	var wg sync.WaitGroup

	if tracks.Video != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p.pumpVideo(tracks.Video)
		}()
	}

	if tracks.Audio != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p.pumpAudio(tracks.Audio)
		}()
	}

	wg.Wait()
	p.stdin.Close()
}

func (p *Pipeline) pumpVideo(reader *whip.TrackReader) {
	writer := h264writer.NewWith(p.stdin)

	for {
		pkt, err := reader.ReadRTP()
		if err != nil {
			if err != io.EOF {
				log.Error().Err(err).Str("session_id", p.sessionID.String()).Msg("video read error")
			}
			return
		}

		if err := writer.WriteRTP(pkt); err != nil {
			log.Error().Err(err).Str("session_id", p.sessionID.String()).Msg("video write error")
			return
		}
	}
}

func (p *Pipeline) pumpAudio(reader *whip.TrackReader) {
	for {
		pkt, err := reader.ReadRTP()
		if err != nil {
			if err != io.EOF {
				log.Error().Err(err).Str("session_id", p.sessionID.String()).Msg("audio read error")
			}
			return
		}

		if _, err := p.stdin.Write(pkt.Payload); err != nil {
			log.Error().Err(err).Str("session_id", p.sessionID.String()).Msg("audio write error")
			return
		}
	}
}

func (p *Pipeline) wait() {
	defer close(p.done)
	if err := p.cmd.Wait(); err != nil {
		log.Warn().Err(err).Str("session_id", p.sessionID.String()).Msg("ffmpeg exited")
	}
}

func buildFFmpegArgs(outDir string) []string {
	return []string{
		"-hide_banner",
		"-loglevel", "warning",

		"-f", "h264",
		"-i", "pipe:0",

		"-filter_complex",
		"[0:v]split=3[v720][v480][v360];" +
			"[v720]scale=1280:720[v720out];" +
			"[v480]scale=854:480[v480out];" +
			"[v360]scale=640:360[v360out]",

		"-map", "[v720out]", "-c:v:0", "libx264", "-b:v:0", "2500k",
		"-preset", "veryfast", "-g", "48", "-keyint_min", "48",

		"-map", "[v480out]", "-c:v:1", "libx264", "-b:v:1", "1000k",
		"-preset", "veryfast", "-g", "48", "-keyint_min", "48",

		"-map", "[v360out]", "-c:v:2", "libx264", "-b:v:2", "500k",
		"-preset", "veryfast", "-g", "48", "-keyint_min", "48",

		"-f", "hls",
		"-hls_time", "2",
		"-hls_list_size", "5",
		"-hls_flags", "delete_segments+independent_segments",
		"-hls_segment_filename", filepath.Join(outDir, "stream_%v_%03d.ts"),

		"-var_stream_map", "v:0 v:1 v:2",
		"-master_pl_name", "master.m3u8",

		filepath.Join(outDir, "stream_%v.m3u8"),

		"-y",
	}
}
