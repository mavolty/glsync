package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"gitlab.surya-am.com/sam/risk/glsync/internal/config"
	"gitlab.surya-am.com/sam/risk/glsync/internal/domain"
	"gitlab.surya-am.com/sam/risk/glsync/internal/extract"
	"gitlab.surya-am.com/sam/risk/glsync/internal/gitlab"
	gitlabapi "gitlab.surya-am.com/sam/risk/glsync/internal/integration/gitlab"
	"gitlab.surya-am.com/sam/risk/glsync/internal/store"
	"gitlab.surya-am.com/sam/risk/glsync/internal/worker"
	"gitlab.surya-am.com/sam/risk/glsync/internal/workflow"
)

const maxWebhookBodyBytes = 5 * 1024 * 1024 // 5 MB

type webhookHandler struct {
	gitlabCfg    config.GitLabConfig
	workflow     config.WorkflowConfig
	resolver     *workflow.Resolver
	gitlabClient *gitlabapi.Client
	events       store.EventRepository
	jobs         store.JobRepository
	audit        store.AuditRepository
	logger       *slog.Logger
	maxAttempts  int
}

func (h *webhookHandler) handleGitLab(w http.ResponseWriter, r *http.Request) {
	// 1. Validate webhook token
	if err := gitlab.ValidateToken(r, h.gitlabCfg.WebhookSecret); err != nil {
		h.logger.Warn("invalid webhook token", "remote_addr", r.RemoteAddr)
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// 2. Read body — enforce size at HTTP layer to prevent silent truncation
	r.Body = http.MaxBytesReader(w, r.Body, maxWebhookBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		if err.Error() == "http: request body too large" {
			writeError(w, http.StatusRequestEntityTooLarge, "payload too large")
			return
		}
		writeError(w, http.StatusBadRequest, "failed to read body")
		return
	}

	raw := json.RawMessage(body)

	// 3. Parse into normalized event
	event, err := gitlab.Parse(raw)
	if err != nil {
		h.logger.Error("parse gitlab webhook", "error", err)
		writeError(w, http.StatusBadRequest, "failed to parse event")
		return
	}

	// 4. Silently accept unrecognized events
	if event.EventType == "unrecognized" {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored"})
		return
	}

	// 5. Idempotency check
	exists, err := h.events.ExistsByIdempotencyKey(r.Context(), event.IdempotencyKey)
	if err != nil {
		h.logger.Error("check idempotency key", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if exists {
		writeJSON(w, http.StatusOK, map[string]string{"status": "already_processed"})
		return
	}

	// 6. For push events, only process:
	//    - New branches (before SHA is all zeros) → In Progress
	//    - Pushes to develop/master → RFQA/Done
	if event.EventType == domain.EventPush {
		var probe struct {
			Before string `json:"before"`
		}
		if err := json.Unmarshal(raw, &probe); err != nil {
			writeError(w, http.StatusBadRequest, "failed to parse push event")
			return
		}
		isTargetBranch := event.SourceBranch == h.workflow.DevelopBranch ||
			event.SourceBranch == h.workflow.MasterBranch
		if !gitlab.IsNewBranch(probe.Before) && !isTargetBranch {
			writeJSON(w, http.StatusOK, map[string]string{"status": "ignored", "reason": "not_new_branch"})
			return
		}
	}

	// 7. Extract issue keys
	event.IssueKeys = extract.IssueKeysFromBranchAndTitle(
		h.workflow.ProjectKey, event.SourceBranch, event.MRTitle,
	)

	// --- Emoji event: separate handling path ---
	if event.EventType == domain.EventEmojiAward {
		h.handleEmojiAward(w, r, event)
		return
	}

	// 8. Determine target workflow state
	rulesCfg := workflow.RuleConfig{
		DevelopBranch: h.workflow.DevelopBranch,
		MasterBranch:  h.workflow.MasterBranch,
	}
	targetState, shouldProcess := workflow.Classify(event, rulesCfg)
	if !shouldProcess || len(event.IssueKeys) == 0 {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored", "reason": "no_action"})
		return
	}

	// 9. Persist event
	event.ID = uuid.NewString()
	event.ReceivedAt = time.Now()
	if err := h.events.Insert(r.Context(), event); err != nil {
		h.logger.Error("insert event", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// 10. Enqueue one job per issue key
	for _, key := range event.IssueKeys {
		job := worker.NewJob(event.ID, key, targetState, h.maxAttempts)
		if err := h.jobs.Enqueue(r.Context(), job); err != nil {
			h.logger.Error("enqueue job", "issue_key", key, "error", err)
			// Continue with other keys rather than failing the whole request
		}
	}

	h.logger.Info("event accepted",
		"event_id", event.ID,
		"event_type", event.EventType,
		"issue_keys", event.IssueKeys,
		"target_state", targetState,
	)

	writeJSON(w, http.StatusAccepted, map[string]any{
		"status":       "accepted",
		"event_id":     event.ID,
		"issue_keys":   event.IssueKeys,
		"target_state": targetState,
	})
}

// handleEmojiAward processes emoji award events on merge requests.
// When the emoji matches the configured done_emoji and the MR is merged
// to the master branch, it counts total awards via the GitLab API.
// If the count meets the threshold, it enqueues a "done" transition job.
func (h *webhookHandler) handleEmojiAward(w http.ResponseWriter, r *http.Request, event domain.NormalizedEvent) {
	// 1. Check emoji name matches configured trigger
	if event.EmojiName != h.workflow.DoneEmoji {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored", "reason": "wrong_emoji"})
		return
	}

	// 2. Check MR is merged and targets master
	if event.MRState != "merged" {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored", "reason": "mr_not_merged"})
		return
	}
	if event.TargetBranch != h.workflow.MasterBranch {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored", "reason": "not_master"})
		return
	}

	// 3. Must have issue keys extracted from MR title
	if len(event.IssueKeys) == 0 {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored", "reason": "no_issue_keys"})
		return
	}

	// 4. Count thumbsups via GitLab API
	if h.gitlabClient == nil {
		h.logger.Error("gitlab api client not configured, cannot count emoji")
		writeError(w, http.StatusInternalServerError, "gitlab api not configured")
		return
	}

	count, err := h.gitlabClient.CountAwardEmoji(r.Context(), event.ProjectID, event.MRIID, h.workflow.DoneEmoji)
	if err != nil {
		h.logger.Error("count award emoji", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to count emoji")
		return
	}

	threshold := h.workflow.DoneEmojiThreshold
	if threshold <= 0 {
		threshold = 2
	}

	if count < threshold {
		writeJSON(w, http.StatusOK, map[string]any{
			"status": "waiting",
			"count":  count,
			"needed": threshold,
		})
		return
	}

	// 5. Threshold met — persist event and enqueue done jobs
	event.ID = uuid.NewString()
	event.ReceivedAt = time.Now()
	if err := h.events.Insert(r.Context(), event); err != nil {
		h.logger.Error("insert emoji event", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	for _, key := range event.IssueKeys {
		job := worker.NewJob(event.ID, key, domain.StateDone, h.maxAttempts)
		if err := h.jobs.Enqueue(r.Context(), job); err != nil {
			h.logger.Error("enqueue done job", "issue_key", key, "error", err)
		}
	}

	h.logger.Info("emoji threshold met, enqueuing done",
		"event_id", event.ID,
		"issue_keys", event.IssueKeys,
		"emoji_count", count,
	)

	writeJSON(w, http.StatusAccepted, map[string]any{
		"status":       "accepted",
		"event_id":     event.ID,
		"issue_keys":   event.IssueKeys,
		"target_state": "done",
		"emoji_count":  count,
	})
}
