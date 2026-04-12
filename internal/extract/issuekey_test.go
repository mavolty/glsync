package extract_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.surya-am.com/sam/risk/glsync/internal/extract"
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
			got := extract.IssueKeys(tt.projectKey, tt.text)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIssueKeysFromBranchAndTitle(t *testing.T) {
	t.Run("branch takes priority over title", func(t *testing.T) {
		got := extract.IssueKeysFromBranchAndTitle("RIS", "RIS-100-branch", "Fix RIS-200 in title")
		assert.Equal(t, []string{"RIS-100"}, got)
	})

	t.Run("falls back to title when branch has no key", func(t *testing.T) {
		got := extract.IssueKeysFromBranchAndTitle("RIS", "hotfix-no-key", "RIS-200: fix issue")
		assert.Equal(t, []string{"RIS-200"}, got)
	})

	t.Run("returns empty when neither has keys", func(t *testing.T) {
		got := extract.IssueKeysFromBranchAndTitle("RIS", "no-key-branch", "no key title")
		assert.Empty(t, got)
	})
}
