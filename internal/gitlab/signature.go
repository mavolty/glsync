package gitlab

import (
	"crypto/subtle"
	"errors"
	"net/http"
)

const tokenHeader = "X-Gitlab-Token"

// ErrInvalidSignature is returned when the webhook token does not match.
var ErrInvalidSignature = errors.New("invalid gitlab webhook token")

// ValidateToken checks the X-Gitlab-Token header against the configured secret.
// Uses constant-time comparison to prevent timing attacks.
func ValidateToken(r *http.Request, secret string) error {
	token := r.Header.Get(tokenHeader)
	if subtle.ConstantTimeCompare([]byte(token), []byte(secret)) != 1 {
		return ErrInvalidSignature
	}
	return nil
}
