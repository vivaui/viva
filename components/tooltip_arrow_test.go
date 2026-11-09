package components

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// tooltipArrowMetrics describes one arrow step in axis-neutral terms: how far a
// rectangle reaches out of the body toward the trigger, how wide it is across
// the edge, and where it is centered along the edge.
func tooltipArrowMetrics(side TooltipSide, body, step gfx.Rect) (reach, width, center float32) {
	switch side {
	case TooltipTop:
		return step.Y + step.Height - (body.Y + body.Height), step.Width, step.X + step.Width/2
	case TooltipBottom:
		return body.Y - step.Y, step.Width, step.X + step.Width/2
	case TooltipRight:
		return body.X - step.X, step.Height, step.Y + step.Height/2
	default:
		return step.X + step.Width - (body.X + body.Width), step.Height, step.Y + step.Height/2
	}
}

func TestTooltipArrowIsATriangleOnEverySidePointingAtTheTrigger(t *testing.T) {
	viewport := gfx.Rect{Width: 600, Height: 400}
	anchor := gfx.Rect{X: 260, Y: 180, Width: 80, Height: 32}
	anchorCenterX, anchorCenterY := anchor.X+anchor.Width/2, anchor.Y+anchor.Height/2
	theme := gfx.DarkTheme()
	for _, side := range []TooltipSide{TooltipTop, TooltipRight, TooltipBottom, TooltipLeft} {
		var canvas gfx.Canvas
		body := BeginTooltip(&canvas, theme, viewport, anchor, TooltipOptions{Side: side}).Paint("More information")
		steps := canvas.Rectangles()[1:]
		if len(steps) != tooltipArrowSteps {
			t.Fatalf("side %d painted %d arrow steps", side, len(steps))
		}
		wantCenter := anchorCenterX
		if side == TooltipLeft || side == TooltipRight {
			wantCenter = anchorCenterY
		}

		previousReach, previousWidth := float32(0), tooltipArrowBase+1
		for index, step := range steps {
			reach, width, center := tooltipArrowMetrics(side, body, step.Bounds)
			if step.Color != theme.Foreground || step.Radius != 0 {
				t.Fatalf("side %d step %d = %+v, want a square-cornered body-colored step", side, index, step)
			}
			if center != wantCenter {
				t.Fatalf("side %d step %d centered at %v, want the trigger center %v", side, index, center, wantCenter)
			}
			// Each step reaches farther out and is narrower than the one before
			// it, which is what makes the union a triangle rather than a block.
			if reach <= previousReach || width >= previousWidth {
				t.Fatalf("side %d step %d reach %v width %v after reach %v width %v", side, index, reach, width, previousReach, previousWidth)
			}
			previousReach, previousWidth = reach, width
		}
		firstReach, firstWidth, _ := tooltipArrowMetrics(side, body, steps[0].Bounds)
		tipReach, tipWidth, _ := tooltipArrowMetrics(side, body, steps[len(steps)-1].Bounds)
		if firstWidth < tooltipArrowBase-1 || firstWidth > tooltipArrowBase {
			t.Fatalf("side %d base width = %v, want about %v", side, firstWidth, tooltipArrowBase)
		}
		if firstReach != tooltipArrowStep || tipReach != tooltipArrowStep*tooltipArrowSteps {
			t.Fatalf("side %d reach runs %v..%v, want %v..%v", side, firstReach, tipReach, tooltipArrowStep, tooltipArrowStep*tooltipArrowSteps)
		}
		if tipWidth < 1 || tipWidth > 3 {
			t.Fatalf("side %d tip width = %v, want a short blunt tip", side, tipWidth)
		}
	}
}

func TestTooltipArrowStepsOverlapTheBodyEdgeSoNoSeamOpens(t *testing.T) {
	var canvas gfx.Canvas
	anchor := gfx.Rect{X: 260, Y: 180, Width: 80, Height: 32}
	body := BeginTooltip(&canvas, gfx.LightTheme(), gfx.Rect{Width: 600, Height: 400}, anchor, TooltipOptions{Side: TooltipBottom}).Paint("Add to library")
	for index, step := range canvas.Rectangles()[1:] {
		bottom := step.Bounds.Y + step.Bounds.Height
		if bottom != body.Y+tooltipArrowInset {
			t.Fatalf("step %d ends at %v, want %v: every step must overlap the body edge so no seam can open", index, bottom, body.Y+tooltipArrowInset)
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
		base := canvas.Rectangles()[1].Bounds
		center := base.X + base.Width/2
		if center < body.X+margin || center > body.X+body.Width-margin {
			t.Fatalf("%s: arrow center %v leaves the flat span [%v, %v] of body %+v", test.name, center, body.X+margin, body.X+body.Width-margin, body)
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
