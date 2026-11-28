package windows

import (
	"errors"
	"testing"

	"github.com/vivaui/viva/internal/platform"
)

// A write that reports ERROR_NOT_FOUND must not look like a missing secret:
// StoreSecret promises ErrSecretNotFound never comes from it, and an
// application that stores a key on first use would otherwise surface the
// sentinel as its save error.
func TestSecretWriteNotFoundIsAnUnavailableStore(t *testing.T) {
	if err := secretError("write", errorNotFound); !errors.Is(err, platform.ErrSecretStoreUnavailable) || errors.Is(err, platform.ErrSecretNotFound) {
		t.Fatalf("write not found = %v, want the store unavailable", err)
	}
	for _, operation := range []string{"read", "delete"} {
		if err := secretError(operation, errorNotFound); !errors.Is(err, platform.ErrSecretNotFound) {
			t.Fatalf("%s not found = %v, want ErrSecretNotFound", operation, err)
		}
	}
}
