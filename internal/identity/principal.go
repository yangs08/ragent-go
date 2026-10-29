package identity

import (
	"errors"
	"strings"
)

// Principal is supplied by trusted application code, never by model arguments.
// The current CLI supplies a development identity; authentication is not implemented.
type Principal struct{ TenantID, UserID string }

func (p Principal) Validate() error {
	if strings.TrimSpace(p.TenantID) == "" || strings.TrimSpace(p.UserID) == "" {
		return errors.New("trusted tenant and user identity are required")
	}
	return nil
}
