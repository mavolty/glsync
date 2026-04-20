package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mavolty/glsync/internal/config"
	"github.com/mavolty/glsync/internal/store"
	"github.com/mavolty/glsync/internal/workflow"
)

// Server wraps the HTTP server and its dependencies.
type Server struct {
	http   *http.Server
	logger *slog.Logger
}

// NewHandler assembles and returns the chi router with all routes wired.
// pool may be nil in tests (health endpoints will be skipped).
// logger may be nil; slog.Default() is used in that case.
func NewHandler(
	cfg config.Config,
	pool *pgxpool.Pool,
	events store.EventRepository,
	jobs store.JobRepository,
	audit store.AuditRepository,
	resolver *workflow.Resolver,
	logger *slog.Logger,
) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(RequestLogger(logger))

	// Health endpoints (skipped when pool is nil, e.g. in tests)
	health := &healthHandler{pool: pool}
	r.Get("/healthz", health.liveness)
	r.Get("/readyz", health.readiness)

	// Webhook receiver — rate-limited to prevent pool exhaustion from retry storms
	wh := &webhookHandler{
		gitlabCfg:   cfg.GitLab,
		workflow:    cfg.Workflow,
		resolver:    resolver,
		events:      events,
		jobs:        jobs,
		audit:       audit,
		logger:      logger,
		maxAttempts: cfg.Worker.MaxAttempts,
	}
	r.With(httprate.LimitByIP(100, time.Minute)).
		Post("/api/v1/webhooks/gitlab", wh.handleGitLab)

	// Admin endpoints — protected by bearer token (set GLSYNC_SERVER__ADMIN_TOKEN).
	// If AdminToken is empty, routes return 404.
	admin := &adminHandler{events: events, jobs: jobs}
	r.Route("/api/v1/admin", func(r chi.Router) {
		r.Use(RequireAdminToken(cfg.Server.AdminToken))
		r.Get("/events", admin.listEvents)
		r.Get("/jobs/failed", admin.listFailedJobs)
	})

	return r
}

// New creates and configures the HTTP server with all routes.
func New(
	cfg config.Config,
	pool *pgxpool.Pool,
	events store.EventRepository,
	jobs store.JobRepository,
	audit store.AuditRepository,
	resolver *workflow.Resolver,
	logger *slog.Logger,
) *Server {
	handler := NewHandler(cfg, pool, events, jobs, audit, resolver, logger)
	httpServer := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:           handler,
		ReadTimeout:       time.Duration(cfg.Server.ReadTimeout),
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      time.Duration(cfg.Server.WriteTimeout),
		IdleTimeout:       60 * time.Second,
	}
	return &Server{http: httpServer, logger: logger}
}

// Start begins serving HTTP requests. It blocks until the server stops.
func (s *Server) Start() error {
	s.logger.Info("starting http server", "addr", s.http.Addr)
	if err := s.http.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("http server error: %w", err)
	}
	return nil
}

// Shutdown gracefully drains active connections within the configured timeout.
func (s *Server) Shutdown(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return s.http.Shutdown(ctx)
}
