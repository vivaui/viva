package app

import (
	"errors"
	"fmt"

	"github.com/vivaui/viva/internal/platform"
)

type decorationControl interface {
	SetDecorated(bool, func(error)) error
}

// SetDecorated changes the native decorations of a normal, mapped window
// without replacing it or moving its content. It does not change fullscreen
// or always-on-top state. It is safe to call from any goroutine.
//
// A non-nil error rejects the request without calling done. An accepted
// request calls done once on the native window loop after the transition is
// confirmed, fails, or is interrupted by closing the window. Do not block
// the window loop waiting for done. Hosts without this capability return an
// error wrapping ErrUnsupported.
func (window *Window) SetDecorated(decorated bool, done func(error)) error {
	if done == nil {
		return fmt.Errorf("app: window decoration completion callback is required")
	}
	if window == nil || window.closed.Load() {
		return fmt.Errorf("app: window is closed")
	}
	control, ok := window.control.(decorationControl)
	if !ok {
		return fmt.Errorf("%w: window decorations", ErrUnsupported)
	}
	err := control.SetDecorated(decorated, func(err error) { done(windowDecorationError(err)) })
	return windowDecorationError(err)
}

func windowDecorationError(err error) error {
	if errors.Is(err, platform.ErrWindowDecorationUnsupported) {
		return fmt.Errorf("%w: window decorations", ErrUnsupported)
	}
	return err
}
