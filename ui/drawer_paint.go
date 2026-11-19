package ui

import (
	"github.com/vivaui/viva/base"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

func drawerSideComponent(side DrawerSide) components.DrawerSide {
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

func drawerDeclComponent(spec *DrawerDecl, Surface *gfx.Canvas, Palette Theme, bounds Rect, layer uint16) components.Drawer {
	drawer := components.BeginDrawer(Surface, Palette, bounds, components.DrawerOptions{
		Layer: layer, Scope: spec.GetScope(), DismissID: spec.GetDismiss(), CloseID: spec.GetClose(),
		Side: drawerSideComponent(spec.Side), ContentExtent: spec.Size, InitialFocusID: spec.InitialFocus,
		HandleID: spec.GetHandle(), Offset: spec.GetOffset(),
	})
	if spec.Swipe {
		drawer = drawer.WithHandle()
	}
	return drawer
}

// paintDrawer paints a Drawer's root: the scrim, the panel and its header, then the
// body and footer inside the layer and focus scope it opens.
func paintDrawer(f *Frame, n *Node) {
	d := &base.WidgetsOf(f).GetDrawers()[n.Aux]
	bounds := d.GetViewport()
	if bounds.Width <= 0 || bounds.Height <= 0 {
		bounds = n.Rect
	}
	drawer := drawerComponent(d.SpecPtr(), f.Surface, f.Palette, bounds, d.GetLayer())
	if n.Flags&FlagDisabled != 0 {
		drawer = drawer.Disabled()
	}
	drawer.Begin(d.SpecPtr().GetTitle(), d.SpecPtr().Description)
	for c := n.First; c >= 0; c = f.Nodes[c].Next {
		f.PaintChild(c)
	}
	drawer.End()
}

// drawerComponent is a drawer as the components paint it.
func drawerComponent(spec *DrawerDeclaration, surface *gfx.Canvas, palette Theme, bounds Rect, layer uint16) components.Drawer {
	drawer := components.BeginDrawer(surface, palette, bounds, components.DrawerOptions{
		Layer: layer, Scope: spec.GetScope(), DismissID: spec.GetDismiss(), CloseID: spec.GetClose(),
		Side: drawerSideComponent(spec.Side), ContentExtent: spec.Size, InitialFocusID: spec.InitialFocus,
		HandleID: spec.GetHandle(), Offset: spec.GetOffset(),
	})
	if spec.Swipe {
		drawer = drawer.WithHandle()
	}
	return drawer
}
