package gitlab_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.surya-am.com/sam/risk/glsync/internal/gitlab"
)

func TestIssueKeys(t *testing.T) {
	tests := []struct {
		name       string
		projectKey string
		text       string
		want       []string
	}{
		{
			name:       "branch name with key",
			projectKey: "RIS",
			text:       "RIS-123-fix-something",
			want:       []string{"RIS-123"},
		},
		{
			name:       "multiple keys in text",
			projectKey: "RIS",
			text:       "Merge RIS-100 and RIS-200 changes",
			want:       []string{"RIS-100", "RIS-200"},
		},
		{
			name:       "duplicate keys deduplicated",
			projectKey: "RIS",
			text:       "RIS-123 fix for RIS-123",
			want:       []string{"RIS-123"},
		},
		{
			name:       "case insensitive match",
			projectKey: "RIS",
			text:       "ris-456-feat",
			want:       []string{"RIS-456"},
		},
		{
			name:       "no match returns empty slice",
			projectKey: "RIS",
			text:       "fix something without jira key",
			want:       []string{},
		},
		{
			name:       "different project key not matched",
			projectKey: "RIS",
			text:       "OTHER-123 feature",
			want:       []string{},
		},
		{
			name:       "partial match not extracted (word boundary)",
			projectKey: "RIS",
			text:       "NORIS-123 irrelevant",
			want:       []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := gitlab.IssueKeys(tt.projectKey, tt.text)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIssueKeysFromBranchAndTitle(t *testing.T) {
	t.Run("branch takes priority over title", func(t *testing.T) {
		got := gitlab.IssueKeysFromBranchAndTitle("RIS", "RIS-100-branch", "Fix RIS-200 in title")
		assert.Equal(t, []string{"RIS-100"}, got)
	})

	t.Run("falls back to title when branch has no key", func(t *testing.T) {
		got := gitlab.IssueKeysFromBranchAndTitle("RIS", "hotfix-no-key", "RIS-200: fix issue")
		assert.Equal(t, []string{"RIS-200"}, got)
	})

	t.Run("returns empty when neither has keys", func(t *testing.T) {
		got := gitlab.IssueKeysFromBranchAndTitle("RIS", "no-key-branch", "no key title")
		assert.Empty(t, got)
	})
}

func TestIssueKeysFromEvent(t *testing.T) {
	t.Run("branch takes priority", func(t *testing.T) {
		got := gitlab.IssueKeysFromEvent("RIS", "RIS-100-branch", "RIS-200 in title", []string{"RIS-300 in commit"})
		assert.Equal(t, []string{"RIS-100"}, got)
	})

	t.Run("title used when branch has no key", func(t *testing.T) {
		got := gitlab.IssueKeysFromEvent("RIS", "no-key", "RIS-200 in title", []string{"RIS-300 in commit"})
		assert.Equal(t, []string{"RIS-200"}, got)
	})

	t.Run("commit messages used when branch and title have no keys", func(t *testing.T) {
		got := gitlab.IssueKeysFromEvent("RIS", "develop", "",
			[]string{"RIS-123: fix null pointer", "minor refactor"})
		assert.Equal(t, []string{"RIS-123"}, got)
	})

	t.Run("multiple keys across commits are deduplicated", func(t *testing.T) {
		got := gitlab.IssueKeysFromEvent("RIS", "develop", "",
			[]string{"RIS-100: first fix", "RIS-200: second fix", "RIS-100 again"})
		assert.Equal(t, []string{"RIS-100", "RIS-200"}, got)
	})

	t.Run("returns empty when nothing has keys", func(t *testing.T) {
		got := gitlab.IssueKeysFromEvent("RIS", "develop", "", []string{"minor refactor", "typo fix"})
		assert.Equal(t, []string{}, got)
	})

	t.Run("returns empty with nil commit messages", func(t *testing.T) {
		got := gitlab.IssueKeysFromEvent("RIS", "develop", "", nil)
		assert.Equal(t, []string{}, got)
	})
}
