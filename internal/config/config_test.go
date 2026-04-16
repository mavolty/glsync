package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.surya-am.com/sam/risk/glsync/internal/config"
)

func writeYAML(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))
	return path
}

func TestLoad_DefaultsApplied(t *testing.T) {
	path := writeYAML(t, ``) // empty YAML — all defaults
	cfg, err := config.Load(path)
	require.NoError(t, err)

	assert.Equal(t, 8090, cfg.Server.Port)
	assert.Equal(t, 10*time.Second, time.Duration(cfg.Server.ReadTimeout))
	assert.Equal(t, 1, cfg.Worker.Concurrency)
	assert.Equal(t, 5, cfg.Worker.MaxAttempts)
	assert.Equal(t, int32(2), cfg.Database.MinConnections)
	assert.Equal(t, 15*time.Minute, time.Duration(cfg.Reconcile.Interval))
	assert.Equal(t, "thumbsup", cfg.Workflow.DoneEmoji)
}

func TestLoad_YAMLOverridesDefaults(t *testing.T) {
	path := writeYAML(t, `
server:
  port: 9090
worker:
  concurrency: 2
`)
	cfg, err := config.Load(path)
	require.NoError(t, err)

	assert.Equal(t, 9090, cfg.Server.Port)
	assert.Equal(t, 2, cfg.Worker.Concurrency)
	// Unset fields keep defaults
	assert.Equal(t, 5, cfg.Worker.MaxAttempts)
}

func TestLoad_DurationParsing(t *testing.T) {
	path := writeYAML(t, `
jira:
  timeout: 30s
worker:
  poll_interval: 10s
reconcile:
  interval: 30m
  stuck_job_timeout: 10m
`)
	cfg, err := config.Load(path)
	require.NoError(t, err)

	assert.Equal(t, 30*time.Second, time.Duration(cfg.Jira.Timeout))
	assert.Equal(t, 10*time.Second, time.Duration(cfg.Worker.PollInterval))
	assert.Equal(t, 30*time.Minute, time.Duration(cfg.Reconcile.Interval))
	assert.Equal(t, 10*time.Minute, time.Duration(cfg.Reconcile.StuckJobTimeout))
}

func TestLoad_EnvVarOverrides(t *testing.T) {
	path := writeYAML(t, `
gitlab:
  webhook_secret: "from-yaml"
`)
	t.Setenv("GLSYNC_GITLAB__WEBHOOK_SECRET", "from-env")
	t.Setenv("GLSYNC_GITLAB__API_TOKEN", "gl-api-token")
	t.Setenv("GLSYNC_JIRA__USERNAME", "jirauser")
	t.Setenv("GLSYNC_JIRA__API_TOKEN", "jiratoken")
	t.Setenv("GLSYNC_DATABASE__URL", "postgres://localhost/test")

	cfg, err := config.Load(path)
	require.NoError(t, err)

	assert.Equal(t, "from-env", cfg.GitLab.WebhookSecret)
	assert.Equal(t, "gl-api-token", cfg.GitLab.APIToken)
	assert.Equal(t, "jirauser", cfg.Jira.Username)
	assert.Equal(t, "jiratoken", cfg.Jira.APIToken)
	assert.Equal(t, "postgres://localhost/test", cfg.Database.URL)
}

func TestLoad_MissingFile(t *testing.T) {
	_, err := config.Load("/nonexistent/config.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read config file")
}

func TestValidate_OK(t *testing.T) {
	cfg := &config.Config{
		GitLab:   config.GitLabConfig{WebhookSecret: "secret"},
		Jira:     config.JiraConfig{Username: "user", APIToken: "token"},
		Database: config.DatabaseConfig{URL: "postgres://localhost/db"},
	}
	assert.NoError(t, cfg.Validate())
}

func TestValidate_MissingRequired(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config.Config
		wantErr string
	}{
		{
			name:    "missing webhook secret",
			cfg:     config.Config{Jira: config.JiraConfig{Username: "u", APIToken: "t"}, Database: config.DatabaseConfig{URL: "u"}},
			wantErr: "GLSYNC_GITLAB__WEBHOOK_SECRET",
		},
		{
			name:    "missing jira username",
			cfg:     config.Config{GitLab: config.GitLabConfig{WebhookSecret: "s"}, Jira: config.JiraConfig{APIToken: "t"}, Database: config.DatabaseConfig{URL: "u"}},
			wantErr: "GLSYNC_JIRA__USERNAME",
		},
		{
			name:    "missing api token",
			cfg:     config.Config{GitLab: config.GitLabConfig{WebhookSecret: "s"}, Jira: config.JiraConfig{Username: "u"}, Database: config.DatabaseConfig{URL: "u"}},
			wantErr: "GLSYNC_JIRA__API_TOKEN",
		},
		{
			name:    "missing db url",
			cfg:     config.Config{GitLab: config.GitLabConfig{WebhookSecret: "s"}, Jira: config.JiraConfig{Username: "u", APIToken: "t"}},
			wantErr: "GLSYNC_DATABASE__URL",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
