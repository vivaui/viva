//go:build linux && vivagtk

package linux

import (
	"fmt"

	"github.com/vivaui/viva/internal/platform"
)

// ShowDevTools turns on the page's developer extras and opens Web Inspector for it. Developer extras
// stay off until the application asks, so a page that never does has no Inspect Element entry.
func (v *gtkWebView) ShowDevTools() error {
	if v.closed {
		return fmt.Errorf("linux: the web view with id %d is closed", v.id)
	}
	if v.k.setDeveloperExtras == nil || v.k.viewInspector == nil || v.k.inspectorShow == nil {
		return fmt.Errorf("%w: WebKitGTK lacks Web Inspector", platform.ErrWebViewUnsupported)
	}
	v.k.setDeveloperExtras(v.k.viewSettings(v.widget), 1)
	inspector := v.k.viewInspector(v.widget)
	if inspector == 0 {
		return fmt.Errorf("%w: WebKitGTK gave the page no inspector", platform.ErrWebViewUnsupported)
	}
	v.k.inspectorShow(inspector)
	return nil
}

var _ platform.WebDevTools = (*gtkWebView)(nil)
