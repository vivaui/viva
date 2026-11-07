//go:build js && wasm

package app

import (
	"errors"
	"fmt"

	webplatform "github.com/vivaui/viva/internal/platform/web"
)

func asyncStoreSecret(service, account string, secret []byte, done func(error)) error {
	return webplatform.StoreSecretAsync(service, account, secret, func(err error) {
		done(webSecretError("store secret", err))
	})
}

func asyncLoadSecret(service, account string, done func([]byte, error)) error {
	return webplatform.LoadSecretAsync(service, account, func(secret []byte, err error) {
		done(secret, webSecretError("load secret", err))
	})
}

func asyncDeleteSecret(service, account string, done func(error)) error {
	return webplatform.DeleteSecretAsync(service, account, func(err error) {
		done(webSecretError("delete secret", err))
	})
}

func webSecretError(action string, err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, webplatform.ErrSecretNotFound):
		return ErrSecretNotFound
	case errors.Is(err, webplatform.ErrPermissionDenied):
		return fmt.Errorf("%s: %w: %v", action, ErrPermissionDenied, err)
	case errors.Is(err, webplatform.ErrUnavailable), errors.Is(err, webplatform.ErrUnsupported):
		return fmt.Errorf("%s: %w: %v", action, ErrSecretStoreUnavailable, err)
	default:
		return fmt.Errorf("app: %s: %w", action, err)
	}
}
