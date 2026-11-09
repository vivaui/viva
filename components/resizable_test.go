package components

import (
	"slices"
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestResizableReturnsPanelGeometryAndMapsCoordinate(t *testing.T) {
	theme := gfx.LightTheme()
	var canvas gfx.Canvas
	resizable := BeginResizable(&canvas, theme, gfx.Item(81), gfx.Rect{X: 10, Y: 20, Width: 301, Height: 180}, 0.4).ValueText("40%")
	layout := resizable.WithHandle().Paint("Resize panels")
	if layout.First.Width != 120 || layout.Handle.Width != 1 || layout.Hit.Width != 4 || layout.Second.Width != 180 {
		t.Fatalf("horizontal layout = %+v", layout)
	}
	if len(canvas.Rectangles()) != 2 || len(canvas.Hits()) != 1 {
		t.Fatalf("resizable commands = %d rectangles, %d hits", len(canvas.Rectangles()), len(canvas.Hits()))
	}
	if canvas.Hits()[0].Role != gfx.RoleSeparator || canvas.Hits()[0].Cursor != gfx.PointerCursorResizeHorizontal || canvas.Hits()[0].Value != "40%" {
		t.Fatalf("resizable semantics = %+v", canvas.Hits()[0])
	}
	if got := resizable.FractionAt(10 + 0.5 + 300*0.75); got != 0.75 {
		t.Fatalf("FractionAt(75%%) = %v", got)
	}
}

func TestResizableHandleIsOptional(t *testing.T) {
	var canvas gfx.Canvas
	BeginResizable(&canvas, gfx.LightTheme(), gfx.Item(83), gfx.Rect{Width: 207, Height: 120}, 0.5).Paint("Resize")
	if len(canvas.Rectangles()) != 1 {
		t.Fatalf("plain separator rectangles = %d, want one visual line", len(canvas.Rectangles()))
	}
}

func TestResizableVerticalDisabledStableAndAllocationFree(t *testing.T) {
	theme := gfx.DarkTheme()
	var canvas gfx.Canvas
	paint := func() {
		canvas.Reset()
		BeginResizable(&canvas, theme, gfx.Item(82), gfx.Rect{Width: 240, Height: 207}, 0.5).Vertical().WithHandle().Disabled().Paint("Resize rows")
	}
	paint()
	rectangles := slices.Clone(canvas.Rectangles())
	paint()
	if len(canvas.Hits()) != 1 || !canvas.Hits()[0].Disabled || canvas.Hits()[0].Focusable || canvas.Hits()[0].Cursor != gfx.PointerCursorResizeVertical {
		t.Fatalf("disabled resizable semantics = %+v", canvas.Hits())
	}
	if !slices.Equal(rectangles, canvas.Rectangles()) {
		t.Fatal("identical resizable controls produced different command streams")
	}
	if allocations := testing.AllocsPerRun(100, paint); allocations != 0 {
		t.Fatalf("resizable paint allocations = %v", allocations)
	}
}

// A divider that can be dragged says so: resting the pointer on it, or
// pressing it, draws a thicker accent line; nothing extra is drawn otherwise,
// when it is disabled, or for a handle without an identity.
func TestResizableHandleHighlightsWhenHoveredOrPressed(t *testing.T) {
	paint := func(hovered, pressed uint32, disabled bool) []gfx.Rectangle {
		var canvas gfx.Canvas
		canvas.SetInteraction(gfx.Item(int(hovered)), gfx.Item(int(pressed)), gfx.ID{}, false)
		resizable := BeginResizable(&canvas, gfx.DarkTheme(), gfx.Item(5), gfx.Rect{Width: 200, Height: 100}, 0.5)
		if disabled {
			resizable = resizable.Disabled()
		}
		resizable.Paint("Split")
		return canvas.Rectangles()
	}
	theme := gfx.DarkTheme()
	rest := paint(0, 0, false)
	hover := paint(5, 0, false)
	press := paint(5, 5, false)
	if len(hover) != len(rest)+1 || hover[len(hover)-1].Color != theme.Ring || hover[len(hover)-1].Bounds.Width != 3 {
		t.Fatalf("hovered handle = %+v (resting %d rectangles), want one 3px ring-colored line more", hover, len(rest))
	}
	if len(press) != len(rest)+1 || press[len(press)-1].Color != theme.Primary {
		t.Fatalf("pressed handle = %+v, want a primary-colored line", press)
	}
	if got := paint(5, 0, true); len(got) != len(rest) {
		t.Fatal("a disabled handle highlighted")
	}
	if got := paint(9, 0, false); len(got) != len(rest) {
		t.Fatal("hovering another control highlighted the handle")
	}
}

func TestResizablePixelMinimumsKeepBothPanelsUsable(t *testing.T) {
	var canvas gfx.Canvas
	bounds := gfx.Rect{Width: 801, Height: 100}
	split := BeginResizable(&canvas, gfx.LightTheme(), gfx.Item(1), bounds, 0.5).Limits(0, 1).LimitsPx(200, 300)
	if got := split.FractionAt(5); got*800 < 199.9 || got*800 > 200.1 {
		t.Fatalf("dragging to the far left gave a first panel of %v px, want the 200 px minimum", got*800)
	}
	if got := split.FractionAt(800); (1-got)*800 < 299.9 || (1-got)*800 > 300.1 {
		t.Fatalf("dragging to the far right left %v px for the second panel, want 300", (1-got)*800)
	}
	layout := BeginResizable(&canvas, gfx.LightTheme(), gfx.Item(1), bounds, 0).Limits(0, 1).LimitsPx(200, 300).Paint("Resize")
	if layout.First.Width < 199.9 {
		t.Fatalf("a stored fraction below the minimum painted a %v px panel", layout.First.Width)
	}
}

func TestResizableArrowKeysMoveByStepAndHomeEndJumpToLimits(t *testing.T) {
	var canvas gfx.Canvas
	split := BeginResizable(&canvas, gfx.LightTheme(), gfx.Item(1), gfx.Rect{Width: 501, Height: 100}, 0.5).Step(50).Limits(0.2, 0.8)
	right, ok := split.FractionForKey(gfx.KeyRight)
	left, _ := split.FractionForKey(gfx.KeyLeft)
	if !ok || right <= 0.5 || left >= 0.5 || right-0.5 < 0.09 || right-0.5 > 0.11 {
		t.Fatalf("Right -> %v, Left -> %v, want 50 px (0.1) either way from 0.5", right, left)
	}
	if _, ok := split.FractionForKey(gfx.KeyUp); ok {
		t.Fatal("a vertical arrow moved a horizontal split")
	}
	if home, _ := split.FractionForKey(gfx.KeyHome); home != 0.2 {
		t.Fatalf("Home -> %v, want the lower limit", home)
	}
	if end, _ := split.FractionForKey(gfx.KeyEnd); end != 0.8 {
		t.Fatalf("End -> %v, want the upper limit", end)
	}
	vertical := split.Vertical()
	if down, ok := vertical.FractionForKey(gfx.KeyDown); !ok || down <= 0.5 {
		t.Fatalf("Down on a vertical split -> %v, %v", down, ok)
	}
}

// Without an explicit Step an arrow moves 5% of the group, Enter collapses and
// restores, and F6 cycles handles.
func TestResizableDefaultStepCollapseAndHandleCycling(t *testing.T) {
	var canvas gfx.Canvas
	split := BeginResizable(&canvas, gfx.LightTheme(), gfx.Item(1), gfx.Rect{Width: 400, Height: 100}, 0.5).Limits(0.1, 0.9)
	if right, ok := split.FractionForKey(gfx.KeyRight); !ok || right < 0.54 || right > 0.56 {
		t.Fatalf("default Right = %v, want 5%%", right)
	}
	split = split.Collapsed(0.2)
	if collapsed, ok := split.CollapseFraction(0.5, 0.4, gfx.KeyEnter); !ok || collapsed != 0.2 {
		t.Fatalf("collapse = %v, %v", collapsed, ok)
	}
	if restored, ok := split.CollapseFraction(0.2, 0.4, gfx.KeyEnter); !ok || restored != 0.4 {
		t.Fatalf("restore = %v, %v", restored, ok)
	}
	handles := []gfx.ID{gfx.Item(1), gfx.Item(2), gfx.Item(3)}
	if next, ok := ResizableHandleForKey(gfx.KeyF6, handles, gfx.Item(1), false); !ok || next != gfx.Item(2) {
		t.Fatalf("F6 = %v, %v", next, ok)
	}
	if next, ok := ResizableHandleForKey(gfx.KeyF6, handles, gfx.Item(1), true); !ok || next != gfx.Item(3) {
		t.Fatalf("Shift+F6 = %v, %v", next, ok)
	}
}

// A double click returns the handle to its default fraction, constrained by the
// current limits.
func TestResizableDoubleClickResets(t *testing.T) {
	var canvas gfx.Canvas
	split := BeginResizable(&canvas, gfx.LightTheme(), gfx.Item(1), gfx.Rect{Width: 400, Height: 100}, 0.5).Limits(0.2, 0.8).Default(0.3)
	if fraction, ok := split.FractionForClick(1); ok || fraction != 0.5 {
		t.Fatalf("single click = (%v, %v)", fraction, ok)
	}
	if fraction, ok := split.FractionForClick(2); !ok || fraction != 0.3 {
		t.Fatalf("double click = (%v, %v)", fraction, ok)
	}
	limited := split.Default(0.95)
	if fraction, _ := limited.FractionForClick(2); fraction != 0.8 {
		t.Fatalf("constrained default = %v, want 0.8", fraction)
	}
}
