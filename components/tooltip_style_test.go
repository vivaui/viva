package components

import (
	"math"
	"testing"

	"github.com/vivaui/viva/gfx"
)

// A theme that sets the tooltip surface paints that surface: the padding and text size
// change the box, so an application's own tip geometry comes from its theme alone.
func TestTooltipStyleSizesTheSurface(t *testing.T) {
	theme := gfx.LightTheme()
	theme.Tooltip = gfx.TooltipStyle{PadX: 6, PadY: 10, FontSize: 13, Radius: 4}
	var canvas gfx.Canvas
	viewport := gfx.Rect{Width: 400, Height: 300}
	anchor := gfx.Rect{X: 100, Y: 100, Width: 40, Height: 20}
	content := BeginTooltip(&canvas, theme, viewport, anchor, TooltipOptions{}).Paint("hint")
	if want := float32((13 + 5) + 2*10); content.Height != want {
		t.Fatalf("height = %v, want %v", content.Height, want)
	}
	if content.Width < 2*6 {
		t.Fatalf("width = %v, want at least the horizontal padding", content.Width)
	}
}

// A theme that sets nothing keeps Viva's own surface: the small text and the padded box.
func TestTooltipStyleDefaultsToTheSharedSurface(t *testing.T) {
	theme := gfx.LightTheme()
	var canvas gfx.Canvas
	viewport := gfx.Rect{Width: 400, Height: 300}
	anchor := gfx.Rect{X: 100, Y: 100, Width: 40, Height: 20}
	content := BeginTooltip(&canvas, theme, viewport, anchor, TooltipOptions{}).Paint("hint")
	if want := (theme.Metrics.FontSizeS + 5) + 2*6; content.Height != want {
		t.Fatalf("height = %v, want %v", content.Height, want)
	}
}

// A bordered tooltip keeps its border around the arrow. The body-colored arrow covers the
// ring where it leaves the body, so a border-colored arrow under the body must carry the
// border on along both slanted edges, or the outline opens where the arrow meets the body
// and a light arrow vanishes into a light window.
func TestTooltipBorderOutlinesTheArrow(t *testing.T) {
	theme := gfx.LightTheme()
	background, border := gfx.RGB24(0xFFFFFF), gfx.RGB24(0xD1D3D9)
	theme.Tooltip = gfx.TooltipStyle{Radius: 4, Background: background, Border: border, BorderSize: 1}
	viewport := gfx.Rect{Width: 600, Height: 400}
	anchor := gfx.Rect{X: 260, Y: 180, Width: 80, Height: 32}
	for _, side := range []TooltipSide{TooltipTop, TooltipRight, TooltipBottom, TooltipLeft} {
		var canvas gfx.Canvas
		body := BeginTooltip(&canvas, theme, viewport, anchor, TooltipOptions{Side: side}).Paint("More information")
		triangles := canvas.Triangles()
		if len(triangles) != 2 {
			t.Fatalf("side %d painted %d arrow triangles, want the border arrow and the body arrow", side, len(triangles))
		}
		outline, fill := triangles[0], triangles[1]
		if outline.Color != border || fill.Color != background {
			t.Fatalf("side %d arrows colored %v then %v, want the border %v under the body %v", side, outline.Color, fill.Color, border, background)
		}
		bodyOrder := uint32(0)
		for _, rectangle := range canvas.Rectangles() {
			if rectangle.Color == background {
				bodyOrder = rectangle.Order
			}
		}
		if !(outline.Order < bodyOrder && bodyOrder < fill.Order) {
			t.Fatalf("side %d paints border arrow %d, body %d, body arrow %d: the border arrow must sit under the body and the body arrow over it", side, outline.Order, bodyOrder, fill.Order)
		}
		if split := splitTooltipArrow(side, body, outline); split.inset != tooltipArrowInset {
			t.Fatalf("side %d border arrow base sits %v inside the body, want %v so the body hides it", side, split.inset, tooltipArrowInset)
		}
		// Every corner of the body arrow lies one border width inside the nearer slanted
		// edge of the border arrow: the two edges run parallel, a border width apart.
		edges := tooltipArrowSlantedEdges(splitTooltipArrow(side, body, outline), outline)
		for _, point := range fill.Points {
			gap := min(distanceToLine(point, edges[0]), distanceToLine(point, edges[1]))
			if diff := gap - theme.Tooltip.BorderSize; diff > 1e-3 || diff < -1e-3 {
				t.Fatalf("side %d body arrow corner %+v is %v from the border arrow's edge, want %v", side, point, gap, theme.Tooltip.BorderSize)
			}
		}
	}
}

// tooltipArrowSlantedEdges returns the two edges of an arrow triangle that run from its
// tip to its base.
func tooltipArrowSlantedEdges(split tooltipArrowSplit, triangle gfx.Triangle) [2][2]gfx.Point {
	var edges [2][2]gfx.Point
	next := 0
	for _, point := range triangle.Points {
		if point != split.tip {
			edges[next] = [2]gfx.Point{split.tip, point}
			next++
		}
	}
	return edges
}

func distanceToLine(point gfx.Point, line [2]gfx.Point) float32 {
	dx, dy := float64(line[1].X-line[0].X), float64(line[1].Y-line[0].Y)
	cross := dx*float64(point.Y-line[0].Y) - dy*float64(point.X-line[0].X)
	return float32(math.Abs(cross) / math.Hypot(dx, dy))
}
