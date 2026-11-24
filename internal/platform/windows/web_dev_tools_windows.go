package windows

import (
	"fmt"

	"github.com/vivaui/viva/internal/platform"
)

// viewOpenDevToolsWindow is ICoreWebView2::OpenDevToolsWindow, after RemoveHostObjectFromScript 50
// in the interface's IDL order.
const viewOpenDevToolsWindow = 51

// ShowDevTools opens the WebView2 DevTools window for the page, once the page exists. WebView2
// leaves AreDevToolsEnabled on unless an application turns it off, which Viva does not.
func (page *windowsWeb) ShowDevTools() error {
	if page.closed {
		return fmt.Errorf("windows: the web view with id %d is closed", page.config.ID)
	}
	page.do(func() { comCall0(page.view, viewOpenDevToolsWindow) })
	return nil
}

var _ platform.WebDevTools = (*windowsWeb)(nil)
