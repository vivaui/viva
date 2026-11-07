//go:build js && wasm

package app

import (
	"errors"
	"fmt"

	webplatform "github.com/vivaui/viva/internal/platform/web"
)

// RequestFullscreen asks the page to enter or leave the browser's fullscreen
// mode. It is the page counterpart of Window.SetFullscreen: a browser has no
// window object for the application to hold, and the request has to start from
// a user gesture. A nil error means the browser accepted the request; the
// transition is reported by Config.OnFullscreen. A target without a page
// returns an error wrapping ErrUnsupported.
func RequestFullscreen(fullscreen bool) error {
	err := webplatform.RequestFullscreen(fullscreen)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, webplatform.ErrPermissionDenied):
		return fmt.Errorf("request fullscreen: %w: %v", ErrPermissionDenied, err)
	case errors.Is(err, webplatform.ErrUserGestureRequired):
		return fmt.Errorf("request fullscreen: %w: %v", ErrUserGestureRequired, err)
	case errors.Is(err, webplatform.ErrUnsupported), errors.Is(err, webplatform.ErrUnavailable):
		return fmt.Errorf("request fullscreen: %w: %v", ErrUnsupported, err)
	default:
		return fmt.Errorf("app: request fullscreen: %w", err)
	}
}
