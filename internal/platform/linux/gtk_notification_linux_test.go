//go:build linux && vivagtk

package linux

import (
	"fmt"
	"os"
	"testing"

	"github.com/vivaui/viva/internal/platform"
)

// Run against a private display. The hints are GTK's request, observable even
// on Wayland, where the compositor decides whether to grant it.
func TestGTKNotificationFocusHints(t *testing.T) {
	if os.Getenv("VIVA_TEST_GTK_NOTIFICATION") != "1" {
		t.Skip("requires a private GTK display")
	}
	err := RunGTKApplication(func(app platform.Application) error {
		popup, err := app.OpenWindow(platform.Config{Width: 160, Height: 80, WindowMode: platform.WindowNotification})
		if err != nil {
			return err
		}
		w := popup.(*gtkWindow)
		for _, property := range []string{"accept-focus", "focus-on-map"} {
			if w.app.boolProperty(w.toplevel, property) != 0 {
				return fmt.Errorf("GTK notification requested %s", property)
			}
		}
		app.Quit()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
