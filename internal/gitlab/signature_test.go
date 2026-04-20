package gitlab_test

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/mavolty/glsync/internal/gitlab"
)

func TestValidateToken(t *testing.T) {
	const secret = "my-webhook-secret"

	t.Run("valid token passes", func(t *testing.T) {
		r := httptest.NewRequest("POST", "/", nil)
		r.Header.Set("X-Gitlab-Token", secret)
		assert.NoError(t, gitlab.ValidateToken(r, secret))
	})

	t.Run("missing token fails", func(t *testing.T) {
		r := httptest.NewRequest("POST", "/", nil)
		assert.ErrorIs(t, gitlab.ValidateToken(r, secret), gitlab.ErrInvalidSignature)
	})

	t.Run("wrong token fails", func(t *testing.T) {
		r := httptest.NewRequest("POST", "/", nil)
		r.Header.Set("X-Gitlab-Token", "wrong-secret")
		assert.ErrorIs(t, gitlab.ValidateToken(r, secret), gitlab.ErrInvalidSignature)
	})
}
