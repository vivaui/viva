package components

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// tooltipArrowSplit describes one arrow triangle in axis-neutral terms: the
// midpoint of its base along the body edge, the width of that base, the tip
// pointing at the trigger, and how far the tip reaches and the base sits inside
// the body.
type tooltipArrowSplit struct {
	center float32
	width  float32
	tip    gfx.Point
	reach  float32 // how far the tip reaches out of the body toward the trigger
	inset  float32 // how far the base sits inside the body
}

// splitTooltipArrow splits the arrow triangle by its axis: the two points that
// share a coordinate across the body edge are its base, the third is the tip.
func splitTooltipArrow(side TooltipSide, body gfx.Rect, triangle gfx.Triangle) tooltipArrowSplit {
	horizontal := side == TooltipTop || side == TooltipBottom
	across := func(point gfx.Point) float32 {
		if horizontal {
			return point.Y
		}
		return point.X
	}
	along := func(point gfx.Point) float32 {
		if horizontal {
			return point.X
		}
		return point.Y
	}
	points := triangle.Points
	var base [2]gfx.Point
	var tip gfx.Point
	switch {
	case across(points[0]) == across(points[1]):
		base, tip = [2]gfx.Point{points[0], points[1]}, points[2]
	case across(points[1]) == across(points[2]):
		base, tip = [2]gfx.Point{points[1], points[2]}, points[0]
	default:
		base, tip = [2]gfx.Point{points[2], points[0]}, points[1]
	}
	split := tooltipArrowSplit{
		center: (along(base[0]) + along(base[1])) / 2,
		width:  max(along(base[1])-along(base[0]), along(base[0])-along(base[1])),
		tip:    tip,
	}
	switch side {
	case TooltipTop:
		split.reach = tip.Y - (body.Y + body.Height)
		split.inset = (body.Y + body.Height) - base[0].Y
	case TooltipBottom:
		split.reach = body.Y - tip.Y
		split.inset = base[0].Y - body.Y
	case TooltipRight:
		split.reach = body.X - tip.X
		split.inset = base[0].X - body.X
	default:
		split.reach = tip.X - (body.X + body.Width)
		split.inset = (body.X + body.Width) - base[0].X
	}
	return split
}

func TestTooltipArrowIsOneFeatheredTriangleOnEverySidePointingAtTheTrigger(t *testing.T) {
	viewport := gfx.Rect{Width: 600, Height: 400}
	anchor := gfx.Rect{X: 260, Y: 180, Width: 80, Height: 32}
	anchorCenterX, anchorCenterY := anchor.X+anchor.Width/2, anchor.Y+anchor.Height/2
	theme := gfx.DarkTheme()
	for _, side := range []TooltipSide{TooltipTop, TooltipRight, TooltipBottom, TooltipLeft} {
		var canvas gfx.Canvas
		body := BeginTooltip(&canvas, theme, viewport, anchor, TooltipOptions{Side: side}).Paint("More information")
		triangles := canvas.Triangles()
		if len(triangles) != 1 {
			t.Fatalf("side %d painted %d arrow triangles, want one", side, len(triangles))
		}
		triangle := triangles[0]
		if triangle.Color != theme.Foreground || triangle.EdgeMask != tooltipArrowEdges {
			t.Fatalf("side %d arrow = %+v, want a body-colored triangle with every edge feathered", side, triangle)
		}
		split := splitTooltipArrow(side, body, triangle)
		wantCenter := anchorCenterX
		if side == TooltipLeft || side == TooltipRight {
			wantCenter = anchorCenterY
		}
		if split.center != wantCenter {
			t.Fatalf("side %d arrow centered at %v, want the trigger center %v", side, split.center, wantCenter)
		}
		if split.width != tooltipArrowBase {
			t.Fatalf("side %d base width = %v, want %v", side, split.width, tooltipArrowBase)
		}
		if split.reach != tooltipArrowDepth {
			t.Fatalf("side %d tip reaches %v out of the body %+v, want %v", side, split.reach, body, tooltipArrowDepth)
		}
		if split.inset != tooltipArrowInset {
			t.Fatalf("side %d base sits %v inside the body %+v, want %v: the feathered base overlaps the body so no seam opens", side, split.inset, body, tooltipArrowInset)
		}
	}
}

func TestTooltipArrowStaysOnTheFlatPartOfAClampedBody(t *testing.T) {
	theme := gfx.LightTheme()
	viewport := gfx.Rect{Width: 300, Height: 200}
	for _, test := range []struct {
		name   string
		anchor gfx.Rect
	}{
		{name: "trigger at the left viewport edge", anchor: gfx.Rect{X: 0, Y: 100, Width: 16, Height: 24}},
		{name: "trigger at the right viewport edge", anchor: gfx.Rect{X: 284, Y: 100, Width: 16, Height: 24}},
	} {
		var canvas gfx.Canvas
		body := BeginTooltip(&canvas, theme, viewport, test.anchor, TooltipOptions{Side: TooltipBottom}).Paint("A longer label here")
		margin := theme.Metrics.RadiusControl + tooltipArrowBase/2
		split := splitTooltipArrow(TooltipBottom, body, canvas.Triangles()[0])
		if split.center < body.X+margin || split.center > body.X+body.Width-margin {
			t.Fatalf("%s: arrow center %v leaves the flat span [%v, %v] of body %+v", test.name, split.center, body.X+margin, body.X+body.Width-margin, body)
		}
	}
}

func TestTooltipArrowCenterFallsBackToBodyCenterWhenTooShort(t *testing.T) {
	if got := tooltipArrowCenter(500, 10, 20, 6); got != 20 {
		t.Fatalf("short body arrow center = %v, want its midpoint 20", got)
	}
	if got := tooltipArrowCenter(50, 10, 100, 6); got != 50 {
		t.Fatalf("unclamped arrow center = %v, want 50", got)
	}
}
