package components

import (
	"slices"
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestCollapsibleOpenReturnsContentAndKeepsGlyphCommands(t *testing.T) {
	theme := gfx.LightTheme()
	var canvas gfx.Canvas
	content := BeginCollapsible(&canvas, theme, gfx.Item(7), gfx.Rect{X: 4, Y: 6, Width: 260}).Paint("Details", true, 56)
	wantY := float32(6) + theme.Metrics.ControlHeight
	if content.Y != wantY || content.Height != 56 {
		t.Fatalf("content bounds = %+v", content)
	}
	if len(canvas.Rectangles()) != 0 {
		t.Fatalf("unstyled collapsible painted %d rectangles", len(canvas.Rectangles()))
	}
	if len(canvas.Hits()) != 1 || canvas.Hits()[0].Label != "Details" || canvas.Hits()[0].Role != gfx.RoleButton || !canvas.Hits()[0].Expanded {
		t.Fatalf("collapsible hits = %+v", canvas.Hits())
	}
}

func TestCollapsibleDisabledStableAndAllocationFree(t *testing.T) {
	var canvas gfx.Canvas
	theme := gfx.DarkTheme()
	paint := func() {
		canvas.Reset()
		BeginCollapsible(&canvas, theme, gfx.Item(5), gfx.Rect{Width: 240}).Disabled().Paint("Advanced", false, 72)
	}
	paint()
	rectangles := slices.Clone(canvas.Rectangles())
	texts := slices.Clone(canvas.Texts())
	paint()
	if len(canvas.Hits()) != 1 || !canvas.Hits()[0].Disabled || !canvas.Hits()[0].Focusable || !canvas.Hits()[0].FocusableDisabled {
		t.Fatalf("disabled collapsible semantics = %+v", canvas.Hits())
	}
	if !slices.Equal(rectangles, canvas.Rectangles()) || !slices.Equal(texts, canvas.Texts()) {
		t.Fatal("identical collapsibles produced different command streams")
	}
	if allocations := testing.AllocsPerRun(100, paint); allocations != 0 {
		t.Fatalf("collapsible paint allocations = %v", allocations)
	}
}

func TestCollapsibleBareDeclaresTheTriggerAndPaintsNothing(t *testing.T) {
	var canvas gfx.Canvas
	header, content := BeginCollapsible(&canvas, gfx.LightTheme(), gfx.Item(5), gfx.Rect{Width: 200}).Bare("Details", true, 60)
	if len(canvas.Texts()) != 0 || len(canvas.Rectangles()) != 0 {
		t.Fatalf("bare trigger painted %d texts, %d rectangles", len(canvas.Texts()), len(canvas.Rectangles()))
	}
	hits := canvas.Hits()
	if len(hits) != 1 || hits[0].ID != gfx.Item(5) || !hits[0].Expanded || hits[0].Bounds != header {
		t.Fatalf("bare trigger hits = %+v for header %+v", hits, header)
	}
	if content.Y != header.Y+header.Height || content.Height != 60 {
		t.Fatalf("content %+v does not follow header %+v", content, header)
	}
}
