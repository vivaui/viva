package ui_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// A terminal's automatic focus must not turn into a fresh request after a
// sibling composer explicitly takes the keyboard.
func TestAutoFocusDoesNotReclaimAnExplicitlyFocusedSibling(t *testing.T) {
	var terminal, composer ui.Field
	var composerID gfx.ID
	request := false
	d := openPage(t, func(c ui.C) {
		ui.TextField(c, &terminal, "Terminal", "").AutoFocus()
		ui.TextField(c, &composer, "Composer", "")
		if request {
			c.Canvas().RequestFocus(composerID)
			request = false
		}
	})
	if d.focused() != "Terminal" {
		t.Fatalf("initial focus = %q", d.focused())
	}
	composerID = d.hit("Composer").ID
	request = true
	d.repaint()
	d.repaint()
	if d.focused() != "Composer" {
		t.Fatalf("autofocus reclaimed sibling focus: %q", d.focused())
	}
	d.typeText("draft")
	if composer.Text() != "draft" || terminal.Text() != "" {
		t.Fatalf("typing reached wrong field: terminal=%q composer=%q", terminal.Text(), composer.Text())
	}
}

func BenchmarkAutoFocusSteadyPaint(b *testing.B) {
	var field ui.Field
	view := func(c ui.C) {
		ui.TextField(c, &field, "Terminal", "").AutoFocus()
	}
	page := ui.NewPage(ui.PageConfig{View: view})
	var canvas gfx.Canvas
	bounds := gfx.Rect{Width: 640, Height: 480}
	page.Draw(&canvas, bounds, view)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		canvas.Reset()
		page.Draw(&canvas, bounds, view)
	}
}
