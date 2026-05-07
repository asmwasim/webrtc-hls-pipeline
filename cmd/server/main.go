package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pion/webrtc/v4"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/asmwasim/webrtc-hls-pipeline/internal/auth"
	"github.com/asmwasim/webrtc-hls-pipeline/internal/chat"
	"github.com/asmwasim/webrtc-hls-pipeline/internal/config"
	"github.com/asmwasim/webrtc-hls-pipeline/internal/events"
	"github.com/asmwasim/webrtc-hls-pipeline/internal/hls"
	"github.com/asmwasim/webrtc-hls-pipeline/internal/metrics"
	"github.com/asmwasim/webrtc-hls-pipeline/internal/recording"
	"github.com/asmwasim/webrtc-hls-pipeline/internal/server"
	"github.com/asmwasim/webrtc-hls-pipeline/internal/session"
	"github.com/asmwasim/webrtc-hls-pipeline/internal/transcode"
	"github.com/asmwasim/webrtc-hls-pipeline/internal/whip"
)

func main() {
	metrics.Register()
	cfg := config.Load()

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = zerolog.New(os.Stdout).With().Timestamp().Caller().Logger()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to postgres")
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatal().Err(err).Msg("failed to ping postgres")
	}
	log.Info().Msg("connected to postgres")

	redisOpts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to parse redis url")
	}
	rdb := redis.NewClient(redisOpts)
	defer rdb.Close()

	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatal().Err(err).Msg("failed to ping redis")
	}
	log.Info().Msg("connected to redis")

	jwtAuth := auth.NewJWTAuth(cfg.JWTSecret)
	sessionRepo := session.NewRepository(pool)
	publisher := events.NewPublisher(rdb)
	transcodeMgr := transcode.NewManager(cfg.SegmentDir)
	hlsHandler := hls.NewHandler(cfg.SegmentDir, sessionRepo)
	chatRepo := chat.NewRepository(pool)
	chatHub := chat.NewHub(rdb, chatRepo)
	defer chatHub.Stop()
	recWorker := recording.NewWorker(pool, sessionRepo, publisher, cfg.SegmentDir)
	recWorker.Start()
	defer recWorker.Stop()
	trackMgr := whip.NewTrackManager()
	whipHandler := whip.NewHandler(sessionRepo, publisher)
	whipHandler.OnTrack(func(sessionID uuid.UUID, track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		trackMgr.HandleTrack(sessionID, track, receiver)
		pair := trackMgr.GetTrackPair(sessionID)
		if pair == nil {
			return
		}
		go func() {
			<-pair.Ready()
			if transcodeMgr.IsRunning(sessionID) {
				return
			}
			if err := transcodeMgr.Start(context.Background(), sessionID, pair); err != nil {
				log.Error().Err(err).Str("session_id", sessionID.String()).Msg("failed to start transcoder")
			}
		}()
	})
	whipHandler.OnDisconnect(func(sessionID uuid.UUID, tenantID uuid.UUID) {
		transcodeMgr.Stop(sessionID)
		trackMgr.Remove(sessionID)
		recWorker.Enqueue(sessionID, tenantID)
	})

	r := server.NewRouter(server.RouterDeps{
		Pool:        pool,
		Redis:       rdb,
		JWTAuth:     jwtAuth,
		SessionRepo: sessionRepo,
		ChatRepo:    chatRepo,
		ChatHub:     chatHub,
		HLSHandler:  hlsHandler,
		WHIPHandler: whipHandler,
		RecWorker:   recWorker,
	})

	srv := server.NewServer(r, cfg.Port)

	go func() {
		log.Info().Int("port", cfg.Port).Msg("starting server")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("server failed")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("shutting down server")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatal().Err(err).Msg("server forced to shutdown")
	}
	log.Info().Msg("server stopped")
}
