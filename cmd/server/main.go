package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pion/webrtc/v4"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/asmwasim/webrtc-hls-pipeline/internal/auth"
	"github.com/asmwasim/webrtc-hls-pipeline/internal/config"
	"github.com/asmwasim/webrtc-hls-pipeline/internal/events"
	"github.com/asmwasim/webrtc-hls-pipeline/internal/hls"
	"github.com/asmwasim/webrtc-hls-pipeline/internal/session"
	"github.com/asmwasim/webrtc-hls-pipeline/internal/transcode"
	"github.com/asmwasim/webrtc-hls-pipeline/internal/whip"
)

func main() {
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

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(zerologMiddleware)

	r.Get("/health", healthHandler(pool, rdb))
	r.Handle("/metrics", promhttp.Handler())

	r.Route("/hls/{sessionID}", func(r chi.Router) {
		r.Get("/*", hlsHandler.ServeSegments())
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/sessions", func(r chi.Router) {
			r.With(jwtAuth.Authenticate).Get("/", session.HandleList(sessionRepo))
			r.With(jwtAuth.Authenticate, jwtAuth.RequireRole("teacher")).Post("/", session.HandleCreate(sessionRepo))

			r.Route("/{sessionID}", func(r chi.Router) {
				r.With(jwtAuth.Authenticate).Get("/", session.HandleGet(sessionRepo))
				r.With(jwtAuth.Authenticate, jwtAuth.RequireRole("teacher")).Post("/end", session.HandleEnd(sessionRepo))
				r.With(jwtAuth.Authenticate, jwtAuth.RequireRole("teacher")).Post("/whip", whipHandler.HandleWHIP())
				r.With(jwtAuth.Authenticate, jwtAuth.RequireRole("teacher")).Delete("/whip", whipHandler.HandleDeleteResource())
				r.With(jwtAuth.Authenticate).Get("/watch", hlsHandler.HandleWatch())
			})
		})
	})

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

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

func healthHandler(pool *pgxpool.Pool, rdb *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		if err := pool.Ping(ctx); err != nil {
			http.Error(w, `{"status":"unhealthy","error":"postgres"}`, http.StatusServiceUnavailable)
			return
		}

		if err := rdb.Ping(ctx).Err(); err != nil {
			http.Error(w, `{"status":"unhealthy","error":"redis"}`, http.StatusServiceUnavailable)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"healthy"}`))
	}
}

func zerologMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		log.Info().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Int("status", ww.Status()).
			Dur("latency", time.Since(start)).
			Msg("request")
	})
}
