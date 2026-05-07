//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/asmwasim/webrtc-hls-pipeline/internal/auth"
	"github.com/asmwasim/webrtc-hls-pipeline/internal/chat"
	"github.com/asmwasim/webrtc-hls-pipeline/internal/events"
	"github.com/asmwasim/webrtc-hls-pipeline/internal/hls"
	"github.com/asmwasim/webrtc-hls-pipeline/internal/metrics"
	"github.com/asmwasim/webrtc-hls-pipeline/internal/recording"
	"github.com/asmwasim/webrtc-hls-pipeline/internal/server"
	"github.com/asmwasim/webrtc-hls-pipeline/internal/session"
	"github.com/asmwasim/webrtc-hls-pipeline/internal/testutil"
	"github.com/asmwasim/webrtc-hls-pipeline/internal/whip"
)

const testSecret = "test-secret"

var (
	testPool   *pgxpool.Pool
	testRedis  *redis.Client
	testRouter chi.Router
	tokens     *testutil.TokenFactory
	tenantA    = uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	tenantB    = uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	teacherID  = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	studentID  = uuid.MustParse("22222222-2222-2222-2222-222222222222")
	segmentDir string
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	infra, err := testutil.SetupContainers(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to setup containers: %v\n", err)
		os.Exit(1)
	}
	defer infra.Teardown(ctx)

	migrationsDir := findMigrationsDir()
	if err := infra.RunMigrations(ctx, migrationsDir); err != nil {
		fmt.Fprintf(os.Stderr, "failed to run migrations: %v\n", err)
		os.Exit(1)
	}

	testPool = infra.Pool
	testRedis = infra.Redis

	segmentDir, _ = os.MkdirTemp("", "hls-test-*")
	defer os.RemoveAll(segmentDir)

	metrics.Register()

	tokens = testutil.NewTokenFactory(testSecret)

	jwtAuth := auth.NewJWTAuth(testSecret)
	sessionRepo := session.NewRepository(testPool)
	chatRepo := chat.NewRepository(testPool)
	chatHub := chat.NewHub(testRedis, chatRepo)
	defer chatHub.Stop()
	publisher := events.NewPublisher(testRedis)
	hlsHandler := hls.NewHandler(segmentDir, sessionRepo)
	whipHandler := whip.NewHandler(sessionRepo, publisher)
	recWorker := recording.NewWorker(testPool, sessionRepo, publisher, segmentDir)

	testRouter = server.NewRouter(server.RouterDeps{
		Pool:        testPool,
		Redis:       testRedis,
		JWTAuth:     jwtAuth,
		SessionRepo: sessionRepo,
		ChatRepo:    chatRepo,
		ChatHub:     chatHub,
		HLSHandler:  hlsHandler,
		WHIPHandler: whipHandler,
		RecWorker:   recWorker,
	})

	os.Exit(m.Run())
}

func findMigrationsDir() string {
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	return filepath.Join(dir, "..", "..", "migrations")
}

func serve(req *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	return w
}

func clean(t *testing.T) {
	t.Helper()
	testutil.CleanDB(context.Background(), testPool)
}

func decodeJSON(t *testing.T, w *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.NewDecoder(w.Body).Decode(v); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}
