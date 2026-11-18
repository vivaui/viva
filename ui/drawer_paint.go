package ui

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
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

func (spec *drawerSpec) component(Surface *gfx.Canvas, Palette Theme, bounds Rect, layer uint16) components.Drawer {
	drawer := components.BeginDrawer(Surface, Palette, bounds, components.DrawerOptions{
		Layer: layer, Scope: spec.scope, DismissID: spec.dismiss, CloseID: spec.close,
		Side: spec.Side.component(), ContentExtent: spec.Size, InitialFocusID: spec.InitialFocus,
		HandleID: spec.handle, Offset: spec.offset,
	})
	if spec.Swipe {
		drawer = drawer.WithHandle()
	}
	return drawer
}
