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
var installButton, installBadge, installPagination, installSplitButton,
	installCheckbox, installSwitch, installToggle, installSegmented,
	installTextEntry, installIndicators, installSplitter, installSlider,
	installTooltip, installField, installSelect, installTabs, installTable,
	installTree, installMenu, installDialog, installAlert, installDrawer,
	installPopover, installColor, installCombo, installScroll sync.Once

// useTooltip installs the tooltip painter. Any element can be given a tooltip,
// so a page asks for it once when it is made rather than through a control.
func useTooltip() { installTooltip.Do(func() { base.PaintTooltip = paintTooltip }) }

func useButton() {
	installButton.Do(func() {
		base.SetControlPainter(KindButton, paintButtonControl)
		base.SetControlMeasurer(KindButton, measureButtonNode)
		base.ControlBaseline = controlBaseline
		base.ButtonLookFn = frameButtonLook
	})
}

func useBadge() {
	installBadge.Do(func() {
		base.SetControlPainter(KindBadge, paintBadgeControl)
		base.SetControlMeasurer(KindBadge, measureBadgeNode)
		base.BadgeLookFn = frameBadgeLook
		base.ControlBaseline = controlBaseline
	})
}

func usePagination() {
	installPagination.Do(func() {
		base.SetControlPainter(KindPagination, paintPaginationControl)
		base.SetControlMeasurer(KindPagination, measurePaginationNode)
	})
}

func useSplitButton() {
	installSplitButton.Do(func() {
		base.SetControlPainter(KindSplitButton, paintSplitButtonControl)
		base.SetControlMeasurer(KindSplitButton, measureSplitButtonNode)
	})
}

func useCheckbox() {
	installCheckbox.Do(func() {
		base.SetControlPainter(KindCheckbox, paintCheckboxControl)
		base.SetControlMeasurer(KindCheckbox, measureCheckboxNode)
		base.CheckboxLookFn = frameCheckboxLook
		base.ControlSurfaceRect = controlSurfaceRect
	})
}

func useSwitch() {
	installSwitch.Do(func() {
		base.SetControlPainter(KindSwitch, paintSwitchControl)
		base.SetControlMeasurer(KindSwitch, measureSwitchNode)
		base.SwitchLookFn = frameSwitchLook
		base.ControlSurfaceRect = controlSurfaceRect
	})
}

func useToggle() {
	installToggle.Do(func() {
		base.SetControlPainter(KindToggle, paintToggleControl)
		base.SetControlMeasurer(KindToggle, measureToggleNode)
		base.ToggleLookFn = frameToggleLook
		base.ControlBaseline = controlBaseline
	})
}

func useSegmented() {
	installSegmented.Do(func() {
		base.SetControlPainter(KindSegmented, paintSegmentedControl)
		base.SetControlMeasurer(KindSegmented, measureSegmentedNode)
	})
}

// useTextEntry installs the looks and measurements of the text-entry controls:
// a single-line field, a field with addons, and the rows of a textarea.
func useTextEntry() {
	installTextEntry.Do(func() {
		base.SetControlPainter(KindInput, paintInputControl)
		base.SetControlMeasurer(KindInput, measureInputNode)
		base.SetControlPainter(KindInputGroup, paintInputGroupControl)
		base.SetControlMeasurer(KindInputGroup, measureInputGroupNode)
		base.SetControlPainter(KindTextarea, paintTextareaControl)
		base.SetControlMeasurer(KindTextarea, measureTextareaNode)
	})
}

func useIndicators() {
	installIndicators.Do(func() {
		base.SetControlPainter(KindProgress, paintProgressControl)
		base.SetControlMeasurer(KindProgress, measureProgressNode)
		base.SetControlPainter(KindSpinner, paintSpinnerControl)
		base.SetControlMeasurer(KindSpinner, measureSpinnerNode)
	})
}

func useSplitter() {
	installSplitter.Do(func() {
		base.SetControlPainter(KindSplitter, paintSplitterControl)
		base.SetControlMeasurer(KindSplitter, measureSplitterNode)
	})
}

func useSlider() {
	installSlider.Do(func() {
		base.SetControlPainter(KindSlider, paintSliderControl)
		base.SetControlMeasurer(KindSlider, measureSliderNode)
		base.SliderPointer = sliderPointer
	})
}

func useField() {
	useTextEntry()
	installField.Do(func() { base.EntryLook = frameTextEntryLook; base.PaintAddons = paintAddons })
}
func useSelect() {
	installSelect.Do(func() {
		base.SetControlPainter(KindSelect, paintWidgetControl)
		base.SetControlMeasurer(KindSelect, measureWidgetNode)
		base.SelectLookFn = frameSelectLook
		base.PaintSelect = paintSelect
		base.MeasureSelect = measureSelect
	})
}
func useTabs() {
	installTabs.Do(func() {
		base.SetControlPainter(KindTabs, paintWidgetControl)
		base.SetControlMeasurer(KindTabs, measureWidgetNode)
		base.TabsLookFn = frameTabsLook
		base.PaintTabs = paintTabs
	})
}
func useTable() {
	installTable.Do(func() {
		base.SetControlPainter(KindTable, paintWidgetControl)
		base.SetControlMeasurer(KindTable, measureWidgetNode)
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
func useCombo()  { useField(); installCombo.Do(func() { base.PaintCombo = comboStateDraw }) }
func useScroll() { installScroll.Do(func() { base.PaintScrollbars = paintScrollbars }) }

// InstallPainters installs every painter the styled layer owns. Applications
// get what they use from the ui constructors; this is for a host that drives
// base directly, and for the engine's tests.
func InstallPainters() {
	useButton()
	useBadge()
	usePagination()
	useSplitButton()
	useCheckbox()
	useSwitch()
	useToggle()
	useSegmented()
	useField()
	useIndicators()
	useSplitter()
	useSlider()
	useTooltip()
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
