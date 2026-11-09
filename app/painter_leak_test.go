package app

import (
	"testing"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

// Showing a busy page and then an empty one must give back what the page held: the
// retained scene, the semantic tree, the hit regions, the renderer's rectangles.
// A page that is shown and removed over and over must not leave a little behind
// each time.
func TestAPageThatIsRemovedLeavesNothingRetained(t *testing.T) {
	var tree gfx.Tree
	page := tree.Feature("leak")
	rows := tree.Sub(page, tree.Name("rows"))
	theme := gfx.LightTheme()
	rowCount := 0
	var canvas gfx.Canvas
	painter := newTestPainter(t, &canvas, Config{Title: "leak", Width: 400, Height: 600, Tree: &tree,
		Paint: func(canvas *gfx.Canvas, size gfx.Size) {
			for i := 0; i < rowCount; i++ {
				components.BeginButton(canvas, theme, rows.At(i), gfx.Rect{X: 10, Y: float32(10 + i*40), Width: 200, Height: 32}).Default("Row")
				canvas.Label("text", gfx.Rect{X: 220, Y: float32(10 + i*40), Width: 100, Height: 20}, gfx.TextStyle{Size: 13})
			}
		}})
	settle := func(count int) {
		t.Helper()
		rowCount = count
		if _, err := painter.repaint(); err != nil {
			t.Fatal(err)
		}
	}

	settle(0)
	scenes, semantics, rectangles := painter.graph.Len(), painter.semantics.Len(), len(painter.renderer.Rectangles())
	for round := 0; round < 5; round++ {
		settle(12)
		if painter.graph.Len() <= scenes || painter.semantics.Len() <= semantics {
			t.Fatal("the busy page retained nothing; the test is not exercising what it claims")
		}
		settle(0)
		if got := painter.graph.Len(); got != scenes {
			t.Fatalf("round %d: the scene holds %d nodes after the page was removed, %d before it was shown", round, got, scenes)
		}
		if got := painter.semantics.Len(); got != semantics {
			t.Fatalf("round %d: the semantic tree holds %d nodes after the page was removed, %d before", round, got, semantics)
		}
		if got := len(painter.renderer.Rectangles()); got != rectangles {
			t.Fatalf("round %d: the renderer holds %d rectangles after the page was removed, %d before", round, got, rectangles)
		}
	}
}

func TestALargePageThenASmallOneReleasesSlots(t *testing.T) {
	var canvas gfx.Canvas
	rowCount := 0
	painter := newTestPainter(t, &canvas, Config{Title: "trim", Width: 400, Height: 600,
		Paint: func(canvas *gfx.Canvas, size gfx.Size) {
			for i := 0; i < rowCount; i++ {
				bounds := gfx.Rect{X: 10, Y: float32(i * 10), Width: 100, Height: 8}
				canvas.Fill(bounds, gfx.Color{Alpha: 1})
				canvas.Hit(gfx.Item(i+1), bounds)
			}
		}})
	settle := func(count int) {
		t.Helper()
		rowCount = count
		if _, err := painter.repaint(); err != nil {
			t.Fatal(err)
		}
	}
	settle(5000)
	if painter.graph.Slots() < 5000 || painter.semantics.Slots() < 5000 {
		t.Fatalf("the large page did not allocate its slots: graph=%d semantics=%d", painter.graph.Slots(), painter.semantics.Slots())
	}
	settle(1)
	if painter.graph.Slots() > 64 || painter.semantics.Slots() > 64 {
		t.Fatalf("the small page kept the large page's slots: graph=%d semantics=%d", painter.graph.Slots(), painter.semantics.Slots())
	}
}
