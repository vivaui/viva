package base

import (
	"time"
	"unsafe"

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
	// Inset confines the drawer and its dismiss target to the window less these
	// edges, as a page leaves a title strip or a status bar it does not own out
	// of an overlay. Zero covers the whole window.
	Inset gfx.Insets
	// Disabled paints the drawer and its header as inert: it cannot be dismissed
	// by the pointer, Escape or its close button. The body's own controls keep
	// their state; disable them too when the whole panel is unavailable.
	Disabled bool
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
	p.ForbidInMemo("Drawer")
	if !*open {
		return false
	}
	if !spec.NonModal {
		p.overlay = true
	}
	scope := p.BoundScope(pkDrawer, unsafe.Pointer(open))
	d := DrawerDeclaration{DrawerSpec: spec, title: title, scope: scope.At(0), dismiss: scope.At(1), close: scope.At(2)}
	if title == "" || spec.NoClose {
		d.close = gfx.ID{}
	}
	if spec.NonModal {
		d.scope = gfx.ID{} // no trap: the page keeps the keyboard it had
	}
	if spec.Swipe {
		d.handle = scope.At(3)
		s := stateIn[DrawerSwipe](p, scope, "swipe")
		if s.seen+1 != p.epoch {
			*s = DrawerSwipe{} // opened again: where it opens, not where it was let go
		}
		s.seen = p.epoch
		if d.swipe(c.f, s) {
			*open = false
			return true
		}
	}
	drawer := c.f.Drawer(d, footer != nil)
	if spec.Disabled {
		drawer.Disabled(true)
	}
	decl := &WidgetsOf(c.f).drawers[drawer.N().Aux]
	p.SetScope(decl.body, scope)
	if decl.footer >= 0 {
		p.SetScope(decl.footer, scope)
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
			if id := c.f.FirstControl(part); !id.IsZero() {
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

// DrawerDeclaration is a Drawer as the frame paints it: its title, its identities,
// and how far a swipe has moved it.
type DrawerDeclaration struct {
	DrawerSpec
	title                         string
	scope, dismiss, close, handle gfx.ID
	offset                        float32
}

// drawerSwipe is a swipe on a Drawer's handle: the component's, and where and when
// the pointer last was, which the speed it is let go at comes from.
type DrawerSwipe struct {
	SwipeGesture
	at   float32
	when time.Duration
	pace float32 // pixels a second toward the edge, over the last move
	seen uint32  // the page's epoch the drawer was last declared in
}

// swipeMotion is how the panel slides back when let go short of closing.
var swipeMotion = gfx.Motion{Duration: 300 * time.Millisecond, Easing: gfx.EaseOut}

// swipe follows the pointer on the handle and reports whether letting it go
// closes the drawer. It sets how far the panel is moved toward its edge.
func (d *DrawerDeclaration) swipe(f *Frame, s *DrawerSwipe) bool {
	canvas := f.Surface
	if canvas == nil {
		return false
	}
	if event, ok := canvas.PointerOn(d.handle); ok {
		// The pointer's travel toward the edge, and how far the panel reaches from it.
		content := d.Boxes(canvas, f.Palette, f.bounds.Inset(d.Inset), 0).Content
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

type drawerState struct {
	spec         DrawerDeclaration
	viewport     Rect
	body, footer int32 // the parts declared into; footer is -1 without one
	layer        uint16
}

// drawer declares a Drawer's root: an element over the whole frame holding the
// body and footer at the rectangles components.Drawer gives them. The drawer
// itself covers the frame less its spec's Inset.
func (f *Frame) Drawer(spec DrawerDeclaration, footer bool) Box {
	layer := f.WidgetLayer() + 2
	viewport := f.bounds.Inset(spec.Inset)
	layout := spec.Boxes(f.Surface, f.Palette, viewport, layer)
	body := layout.Body
	switch {
	case spec.title == "":
		body = layout.Content // the panel is the body's
	case !footer:
		body.Height = max(layout.Content.Y+layout.Content.Height-f.Palette.Metrics.PadSurface-body.Y, 0)
	}
	root := f.Popup().At(0, 0).Fill()
	r := root.N()
	r.Dir = StackDir
	r.Flags |= flagDrawer
	if PaintDrawer != nil {
		f.paintDrawerFn = PaintDrawer
	}
	r.Aux = int32(len(WidgetsOf(f).drawers))
	d := drawerState{spec: spec, viewport: viewport, layer: layer, footer: -1}
	// A Stack's children sit at its top left; the margin moves each to its rectangle.
	place := func(b Box, r Rect) Box {
		return b.Size(r.Width, r.Height).Self(Start).MarginTop(r.Y - f.bounds.Y).MarginLeft(r.X - f.bounds.X)
	}
	root.Children(func() {
		d.body = place(f.Column(), body).i
		if footer {
			// The buttons against the right edge in the order given, as a Dialog's.
			d.footer = place(f.Row(), layout.Footer).Gap(f.Palette.Metrics.GapItem).Justify(End).Items(Center).i
		}
	})
	WidgetsOf(f).drawers = append(WidgetsOf(f).drawers, d)
	return root
}

// drawerPart declares fn inside a part of a Drawer, on its layer.
func (b Box) drawerPart(fn func(), layer uint16) {
	b.N()
	saved := WidgetsOf(b.f).layer
	WidgetsOf(b.f).layer = layer
	b.Children(fn)
	WidgetsOf(b.f).layer = saved
}
