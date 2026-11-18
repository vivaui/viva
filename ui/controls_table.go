package ui

import "github.com/vivaui/viva/base"

// paintControls paints the controls: each kind's look, from package components
// for the ones that ship with a theme, and from the caller's own painter for
// the ones it declared. The engine calls it through base.ControlPainter.
func paintControls(f *Frame, n *Node, r Rect) {
	if painter := controlPainters[n.Kind]; painter != nil {
		painter(f, n, r)
	}
}

var controlPainters = [...]func(f *Frame, n *Node, r Rect){
	KindButton:      paintButtonControl,
	KindCheckbox:    paintCheckboxControl,
	KindSwitch:      paintSwitchControl,
	KindInput:       paintInputControl,
	KindInputGroup:  paintInputGroupControl,
	KindTextarea:    paintTextareaControl,
	KindSegmented:   paintSegmentedControl,
	KindTable:       paintWidgetControl,
	KindTabs:        paintWidgetControl,
	KindSelect:      paintWidgetControl,
	KindProgress:    paintProgressControl,
	KindSpinner:     paintSpinnerControl,
	KindSplitter:    paintSplitterControl,
	KindSplitButton: paintSplitButtonControl,
	KindBadge:       paintBadgeControl,
	KindPagination:  paintPaginationControl,
	KindSlider:      paintSliderControl,
}

func paintButtonControl(f *Frame, n *Node, _ Rect)      { paintButton(f, n) }
func paintInputControl(f *Frame, n *Node, _ Rect)       { paintInput(f, n) }
func paintTextareaControl(f *Frame, n *Node, _ Rect)    { paintTextarea(f, n) }
func paintProgressControl(f *Frame, n *Node, _ Rect)    { paintProgress(f, n) }
func paintSpinnerControl(f *Frame, n *Node, _ Rect)     { paintSpinner(f, n) }
func paintSplitterControl(f *Frame, n *Node, _ Rect)    { f.PaintSplitter(n) }
func paintSliderControl(f *Frame, n *Node, _ Rect)      { paintSlider(f, n) }
func paintWidgetControl(f *Frame, n *Node, _ Rect)      { f.PaintWidget(n) }
func paintSplitButtonControl(f *Frame, n *Node, _ Rect) { paintSplitButton(f, base.ViewOf(f, n)) }
func paintBadgeControl(f *Frame, n *Node, _ Rect)       { paintBadge(f, base.ViewOf(f, n)) }
func paintPaginationControl(f *Frame, n *Node, _ Rect)  { paintPagination(f, base.ViewOf(f, n)) }
func paintCheckboxControl(f *Frame, n *Node, _ Rect)    { paintCheckbox(f, base.ViewOf(f, n)) }
func paintSwitchControl(f *Frame, n *Node, _ Rect)      { paintSwitch(f, base.ViewOf(f, n)) }
func paintInputGroupControl(f *Frame, n *Node, _ Rect)  { paintInputGroup(f, base.ViewOf(f, n)) }
func paintSegmentedControl(f *Frame, n *Node, _ Rect)   { paintSegmented(f, base.ViewOf(f, n)) }
