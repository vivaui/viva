package ui_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// A Dialog given no Height is as tall as its header with the description, its body and its
// footer need: the same height an application gets by measuring the header with
// components.BeginDialog (FitBody) and adding the body's height to it, which is what it
// had to do before. Declaring the body with the height it has is enough.
func TestDialogWithoutAHeightFitsItsDescriptionAndBody(t *testing.T) {
	short := "Delete this host?"
	long := strings.Repeat("A description long enough that it wraps onto several lines in the dialog. ", 4)
	for _, tc := range []struct {
		name        string
		description string
		width, body float32
	}{
		{"no description", "", 420, 80},
		{"short description", short, 480, 124},
		{"wrapped description", long, 480, 124},
		{"wrapped, narrow", long, 360, 200},
		{"no body", long, 480, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var want float32
			dialogHeight := func(explicit bool) float32 {
				d := openPage(t, func(c ui.C) {
					spec := ui.DialogSpec{Title: "Title", Description: tc.description, Width: tc.width,
						Scope: gfx.Item(7), CloseID: gfx.Item(8)}
					if explicit {
						layout := components.BeginDialog(c.Canvas(), *c.Theme(), gfx.Rect{Width: c.Bounds().Width, Height: c.Bounds().Height},
							components.DialogOptions{ContentSize: gfx.Size{Width: tc.width}, FitBody: true}).Layout(tc.description)
						spec.Height = layout.Content.Height + tc.body
						want = spec.Height
					}
					dialog := ui.Dialog(c, spec)
					dialog.Body(func() {
						ui.Column(c).Height(tc.body).Hit(ui.ID(c), "body")
					})
					dialog.Footer(func() { ui.Button(c, "OK") })
				})
				// The surface runs from the top of the title to the bottom of the footer, less the
				// padding the surface keeps around them; the footer button is the last thing in it.
				var top, bottom float32 = 1e9, 0
				for _, h := range d.driver.Snapshot().Hits {
					if h.Label == "Title" || h.Label == "OK" || h.Label == "body" {
						top = min(top, h.Bounds.Y)
						bottom = max(bottom, h.Bounds.Y+h.Bounds.Height)
					}
				}
				return bottom - top
			}
			byHand, byFit := dialogHeight(true), dialogHeight(false)
			if fmt.Sprintf("%.2f", byHand) != fmt.Sprintf("%.2f", byFit) {
				t.Fatalf("with the height measured by hand (%v) the controls span %.2f, with none %.2f", want, byHand, byFit)
			}
		})
	}
}
