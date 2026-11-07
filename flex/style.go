package flex

import "github.com/vivaui/viva/ui"

// An element changes with the pointer and the keyboard through three looks, each a
// Look listing only what differs from how the element is drawn at rest:
//
//	f.Row().Pad(8).Radius(8).Hit(id, "Open").Transition().
//		Hover(flex.Look{}.Background(theme.Hover)).
//		Pressed(flex.Look{}.Background(theme.Muted)).
//		Focus(flex.Look{}.Ring(3, ring))
//
// Hover applies while the pointer is over the element, Pressed while it is held
// down, and Focus while the keyboard has it and the focus is to be seen (the way
// :focus-visible does, so a click does not draw a ring). The element must have
// the identity those states are tracked by (Hit, or a control that has one), which
// is also what makes it focusable unless HitWith says ui.HitOptions.SkipFocus. A
// look on an element with no identity changes nothing.
//
// Looks combine field by field in the order Focus, Hover, Pressed, so a focused row
// that is also hovered shows the ring of the one and the fill of the other, and
// Pressed wins over Hover where both say something about the same field.
//
// StateFrom makes an element follow the state of another instead of its own, so
// a pencil that appears when its row is hovered, or an icon that takes the row's
// color, is declared on the pencil and the icon:
//
//	row := f.Row().Hit(id, "Host")
//	row.Children(func() {
//		f.Text(name)
//		f.Icon(edit).Opacity(0).StateFrom(row).Hover(flex.Look{}.Opacity(1))
//	})
//
// Transition fades between the looks over the theme's motion (TransitionWith takes
// another), the way a control does; without it they change at once. A reversed
// transition starts from what is shown. The elements that follow one element share
// its timeline, so give them the same motion.

// Look is how an element differs from its resting look in one state. The zero Look
// changes nothing; each method sets one thing, and what was not set is left as it
// is. A Look is a small value to build where it is used.
type Look struct {
	set                lookFields
	background, border ui.Color
	color, ring        ui.Color
	ringWidth, opacity float32
	shadow             ui.ShadowLevel
}

type lookFields uint8

const (
	lookBackground lookFields = 1 << iota
	lookBorder
	lookColor
	lookRing
	lookShadow
	lookOpacity
)

// Background replaces the fill of the element. A gradient is painted over the fill,
// so a Look that changes the fill of an element that has one changes only what
// shows through the gradient.
func (l Look) Background(c ui.Color) Look {
	l.set, l.background = l.set|lookBackground, c
	return l
}

// Border replaces the color of the element's border, which must have been given a
// width by Border; a border of Color{} at rest is one that appears.
func (l Look) Border(c ui.Color) Look {
	l.set, l.border = l.set|lookBorder, c
	return l
}

// Color replaces the color of a Text or the tint of an Icon.
func (l Look) Color(c ui.Color) Look {
	l.set, l.color = l.set|lookColor, c
	return l
}

// Ring draws a line of the given width around the outside of the element, as Ring
// does for the element at rest. Over an element with no ring it fades in.
func (l Look) Ring(width float32, c ui.Color) Look {
	l.set, l.ringWidth, l.ring = l.set|lookRing, width, c
	return l
}

// Shadow replaces the element's shadow with that level of the elevation scale.
// Shadows do not blend: during a Transition the new one appears halfway.
func (l Look) Shadow(level ui.ShadowLevel) Look {
	l.set, l.shadow = l.set|lookShadow, level
	return l
}

// Opacity replaces the element's opacity, 0 to 1, as Box.Opacity does.
func (l Look) Opacity(v float32) Look {
	l.set, l.opacity = l.set|lookOpacity, min(max(v, 0), 1)
	return l
}

// stateLooks is what Hover, Pressed, Focus, StateFrom and Transition declared on
// one element.
type stateLooks struct {
	hover, pressed, focus Look
	follow                int32 // 1 + the element whose state is followed, or 0 for its own
	motion                ui.Motion
	animated              bool
}

// The slots of the transitions of an element's states, among the animated values
// the canvas keeps for its identity. They are far from the small numbers the
// components use for the controls that share an identity with an element.
const (
	slotHover uint8 = 250 + iota
	slotPressed
	slotFocus
)

func (b Box) states() *stateLooks {
	extra := b.f.extra(b.i)
	if extra.states == 0 {
		b.f.states = append(b.f.states, stateLooks{})
		extra.states = int32(len(b.f.states))
	}
	return &b.f.states[extra.states-1]
}

// Hover is how the element looks while the pointer is over it.
func (b Box) Hover(look Look) Box { b.states().hover = look; return b }

// Pressed is how the element looks while it is held down.
func (b Box) Pressed(look Look) Box { b.states().pressed = look; return b }

// Focus is how the element looks while the keyboard has it and the focus is
// visible, which is after the keyboard moved it there and not after a click.
func (b Box) Focus(look Look) Box { b.states().focus = look; return b }

// StateFrom makes the element take its Hover, Pressed and Focus looks, and the
// timeline of its Transition, from the state of container instead of its own, so
// that what is inside a row changes with the row.
func (b Box) StateFrom(container Box) Box { b.states().follow = container.i + 1; return b }

// Transition fades the element between its looks over the theme's Transition
// motion; TransitionWith does it over another. An idle element schedules nothing.
func (b Box) Transition() Box {
	return b.TransitionWith(b.f.theme.Transition)
}

func (b Box) TransitionWith(motion ui.Motion) Box {
	states := b.states()
	states.motion, states.animated = motion, true
	return b
}

// Opacity draws the element and everything inside it with its colors' opacity
// multiplied by v, 0 to 1: its fill, border, shadow, ring, text, icons and the
// controls inside it, which are painted with a theme faded the same way. The
// opacity of an element inside one is the product of both. Each color is faded,
// not the picture they make, so where two of them overlap one shows through the
// other. A faded element is still laid out, hit and focused: an element at 0 is
// invisible and clickable. An Image is not faded.
func (b Box) Opacity(v float32) Box {
	b.f.extra(b.i).fade = 1 - min(max(v, 0), 1)
	return b
}

// Ring draws a line of the given width around the outside of the element, hugging
// its rounded corners, without taking room: a focus ring for a control that has no
// ring of its own, the way a 3 pixel ring-ring/50 is drawn. It is painted outside
// the element's rectangle, so a Clip on a container around it cuts it, and not
// inside it as Stroke is.
func (b Box) Ring(width float32, c ui.Color) Box {
	extra := b.f.extra(b.i)
	extra.ringWidth, extra.ring = width, c
	return b
}

// Angle is a direction of a gradient, in degrees clockwise from pointing up, as in
// CSS: ToRight runs from the left edge to the right one. The diagonals are
// 45 degree angles: on an element that is not square the gradient line is still
// at that angle, not toward the corner.
type Angle float32

const (
	ToTop         Angle = 0
	ToTopRight    Angle = 45
	ToRight       Angle = 90
	ToBottomRight Angle = 135
	ToBottom      Angle = 180
	ToBottomLeft  Angle = 225
	ToLeft        Angle = 270
	ToTopLeft     Angle = 315
)

// gradient is a gradient of an element: stops[first:first+count] of the frame.
type gradient struct {
	angle        Angle
	first, count int32
}

// Gradient fills the element, with its corners, with a gradient from one color to
// another in the direction given. It is painted over the Background and under the
// Border, as CSS paints a background image.
func (b Box) Gradient(angle Angle, from, to ui.Color) Box {
	return b.GradientStops(angle, ui.Stop(0, from), ui.Stop(1, to))
}

// GradientStops is Gradient through any number of colors, each at a position along
// the line from 0 to 1 (see ui.Canvas.FillGradient). The stops are copied.
//
//	f.Row().Radius(12).GradientStops(flex.ToBottomRight, ui.Stop(0, a), ui.Stop(0.6, b), ui.Stop(1, c))
func (b Box) GradientStops(angle Angle, stops ...ui.GradientStop) Box {
	if len(stops) == 0 {
		return b
	}
	f := b.f
	f.gradients = append(f.gradients, gradient{angle: angle, first: int32(len(f.stops)), count: int32(len(stops))})
	f.stops = append(f.stops, stops...)
	f.extra(b.i).gradient = int32(len(f.gradients))
	return b
}

// resolved is how an element is drawn this frame: what it is at rest with its
// looks applied.
type resolved struct {
	background, border, color, ring ui.Color
	ringWidth, opacity              float32
	shadow                          ui.ShadowLevel
}

func (f *Frame) resolve(n *node, extra *extra) resolved {
	s := resolved{
		background: n.background, border: extra.border, color: n.color, ring: extra.ring,
		ringWidth: extra.ringWidth, opacity: 1 - extra.fade, shadow: extra.shadow,
	}
	if extra.states == 0 {
		return s
	}
	looks := &f.states[extra.states-1]
	source := n.id
	if looks.follow != 0 {
		source = f.nodes[looks.follow-1].id
	}
	if source.IsZero() {
		return s
	}
	canvas := f.canvas
	focus, hover, pressed := target(canvas.FocusVisible() == source), target(canvas.Hovered() == source), target(canvas.Pressed() == source)
	if looks.animated {
		focus = canvas.Animate(source, slotFocus, focus, looks.motion)
		hover = canvas.Animate(source, slotHover, hover, looks.motion)
		pressed = canvas.Animate(source, slotPressed, pressed, looks.motion)
	}
	s.apply(&looks.focus, focus)
	s.apply(&looks.hover, hover)
	s.apply(&looks.pressed, pressed)
	return s
}

func target(on bool) float32 {
	if on {
		return 1
	}
	return 0
}

// apply moves what look sets toward it by progress, 0 to 1.
func (s *resolved) apply(look *Look, progress float32) {
	if progress <= 0 || look.set == 0 {
		return
	}
	if look.set&lookBackground != 0 {
		s.background = s.background.Mix(look.background, progress)
	}
	if look.set&lookBorder != 0 {
		s.border = s.border.Mix(look.border, progress)
	}
	if look.set&lookColor != 0 {
		s.color = s.color.Mix(look.color, progress)
	}
	if look.set&lookRing != 0 {
		if s.ringWidth <= 0 {
			s.ring, s.ringWidth = ui.Color{}, look.ringWidth // a ring that was not there fades in at its width
		}
		s.ring = s.ring.Mix(look.ring, progress)
		s.ringWidth = ui.Lerp(s.ringWidth, look.ringWidth, progress)
	}
	if look.set&lookShadow != 0 && progress >= 0.5 {
		s.shadow = look.shadow
	}
	if look.set&lookOpacity != 0 {
		s.opacity = ui.Lerp(s.opacity, look.opacity, progress)
	}
}

func fade(c ui.Color, opacity float32) ui.Color {
	c.Alpha *= opacity
	return c
}

// paintRing draws the ring around r, outside it.
func (f *Frame) paintRing(r ui.Rect, radius float32, s *resolved) {
	color := fade(s.ring, f.opacity)
	if s.ringWidth <= 0 || color.Alpha <= 0 {
		return
	}
	w := s.ringWidth
	outer := ui.Rect{X: r.X - w, Y: r.Y - w, Width: r.Width + 2*w, Height: r.Height + 2*w}
	if radius > 0 {
		radius += w
	}
	f.canvas.RoundedOutline(outer, radius, w, color)
}

// paintGradient draws the gradient of an element over its whole rectangle.
func (f *Frame) paintGradient(n *node, extra *extra) {
	g := f.gradients[extra.gradient-1]
	stops := f.stops[g.first : g.first+g.count]
	if f.opacity < 1 {
		stops = append(f.paintStops[:0], stops...)
		f.paintStops = stops
		for i := range stops {
			stops[i].Color = fade(stops[i].Color, f.opacity)
		}
	}
	f.canvas.FillGradient(n.rect, n.radius, float32(g.angle), stops)
}
