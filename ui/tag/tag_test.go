//go:build !js

package tag_test

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/tag"
)

// open runs a page of one tag and reports what the page read from it.
func open(t *testing.T, spec tag.Spec, closed, toggled *int) *headless.Session {
	t.Helper()
	page := ui.NewPage(ui.PageConfig{View: func(c ui.C) {
		ui.Column(c).Pad(16).Children(func() {
			result := tag.View(c, "production", spec)
			if result.Closed {
				*closed++
			}
			if result.Toggled {
				*toggled++
			}
		})
	}})
	session, err := headless.Open(ui.Configure(page, app.Config{Title: "Tag", Width: 320, Height: 120}), nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(session.Close)
	return session
}

// A press on the close button is reported as Closed once, and not as a toggle.
func TestCloseButtonIsReported(t *testing.T) {
	var closed, toggled int
	session := open(t, tag.Spec{Closable: true}, &closed, &toggled)
	if err := session.ClickLabel("Remove"); err != nil {
		t.Fatal(err)
	}
	if closed != 1 || toggled != 0 {
		t.Fatalf("closed %d, toggled %d after a press on the close button, want 1, 0", closed, toggled)
	}
}

// A checkable tag is a checkbox that reports each press, and shows its state to
// assistive technology.
func TestCheckableTagToggles(t *testing.T) {
	var closed, toggled int
	session := open(t, tag.Spec{Checkable: true, Checked: true}, &closed, &toggled)
	node, ok := session.AccessByName("production")
	if !ok || node.Role != gfx.RoleCheckbox || !node.Checked {
		t.Fatalf("access node = %+v, %v; want a checked checkbox", node, ok)
	}
	if err := session.ClickLabel("production"); err != nil {
		t.Fatal(err)
	}
	if toggled != 1 {
		t.Fatalf("toggled %d after one press, want 1", toggled)
	}
}

// A plain tag takes no input: no control to press.
func TestPlainTagOffersNothingToPress(t *testing.T) {
	var closed, toggled int
	session := open(t, tag.Spec{}, &closed, &toggled)
	for _, name := range []string{"production", "Remove"} {
		if _, err := session.Locate(name); err == nil {
			t.Fatalf("a plain tag offers %q as a control", name)
		}
	}
}

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
