package flex

import "github.com/vivaui/viva/ui"

// widgets holds the declarations of the components that are painted by package
// components at paint time, rather than built from containers and text: a
// Table, a Tabs strip, a Select and a Dialog. Each is a slice reused every frame,
// like the Frame's input and segmented tables, so declaring them allocates nothing
// once the page has been seen.
type widgets struct {
	tables   []tableDecl
	tabs     []tabsDecl
	selects  []selectDecl
	dialogs  []dialogDecl
	popovers []popoverDecl
	splits   []splitDecl
	sliders  []sliderDecl
	cells    []string  // one row's text, handed to components.Table
	rects    []ui.Rect // one row's cells
	rows     []ui.Rect // the rows of every open SelectBase list in the frame
	layer    uint16    // the layer the declaration is in: a Dialog raises it for its body
}

func (w *widgets) reset() {
	w.tables, w.tabs, w.selects, w.dialogs = w.tables[:0], w.tabs[:0], w.selects[:0], w.dialogs[:0]
	w.popovers = w.popovers[:0]
	w.splits, w.sliders, w.rows = w.splits[:0], w.sliders[:0], w.rows[:0]
	w.layer = 0
}

// measureWidget gives a Table, Tabs or Select the size its content wants.
func (f *Frame) measureWidget(n *node) {
	switch n.kind {
	case kindTable:
		f.measureTable(n)
	case kindTabs:
		f.measureTabs(n)
	case kindSelect:
		f.measureSelect(n)
	}
}

// paintWidget paints a Table, Tabs or Select with the component it declares.
func (f *Frame) paintWidget(n *node) {
	switch n.kind {
	case kindTable:
		f.paintTable(n)
	case kindTabs:
		f.paintTabs(n)
	case kindSelect:
		f.paintSelect(n)
	}
}
