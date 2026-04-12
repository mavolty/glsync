// Package larkbase will hold the Lark Base integration for deployment tracking.
// This is a stub — the interface is defined here to establish the contract,
// but implementation is deferred until the deployment tracking feature is built.
package larkbase

import "context"

// DeploymentRecord will be the data written to Lark Base per deployment.
// Fields are tentative and will be finalized when the feature is implemented.
type DeploymentRecord struct {
	JiraKey               string
	JiraLink              string
	MRLink                string
	Summary               string
	Author                string
	SourceBranch          string
	TargetBranch          string
	JiraStatus            string
	GitLabStatus          string
	MigrationRequired     bool
	EnvConfigChange       bool
	BackgroundImpact      bool
	SupervisorRestart     bool
	DeploymentNotes       string
	RiskNotes             string
}

// Syncer is the interface the worker will call to write deployment records.
type Syncer interface {
	SyncDeployment(ctx context.Context, record DeploymentRecord) error
}

// NoopSyncer is a no-op implementation used until the real one is built.
type NoopSyncer struct{}

func (NoopSyncer) SyncDeployment(_ context.Context, _ DeploymentRecord) error {
	return nil
}
