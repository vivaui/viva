package componentdemo

import (
	"fmt"
	"os"
	"testing"

	"github.com/vivaui/viva/app"
)

// TestMain keeps the system out of the tests: the catalog's tests press every
// control of every example, and a driver answers the system's file picker and
// the like in place of a native dialog that nobody is there to close.
func TestMain(m *testing.M) {
	driver, err := app.NewDriver(app.Config{Width: 320, Height: 240}, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "componentdemo tests:", err)
		os.Exit(1)
	}
	code := m.Run()
	driver.Close()
	os.Exit(code)
}
