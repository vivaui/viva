package base

import (
	"time"

	"github.com/vivaui/viva/gfx"
)

// IconHover is the declarative form of IconHoverState: it binds the
// caller-owned hover state to an icon element's hit ID. Keep one value in the
// page's state and call Bind on the element that declares the hover region.
//
//	f.Icon(icon).Hit(edit, "Edit").BindIconHover(ui.BindIconHover(&page.hover, edit))
//
// The element must be hoverable; Hit declares the region. The transform is
// painted inside the icon's layout box, so it moves the glyph without moving the
// page or the hit region.
type IconHover struct {
	state *IconHoverState
	ID    gfx.ID
	slot  uint8
	dwell time.Duration
	span  time.Duration
}

// Bind creates the binding for one icon element.
func BindIconHover(state *IconHoverState, ID gfx.ID) IconHover {
	return IconHover{state: state, ID: ID, dwell: 130 * time.Millisecond, span: 180 * time.Millisecond}
}

// Dwell replaces how long the pointer must rest before the gesture starts.
func (hover IconHover) Dwell(dwell time.Duration) IconHover {
	hover.dwell = dwell
	return hover
}

// Span replaces the gesture's duration.
func (hover IconHover) Span(d time.Duration) IconHover {
	hover.span = d
	return hover
}

// Slot separates several gestures on one identity.
func (hover IconHover) Slot(slot uint8) IconHover {
	hover.slot = slot
	return hover
}

// Paint advances the gesture inside f's paint pass and returns its transform.
// The zero hover (no state) leaves the icon still.
func (hover IconHover) Transform(f *Frame) gfx.IconTransform {
	if hover.state == nil || f == nil || hover.ID.IsZero() {
		return gfx.IconTransform{}
	}
	progress, _ := hover.state.Progress(f.Surface, hover.ID, hover.slot, hover.dwell, hover.span)
	if progress >= 1 {
		return gfx.IconTransform{}
	}
	return iconHoverTransform(progress)
}

// iconHoverTransform is the built-in gesture: a one-unit lift and a small
// settle. Applications can ignore the transform and map Progress themselves,
// which is what a glyph-specific gesture does.
func iconHoverTransform(progress float32) gfx.IconTransform {
	if progress <= 0 || progress >= 1 {
		return gfx.IconTransform{}
	}
	lift := 1 - abs32(2*progress-1)
	return gfx.IconTransform{Translate: Point{Y: -lift}}
}

func abs32(Value float32) float32 {
	if Value < 0 {
		return -Value
	}
	return Value
}
