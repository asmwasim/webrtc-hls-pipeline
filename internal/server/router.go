package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"github.com/asmwasim/webrtc-hls-pipeline/internal/auth"
	"github.com/asmwasim/webrtc-hls-pipeline/internal/chat"
	"github.com/asmwasim/webrtc-hls-pipeline/internal/hls"
	"github.com/asmwasim/webrtc-hls-pipeline/internal/recording"
	"github.com/asmwasim/webrtc-hls-pipeline/internal/session"
	"github.com/asmwasim/webrtc-hls-pipeline/internal/whip"
)

type RouterDeps struct {
	Pool        *pgxpool.Pool
	Redis       *redis.Client
	JWTAuth     *auth.JWTAuth
	SessionRepo *session.Repository
	ChatRepo    *chat.Repository
	ChatHub     *chat.Hub
	HLSHandler  *hls.Handler
	WHIPHandler *whip.Handler
	RecWorker   *recording.Worker
}

func NewRouter(deps RouterDeps) chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(zerologMiddleware)

	r.Get("/health", healthHandler(deps.Pool, deps.Redis))
	r.Handle("/metrics", promhttp.Handler())

	r.Route("/hls/{sessionID}", func(r chi.Router) {
		r.Get("/*", deps.HLSHandler.ServeSegments())
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/sessions", func(r chi.Router) {
			r.With(deps.JWTAuth.Authenticate).Get("/", session.HandleList(deps.SessionRepo))
			r.With(deps.JWTAuth.Authenticate, deps.JWTAuth.RequireRole("teacher")).Post("/", session.HandleCreate(deps.SessionRepo))

			r.Route("/{sessionID}", func(r chi.Router) {
				r.With(deps.JWTAuth.Authenticate).Get("/", session.HandleGet(deps.SessionRepo))
				r.With(deps.JWTAuth.Authenticate, deps.JWTAuth.RequireRole("teacher")).Post("/end", session.HandleEnd(deps.SessionRepo))
				r.With(deps.JWTAuth.Authenticate, deps.JWTAuth.RequireRole("teacher")).Post("/whip", deps.WHIPHandler.HandleWHIP())
				r.With(deps.JWTAuth.Authenticate, deps.JWTAuth.RequireRole("teacher")).Delete("/whip", deps.WHIPHandler.HandleDeleteResource())
				r.With(deps.JWTAuth.Authenticate).Get("/watch", deps.HLSHandler.HandleWatch())
				r.With(deps.JWTAuth.Authenticate).Get("/chat", chat.HandleWebSocket(deps.ChatHub))
				r.With(deps.JWTAuth.Authenticate).Get("/chat/history", chat.HandleHistory(deps.ChatRepo))
				r.With(deps.JWTAuth.Authenticate).Get("/recording", recording.HandleGetRecording(deps.Pool))
			})
		})
	})

	return r
}

func NewServer(handler http.Handler, port int) *http.Server {
	return &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
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
