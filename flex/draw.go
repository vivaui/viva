package flex

import "github.com/vivaui/viva/ui"

// An element can paint something of its own inside the page, in the order the page is
// painted, instead of after it. Draw runs once the element's background, border and
// control are down and before its children; DrawOver runs after them:
//
//	f.Row().Size(120, 24).Radius(4).Hit(id, "Level").Draw(func(d *flex.DrawContext) {
//		d.Canvas.Fill(d.Rect, d.Fade(level.Color()))
//	}).DrawOver(func(d *flex.DrawContext) {
//		if d.State.Hovered {
//			d.Canvas.Fill(d.Rect, d.Fade(hoverTint))
//		}
//	})
//
// What an element draws is laid out before it is painted, so Rect is where the element
// ended up, which a Slot gives only after End. It sits in the element's place in the
// page: inside its Clip, under what is painted over it, and faded with it.
//
// A callback only paints: it may run more than once a frame, it must not declare
// elements, and it is not kept past the frame. The DrawContext it is given is reused,
// so a callback that wants anything from it later copies it.

// DrawContext is what a Draw or DrawOver callback paints with.
type DrawContext struct {
	// Canvas is what is painted on.
	Canvas *ui.Canvas
	// Theme is the frame's theme, faded with the element and with what it is inside.
	// It is the frame's own, not a copy: a callback reads it and does not keep it.
	Theme *ui.Theme
	// Rect is where the element ended up.
	Rect ui.Rect
	// State is the element's pointer and keyboard state.
	State ui.State
	// Opacity is the product of the opacities of the element and what it is inside,
	// 0 to 1.
	Opacity float32

	frame *Frame
	node  *node
}

// Part is the rectangle of part i of the element being drawn, for an element that has parts
// the caller paints: the options of an open SelectBase. It is empty for any other element.
func (d *DrawContext) Part(i int) ui.Rect { return d.frame.selectPart(d.node, i) }

// Fade returns c with the opacity of the element, as the colors of the element itself
// are. Colors taken from Theme are faded already.
func (d *DrawContext) Fade(c ui.Color) ui.Color {
	c.Alpha *= d.Opacity
	return c
}

// drawHooks is what Draw and DrawOver declared on one element.
type drawHooks struct {
	under, over func(*DrawContext)
}

// Draw paints on the element with fn after its background, border and control, before
// its children. Calling it again replaces the earlier callback.
func (b Box) Draw(fn func(*DrawContext)) Box { b.hooks().under = fn; return b }

// DrawOver paints on the element with fn after its children, and after the elements
// placed out of the flow inside it.
func (b Box) DrawOver(fn func(*DrawContext)) Box { b.hooks().over = fn; return b }

func (b Box) hooks() *drawHooks {
	extra := b.f.extra(b.i)
	if extra.draws == 0 {
		b.f.draws = append(b.f.draws, drawHooks{})
		extra.draws = int32(len(b.f.draws))
	}
	return &b.f.draws[extra.draws-1]
}

// State is the element's state as the canvas and the declaration have it now: what its
// looks (Hover, Pressed, Focus) are chosen by, available to a declaration that decides
// what to build from it. Disabled, Checked, Selected and Open are the ones the element was declared with.
func (b Box) State() ui.State { return b.f.stateOf(b.n()) }

func (f *Frame) stateOf(n *node) ui.State {
	state := f.canvas.StateOf(n.id)
	state.Disabled = n.flags&flagDisabled != 0
	state.Checked = n.flags&flagChecked != 0
	state.Selected = n.flags&flagSelected != 0
	state.Open = n.flags&flagOpen != 0
	return state
}

// runDraw calls fn for n with the context reused for the frame.
func (f *Frame) runDraw(n *node, fn func(*DrawContext)) {
	d := &f.drawContext
	d.Canvas, d.Theme, d.Rect, d.State, d.Opacity = f.canvas, &f.theme, n.rect, f.stateOf(n), f.opacity
	d.frame, d.node = f, n
	fn(d)
}
