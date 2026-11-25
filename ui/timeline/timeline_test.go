package timeline_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/timeline"
)

var events = []timeline.Item{
	{Title: "Connected", Description: "accepted the key", Label: "09:41"},
	{Title: "Uploaded", Label: "09:42", Tone: timeline.Success},
	{Title: "Failed", Label: "09:44", Tone: timeline.Danger},
	{Title: "Rolled back", Label: "09:45", Tone: timeline.Muted},
}

func measure(value string, size, weight float32) float32 {
	return float32(len(value)) * size * 0.55
}

func draw(page *ui.Page, canvas *gfx.Canvas, view func(ui.C)) {
	canvas.BeginFrame()
	canvas.Reset()
	page.Draw(canvas, gfx.Rect{Width: 400, Height: 400}, view)
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

// dots counts the round dots filled with a color.
func dots(canvas *gfx.Canvas, color gfx.Color) int {
	count := 0
	for _, rectangle := range canvas.Rectangles() {
		if rectangle.Color == color && rectangle.Radius > 0 && rectangle.Bounds.Width == rectangle.Bounds.Height {
			count++
		}
	}
	return count
}

// Each event paints its words and a dot in the color of its tone.
func TestEventsPaintTheirWordsAndToneDots(t *testing.T) {
	theme := gfx.LightTheme()
	view := func(c ui.C) { timeline.View(c, "History", timeline.Spec{Items: events}) }
	_, canvas := open(t, view)
	for _, value := range []string{"Connected", "accepted the key", "09:41", "Uploaded", "Failed", "Rolled back"} {
		if _, ok := textAt(canvas, value); !ok {
			t.Fatalf("the timeline did not paint %q", value)
		}
	}
	for name, color := range map[string]gfx.Color{
		"default": theme.Primary, "success": theme.Success, "danger": theme.Danger, "muted": theme.MutedForeground,
	} {
		if dots(canvas, color) != 1 {
			t.Fatalf("%s dots = %d, want 1", name, dots(canvas, color))
		}
	}
}

// Events follow one another down the page, and Pending comes after the last.
func TestEventsRunDownAndPendingComesLast(t *testing.T) {
	view := func(c ui.C) { timeline.View(c, "History", timeline.Spec{Items: events, Pending: "Waiting"}) }
	_, canvas := open(t, view)
	previous := float32(-1)
	for _, title := range []string{"Connected", "Uploaded", "Failed", "Rolled back", "Waiting"} {
		text, ok := textAt(canvas, title)
		if !ok {
			t.Fatalf("the timeline did not paint %q", title)
		}
		if text.Bounds.Y <= previous {
			t.Fatalf("%q is at %v, not below the event before it (%v)", title, text.Bounds.Y, previous)
		}
		previous = text.Bounds.Y
	}
}

// With a label column the labels line up and sit beside their titles.
func TestLabelWidthLinesUpTheLabels(t *testing.T) {
	view := func(c ui.C) { timeline.View(c, "History", timeline.Spec{Items: events, LabelWidth: 60}) }
	_, canvas := open(t, view)
	first, _ := textAt(canvas, "09:41")
	other, _ := textAt(canvas, "09:44")
	if first.Bounds.X+first.Bounds.Width != other.Bounds.X+other.Bounds.Width {
		t.Fatalf("labels end at %v and %v, want one right edge", first.Bounds.X+first.Bounds.Width, other.Bounds.X+other.Bounds.Width)
	}
	title, _ := textAt(canvas, "Connected")
	if first.Bounds.X+first.Bounds.Width > title.Bounds.X {
		t.Fatal("a label overlaps the title beside it")
	}
}

func TestWarmPaintAllocatesNothing(t *testing.T) {
	for name, spec := range map[string]timeline.Spec{
		"plain":  {Items: events, Pending: "Waiting"},
		"labels": {Items: events, LabelWidth: 60},
	} {
		t.Run(name, func(t *testing.T) {
			view := func(c ui.C) { timeline.View(c, "History", spec) }
			page, canvas := open(t, view)
			if allocations := testing.AllocsPerRun(100, func() { draw(page, canvas, view) }); allocations != 0 {
				t.Fatalf("warm paint allocated %v times, want 0", allocations)
			}
		})
	}
}

func BenchmarkViewPaint(b *testing.B) {
	view := func(c ui.C) { timeline.View(c, "History", timeline.Spec{Items: events, Pending: "Waiting"}) }
	page, canvas := open(b, view)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		draw(page, canvas, view)
	}
}
