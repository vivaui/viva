package ui

import "github.com/vivaui/viva/base"

func paintButtonControl(f *Frame, n *Node, _ Rect)      { paintButton(f, n) }
func paintInputControl(f *Frame, n *Node, _ Rect)       { paintInput(f, n) }
func paintTextareaControl(f *Frame, n *Node, _ Rect)    { paintTextarea(f, n) }
func paintProgressControl(f *Frame, n *Node, _ Rect)    { paintProgress(f, n) }
func paintSpinnerControl(f *Frame, n *Node, _ Rect)     { paintSpinner(f, n) }
func paintSplitterControl(f *Frame, n *Node, _ Rect)    { f.PaintSplitter(n) }
func paintSliderControl(f *Frame, n *Node, _ Rect)      { paintSlider(f, n) }
func paintWidgetControl(f *Frame, n *Node, _ Rect)      { f.PaintWidget(n) }
func paintSplitButtonControl(f *Frame, n *Node, _ Rect) { paintSplitButton(f, base.ViewOf(f, n)) }
func paintBadgeControl(f *Frame, n *Node, _ Rect)       { paintBadge(f, n) }
func paintPaginationControl(f *Frame, n *Node, _ Rect)  { paintPagination(f, base.ViewOf(f, n)) }
func paintCheckboxControl(f *Frame, n *Node, _ Rect)    { paintCheckbox(f, base.ViewOf(f, n), n) }
func paintSwitchControl(f *Frame, n *Node, _ Rect)      { paintSwitch(f, base.ViewOf(f, n), n) }
func paintToggleControl(f *Frame, n *Node, _ Rect)      { paintToggle(f, n) }
func paintInputGroupControl(f *Frame, n *Node, _ Rect)  { paintInputGroup(f, base.ViewOf(f, n)) }
func paintSegmentedControl(f *Frame, n *Node, _ Rect)   { paintSegmented(f, base.ViewOf(f, n)) }
