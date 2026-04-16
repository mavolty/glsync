package gitlab

import (
	"regexp"
	"strings"
	"sync"
)

// issueKeyPatterns caches compiled regexes by project key — compiled once per unique key.
var issueKeyPatterns sync.Map

func compiledIssueKeyPattern(projectKey string) *regexp.Regexp {
	if v, ok := issueKeyPatterns.Load(projectKey); ok {
		return v.(*regexp.Regexp)
	}
	p := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(projectKey) + `-(\d+)\b`)
	issueKeyPatterns.Store(projectKey, p)
	return p
}

// IssueKeys extracts Jira issue keys matching the given project key prefix from text.
// It deduplicates results while preserving order of first occurrence.
// Returns an empty slice (never nil) when no keys are found.
func IssueKeys(projectKey, text string) []string {
	pattern := compiledIssueKeyPattern(projectKey)
	matches := pattern.FindAllString(text, -1)

	seen := make(map[string]bool, len(matches))
	result := make([]string, 0, len(matches))
	for _, m := range matches {
		upper := strings.ToUpper(m)
		if !seen[upper] {
			seen[upper] = true
			result = append(result, upper)
		}
	}
	return result
}

// IssueKeysFromBranchAndTitle extracts issue keys from the branch name first,
// then falls back to the MR title if none are found.
func IssueKeysFromBranchAndTitle(projectKey, branch, title string) []string {
	keys := IssueKeys(projectKey, branch)
	if len(keys) > 0 {
		return keys
	}
	return IssueKeys(projectKey, title)
}

// IssueKeysFromEvent extracts issue keys with a 3-level fallback:
//  1. Branch name (most reliable — machine-generated)
//  2. MR title (free-form but intentional)
//  3. Commit messages (for direct pushes to develop/master without an MR)
func IssueKeysFromEvent(projectKey, branch, title string, commitMessages []string) []string {
	if keys := IssueKeys(projectKey, branch); len(keys) > 0 {
		return keys
	}
	if keys := IssueKeys(projectKey, title); len(keys) > 0 {
		return keys
	}
	if len(commitMessages) > 0 {
		return IssueKeys(projectKey, strings.Join(commitMessages, " "))
	}
	return []string{}
}
