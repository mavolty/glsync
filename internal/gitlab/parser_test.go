package gitlab_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.surya-am.com/sam/risk/glsync/internal/domain"
	"gitlab.surya-am.com/sam/risk/glsync/internal/gitlab"
)

func TestParse_EmojiAwardOnMR(t *testing.T) {
	payload := map[string]any{
		"object_kind": "emoji",
		"user":        map[string]any{"email": "dev@example.com"},
		"project":     map[string]any{"id": 42},
		"object_attributes": map[string]any{
			"id":             99,
			"name":           "thumbsup",
			"action":         "award",
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
		"user":        map[string]any{"email": "dev@example.com"},
		"project":     map[string]any{"id": 42},
		"object_attributes": map[string]any{
			"id":             99,
			"name":           "thumbsup",
			"action":         "revoke",
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
		"user":        map[string]any{"email": "dev@example.com"},
		"project":     map[string]any{"id": 42},
		"object_attributes": map[string]any{
			"id":             99,
			"name":           "thumbsup",
			"action":         "award",
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
		"user":        map[string]any{"email": "dev@example.com"},
		"project":     map[string]any{"id": 42},
		"object_attributes": map[string]any{
			"id":             100,
			"name":           "rocket",
			"action":         "award",
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
