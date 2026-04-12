package workflow

import (
	"fmt"

	"gitlab.surya-am.com/sam/risk/glsync/internal/domain"
)

// Resolver maps WorkflowState values to Jira transition IDs using a config-driven map.
// The map keys are the string forms of WorkflowState constants.
type Resolver struct {
	transitions map[string]string
}

func NewResolver(transitions map[string]string) *Resolver {
	return &Resolver{transitions: transitions}
}

// ResolveTransitionID returns the Jira transition ID for the given workflow state.
// Returns an error if the state is not configured — this is intentional so
// misconfigured mappings fail loudly rather than silently.
func (r *Resolver) ResolveTransitionID(state domain.WorkflowState) (string, error) {
	id, ok := r.transitions[string(state)]
	if !ok || id == "" {
		return "", fmt.Errorf("no jira transition configured for state %q", state)
	}
	return id, nil
}
