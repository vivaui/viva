package components

import (
	"strings"
	"testing"
	"time"

	"github.com/vivaui/viva/gfx"
)

func TestTooltipPositionsOnEverySideAndUsesLayer(t *testing.T) {
	viewport := gfx.Rect{Width: 600, Height: 400}
	anchor := gfx.Rect{X: 260, Y: 180, Width: 80, Height: 32}
	tests := []struct {
		side  TooltipSide
		check func(gfx.Rect) bool
	}{
		{side: TooltipTop, check: func(bounds gfx.Rect) bool { return bounds.Y+bounds.Height == anchor.Y-TooltipDefaultSideOffset }},
		{side: TooltipRight, check: func(bounds gfx.Rect) bool { return bounds.X == anchor.X+anchor.Width+TooltipDefaultSideOffset }},
		{side: TooltipBottom, check: func(bounds gfx.Rect) bool { return bounds.Y == anchor.Y+anchor.Height+TooltipDefaultSideOffset }},
		{side: TooltipLeft, check: func(bounds gfx.Rect) bool { return bounds.X+bounds.Width == anchor.X-TooltipDefaultSideOffset }},
	}
	for _, test := range tests {
		var canvas gfx.Canvas
		bounds := BeginTooltip(&canvas, gfx.DarkTheme(), viewport, anchor, TooltipOptions{Layer: 9, Side: test.side}).Paint("More information")
		if !test.check(bounds) {
			t.Fatalf("side %d bounds = %+v", test.side, bounds)
		}
		if len(canvas.Rectangles()) != 1 || len(canvas.Triangles()) != 1 || canvas.Texts()[0].Layer != 9 {
			t.Fatalf("tooltip commands = %+v / %+v / %+v", canvas.Rectangles(), canvas.Triangles(), canvas.Texts())
		}
		for _, triangle := range canvas.Triangles() {
			if triangle.Layer != 9 {
				t.Fatalf("tooltip triangle left layer 9: %+v", triangle)
			}
		}
		for _, rectangle := range canvas.Rectangles() {
			if rectangle.Layer != 9 {
				t.Fatalf("tooltip rectangle left layer 9: %+v", rectangle)
			}
		}
		if len(canvas.Hits()) != 0 {
			t.Fatalf("tooltip declared hits: %+v", canvas.Hits())
		}
	}
}

func TestTooltipExplicitSideOffsetOverridesTheDefault(t *testing.T) {
	var canvas gfx.Canvas
	anchor := gfx.Rect{X: 260, Y: 180, Width: 80, Height: 32}
	bounds := BeginTooltip(&canvas, gfx.DarkTheme(), gfx.Rect{Width: 600, Height: 400}, anchor, TooltipOptions{Layer: 1, Side: TooltipBottom, SideOffset: 2}).Paint("Hint")
	if bounds.Y != anchor.Y+anchor.Height+2 {
		t.Fatalf("bounds = %+v, want a 2px gap", bounds)
	}
}

func TestTooltipHonorsMaxWidthAndDisabledState(t *testing.T) {
	var canvas gfx.Canvas
	bounds := BeginTooltip(&canvas, gfx.LightTheme(), gfx.Rect{Width: 300, Height: 200}, gfx.Rect{X: 120, Y: 100, Width: 20, Height: 20}, TooltipOptions{Layer: 1, MaxWidth: 96}).Paint("A deliberately long tooltip label")
	if bounds.Width != 96 {
		t.Fatalf("tooltip width = %v, want 96", bounds.Width)
	}
	canvas.Reset()
	bounds = BeginTooltip(&canvas, gfx.LightTheme(), gfx.Rect{Width: 300, Height: 200}, gfx.Rect{X: 120, Y: 100, Width: 20, Height: 20}, TooltipOptions{Layer: 1}).Disabled().Paint("Hidden")
	if bounds != (gfx.Rect{}) || len(canvas.Rectangles()) != 0 || len(canvas.Texts()) != 0 {
		t.Fatalf("disabled tooltip painted: %+v", bounds)
	}
}

func TestTooltipStateAndDeadlinesAreAllocationFree(t *testing.T) {
	var state TooltipState
	if state.IsOpen() {
		t.Fatal("zero state is open")
	}
	state = state.Open()
	if !state.IsOpen() || state.Toggle().IsOpen() || state.Close().IsOpen() || state.Dismiss().IsOpen() || state.Escape().IsOpen() || state.OutsidePress().IsOpen() {
		t.Fatal("tooltip state transition failed")
	}
	now := time.Unix(100, 250)
	delays := TooltipDelays{Open: 700 * time.Millisecond, Close: 120 * time.Millisecond}
	if got := delays.OpenDeadline(now); !got.Equal(now.Add(700 * time.Millisecond)) {
		t.Fatalf("open deadline = %v", got)
	}
	if got := delays.CloseDeadline(now); !got.Equal(now.Add(120 * time.Millisecond)) {
		t.Fatalf("close deadline = %v", got)
	}
	if got := (TooltipDelays{Open: -time.Second}).OpenDeadline(now); !got.Equal(now) {
		t.Fatalf("negative-delay deadline = %v", got)
	}
	transition := func() {
		state = state.Open().Toggle().Open().Dismiss().Open().Escape().Open().OutsidePress().Open().Close()
		_ = delays.OpenDeadline(now)
		_ = delays.CloseDeadline(now)
	}
	if allocations := testing.AllocsPerRun(1000, transition); allocations != 0 {
		t.Fatalf("tooltip state/deadline allocations = %v", allocations)
	}
}

func TestTooltipWrapsALabelWiderThanItsMaximum(t *testing.T) {
	var canvas gfx.Canvas
	viewport, anchor := gfx.Rect{Width: 600, Height: 400}, gfx.Rect{X: 260, Y: 180, Width: 80, Height: 32}
	one := BeginTooltip(&canvas, gfx.DarkTheme(), viewport, anchor, TooltipOptions{Layer: 1, MaxWidth: 120}).Paint("Short")
	canvas.Reset()
	wrapped := BeginTooltip(&canvas, gfx.DarkTheme(), viewport, anchor, TooltipOptions{Layer: 1, MaxWidth: 120}).Paint("A label that is far wider than the tooltip may be")
	if wrapped.Width != 120 || wrapped.Height <= one.Height || len(canvas.Texts()) < 2 {
		t.Fatalf("wrapped tooltip %+v (%d lines) next to %+v", wrapped, len(canvas.Texts()), one)
	}
}

func TestTooltipKeepsAnUnbrokenLabelWholeWhenItWraps(t *testing.T) {
	var canvas gfx.Canvas
	label := strings.Repeat("long-file-name-part-", 12) + ".txt"
	BeginTooltip(&canvas, gfx.DarkTheme(), gfx.Rect{Width: 600, Height: 400}, gfx.Rect{X: 260, Y: 180, Width: 80, Height: 32}, TooltipOptions{}).Paint(label)
	var painted string
	for _, text := range canvas.Texts() {
		if strings.ContainsRune(text.Value, '…') {
			t.Fatalf("the tooltip dropped part of the name: %q", text.Value)
		}
		if text.Style.Size != gfx.DarkTheme().Metrics.FontSizeS || text.Style.Weight != gfx.WeightRegular {
			t.Fatalf("line is not measured with the tooltip's text style: %+v", text)
		}
		painted += text.Value
	}
	if painted != label || len(canvas.Texts()) < 2 {
		t.Fatalf("painted = %q, want all of %q in multiple lines", painted, label)
	}
}

func BenchmarkTooltipLongUnbrokenLabel(b *testing.B) {
	var canvas gfx.Canvas
	label := strings.Repeat("long-file-name-part-", 12) + ".txt"
	theme := gfx.DarkTheme()
	viewport := gfx.Rect{Width: 600, Height: 400}
	anchor := gfx.Rect{X: 260, Y: 180, Width: 80, Height: 32}
	b.ReportAllocs()
	for b.Loop() {
		canvas.Reset()
		BeginTooltip(&canvas, theme, viewport, anchor, TooltipOptions{}).Paint(label)
	}
}

func TestTooltipOpensOnTheOtherSideWhenThereIsNoRoomAndItsArrowFollows(t *testing.T) {
	var canvas gfx.Canvas
	viewport, anchor := gfx.Rect{Width: 600, Height: 400}, gfx.Rect{X: 260, Y: 8, Width: 80, Height: 32}
	bounds := BeginTooltip(&canvas, gfx.DarkTheme(), viewport, anchor, TooltipOptions{Layer: 1, Side: TooltipTop}).Paint("Hint")
	if bounds.Y < anchor.Y+anchor.Height {
		t.Fatalf("tooltip %+v overlaps its anchor %+v instead of going below it", bounds, anchor)
	}
	arrowAboveBody := false
	for _, triangle := range canvas.Triangles() {
		for _, point := range triangle.Points {
			if point.Y < bounds.Y && point.Y >= anchor.Y+anchor.Height {
				arrowAboveBody = true
			}
		}
	}
	if !arrowAboveBody {
		t.Fatal("the arrow was not painted on the side facing the anchor")
	}
}
