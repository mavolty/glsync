package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Duration is a YAML-aware time.Duration.
// It accepts human-readable strings like "5s", "10m", "1h" in config files.
type Duration time.Duration

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	dur, err := time.ParseDuration(value.Value)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", value.Value, err)
	}
	*d = Duration(dur)
	return nil
}

type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Database  DatabaseConfig  `yaml:"database"`
	GitLab    GitLabConfig    `yaml:"gitlab"`
	Jira      JiraConfig      `yaml:"jira"`
	Workflow  WorkflowConfig  `yaml:"workflow"`
	Worker    WorkerConfig    `yaml:"worker"`
	Reconcile ReconcileConfig `yaml:"reconcile"`
}

type ServerConfig struct {
	Port            int      `yaml:"port"`
	ReadTimeout     Duration `yaml:"read_timeout"`
	WriteTimeout    Duration `yaml:"write_timeout"`
	ShutdownTimeout Duration `yaml:"shutdown_timeout"`
	AdminToken      string   `yaml:"admin_token"`
}

type DatabaseConfig struct {
	URL            string `yaml:"url"`
	MaxConnections int32  `yaml:"max_connections"`
	MinConnections int32  `yaml:"min_connections"`
}

type GitLabConfig struct {
	WebhookSecret string `yaml:"webhook_secret"`
	APIToken      string `yaml:"api_token"`
	BaseURL       string `yaml:"base_url"`
}

type JiraConfig struct {
	BaseURL  string   `yaml:"base_url"`
	Username string   `yaml:"username"`
	APIToken string   `yaml:"api_token"`
	Timeout  Duration `yaml:"timeout"`
}

type WorkflowConfig struct {
	ProjectKey         string            `yaml:"project_key"`
	DevelopBranch      string            `yaml:"develop_branch"`
	MasterBranch       string            `yaml:"master_branch"`
	Transitions        map[string]string `yaml:"transitions"`
	DoneEmoji          string            `yaml:"done_emoji"`
	DoneEmojiThreshold int               `yaml:"done_emoji_threshold"`
	InProgress         InProgressConfig  `yaml:"in_progress"`
}

// InProgressConfig controls how the "In Progress" transition is executed.
// It requires sending date fields that other transitions don't need.
type InProgressConfig struct {
	TransitionID     string `yaml:"transition_id"`
	DueDateStrategy  string `yaml:"due_date_strategy"`  // "sprint_end" or "story_points"
	SprintEndWeekday string `yaml:"sprint_end_weekday"` // e.g. "tuesday"
	StartDateField   string `yaml:"start_date_field"`
	DueDateField     string `yaml:"due_date_field"`
	StoryPointsField string `yaml:"story_points_field"`
}

type WorkerConfig struct {
	Concurrency  int      `yaml:"concurrency"`
	PollInterval Duration `yaml:"poll_interval"`
	MaxAttempts  int      `yaml:"max_attempts"`
}

type ReconcileConfig struct {
	Enabled         bool     `yaml:"enabled"`
	Interval        Duration `yaml:"interval"`
	StuckJobTimeout Duration `yaml:"stuck_job_timeout"`
}

// Validate checks that all required secrets and settings are present.
// Call this immediately after Load to fail fast before any connections are made.
func (c *Config) Validate() error {
	var missing []string
	if c.GitLab.WebhookSecret == "" {
		missing = append(missing, "GLSYNC_GITLAB__WEBHOOK_SECRET (gitlab.webhook_secret)")
	}
	if c.Jira.Username == "" {
		missing = append(missing, "GLSYNC_JIRA__USERNAME (jira.username)")
	}
	if c.Jira.APIToken == "" {
		missing = append(missing, "GLSYNC_JIRA__API_TOKEN (jira.api_token)")
	}
	if c.Database.URL == "" {
		missing = append(missing, "GLSYNC_DATABASE__URL (database.url)")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required config values: %s", strings.Join(missing, ", "))
	}
	return nil
}

// Load reads config from a YAML file, then applies GLSYNC_ prefixed env var overrides.
func Load(path string) (*Config, error) {
	cfg := defaults()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file %q: %w", path, err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config file %q: %w", path, err)
	}

	applyEnv(cfg)
	return cfg, nil
}

// applyEnv overrides config values from GLSYNC_-prefixed environment variables.
// Uses double-underscore (__) as the nesting separator to preserve single underscores
// in field names (e.g. GLSYNC_JIRA__API_TOKEN → jira.api_token).
func applyEnv(cfg *Config) {
	if v := os.Getenv("GLSYNC_SERVER__ADMIN_TOKEN"); v != "" {
		cfg.Server.AdminToken = v
	}
	if v := os.Getenv("GLSYNC_GITLAB__WEBHOOK_SECRET"); v != "" {
		cfg.GitLab.WebhookSecret = v
	}
	if v := os.Getenv("GLSYNC_GITLAB__API_TOKEN"); v != "" {
		cfg.GitLab.APIToken = v
	}
	if v := os.Getenv("GLSYNC_GITLAB__BASE_URL"); v != "" {
		cfg.GitLab.BaseURL = v
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
			ReadTimeout:     Duration(10 * time.Second),
			WriteTimeout:    Duration(10 * time.Second),
			ShutdownTimeout: Duration(15 * time.Second),
		},
		Database: DatabaseConfig{
			MaxConnections: 20,
			MinConnections: 2,
		},
		Jira: JiraConfig{
			Timeout: Duration(15 * time.Second),
		},
		Workflow: WorkflowConfig{
			ProjectKey:         "RIS",
			DevelopBranch:      "develop",
			MasterBranch:       "master",
			Transitions:        map[string]string{},
			DoneEmoji:          "thumbsup",
			DoneEmojiThreshold: 2,
			InProgress: InProgressConfig{
				DueDateStrategy:  "sprint_end",
				SprintEndWeekday: "tuesday",
				StartDateField:   "customfield_10040",
				DueDateField:     "duedate",
				StoryPointsField: "customfield_10027",
			},
		},
		Worker: WorkerConfig{
			Concurrency:  1,
			PollInterval: Duration(5 * time.Second),
			MaxAttempts:  5,
		},
		Reconcile: ReconcileConfig{
			Enabled:         true,
			Interval:        Duration(15 * time.Minute),
			StuckJobTimeout: Duration(5 * time.Minute),
		},
	}
}
