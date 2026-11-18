package ui

import (
	"github.com/vivaui/viva/base"
	"github.com/vivaui/viva/gfx"
)

// The engine below is behaviour only: it asks these hooks for the appearances,
// the measurements and the overlay construction the styled layer owns. They are
// installed once, when package ui is linked, and are plain function values so no
// page pays for a person.
func init() {
	base.ControlPainter = paintControls
	base.ControlMeasurer = measureControls
	base.ControlBaseline = controlBaseline
	base.PaintSwatch = paintSwatch
	base.PaintPickerPanel = paintColorPickerPanel
	base.PaintCombo = comboStateDraw
	base.PaintAddons = paintAddons
	base.PaintScrollbars = paintScrollbars
	base.PaintTooltip = paintTooltip
	base.PaintTabs = paintTabs
	base.PaintSelect = paintSelect
	base.MeasureSelect = measureSelect
	base.PaintDialog = paintDialog
	base.PaintDrawer = paintDrawer
	base.PaintAlertDialog = paintAlertDialog
	base.PaintTableRows = paintTableRows
	base.PaintTableHeaderRow = paintTableHeaderRow
	base.PaintTableScrollbars = paintTableScrollbars
	base.PaintMenu = paintAt
	base.MenuSize = menuSize
	base.MenuRowExtent = rowExtent
	base.PaintTreeRow = treeStateRow
	base.TreeDraw = treeStateDraw
	base.EntryLook = frameTextEntryLook
	base.SliderPointer = sliderPointer
	base.BuildDialog = frameDialog
	base.BuildPopover = framePopover
	base.DrawerBoxes = drawerBoxes
}

// drawerBoxes is where a drawer's parts go, from the component's layout.
func drawerBoxes(spec base.DrawerDeclaration, surface *gfx.Canvas, palette Theme, bounds Rect, layer uint16) base.DrawerRects {
	layout := drawerComponent(&spec, surface, palette, bounds, layer).Layout(spec.Description)
	return base.DrawerRects{Content: layout.Content, Body: layout.Body, Footer: layout.Footer}
}
