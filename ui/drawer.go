package ui

import (
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
// a press beside it still closes it), and the control that takes the keyboard as it
// opens (the first of the body when zero; none for a NonModal one).
type DrawerSpec struct {
	Side         DrawerSide
	Size         float32
	Description  string
	NoClose      bool
	NonModal     bool
	InitialFocus gfx.ID
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
	drawer := c.f.drawer(d, footer != nil)
	decl := &c.f.widgets.drawers[drawer.n().aux]
	p.setScope(decl.body, scope)
	if decl.footer >= 0 {
		p.setScope(decl.footer, scope)
	}
	if body != nil {
		Box{f: c.f, i: decl.body}.drawerPart(body, decl.layer)
	}
	if footer != nil {
		Box{f: c.f, i: decl.footer}.drawerPart(footer, decl.layer)
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

// drawerSpec is a Drawer as the frame paints it: its title and identities.
type drawerSpec struct {
	DrawerSpec
	title                 string
	scope, dismiss, close gfx.ID
}

type drawerDecl struct {
	spec         drawerSpec
	body, footer int32 // the parts declared into; footer is -1 without one
	layer        uint16
}

func (spec *drawerSpec) component(canvas *gfx.Canvas, theme Theme, bounds Rect, layer uint16) components.Drawer {
	return components.BeginDrawer(canvas, theme, bounds, components.DrawerOptions{
		Layer: layer, Scope: spec.scope, DismissID: spec.dismiss, CloseID: spec.close,
		Side: spec.Side.component(), ContentExtent: spec.Size, InitialFocusID: spec.InitialFocus,
	})
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
