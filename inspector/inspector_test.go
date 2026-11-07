package inspector

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/ui"
)

// page declares two controls and a heading on canvas, as an application's paint
// function would, and returns the identity of the first.
func page(canvas *ui.Canvas) (save, cancel ui.ID) {
	tree := canvas.Tree()
	scope := tree.Feature("demo")
	save, cancel = tree.ID(scope, "save"), tree.ID(scope, "cancel")
	canvas.HitWithOptions(save, ui.Rect{X: 20, Y: 20, Width: 80, Height: 28}, "Save", ui.HitOptions{Role: ui.RoleButton})
	canvas.HitWithOptions(cancel, ui.Rect{X: 120, Y: 20, Width: 80, Height: 28}, "Cancel", ui.HitOptions{Role: ui.RoleButton, Disabled: true})
	canvas.Container(ui.Rect{X: 0, Y: 0, Width: 300, Height: 16}, ui.RoleHeading, "Title")
	return save, cancel
}

func painted(canvas *ui.Canvas) string {
	var all []string
	for _, text := range canvas.Texts() {
		all = append(all, text.Value)
	}
	return strings.Join(all, "\n")
}

func TestOffPaintsNothing(t *testing.T) {
	var canvas ui.Canvas
	save, _ := page(&canvas)
	canvas.SetInteraction(save, ui.ID{}, ui.ID{}, false)
	before := len(canvas.Rectangles())
	var inspector Inspector
	inspector.Paint(&canvas, ui.Rect{Width: 300, Height: 200})
	if len(canvas.Rectangles()) != before || len(canvas.Texts()) != 0 {
		t.Fatal("an inspector that is off painted something")
	}
}

func TestHoveredDescribesTheRegionUnderThePointer(t *testing.T) {
	var canvas ui.Canvas
	save, _ := page(&canvas)
	canvas.SetInteraction(save, ui.ID{}, save, true)
	inspector := Inspector{}
	inspector.Set(Hovered)
	inspector.Paint(&canvas, ui.Rect{Width: 300, Height: 200})
	text := painted(&canvas)
	for _, want := range []string{"button", "demo/save", `"Save"`, "80 × 28", "focused"} {
		if !strings.Contains(text, want) {
			t.Errorf("the description lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "demo/cancel") {
		t.Errorf("a region that is not hovered was described:\n%s", text)
	}
}

func TestOverlayDeclaresNoHitAndStaysOnScreen(t *testing.T) {
	var canvas ui.Canvas
	save, _ := page(&canvas)
	canvas.SetInteraction(save, ui.ID{}, ui.ID{}, false)
	hits := len(canvas.Hits())
	viewport := ui.Rect{Width: 150, Height: 120}
	inspector := Inspector{}
	inspector.Set(All)
	inspector.Paint(&canvas, viewport)
	if len(canvas.Hits()) != hits {
		t.Fatal("the overlay declared a hit and would take presses from the page")
	}
	for _, text := range canvas.Texts() {
		bounds := text.Bounds
		if bounds.X < viewport.X || bounds.X+bounds.Width > viewport.X+viewport.Width+0.5 ||
			bounds.Y < viewport.Y || bounds.Y+bounds.Height > viewport.Y+viewport.Height+0.5 {
			t.Errorf("%q lies at %+v, outside %+v", text.Value, bounds, viewport)
		}
	}
}

func TestAllOutlinesEveryRegionAndHoveredOnlyOne(t *testing.T) {
	count := func(mode Mode) int {
		var canvas ui.Canvas
		page(&canvas)
		before := len(canvas.Rectangles())
		inspector := Inspector{}
		inspector.Set(mode)
		inspector.Paint(&canvas, ui.Rect{Width: 300, Height: 200})
		return len(canvas.Rectangles()) - before
	}
	if hovered, all := count(Hovered), count(All); hovered != 0 || all == 0 {
		t.Fatalf("with nothing hovered, Hovered painted %d and All painted %d rectangles", hovered, all)
	}
}

func TestCycleVisitsEveryModeAndReturnsToOff(t *testing.T) {
	var inspector Inspector
	var seen []Mode
	for range 3 {
		inspector.Cycle()
		seen = append(seen, inspector.Mode())
	}
	if seen[0] != Hovered || seen[1] != All || seen[2] != Off || inspector.Enabled() {
		t.Fatalf("cycle visited %v", seen)
	}
}

func TestOffDoesNotAllocate(t *testing.T) {
	var canvas ui.Canvas
	page(&canvas)
	var inspector Inspector
	viewport := ui.Rect{Width: 300, Height: 200}
	if allocs := testing.AllocsPerRun(100, func() { inspector.Paint(&canvas, viewport) }); allocs != 0 {
		t.Fatalf("an inspector that is off allocates %v times", allocs)
	}
}

func BenchmarkPaintHovered(b *testing.B) {
	var canvas ui.Canvas
	save, _ := page(&canvas)
	canvas.SetInteraction(save, ui.ID{}, ui.ID{}, false)
	inspector := Inspector{}
	inspector.Set(Hovered)
	viewport := ui.Rect{Width: 300, Height: 200}
	b.ReportAllocs()
	for b.Loop() {
		canvas.Reset()
		page(&canvas)
		canvas.SetInteraction(save, ui.ID{}, ui.ID{}, false)
		inspector.Paint(&canvas, viewport)
	}
}
