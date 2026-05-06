package recording

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/asmwasim/webrtc-hls-pipeline/internal/events"
	"github.com/asmwasim/webrtc-hls-pipeline/internal/hls"
	"github.com/asmwasim/webrtc-hls-pipeline/internal/session"
)

type Worker struct {
	pool        *pgxpool.Pool
	sessionRepo *session.Repository
	publisher   *events.Publisher
	segmentDir  string
	jobs        chan Job
	ctx         context.Context
	cancel      context.CancelFunc
}

type Job struct {
	SessionID uuid.UUID
	TenantID  uuid.UUID
}

func NewWorker(pool *pgxpool.Pool, sessionRepo *session.Repository, publisher *events.Publisher, segmentDir string) *Worker {
	ctx, cancel := context.WithCancel(context.Background())
	return &Worker{
		pool:        pool,
		sessionRepo: sessionRepo,
		publisher:   publisher,
		segmentDir:  segmentDir,
		jobs:        make(chan Job, 100),
		ctx:         ctx,
		cancel:      cancel,
	}
}

func (w *Worker) Start() {
	go w.processLoop()
	log.Info().Msg("recording worker started")
}

func (w *Worker) Stop() {
	w.cancel()
}

func (w *Worker) Enqueue(sessionID, tenantID uuid.UUID) {
	select {
	case w.jobs <- Job{SessionID: sessionID, TenantID: tenantID}:
		log.Info().Str("session_id", sessionID.String()).Msg("recording job enqueued")
	default:
		log.Warn().Str("session_id", sessionID.String()).Msg("recording job queue full")
	}
}

func (w *Worker) processLoop() {
	for {
		select {
		case <-w.ctx.Done():
			return
		case job := <-w.jobs:
			w.process(job)
		}
	}
}

func (w *Worker) process(job Job) {
	sessionID := job.SessionID
	log.Info().Str("session_id", sessionID.String()).Msg("processing recording")

	recID := uuid.New()
	_, err := w.pool.Exec(w.ctx,
		`INSERT INTO recordings (id, session_id, tenant_id, status, created_at)
		 VALUES ($1, $2, $3, 'processing', $4)`,
		recID, sessionID, job.TenantID, time.Now().UTC(),
	)
	if err != nil {
		log.Error().Err(err).Msg("failed to create recording record")
		return
	}

	if err := hls.MarkAsVOD(w.segmentDir, sessionID.String()); err != nil {
		log.Error().Err(err).Msg("failed to mark playlist as VOD")
	}

	dir := filepath.Join(w.segmentDir, sessionID.String())
	mp4Path := filepath.Join(dir, "recording.mp4")

	if err := w.concatenateSegments(dir, mp4Path); err != nil {
		log.Error().Err(err).Msg("failed to concatenate segments")
		w.pool.Exec(w.ctx,
			`UPDATE recordings SET status = 'failed', completed_at = $2 WHERE id = $1`,
			recID, time.Now().UTC(),
		)
		return
	}

	mp4URL := "/recordings/" + sessionID.String() + "/recording.mp4"

	now := time.Now().UTC()
	w.pool.Exec(w.ctx,
		`UPDATE recordings SET status = 'ready', mp4_url = $2, completed_at = $3 WHERE id = $1`,
		recID, mp4URL, now,
	)
	w.sessionRepo.SetMP4URL(w.ctx, sessionID, mp4URL)

	w.publisher.Publish(w.ctx, events.RecordingReady, map[string]string{
		"session_id": sessionID.String(),
		"tenant_id":  job.TenantID.String(),
		"mp4_url":    mp4URL,
	})

	log.Info().Str("session_id", sessionID.String()).Str("mp4", mp4Path).Msg("recording complete")
}

func (w *Worker) concatenateSegments(dir, outputPath string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	var tsFiles []string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".ts") {
			tsFiles = append(tsFiles, entry.Name())
		}
	}

	if len(tsFiles) == 0 {
		return fmt.Errorf("no .ts segments found")
	}

	concatList := filepath.Join(dir, "concat.txt")
	var lines []string
	for _, f := range tsFiles {
		lines = append(lines, fmt.Sprintf("file '%s'", f))
	}
	if err := os.WriteFile(concatList, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		return err
	}
	defer os.Remove(concatList)

	cmd := exec.CommandContext(w.ctx, "ffmpeg",
		"-hide_banner",
		"-loglevel", "warning",
		"-f", "concat",
		"-safe", "0",
		"-i", concatList,
		"-c", "copy",
		"-movflags", "+faststart",
		outputPath,
		"-y",
	)
	cmd.Dir = dir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg concat: %w: %s", err, string(output))
	}

	return nil
}
