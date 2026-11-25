package descriptions_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/descriptions"
)

var items = []descriptions.Item{
	{Label: "Host", Value: "build-01"},
	{Label: "Port", Value: "22"},
	{Label: "User", Value: "deploy"},
	{Label: "Key", Value: "ed25519", Span: 2},
	{Label: "Jump", Value: "bastion"},
	{Label: "Notes", Value: "Nightly agent", Span: 3},
}

func measure(value string, size, weight float32) float32 {
	return float32(len(value)) * size * 0.55
}

func draw(page *ui.Page, canvas *gfx.Canvas, view func(ui.C)) {
	canvas.BeginFrame()
	canvas.Reset()
	page.Draw(canvas, gfx.Rect{Width: 600, Height: 400}, view)
	canvas.EndFrame()
}

func open(t testing.TB, view func(ui.C)) (*ui.Page, *gfx.Canvas) {
	t.Helper()
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: view, Theme: &theme})
	canvas := &gfx.Canvas{}
	canvas.SetMeasurer(measure)
	canvas.SetReduceMotion(true)
	draw(page, canvas, view)
	return page, canvas
}

func textAt(canvas *gfx.Canvas, value string) (gfx.Text, bool) {
	for _, text := range canvas.Texts() {
		if text.Value == value {
			return text, true
		}
	}
	return gfx.Text{}, false
}

// Every label and value is painted, the items fill the lines in order and a
// line starts where the columns run out.
func TestItemsFillLinesInOrder(t *testing.T) {
	view := func(c ui.C) {
		descriptions.View(c, "Details", descriptions.Spec{Title: "Details", Items: items}).FillWidth()
	}
	_, canvas := open(t, view)
	for _, value := range []string{"Details", "Host", "build-01", "Port", "User", "Key", "ed25519", "Jump", "Notes", "Nightly agent"} {
		if _, ok := textAt(canvas, value); !ok {
			t.Fatalf("the list did not paint %q", value)
		}
	}
	host, _ := textAt(canvas, "Host")
	user, _ := textAt(canvas, "User")
	key, _ := textAt(canvas, "Key")
	notes, _ := textAt(canvas, "Notes")
	if host.Bounds.Y != user.Bounds.Y {
		t.Fatalf("Host and User are on different lines (%v, %v), want the first line of three columns", host.Bounds.Y, user.Bounds.Y)
	}
	if key.Bounds.Y <= host.Bounds.Y || notes.Bounds.Y <= key.Bounds.Y {
		t.Fatalf("the lines are not in order: first %v, second %v, third %v", host.Bounds.Y, key.Bounds.Y, notes.Bounds.Y)
	}
}

// The columns line up: what sits in the same column of two lines starts at the
// same place, whatever the number of cells in either line.
func TestColumnsLineUpAcrossLines(t *testing.T) {
	for name, spec := range map[string]descriptions.Spec{
		"plain":    {Items: items},
		"bordered": {Items: items, Bordered: true},
		"vertical": {Items: items, Vertical: true},
	} {
		t.Run(name, func(t *testing.T) {
			view := func(c ui.C) { descriptions.View(c, "Details", spec).FillWidth() }
			_, canvas := open(t, view)
			user, _ := textAt(canvas, "User")
			jump, _ := textAt(canvas, "Jump")
			// User is the third column of the first line; Jump is the third column
			// of the second, after an item that spans two.
			if user.Bounds.X != jump.Bounds.X {
				t.Fatalf("User starts at %v and Jump at %v, want one third column", user.Bounds.X, jump.Bounds.X)
			}
		})
	}
}

// A value the page declares itself takes the place of the text.
func TestContentReplacesTheValue(t *testing.T) {
	called := 0
	custom := []descriptions.Item{{Label: "State", Value: "ignored", Content: func(c ui.C) {
		called++
		ui.Text(c, "Online")
	}}}
	view := func(c ui.C) { descriptions.View(c, "Details", descriptions.Spec{Items: custom}).FillWidth() }
	_, canvas := open(t, view)
	if called == 0 {
		t.Fatal("Content was not called")
	}
	if _, ok := textAt(canvas, "Online"); !ok {
		t.Fatal("the content was not painted")
	}
	if _, ok := textAt(canvas, "ignored"); ok {
		t.Fatal("the value was painted beside its content")
	}
}

func TestWarmPaintAllocatesNothing(t *testing.T) {
	for name, spec := range map[string]descriptions.Spec{
		"plain":    {Title: "Details", Items: items},
		"bordered": {Items: items, Bordered: true, Columns: 2, LabelWidth: 80},
		"vertical": {Items: items, Vertical: true, Size: descriptions.Small},
	} {
		t.Run(name, func(t *testing.T) {
			view := func(c ui.C) { descriptions.View(c, "Details", spec).FillWidth() }
			page, canvas := open(t, view)
			if allocations := testing.AllocsPerRun(100, func() { draw(page, canvas, view) }); allocations != 0 {
				t.Fatalf("warm paint allocated %v times, want 0", allocations)
			}
		})
	}
}

func BenchmarkViewPaint(b *testing.B) {
	view := func(c ui.C) {
		descriptions.View(c, "Details", descriptions.Spec{Items: items, Bordered: true}).FillWidth()
	}
	page, canvas := open(b, view)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		draw(page, canvas, view)
	}
}
