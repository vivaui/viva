package app

import (
	"errors"
	"fmt"

	"github.com/vivaui/viva/internal/platform"
)

const maximumProgrammaticWindowSize = 65535

// SetSize asks the window manager to make this window's content area width by
// height logical pixels. The request is asynchronous: OnResize reports the size
// actually granted. The window manager may adjust or ignore a request, for
// example in fullscreen. It is safe to call from any goroutine.
//
// Unsupported hosts return an error wrapping ErrUnsupported.
func (window *Window) SetSize(width, height int) error {
	if window == nil || window.closed.Load() {
		return fmt.Errorf("app: window is closed")
	}
	if width < 1 || height < 1 || width > maximumProgrammaticWindowSize || height > maximumProgrammaticWindowSize {
		return fmt.Errorf("app: invalid window size %dx%d", width, height)
	}
	sizer, ok := window.control.(platform.WindowSizer)
	if !ok {
		return fmt.Errorf("%w: programmatic window size", ErrUnsupported)
	}
	if err := sizer.SetSize(width, height); err != nil {
		if errors.Is(err, platform.ErrWindowSizeUnsupported) {
			return fmt.Errorf("%w: programmatic window size", ErrUnsupported)
		}
		return err
	}
	return nil
}
