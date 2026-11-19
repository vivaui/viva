package base

import "github.com/vivaui/viva/gfx"

// An element can paint something of its own inside the page, in the order the page is
// painted, instead of after it. Draw runs once the element's background, border and
// control are down and before its children; DrawOver runs after them:
//
//	f.Row().Size(120, 24).Radius(4).Hit(id, "Level").Draw(func(d *ui.DrawContext) {
//		d.Canvas.Fill(d.Rect, d.Fade(level.Color()))
//	}).DrawOver(func(d *ui.DrawContext) {
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
	Canvas *gfx.Canvas
	// Theme is the frame's theme, faded with the element and with what it is inside.
	// It is the frame's own, not a copy: a callback reads it and does not keep it.
	Theme *Theme
	// Rect is where the element ended up.
	Rect Rect
	// State is the element's pointer and keyboard state.
	State gfx.State
	// Opacity is the product of the opacities of the element and what it is inside,
	// 0 to 1.
	Opacity float32
	// Label is the element's accessibility name, as Box.Label set it: what a
	// control of your own announces, and a list names itself by.
	Label string

	Frame *Frame
	Node  *Node
}

// Part is the rectangle of part i of the element being drawn, for an element that has parts
// the caller paints: the options of an open SelectBase. It is empty for any other element.
func (d *DrawContext) Part(i int) Rect { return d.Frame.SelectPart(d.Node, i) }

// Fade returns c with the opacity of the element, as the colors of the element itself
// are. The color of State looks is already faded; other colors taken from Theme
// are not.
func (d *DrawContext) Fade(c Color) Color {
	c.Alpha *= d.Opacity
	return c
}

// DashedBorder draws a repeating border inside bounds with the element's fade.
// Set bounds to d.Rect for the element whose callback is drawing.
func (d *DrawContext) DashedBorder(bounds Rect, Radius, width float32, Color Color) {
	d.Canvas.DashedRoundedOutline(bounds, Radius, width, d.Fade(Color))
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
	b.N()
	extra := b.f.Extra(b.i)
	if extra.Draws == 0 {
		b.f.Draws = append(b.f.Draws, drawHooks{})
		extra.Draws = int32(len(b.f.Draws))
	}
	return &b.f.Draws[extra.Draws-1]
}

// State is the element's state as the canvas and the declaration have it now: what its
// looks (Hover, Pressed, Focus) are chosen by, available to a declaration that decides
// what to build from it. Disabled, Checked, Selected and Open are the ones the element was declared with.
func (b Box) State() gfx.State { return b.f.StateOf(b.N()) }

func (f *Frame) StateOf(n *Node) gfx.State {
	state := f.Surface.StateOf(n.ID)
	state.Disabled = n.Flags&flagDisabled != 0
	state.Checked = n.Flags&flagChecked != 0
	state.Selected = n.Flags&flagSelected != 0
	state.Open = n.Flags&flagOpen != 0
	return state
}

// runDraw calls fn for n with the context reused for the frame.
func (f *Frame) runDraw(n *Node, fn func(*DrawContext)) {
	d := &f.drawContext
	d.Canvas, d.Theme, d.Rect, d.State, d.Opacity, d.Label = f.Surface, &f.Palette, n.Rect, f.StateOf(n), f.Opacity, n.Label
	d.Frame, d.Node = f, n
	fn(d)
}
