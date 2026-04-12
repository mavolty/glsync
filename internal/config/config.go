package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

type Config struct {
	Server    ServerConfig    `koanf:"server"`
	Database  DatabaseConfig  `koanf:"database"`
	GitLab    GitLabConfig    `koanf:"gitlab"`
	Jira      JiraConfig      `koanf:"jira"`
	Workflow  WorkflowConfig  `koanf:"workflow"`
	Worker    WorkerConfig    `koanf:"worker"`
	Reconcile ReconcileConfig `koanf:"reconcile"`
}

type ServerConfig struct {
	Port            int           `koanf:"port"`
	ReadTimeout     time.Duration `koanf:"read_timeout"`
	WriteTimeout    time.Duration `koanf:"write_timeout"`
	ShutdownTimeout time.Duration `koanf:"shutdown_timeout"`
}

type DatabaseConfig struct {
	URL            string `koanf:"url"`
	MaxConnections int32  `koanf:"max_connections"`
	MinConnections int32  `koanf:"min_connections"`
}

type GitLabConfig struct {
	WebhookSecret string `koanf:"webhook_secret"`
}

type JiraConfig struct {
	BaseURL  string        `koanf:"base_url"`
	Username string        `koanf:"username"`
	APIToken string        `koanf:"api_token"`
	Timeout  time.Duration `koanf:"timeout"`
}

type WorkflowConfig struct {
	ProjectKey    string            `koanf:"project_key"`
	DevelopBranch string            `koanf:"develop_branch"`
	MasterBranch  string            `koanf:"master_branch"`
	Transitions   map[string]string `koanf:"transitions"`
	InProgress    InProgressConfig  `koanf:"in_progress"`
}

// InProgressConfig controls how the "In Progress" transition is executed.
// It requires sending date fields that other transitions don't need.
type InProgressConfig struct {
	TransitionID      string `koanf:"transition_id"`
	DueDateStrategy   string `koanf:"due_date_strategy"`   // "sprint_end" or "story_points"
	SprintEndWeekday  string `koanf:"sprint_end_weekday"`  // e.g. "tuesday"
	StartDateField    string `koanf:"start_date_field"`
	DueDateField      string `koanf:"due_date_field"`
	StoryPointsField  string `koanf:"story_points_field"`
}

type WorkerConfig struct {
	Concurrency  int           `koanf:"concurrency"`
	PollInterval time.Duration `koanf:"poll_interval"`
	MaxAttempts  int           `koanf:"max_attempts"`
}

type ReconcileConfig struct {
	Enabled          bool          `koanf:"enabled"`
	Interval         time.Duration `koanf:"interval"`
	StuckJobTimeout  time.Duration `koanf:"stuck_job_timeout"`
	DriftLookback    time.Duration `koanf:"drift_lookback"`
}

// Load reads config from a YAML file, then overrides with GLSYNC_ prefixed env vars.
func Load(path string) (*Config, error) {
	k := koanf.New(".")

	if err := k.Load(file.Provider(path), yaml.Parser()); err != nil {
		return nil, fmt.Errorf("load yaml config %q: %w", path, err)
	}

	// Env vars override file values. GLSYNC_SERVER_PORT -> server.port
	if err := k.Load(env.Provider("GLSYNC_", ".", func(s string) string {
		return replaceEnvKey(s)
	}), nil); err != nil {
		return nil, fmt.Errorf("load env config: %w", err)
	}

	cfg := defaults()
	if err := k.Unmarshal("", cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	// Override secrets directly from env vars — koanf's env provider
	// has difficulty mapping keys with underscores in field names.
	overrideFromEnv(cfg)

	return cfg, nil
}

// overrideFromEnv applies environment variable overrides directly,
// bypassing koanf's key transformation for values that contain underscores.
func overrideFromEnv(cfg *Config) {
	if v := os.Getenv("GLSYNC_GITLAB__WEBHOOK_SECRET"); v != "" {
		cfg.GitLab.WebhookSecret = v
	}
	if v := os.Getenv("GLSYNC_JIRA__BASE_URL"); v != "" {
		cfg.Jira.BaseURL = v
	}
	if v := os.Getenv("GLSYNC_JIRA__USERNAME"); v != "" {
		cfg.Jira.Username = v
	}
	if v := os.Getenv("GLSYNC_JIRA__API_TOKEN"); v != "" {
		cfg.Jira.APIToken = v
	}
	if v := os.Getenv("GLSYNC_DATABASE__URL"); v != "" {
		cfg.Database.URL = v
	}
}

func defaults() *Config {
	return &Config{
		Server: ServerConfig{
			Port:            8090,
			ReadTimeout:     10 * time.Second,
			WriteTimeout:    10 * time.Second,
			ShutdownTimeout: 15 * time.Second,
		},
		Database: DatabaseConfig{
			MaxConnections: 20,
			MinConnections: 5,
		},
		Jira: JiraConfig{
			Timeout: 15 * time.Second,
		},
		Workflow: WorkflowConfig{
			ProjectKey:    "RIS",
			DevelopBranch: "develop",
			MasterBranch:  "master",
			Transitions:   map[string]string{},
			InProgress: InProgressConfig{
				DueDateStrategy:  "sprint_end",
				SprintEndWeekday: "tuesday",
				StartDateField:   "customfield_10236",
				DueDateField:     "customfield_10246",
				StoryPointsField: "customfield_10027",
			},
		},
		Worker: WorkerConfig{
			Concurrency:  3,
			PollInterval: 5 * time.Second,
			MaxAttempts:  5,
		},
		Reconcile: ReconcileConfig{
			Enabled:         true,
			Interval:        15 * time.Minute,
			StuckJobTimeout: 5 * time.Minute,
			DriftLookback:   1 * time.Hour,
		},
	}
}

// replaceEnvKey converts env var names to koanf key paths.
// Uses __ (double underscore) as the nesting separator so single underscores
// in field names are preserved.
// Examples:
//   GITLAB__WEBHOOK_SECRET -> gitlab.webhook_secret
//   SERVER__READ_TIMEOUT   -> server.read_timeout
// The GLSYNC_ prefix is already stripped by env.Provider before this is called.
func replaceEnvKey(s string) string {
	return strings.ReplaceAll(strings.ToLower(s), "__", ".")
}
