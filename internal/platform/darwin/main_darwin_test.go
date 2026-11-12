//go:build darwin

package darwin

import (
	"os"
	"testing"

	"github.com/vivaui/viva/internal/platform/darwin/mainpump"
)

// The tests reach functions that hop to the main thread; serve that queue. They compare
// Viva's labels against AppKit's, so they thicken the text as AppKit draws it.
func TestMain(m *testing.M) {
	SharedTextEngine().SetFontThickening(true)
	os.Exit(mainpump.Run(m))
}
