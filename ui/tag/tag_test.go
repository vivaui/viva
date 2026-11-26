package tag_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/tag"
)

// Each status paints its words in its color, and a checked tag takes the primary.
func TestStatusAndCheckColorTheLabel(t *testing.T) {
	theme := gfx.LightTheme()
	for name, test := range map[string]struct {
		spec tag.Spec
		want gfx.Color
	}{
		"success": {tag.Spec{Status: ui.StatusSuccess}, theme.Success},
		"warning": {tag.Spec{Status: ui.StatusWarning}, theme.Warning},
		"danger":  {tag.Spec{Status: ui.StatusDanger}, theme.Danger},
		"checked": {tag.Spec{Checkable: true, Checked: true}, theme.PrimaryForeground},
	} {
		view := func(c ui.C) { tag.View(c, "label", test.spec) }
		page := ui.NewPage(ui.PageConfig{View: view, Theme: &theme})
		canvas := &gfx.Canvas{}
		canvas.BeginFrame()
		page.Draw(canvas, gfx.Rect{Width: 200, Height: 60}, view)
		canvas.EndFrame()
		found := false
		for _, text := range canvas.Texts() {
			if text.Value == "label" {
				found = true
				if text.Style.Color != test.want {
					t.Fatalf("%s: label color = %+v, want %+v", name, text.Style.Color, test.want)
				}
			}
		}
		if !found {
			t.Fatalf("%s: the tag did not paint its label", name)
		}
	}
}

func TestWarmPaintAllocatesNothing(t *testing.T) {
	for name, spec := range map[string]tag.Spec{
		"plain":    {},
		"closable": {Closable: true, Status: ui.StatusWarning, Size: tag.Small},
		"checkbox": {Checkable: true, Checked: true, Outlined: true, Size: tag.Large},
	} {
		t.Run(name, func(t *testing.T) {
			view := func(c ui.C) { tag.View(c, "production", spec) }
			theme := gfx.LightTheme()
			page := ui.NewPage(ui.PageConfig{View: view, Theme: &theme})
			canvas := &gfx.Canvas{}
			canvas.SetReduceMotion(true)
			draw := func() {
				canvas.BeginFrame()
				canvas.Reset()
				page.Draw(canvas, gfx.Rect{Width: 320, Height: 120}, view)
				canvas.EndFrame()
			}
			draw()
			if allocations := testing.AllocsPerRun(100, draw); allocations != 0 {
				t.Fatalf("warm paint allocated %v times, want 0", allocations)
			}
		})
	}
}

func BenchmarkViewPaint(b *testing.B) {
	view := func(c ui.C) { tag.View(c, "production", tag.Spec{Closable: true, Status: ui.StatusSuccess}) }
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: view, Theme: &theme})
	canvas := &gfx.Canvas{}
	canvas.SetReduceMotion(true)
	b.ReportAllocs()
	for range b.N {
		canvas.BeginFrame()
		canvas.Reset()
		page.Draw(canvas, gfx.Rect{Width: 320, Height: 120}, view)
		canvas.EndFrame()
	}
}
