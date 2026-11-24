package app

import (
	"errors"
	"fmt"

	"github.com/vivaui/viva/internal/platform"
)

// ShowDevTools opens the system engine's developer tools for this page: Web Inspector on macOS and
// Linux, the DevTools window on Windows. Developer tools are off until a page asks: nothing changes
// for a page that never calls it. It returns an error wrapping ErrUnsupported where the platform
// cannot open them. On macOS the page is still made inspectable then, so the user can reach it from
// the page's context menu or Safari's Develop menu. Call it on the event-loop thread.
func (view *WebView) ShowDevTools() error {
	tools, ok := view.native.(platform.WebDevTools)
	if !ok {
		return fmt.Errorf("%w: web developer tools", ErrUnsupported)
	}
	if err := tools.ShowDevTools(); err != nil {
		if errors.Is(err, platform.ErrWebViewUnsupported) {
			return fmt.Errorf("%w: %w", ErrUnsupported, err)
		}
		return err
	}
	return nil
}
