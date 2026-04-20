package gitlab_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/mavolty/glsync/internal/domain"
	"github.com/mavolty/glsync/internal/gitlab"
)

func TestParse_EmojiAwardOnMR(t *testing.T) {
	payload := map[string]any{
		"object_kind": "emoji",
		"event_type":  "award",
		"user":        map[string]any{"email": "dev@example.com"},
		"project":     map[string]any{"id": 42},
		"object_attributes": map[string]any{
			"id":             99,
			"name":           "thumbsup",
			"awardable_type": "MergeRequest",
			"awardable_id":   123,
		},
		"merge_request": map[string]any{
			"iid":           7,
			"title":         "Fix RIS-100 login bug",
			"target_branch": "master",
			"source_branch": "feature/RIS-100",
			"state":         "merged",
		},
	}
	raw, _ := json.Marshal(payload)

	event, err := gitlab.Parse(json.RawMessage(raw))
	require.NoError(t, err)

	assert.Equal(t, domain.EventEmojiAward, event.EventType)
	assert.Equal(t, "thumbsup", event.EmojiName)
	assert.Equal(t, "merged", event.MRState)
	assert.Equal(t, "master", event.TargetBranch)
	assert.Equal(t, "feature/RIS-100", event.SourceBranch)
	assert.Equal(t, "Fix RIS-100 login bug", event.MRTitle)
	assert.Equal(t, 7, event.MRIID)
	assert.Equal(t, 42, event.ProjectID)
	assert.Contains(t, event.IdempotencyKey, "emoji:42:thumbsup:99")
}

func TestParse_EmojiRevoke_Ignored(t *testing.T) {
	payload := map[string]any{
		"object_kind": "emoji",
		"event_type":  "revoke",
		"user":        map[string]any{"email": "dev@example.com"},
		"project":     map[string]any{"id": 42},
		"object_attributes": map[string]any{
			"id":             99,
			"name":           "thumbsup",
			"awardable_type": "MergeRequest",
			"awardable_id":   123,
		},
	}
	raw, _ := json.Marshal(payload)

	event, err := gitlab.Parse(json.RawMessage(raw))
	require.NoError(t, err)

	assert.Equal(t, domain.EventUnrecognized, event.EventType)
}

func TestParse_EmojiOnNote_Ignored(t *testing.T) {
	payload := map[string]any{
		"object_kind": "emoji",
		"event_type":  "award",
		"user":        map[string]any{"email": "dev@example.com"},
		"project":     map[string]any{"id": 42},
		"object_attributes": map[string]any{
			"id":             99,
			"name":           "thumbsup",
			"awardable_type": "Note",
			"awardable_id":   456,
		},
	}
	raw, _ := json.Marshal(payload)

	event, err := gitlab.Parse(json.RawMessage(raw))
	require.NoError(t, err)

	assert.Equal(t, domain.EventUnrecognized, event.EventType)
}

func TestParse_EmojiWithoutMRDetails(t *testing.T) {
	payload := map[string]any{
		"object_kind": "emoji",
		"event_type":  "award",
		"user":        map[string]any{"email": "dev@example.com"},
		"project":     map[string]any{"id": 42},
		"object_attributes": map[string]any{
			"id":             100,
			"name":           "rocket",
			"awardable_type": "MergeRequest",
			"awardable_id":   200,
		},
	}
	raw, _ := json.Marshal(payload)

	event, err := gitlab.Parse(json.RawMessage(raw))
	require.NoError(t, err)

	assert.Equal(t, domain.EventEmojiAward, event.EventType)
	assert.Equal(t, "rocket", event.EmojiName)
	assert.Equal(t, "", event.TargetBranch)
	assert.Equal(t, "", event.MRState)
	assert.Equal(t, 0, event.MRIID)
}

func TestParse_Push(t *testing.T) {
	payload := map[string]any{
		"object_kind": "push",
		"ref":         "refs/heads/RIS-42-fix-login",
		"before":      "0000000000000000000000000000000000000000",
		"after":       "abc123",
		"project_id":  7,
		"user_email":  "dev@example.com",
		"commits": []map[string]any{
			{"id": "abc123", "message": "RIS-42: fix login bug"},
			{"id": "def456", "message": "cleanup"},
		},
	}
	raw, _ := json.Marshal(payload)

	event, err := gitlab.Parse(json.RawMessage(raw))
	require.NoError(t, err)

	assert.Equal(t, domain.EventPush, event.EventType)
	assert.Equal(t, "RIS-42-fix-login", event.SourceBranch)
	assert.Equal(t, 7, event.ProjectID)
	assert.Equal(t, "dev@example.com", event.AuthorEmail)
	assert.Equal(t, []string{"RIS-42: fix login bug", "cleanup"}, event.CommitMessages)
	assert.Contains(t, event.IdempotencyKey, "push:7:RIS-42-fix-login:")
}

func TestParse_PushEmptyCommits(t *testing.T) {
	payload := map[string]any{
		"object_kind": "push",
		"ref":         "refs/heads/develop",
		"before":      "abc000",
		"after":       "abc123",
		"project_id":  1,
		"commits":     []any{},
	}
	raw, _ := json.Marshal(payload)

	event, err := gitlab.Parse(json.RawMessage(raw))
	require.NoError(t, err)
	assert.Equal(t, domain.EventPush, event.EventType)
	assert.Empty(t, event.CommitMessages)
}

func TestParse_MROpened(t *testing.T) {
	payload := map[string]any{
		"object_kind": "merge_request",
		"user":        map[string]any{"email": "dev@example.com"},
		"project":     map[string]any{"id": 5},
		"object_attributes": map[string]any{
			"iid":           3,
			"title":         "Fix RIS-10 auth",
			"source_branch": "RIS-10-auth",
			"target_branch": "develop",
			"state":         "opened",
			"action":        "open",
			"draft":         false,
		},
	}
	raw, _ := json.Marshal(payload)

	event, err := gitlab.Parse(json.RawMessage(raw))
	require.NoError(t, err)

	assert.Equal(t, domain.EventMROpened, event.EventType)
	assert.Equal(t, "RIS-10-auth", event.SourceBranch)
	assert.Equal(t, "develop", event.TargetBranch)
	assert.Equal(t, "Fix RIS-10 auth", event.MRTitle)
	assert.Equal(t, 3, event.MRIID)
	assert.Equal(t, 5, event.ProjectID)
	assert.Equal(t, "mr:5:3:open", event.IdempotencyKey)
}

func TestParse_MRDraft(t *testing.T) {
	payload := map[string]any{
		"object_kind": "merge_request",
		"user":        map[string]any{"email": "dev@example.com"},
		"project":     map[string]any{"id": 5},
		"object_attributes": map[string]any{
			"iid": 4, "title": "Draft: WIP", "source_branch": "feat", "target_branch": "develop",
			"state": "opened", "action": "open", "draft": true,
		},
	}
	raw, _ := json.Marshal(payload)

	event, err := gitlab.Parse(json.RawMessage(raw))
	require.NoError(t, err)
	assert.Equal(t, domain.EventMRDraft, event.EventType)
}

func TestParse_MRMerged(t *testing.T) {
	payload := map[string]any{
		"object_kind": "merge_request",
		"user":        map[string]any{"email": "dev@example.com"},
		"project":     map[string]any{"id": 5},
		"object_attributes": map[string]any{
			"iid": 5, "title": "Fix", "source_branch": "feat", "target_branch": "master",
			"state": "merged", "action": "merge", "draft": false,
		},
	}
	raw, _ := json.Marshal(payload)

	event, err := gitlab.Parse(json.RawMessage(raw))
	require.NoError(t, err)
	assert.Equal(t, domain.EventMRMerged, event.EventType)
}

func TestParse_MRUpdateNonDraft(t *testing.T) {
	payload := map[string]any{
		"object_kind": "merge_request",
		"user":        map[string]any{"email": "dev@example.com"},
		"project":     map[string]any{"id": 5},
		"object_attributes": map[string]any{
			"iid": 6, "title": "Fix", "source_branch": "feat", "target_branch": "develop",
			"state": "opened", "action": "update", "draft": false,
		},
	}
	raw, _ := json.Marshal(payload)

	event, err := gitlab.Parse(json.RawMessage(raw))
	require.NoError(t, err)
	// non-draft update treated as opened (draft→ready transition)
	assert.Equal(t, domain.EventMROpened, event.EventType)
}

func TestParse_UnknownObjectKind(t *testing.T) {
	payload := map[string]any{"object_kind": "pipeline", "id": 99}
	raw, _ := json.Marshal(payload)

	event, err := gitlab.Parse(json.RawMessage(raw))
	require.NoError(t, err)
	assert.Equal(t, domain.EventUnrecognized, event.EventType)
}

func TestParse_InvalidJSON(t *testing.T) {
	_, err := gitlab.Parse(json.RawMessage(`{bad json`))
	assert.Error(t, err)
}

func TestIsNewBranch(t *testing.T) {
	tests := []struct {
		name   string
		before string
		want   bool
	}{
		{"all-zero SHA is new branch", "0000000000000000000000000000000000000000", true},
		{"empty string is new branch", "", true},
		{"real SHA is existing branch", "abc123def456abc123def456abc123def456abc1", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, gitlab.IsNewBranch(tt.before))
		})
	}
}
