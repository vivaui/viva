//go:build dev || vivadev

package ui_test

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/ui"
)

// In a dev build a Key given to an element without identity stops the page and
// says where the Key belongs.
func TestAKeyGivenToATextIsReported(t *testing.T) {
	defer func() {
		message, _ := recover().(string)
		if !strings.Contains(message, `Key "row"`) || !strings.Contains(message, "Text") {
			t.Fatalf("recovered %q, want the Key and the Text named", message)
		}
	}()
	openPage(t, func(c ui.C) {
		ui.Text(c.Key("row"), "row")
		ui.Row(c)
	})
	t.Fatal("a Key on a Text was not reported")
}
