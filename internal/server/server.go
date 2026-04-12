package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"gitlab.surya-am.com/sam/risk/glsync/internal/config"
	"gitlab.surya-am.com/sam/risk/glsync/internal/store"
	"gitlab.surya-am.com/sam/risk/glsync/internal/workflow"
)

// Server wraps the HTTP server and its dependencies.
type Server struct {
	http   *http.Server
	logger *slog.Logger
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
	r := chi.NewRouter()

	// Middleware stack
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(RequestLogger(logger))

	// Health endpoints
	health := &healthHandler{pool: pool}
	r.Get("/healthz", health.liveness)
	r.Get("/readyz", health.readiness)

	// Webhook receiver
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
	r.Post("/api/v1/webhooks/gitlab", wh.handleGitLab)

	// Admin endpoints (no auth yet — add IP allowlist or token in production)
	admin := &adminHandler{events: events, jobs: jobs}
	r.Get("/api/v1/admin/events", admin.listEvents)
	r.Get("/api/v1/admin/jobs/failed", admin.listFailedJobs)

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
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
