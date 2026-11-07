//go:build js && wasm

package app

import (
	"errors"
	"fmt"

	webplatform "github.com/vivaui/viva/internal/platform/web"
)

func asyncRegisterFont(data []byte, done func(string, func(), error)) error {
	return webplatform.RegisterFontAsync(data, func(family string, unregister func(), err error) {
		if err == nil {
			done(family, unregister, nil)
			return
		}
		switch {
		case errors.Is(err, webplatform.ErrPermissionDenied):
			done("", nil, fmt.Errorf("register font: %w: %v", ErrPermissionDenied, err))
		case errors.Is(err, webplatform.ErrUnsupported), errors.Is(err, webplatform.ErrUnavailable):
			done("", nil, fmt.Errorf("register font: %w: %v", ErrUnsupported, err))
		default:
			done("", nil, fmt.Errorf("app: register font: %w", err))
		}
	})
}
