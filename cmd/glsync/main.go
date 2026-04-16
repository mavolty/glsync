package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"gitlab.surya-am.com/sam/risk/glsync/internal/config"
	"gitlab.surya-am.com/sam/risk/glsync/internal/integration/jira"
	"gitlab.surya-am.com/sam/risk/glsync/internal/server"
	"gitlab.surya-am.com/sam/risk/glsync/internal/store"
	"gitlab.surya-am.com/sam/risk/glsync/internal/worker"
	"gitlab.surya-am.com/sam/risk/glsync/internal/workflow"
)

func main() {
	configPath := flag.String("config", "config/glsync.yaml", "path to config file")
	flag.Parse()

	logLevel := slog.LevelInfo
	if os.Getenv("GLSYNC_LOG_LEVEL") == "debug" {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Error("load config", "error", err)
		os.Exit(1)
	}
	if err := cfg.Validate(); err != nil {
		logger.Error("invalid config", "error", err)
		os.Exit(1)
	}

	// Root context cancelled on OS signal
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Connect to database
	pool, err := store.NewPool(ctx, cfg.Database)
	if err != nil {
		logger.Error("connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	logger.Info("connected to database")

	// Wire repositories
	events := store.NewEventRepository(pool)
	jobs := store.NewJobRepository(pool)
	audit := store.NewAuditRepository(pool)

	// Wire workflow resolver
	resolver := workflow.NewResolver(cfg.Workflow.Transitions)

	// Wire Jira client
	jiraClient := jira.NewClient(
		cfg.Jira.BaseURL,
		cfg.Jira.Username,
		cfg.Jira.APIToken,
		&http.Client{Timeout: time.Duration(cfg.Jira.Timeout)},
	)

	// Wire job executor and processor
	exec := worker.NewExecutor(jiraClient, resolver, cfg.Workflow.InProgress, logger.With("component", "executor"))
	proc := worker.NewProcessor(
		jobs,
		audit,
		exec,
		worker.ProcessorConfig{
			Concurrency:  cfg.Worker.Concurrency,
			PollInterval: time.Duration(cfg.Worker.PollInterval),
			MaxAttempts:  cfg.Worker.MaxAttempts,
		},
		logger.With("component", "worker"),
	)

	// Wire reconciler
	rec := worker.NewReconciler(
		jobs,
		audit,
		worker.ReconcileConfig{
			Interval:        time.Duration(cfg.Reconcile.Interval),
			StuckJobTimeout: time.Duration(cfg.Reconcile.StuckJobTimeout),
		},
		logger.With("component", "reconciler"),
	)

	// Wire HTTP server
	srv := server.New(
		*cfg, pool, events, jobs, audit, resolver,
		logger.With("component", "server"),
	)

	// Start background components — tracked so we can wait for them at shutdown
	var bgWg sync.WaitGroup
	bgWg.Add(1)
	go func() { defer bgWg.Done(); proc.Run(ctx) }()
	if cfg.Reconcile.Enabled {
		bgWg.Add(1)
		go func() { defer bgWg.Done(); rec.Run(ctx) }()
	}

	// Start HTTP server in background; block until signal
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start() }()

	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	case err := <-errCh:
		if err != nil {
			logger.Error("http server error", "error", err)
		}
	}

	// Graceful shutdown: HTTP first, then wait for in-flight jobs to drain
	logger.Info("shutting down http server")
	if err := srv.Shutdown(time.Duration(cfg.Server.ShutdownTimeout)); err != nil {
		logger.Error("shutdown error", "error", err)
	}

	logger.Info("waiting for background workers to finish")
	bgWg.Wait()
	logger.Info("shutdown complete")
}
