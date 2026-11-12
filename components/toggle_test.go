package components

import (
	"slices"
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestToggleActiveUsesAccentAndDeclaresHit(t *testing.T) {
	theme := gfx.LightTheme()
	var canvas gfx.Canvas
	BeginToggle(&canvas, theme, gfx.Item(41), gfx.Rect{Width: 96}, true).Default("Bold")
	if len(canvas.Rectangles()) != 1 || canvas.Rectangles()[0].Color != theme.Muted {
		t.Fatalf("active toggle rectangles = %+v", canvas.Rectangles())
	}
	if len(canvas.Hits()) != 1 || canvas.Hits()[0].ID != gfx.Item(41) || canvas.Hits()[0].Role != gfx.RoleButton || !canvas.Hits()[0].Checked || !canvas.Hits()[0].Selected {
		t.Fatalf("toggle hits = %+v", canvas.Hits())
	}
	if canvas.Texts()[0].Style.Color != theme.Foreground {
		t.Fatalf("active foreground = %+v", canvas.Texts()[0].Style.Color)
	}
}

func TestToggleOutlineDisabledStableAndAllocationFree(t *testing.T) {
	theme := gfx.DarkTheme()
	var canvas gfx.Canvas
	paint := func() {
		canvas.Reset()
		BeginToggle(&canvas, theme, gfx.Item(42), gfx.Rect{Width: 88}, false).Disabled().Outline("Italic")
	}
	paint()
	rectangles := slices.Clone(canvas.Rectangles())
	texts := slices.Clone(canvas.Texts())
	paint()
	if len(canvas.Hits()) != 1 || !canvas.Hits()[0].Disabled || canvas.Hits()[0].Focusable || len(canvas.Rectangles()) != 2 {
		t.Fatalf("disabled outline toggle = %+v hits, %d rectangles", canvas.Hits(), len(canvas.Rectangles()))
	}
	if !slices.Equal(rectangles, canvas.Rectangles()) || !slices.Equal(texts, canvas.Texts()) {
		t.Fatal("identical toggles produced different command streams")
	}
	if allocations := testing.AllocsPerRun(100, paint); allocations != 0 {
		t.Fatalf("toggle paint allocations = %v", allocations)
	}
}

func TestToggleSizeAndInvalidVariants(t *testing.T) {
	var small gfx.Canvas
	BeginToggle(&small, gfx.LightTheme(), gfx.Item(1), gfx.Rect{Width: 80}, false).Small().Default("S")
	if small.Rectangles()[0].Bounds.Height != 28 {
		t.Fatalf("small toggle height = %v", small.Rectangles()[0].Bounds.Height)
	}
	var invalid gfx.Canvas
	BeginToggle(&invalid, gfx.LightTheme(), gfx.Item(2), gfx.Rect{Width: 80}, false).Invalid().Default("Invalid")
	if len(invalid.Rectangles()) != 3 || invalid.Rectangles()[1].Color != gfx.LightTheme().Danger {
		t.Fatalf("invalid toggle rectangles = %+v", invalid.Rectangles())
	}
}

func TestToggleClickedReportsTheClick(t *testing.T) {
	var canvas gfx.Canvas
	canvas.BeginEventPass(gfx.Item(41), gfx.ClickInfo{})
	defer canvas.EndEventPass()
	if !BeginToggle(&canvas, gfx.LightTheme(), gfx.Item(41), gfx.Rect{Width: 96}, false).Default("Bold").Clicked() {
		t.Fatal("toggle did not report its own click")
	}
	if BeginToggle(&canvas, gfx.LightTheme(), gfx.Item(42), gfx.Rect{Width: 96}, false).Outline("Italic").Clicked() {
		t.Fatal("toggle reported a click on another control")
	}
}

func TestToggleSizeToFitWrapsTheLabelButStaysAsWideAsTall(t *testing.T) {
	theme := gfx.LightTheme()
	var canvas gfx.Canvas
	canvas.SetMeasurer(func(value string, size, weight float32) float32 { return float32(len(value)) * 8 })
	wide := BeginToggle(&canvas, theme, gfx.Item(1), gfx.Rect{}, false).SizeToFit("Underline")
	narrow := BeginToggle(&canvas, theme, gfx.Item(2), gfx.Rect{}, false).SizeToFit("B")
	if wide.BoundsWidth() != 9*8+20 || narrow.BoundsWidth() != 32 {
		t.Fatalf("fitted widths = %v and %v", wide.BoundsWidth(), narrow.BoundsWidth())
	}
}
