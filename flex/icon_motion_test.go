package flex

import (
	"testing"
	"time"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/ui"
)

func TestIconHoverBindingMovesOnlyTheIconInsideItsBox(t *testing.T) {
	var canvas ui.Canvas
	var view View
	bounds := ui.Rect{Width: 120, Height: 40}
	icon := ui.Icon{
		ViewBox:  24,
		Stroke:   2,
		Commands: []ui.PathCommand{ui.MoveTo(4, 12), ui.LineTo(20, 12)},
		Cap:      ui.CapRound,
	}
	id := ui.Item(1)
	var hover components.IconHover
	now := time.Duration(0)
	canvas.SetClock(func() time.Duration { return now })
	canvas.SetInteraction(id, ui.ID{}, ui.ID{}, false)
	view.Draw(&canvas, ui.LightTheme(), bounds, func(f *Frame) {
		f.Icon(icon).Size(24, 24).Hit(id, "Move").BindIconHover(BindIconHover(&hover, id))
	})
	rest := append([]ui.Triangle(nil), canvas.Triangles()...)
	if len(rest) == 0 {
		t.Fatal("the icon painted nothing")
	}

	canvas.Reset()
	now = 130 * time.Millisecond
	canvas.SetInteraction(id, ui.ID{}, ui.ID{}, false)
	canvas.BeginFrame()
	view.Draw(&canvas, ui.LightTheme(), bounds, func(f *Frame) {
		f.Icon(icon).Size(24, 24).Hit(id, "Move").BindIconHover(BindIconHover(&hover, id))
	})
	canvas.EndFrame()
	canvas.Reset()
	now = 170 * time.Millisecond
	canvas.BeginFrame()
	view.Draw(&canvas, ui.LightTheme(), bounds, func(f *Frame) {
		f.Icon(icon).Size(24, 24).Hit(id, "Move").BindIconHover(BindIconHover(&hover, id))
	})
	canvas.EndFrame()
	moved := append([]ui.Triangle(nil), canvas.Triangles()...)
	if len(moved) != len(rest) {
		t.Fatalf("the hovering icon painted %d triangles, at rest %d", len(moved), len(rest))
	}
	movedTop := float32(1e9)
	restTop := float32(1e9)
	for _, triangle := range moved {
		for _, point := range triangle.Points {
			movedTop = min(movedTop, point.Y)
		}
	}
	for _, triangle := range rest {
		for _, point := range triangle.Points {
			restTop = min(restTop, point.Y)
		}
	}
	if movedTop >= restTop {
		t.Fatal("the hovering icon did not move")
	}
	if hit := canvas.Hits()[0]; hit.Bounds != (ui.Rect{X: 0, Y: 0, Width: 24, Height: 24}) {
		t.Fatalf("the binding moved the hit region to %+v", hit.Bounds)
	}
}

func TestIconHoverBindingWithoutStateLeavesTheIconStill(t *testing.T) {
	var canvas ui.Canvas
	var view View
	icon := ui.Icon{ViewBox: 24, Stroke: 2, Commands: []ui.PathCommand{ui.MoveTo(4, 4), ui.LineTo(20, 20)}}
	view.Draw(&canvas, ui.LightTheme(), ui.Rect{Width: 40, Height: 40}, func(f *Frame) {
		f.Icon(icon).BindIconHover(IconHover{})
	})
	if len(canvas.Triangles()) == 0 {
		t.Fatal("an unbound icon painted nothing")
	}
}
