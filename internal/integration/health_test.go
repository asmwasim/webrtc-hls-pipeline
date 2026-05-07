//go:build integration

package integration

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asmwasim/webrtc-hls-pipeline/internal/auth"
	"github.com/asmwasim/webrtc-hls-pipeline/internal/chat"
	"github.com/asmwasim/webrtc-hls-pipeline/internal/events"
	"github.com/asmwasim/webrtc-hls-pipeline/internal/hls"
	"github.com/asmwasim/webrtc-hls-pipeline/internal/recording"
	"github.com/asmwasim/webrtc-hls-pipeline/internal/server"
	"github.com/asmwasim/webrtc-hls-pipeline/internal/session"
	"github.com/asmwasim/webrtc-hls-pipeline/internal/whip"
)

func TestHealth_Healthy(t *testing.T) {
	req := httptest.NewRequest("GET", "/health", nil)
	w := serve(req)

	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), `"healthy"`)
}

func TestHealth_PostgresDown(t *testing.T) {
	closedPool, err := pgxpool.New(context.Background(), "postgres://invalid:5432/nodb?connect_timeout=1")
	require.NoError(t, err)
	closedPool.Close()

	jwtAuth := auth.NewJWTAuth(testSecret)
	sessionRepo := session.NewRepository(closedPool)
	chatRepo := chat.NewRepository(closedPool)
	chatHub := chat.NewHub(testRedis, chatRepo)
	defer chatHub.Stop()
	publisher := events.NewPublisher(testRedis)
	hlsHandler := hls.NewHandler(segmentDir, sessionRepo)
	whipHandler := whip.NewHandler(sessionRepo, publisher)
	recWorker := recording.NewWorker(closedPool, sessionRepo, publisher, segmentDir)

	r := server.NewRouter(server.RouterDeps{
		Pool:        closedPool,
		Redis:       testRedis,
		JWTAuth:     jwtAuth,
		SessionRepo: sessionRepo,
		ChatRepo:    chatRepo,
		ChatHub:     chatHub,
		HLSHandler:  hlsHandler,
		WHIPHandler: whipHandler,
		RecWorker:   recWorker,
	})

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, 503, w.Code)
	assert.Contains(t, w.Body.String(), `"postgres"`)
}
