package ui

import (
	"time"
	"unsafe"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

// DrawerSide is the edge of the window a Drawer comes in from; the zero value is
// the right.
type DrawerSide uint8

const (
	DrawerRight DrawerSide = iota
	DrawerLeft
	DrawerBottom
	DrawerTop
)

func (side DrawerSide) component() components.DrawerSide {
	switch side {
	case DrawerLeft:
		return components.DrawerLeft
	case DrawerBottom:
		return components.DrawerBottom
	case DrawerTop:
		return components.DrawerTop
	}
	return components.DrawerRight
}

// DrawerSpec declares a Drawer beyond its title: the edge it comes in from (the
// right when zero), its size across that edge (384 wide, or as tall as its parts,
// when zero; never more than the window), a description, NoClose to leave out the
// close button, NonModal for a panel that does not hold the keyboard (Tab and the
// page's shortcuts go on reaching the page behind it, whose focus it does not take;
// a press beside it still closes it), the control that takes the keyboard as it
// opens (the first of the body when zero; none for a NonModal one), and Swipe for a
// handle at its inner edge that a drag toward the edge closes it by, as vaul's
// drawer: thrown, or dragged past half its size, it closes; let go short of that, it
// slides back.
type DrawerSpec struct {
	Side         DrawerSide
	Size         float32
	Description  string
	NoClose      bool
	NonModal     bool
	InitialFocus gfx.ID
	Swipe        bool
}

// Drawer shows a panel that slides in from an edge of the window while *open, as
// shadcn's Sheet and Drawer: title and description at its top, what body declares
// under them, footer's buttons at its foot. Escape, a press beside it and its close
// button set *open to false, and it reports the paint that closed it. With an empty
// title the panel is the body's alone, edge to edge, as an assistant or an inspector
// that draws its own header is. footer may be nil.
//
//	ui.Drawer(c, &m.details, "Host", ui.DrawerSpec{Description: h.Name}, func() { ... }, nil)
//
// Not inside a Memo.
func Drawer(c C, open *bool, title string, spec DrawerSpec, body, footer func()) bool {
	p := c.page
	p.forbidInMemo("Drawer")
	if !*open {
		return false
	}
	if !spec.NonModal {
		p.overlay = true
	}
	scope := p.boundScope(pkDrawer, unsafe.Pointer(open))
	d := drawerSpec{DrawerSpec: spec, title: title, scope: scope.At(0), dismiss: scope.At(1), close: scope.At(2)}
	if title == "" || spec.NoClose {
		d.close = gfx.ID{}
	}
	if spec.NonModal {
		d.scope = gfx.ID{} // no trap: the page keeps the keyboard it had
	}
	if spec.Swipe {
		d.handle = scope.At(3)
		s := stateIn[drawerSwipe](p, scope, "swipe")
		if s.seen+1 != p.epoch {
			*s = drawerSwipe{} // opened again: where it opens, not where it was let go
		}
		s.seen = p.epoch
		if d.swipe(c.f, s) {
			*open = false
			return true
		}
	}
	drawer := c.f.drawer(d, footer != nil)
	decl := &c.f.widgets.drawers[drawer.n().aux]
	p.setScope(decl.body, scope)
	if decl.footer >= 0 {
		p.setScope(decl.footer, scope)
	}
	if body != nil {
		Box{f: c.f, i: decl.body, gen: c.f.gen}.drawerPart(body, decl.layer)
	}
	if footer != nil {
		Box{f: c.f, i: decl.footer, gen: c.f.gen}.drawerPart(footer, decl.layer)
	}
	if decl.spec.InitialFocus.IsZero() && !spec.NonModal {
		for _, part := range [2]int32{decl.body, decl.footer} {
			if part < 0 {
				continue
			}
			if id := c.f.firstControl(part); !id.IsZero() {
				decl.spec.InitialFocus = id
				break
			}
		}
	}
	if drawer.Dismissed() {
		*open = false
		return true
	}
	return false
}

// drawerSpec is a Drawer as the frame paints it: its title, its identities, and how
// far a swipe has moved it.
type drawerSpec struct {
	DrawerSpec
	title                         string
	scope, dismiss, close, handle gfx.ID
	offset                        float32
}

// drawerSwipe is a swipe on a Drawer's handle: the component's, and where and when
// the pointer last was, which the speed it is let go at comes from.
type drawerSwipe struct {
	components.DrawerSwipe
	at   float32
	when time.Duration
	pace float32 // pixels a second toward the edge, over the last move
	seen uint32  // the page's epoch the drawer was last declared in
}

// swipeMotion is how the panel slides back when let go short of closing.
var swipeMotion = gfx.Motion{Duration: 300 * time.Millisecond, Easing: gfx.EaseOut}

// swipe follows the pointer on the handle and reports whether letting it go
// closes the drawer. It sets how far the panel is moved toward its edge.
func (d *drawerSpec) swipe(f *Frame, s *drawerSwipe) bool {
	canvas := f.canvas
	if canvas == nil {
		return false
	}
	if event, ok := canvas.PointerOn(d.handle); ok {
		// The pointer's travel toward the edge, and how far the panel reaches from it.
		content := d.component(canvas, f.theme, f.bounds, 0).Layout(d.Description).Content
		toward, extent := event.Y, content.Height
		switch d.Side {
		case DrawerTop:
			toward = -event.Y
		case DrawerRight:
			toward, extent = event.X, content.Width
		case DrawerLeft:
			toward, extent = -event.X, content.Width
		}
		now := canvas.Now()
		switch {
		case event.Kind == gfx.PointerDown && event.Button <= gfx.PointerButtonPrimary:
			s.Begin(toward)
			s.at, s.when, s.pace = toward, now, 0
			canvas.Consume()
		case event.Kind == gfx.PointerMove && s.Dragging():
			if dt := (now - s.when).Seconds(); dt > 0 {
				s.pace = float32(float64(toward-s.at) / dt)
			}
			s.at, s.when = toward, now
			s.Move(toward, extent)
			canvas.Consume()
		case event.Kind == gfx.PointerUp && s.Dragging():
			canvas.Consume()
			if _, dismissed := s.End(extent, s.pace, nil); dismissed {
				return true
			}
		}
	}
	motion := swipeMotion
	if s.Dragging() {
		motion = gfx.Motion{} // under the pointer, as it moves
	}
	d.offset = canvas.Animate(d.handle, 0, s.Offset(), motion)
	return false
}

type drawerDecl struct {
	spec         drawerSpec
	body, footer int32 // the parts declared into; footer is -1 without one
	layer        uint16
}

func (spec *drawerSpec) component(canvas *gfx.Canvas, theme Theme, bounds Rect, layer uint16) components.Drawer {
	drawer := components.BeginDrawer(canvas, theme, bounds, components.DrawerOptions{
		Layer: layer, Scope: spec.scope, DismissID: spec.dismiss, CloseID: spec.close,
		Side: spec.Side.component(), ContentExtent: spec.Size, InitialFocusID: spec.InitialFocus,
		HandleID: spec.handle, Offset: spec.offset,
	})
	if spec.Swipe {
		drawer = drawer.WithHandle()
	}
	return drawer
}

// drawer declares a Drawer's root: an element over the whole frame holding the
// body and footer at the rectangles components.Drawer gives them.
func (f *Frame) drawer(spec drawerSpec, footer bool) Box {
	layer := f.widgets.layer + 2
	layout := spec.component(f.canvas, f.theme, f.bounds, layer).Layout(spec.Description)
	body := layout.Body
	switch {
	case spec.title == "":
		body = layout.Content // the panel is the body's
	case !footer:
		body.Height = max(layout.Content.Y+layout.Content.Height-f.theme.Metrics.PadSurface-body.Y, 0)
	}
	root := f.Popup().At(0, 0).Fill()
	r := root.n()
	r.dir = stack
	r.flags |= flagDrawer
	f.paintDrawerFn = (*Frame).paintDrawer
	r.aux = int32(len(f.widgets.drawers))
	d := drawerDecl{spec: spec, layer: layer, footer: -1}
	// A Stack's children sit at its top left; the margin moves each to its rectangle.
	place := func(b Box, r Rect) Box {
		return b.Size(r.Width, r.Height).Self(Start).MarginSides(r.Y-f.bounds.Y, 0, 0, r.X-f.bounds.X)
	}
	root.Children(func() {
		d.body = place(f.Column(), body).i
		if footer {
			// The buttons against the right edge in the order given, as a Dialog's.
			d.footer = place(f.Row(), layout.Footer).Gap(f.theme.Metrics.GapItem).Justify(End).Items(Center).i
		}
	})
	f.widgets.drawers = append(f.widgets.drawers, d)
	return root
}

// drawerPart declares fn inside a part of a Drawer, on its layer.
func (b Box) drawerPart(fn func(), layer uint16) {
	b.n()
	saved := b.f.widgets.layer
	b.f.widgets.layer = layer
	b.Children(fn)
	b.f.widgets.layer = saved
}

// paintDrawer paints a Drawer's root: the scrim, the panel and its header, then the
// body and footer inside the layer and focus scope it opens.
func (f *Frame) paintDrawer(n *node) {
	d := &f.widgets.drawers[n.aux]
	drawer := d.spec.component(f.canvas, f.theme, n.rect, d.layer)
	if n.flags&flagDisabled != 0 {
		drawer = drawer.Disabled()
	}
	drawer.Begin(d.spec.title, d.spec.Description)
	for c := n.first; c >= 0; c = f.nodes[c].next {
		f.paint(c)
	}
	drawer.End()
}
