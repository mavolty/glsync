package workflow_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.surya-am.com/sam/risk/glsync/internal/domain"
	"gitlab.surya-am.com/sam/risk/glsync/internal/workflow"
)

func TestResolveTransitionID(t *testing.T) {
	transitions := map[string]string{
		"in_progress": "21",
		"code_review": "31",
		"rfqa":        "41",
		"done":        "51",
	}
	r := workflow.NewResolver(transitions)

	t.Run("resolves known states", func(t *testing.T) {
		id, err := r.ResolveTransitionID(domain.StateInProgress)
		assert.NoError(t, err)
		assert.Equal(t, "21", id)

		id, err = r.ResolveTransitionID(domain.StateDone)
		assert.NoError(t, err)
		assert.Equal(t, "51", id)
	})

	t.Run("errors on unconfigured state", func(t *testing.T) {
		r2 := workflow.NewResolver(map[string]string{})
		_, err := r2.ResolveTransitionID(domain.StateRFQA)
		assert.Error(t, err)
	})
}
