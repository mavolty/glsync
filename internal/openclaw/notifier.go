// Package openclaw provides an EventHandler that forwards relevant MR events
// to an OpenClaw/Rina agent via the POST /hooks/agent endpoint.
package openclaw

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/mavolty/glsync/internal/domain"
)

// Config holds connection settings for the OpenClaw hooks endpoint.
type Config struct {
	HookURL   string
	HookToken string
	DryRun    bool
	Timeout   time.Duration
}

// Notifier implements plugin.EventHandler and dispatches relevant MR events
// to OpenClaw by POSTing to /hooks/agent. Errors are non-fatal — the caller
// (webhook_handler) logs them as warnings and continues job enqueuing.
type Notifier struct {
	cfg    Config
	client *http.Client
	logger *slog.Logger
}

// NewNotifier returns a Notifier ready to use. cfg.HookURL must be non-empty.
func NewNotifier(cfg Config, logger *slog.Logger) *Notifier {
	return &Notifier{
		cfg:    cfg,
		client: &http.Client{Timeout: cfg.Timeout},
		logger: logger,
	}
}

// eventContract is the downstream JSON payload sent inside the hooks/agent message.
type eventContract struct {
	Version        int       `json:"version"`
	Source         string    `json:"source"`
	EventID        string    `json:"event_id"`
	IdempotencyKey string    `json:"idempotency_key"`
	EventType      string    `json:"event_type"`
	ReceivedAt     time.Time `json:"received_at"`
	ProjectID      int       `json:"project_id"`
	MRIID          int       `json:"mr_iid"`
	MRTitle        string    `json:"mr_title"`
	SourceBranch   string    `json:"source_branch"`
	TargetBranch   string    `json:"target_branch"`
	IssueKeys      []string  `json:"issue_keys"`
	AuthorEmail    string    `json:"author_email"`
}

type hookPayload struct {
	Message string `json:"message"`
	AgentID string `json:"agentId"`
}

// HandleEvent filters for mr_opened and mr_merged events with issue keys,
// then POSTs to OpenClaw's /hooks/agent endpoint. Returns a non-nil error
// only when the dispatch itself fails; filtered events return nil.
func (n *Notifier) HandleEvent(ctx context.Context, event domain.NormalizedEvent) error {
	if !shouldDispatch(event) {
		return nil
	}

	contract := eventContract{
		Version:        1,
		Source:         "glsync",
		EventID:        event.ID,
		IdempotencyKey: event.IdempotencyKey,
		EventType:      string(event.EventType),
		ReceivedAt:     event.ReceivedAt,
		ProjectID:      event.ProjectID,
		MRIID:          event.MRIID,
		MRTitle:        event.MRTitle,
		SourceBranch:   event.SourceBranch,
		TargetBranch:   event.TargetBranch,
		IssueKeys:      event.IssueKeys,
		AuthorEmail:    event.AuthorEmail,
	}

	contractJSON, err := json.Marshal(contract)
	if err != nil {
		return fmt.Errorf("marshal event contract: %w", err)
	}

	message := fmt.Sprintf("Mode: direct-event\n\nEvent JSON:\n%s", contractJSON)

	if n.cfg.DryRun {
		n.logger.Info("openclaw dispatch dry-run",
			"event_id", event.ID,
			"event_type", event.EventType,
			"idempotency_key", event.IdempotencyKey,
		)
		return nil
	}

	return n.post(ctx, message, event.ID)
}

func (n *Notifier) post(ctx context.Context, message, eventID string) error {
	body, err := json.Marshal(hookPayload{Message: message, AgentID: "main"})
	if err != nil {
		return fmt.Errorf("marshal hook payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.cfg.HookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+n.cfg.HookToken)

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("post to openclaw: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("openclaw returned unexpected status %d", resp.StatusCode)
	}

	n.logger.Info("openclaw dispatch sent",
		"event_id", eventID,
		"status", resp.StatusCode,
	)
	return nil
}

// shouldDispatch returns true only for mr_opened and mr_merged events that
// have at least one issue key. push, drafts, and emoji events are skipped.
func shouldDispatch(event domain.NormalizedEvent) bool {
	if event.EventType != domain.EventMROpened && event.EventType != domain.EventMRMerged {
		return false
	}
	return len(event.IssueKeys) > 0
}
