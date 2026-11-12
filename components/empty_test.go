package components

import (
	"slices"
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestEmptyIconMediaAndContentSlot(t *testing.T) {
	theme := gfx.LightTheme()
	var canvas gfx.Canvas
	bounds := gfx.Rect{X: 10, Y: 20, Width: 280, Height: 260}
	content := Empty(&canvas, theme, bounds, EmptyData{
		Media: EmptyMediaIcon, Icon: checkIcon, Title: "No results", Description: "Try a different query.", ContentHeight: 32,
	})
	if len(canvas.Rectangles()) != 1 || canvas.Rectangles()[0].Color != theme.Muted || len(canvas.Triangles()) == 0 || len(canvas.Texts()) != 2 {
		t.Fatalf("empty state commands: %d rectangles, %d texts, %d triangles", len(canvas.Rectangles()), len(canvas.Texts()), len(canvas.Triangles()))
	}
	if description := canvas.Texts()[1]; description.Style.Color != theme.MutedForeground || content.Y < description.Bounds.Y+description.Bounds.Height {
		t.Fatalf("content slot %+v is not below the description %+v", content, description.Bounds)
	}
	if content.Height != 32 || content.X < bounds.X || content.X+content.Width > bounds.X+bounds.Width ||
		content.Y+content.Height > bounds.Y+bounds.Height {
		t.Fatalf("content slot %+v leaves bounds %+v", content, bounds)
	}
}

func TestEmptyWithoutIconHasNoMediaAndWrapsDescription(t *testing.T) {
	var canvas gfx.Canvas
	slot := Empty(&canvas, gfx.LightTheme(), gfx.Rect{Width: 240, Height: 240}, EmptyData{
		Title: "No files", Description: "Files you upload or sync from the remote host will be listed here, one after another.",
	})
	if len(canvas.Rectangles()) != 0 || len(canvas.Triangles()) != 0 || slot != (gfx.Rect{}) {
		t.Fatalf("empty without icon painted media or a slot: %d rectangles, %+v", len(canvas.Rectangles()), slot)
	}
	if len(canvas.Texts()) < 3 {
		t.Fatalf("description did not wrap: %d texts", len(canvas.Texts()))
	}
}

func TestEmptyRepeatedPaintIsStableAndAllocationFree(t *testing.T) {
	var canvas gfx.Canvas
	paint := func() {
		canvas.Reset()
		Empty(&canvas, gfx.DarkTheme(), gfx.Rect{Width: 260, Height: 140}, EmptyData{Icon: checkIcon, Title: "Nothing here", Description: "New items will appear here."})
	}
	paint()
	rectangles := slices.Clone(canvas.Rectangles())
	texts := slices.Clone(canvas.Texts())
	paint()
	if !slices.Equal(rectangles, canvas.Rectangles()) || !slices.Equal(texts, canvas.Texts()) {
		t.Fatal("identical empty states produced different command streams")
	}
	if allocations := testing.AllocsPerRun(100, paint); allocations != 0 {
		t.Fatalf("empty paint allocations = %v, want 0 after warm-up", allocations)
	}
}
