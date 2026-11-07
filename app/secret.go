package app

import (
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrSecretNotFound reports that the secret store holds nothing for the
	// service and account.
	ErrSecretNotFound = errors.New("app: secret not found")
	// ErrSecretStoreUnavailable reports that this system has no secret store to
	// ask: no Secret Service on the session bus, or a target without one. An
	// application then keeps its secrets another way or does not keep them.
	ErrSecretStoreUnavailable = errors.New("app: the system has no secret store")
)

// MaxSecretSize is the longest secret StoreSecret accepts, in bytes. It is what
// the smallest store (the Windows Credential Manager) holds with room to spare:
// the intended contents are keys and tokens, not documents.
const MaxSecretSize = 2048

// StoreSecret saves a secret in the system's own store for credentials: the
// macOS Keychain, the Windows Credential Manager, or a Secret Service such as
// GNOME Keyring or KWallet. service names the application's use ("my-app
// vault key") and account tells entries of that use apart; together they are
// the key, and storing again replaces the secret. Another application cannot
// read it without the operating system's consent, and it is not in any file the
// application keeps.
//
// The call is synchronous and may wait for the user: a locked store asks to be
// unlocked. It must not be made from a hot path.
//
// The Web target has no synchronous system secret store and returns
// ErrSecretStoreUnavailable; use StoreSecretAsync there, which encrypts the
// secret in the page's origin storage.
func StoreSecret(service, account string, secret []byte) error {
	if err := validateSecretKey(service, account); err != nil {
		return err
	}
	if err := validateSecretValue(secret); err != nil {
		return err
	}
	if err, handled := driverStoreSecret(service, account, secret); handled {
		return err
	}
	return nativeStoreSecret(service, account, secret)
}

func validateSecretValue(secret []byte) error {
	if len(secret) == 0 || len(secret) > MaxSecretSize {
		return fmt.Errorf("app: a secret is 1 to %d bytes, got %d", MaxSecretSize, len(secret))
	}
	return nil
}

// LoadSecret returns the secret saved for the service and account, or
// ErrSecretNotFound. Like StoreSecret it may wait for the user. The Web target
// returns ErrSecretStoreUnavailable; use LoadSecretAsync.
func LoadSecret(service, account string) ([]byte, error) {
	if err := validateSecretKey(service, account); err != nil {
		return nil, err
	}
	if secret, err, handled := driverLoadSecret(service, account); handled {
		return secret, err
	}
	return nativeLoadSecret(service, account)
}

// DeleteSecret removes the secret saved for the service and account. Removing
// what is not there is not an error. The Web target returns
// ErrSecretStoreUnavailable; use DeleteSecretAsync.
func DeleteSecret(service, account string) error {
	if err := validateSecretKey(service, account); err != nil {
		return err
	}
	if err, handled := driverDeleteSecret(service, account); handled {
		return err
	}
	return nativeDeleteSecret(service, account)
}

func validateSecretKey(service, account string) error {
	for _, part := range [...]struct{ name, value string }{{"service", service}, {"account", account}} {
		if part.value == "" || len(part.value) > 256 || strings.ContainsRune(part.value, 0) {
			return fmt.Errorf("app: a secret's %s is 1 to 256 bytes without NUL", part.name)
		}
	}
	return nil
}
