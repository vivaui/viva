package ui

import (
	"sync"

	"github.com/vivaui/viva/base"
	"github.com/vivaui/viva/gfx"
)

// The engine below is behaviour only: it asks these hooks for the appearances,
// the measurements and the overlay construction the styled layer owns. A page
// installs the hooks of the features it declares, so an application links only
// the painters it uses. InstallPainters installs all of them, for a host that
// drives base directly or for the engine's own tests.
var installControls, installField, installSelect, installTabs, installTable, installTree,
	installMenu, installDialog, installAlert, installDrawer, installPopover, installColor, installCombo, installScroll sync.Once

func init() { useControls() }

func useControls() {
	installControls.Do(func() {
		base.SetControlPainter(KindButton, paintButtonControl)
		base.SetControlPainter(KindCheckbox, paintCheckboxControl)
		base.SetControlPainter(KindSwitch, paintSwitchControl)
		base.SetControlPainter(KindToggle, paintToggleControl)
		base.SetControlPainter(KindInput, paintInputControl)
		base.SetControlPainter(KindInputGroup, paintInputGroupControl)
		base.SetControlPainter(KindTextarea, paintTextareaControl)
		base.SetControlPainter(KindSegmented, paintSegmentedControl)
		base.SetControlPainter(KindProgress, paintProgressControl)
		base.SetControlPainter(KindSpinner, paintSpinnerControl)
		base.SetControlPainter(KindSplitter, paintSplitterControl)
		base.SetControlPainter(KindSlider, paintSliderControl)
		base.SetControlPainter(KindSplitButton, paintSplitButtonControl)
		base.SetControlPainter(KindBadge, paintBadgeControl)
		base.SetControlPainter(KindPagination, paintPaginationControl)
		base.ControlMeasurer = measureControls
		base.ControlBaseline = controlBaseline
		base.SliderPointer = sliderPointer
		base.PaintTooltip = paintTooltip
	})
}

func useField() {
	useControls()
	installField.Do(func() { base.EntryLook = frameTextEntryLook; base.PaintAddons = paintAddons })
}
func useSelect() {
	useControls()
	installSelect.Do(func() {
		base.SetControlPainter(KindSelect, paintWidgetControl)
		base.PaintSelect = paintSelect
		base.MeasureSelect = measureSelect
	})
}
func useTabs() {
	useControls()
	installTabs.Do(func() { base.SetControlPainter(KindTabs, paintWidgetControl); base.PaintTabs = paintTabs })
}
func useTable() {
	useControls()
	installTable.Do(func() {
		base.SetControlPainter(KindTable, paintWidgetControl)
		base.PaintTableRows = paintTableRows
		base.PaintTableHeaderRow = paintTableHeaderRow
		base.PaintTableScrollbars = paintTableScrollbars
	})
}
func useTree() {
	installTree.Do(func() { base.PaintTreeRow = treeStateRow; base.TreeDraw = treeStateDraw })
}
func useMenu() {
	installMenu.Do(func() { base.PaintMenu = paintAt; base.MenuSize = menuSize; base.MenuRowExtent = rowExtent })
}
func useDialog() {
	installDialog.Do(func() { base.BuildDialog = frameDialog; base.PaintDialog = paintDialog })
}
func useAlert() { installAlert.Do(func() { base.PaintAlertDialog = paintAlertDialog }) }
func useDrawer() {
	installDrawer.Do(func() { base.PaintDrawer = paintDrawer; base.DrawerBoxes = drawerBoxes })
}
func usePopover() { installPopover.Do(func() { base.BuildPopover = framePopover }) }
func useColor() {
	installColor.Do(func() { base.PaintSwatch = paintSwatch; base.PaintPickerPanel = paintColorPickerPanel })
}
func useCombo()  { useControls(); installCombo.Do(func() { base.PaintCombo = comboStateDraw }) }
func useScroll() { installScroll.Do(func() { base.PaintScrollbars = paintScrollbars }) }

// InstallPainters installs every painter the styled layer owns. Applications
// get what they use from the ui constructors; this is for a host that drives
// base directly, and for the engine's tests.
func InstallPainters() {
	useControls()
	useField()
	useSelect()
	useTabs()
	useTable()
	useTree()
	useMenu()
	useDialog()
	useAlert()
	useDrawer()
	usePopover()
	useColor()
	useCombo()
	useScroll()
}

// drawerBoxes is where a drawer's parts go, from the component's layout.
func drawerBoxes(spec base.DrawerDeclaration, surface *gfx.Canvas, palette Theme, bounds Rect, layer uint16) base.DrawerRects {
	layout := drawerComponent(&spec, surface, palette, bounds, layer).Layout(spec.Description)
	return base.DrawerRects{Content: layout.Content, Body: layout.Body, Footer: layout.Footer}
}
