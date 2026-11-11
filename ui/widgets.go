package ui

import "github.com/vivaui/viva/gfx"

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
	drawers  []drawerDecl
	popovers []popoverDecl
	// popoverFocus holds the panels whose InitialFocusID was already asked for.
	// It outlives reset because a panel is open across paints, and is swept at
	// the next Begin for the panels the last paint no longer declared.
	popoverFocus []gfx.ID
	splits       []splitDecl
	sliders      []sliderDecl
	cells        []string // one row's text, handed to components.Table
	rects        []Rect   // one row's cells
	rows         []Rect   // the rows of every open SelectBase list in the frame
	layer        uint16   // the layer the declaration is in: a Dialog raises it for its body
}

func (w *widgets) reset() {
	w.tables, w.tabs, w.selects, w.dialogs = w.tables[:0], w.tabs[:0], w.selects[:0], w.dialogs[:0]
	w.popovers, w.drawers = w.popovers[:0], w.drawers[:0]
	w.splits, w.sliders, w.rows = w.splits[:0], w.sliders[:0], w.rows[:0]
	w.layer = 0
}

// popoverFocusKey names an open panel for the once-only InitialFocusID request:
// its focus scope, or the control itself when the panel has no scope.
func popoverFocusKey(spec *PopoverSpec) gfx.ID {
	if !spec.Scope.IsZero() {
		return spec.Scope
	}
	return spec.InitialFocusID
}

// sweepPopoverFocus forgets the panels the last paint no longer declared, so a
// panel that opens again, or another popover, asks for the keyboard once more.
func (w *widgets) sweepPopoverFocus() {
	kept := w.popoverFocus[:0]
	for _, key := range w.popoverFocus {
		for i := range w.popovers {
			if popoverFocusKey(&w.popovers[i].spec) == key {
				kept = append(kept, key)
				break
			}
		}
	}
	w.popoverFocus = kept
}

// popoverAsked reports whether this panel already asked for its InitialFocusID.
func (w *widgets) popoverAsked(key gfx.ID) bool {
	for _, seen := range w.popoverFocus {
		if seen == key {
			return true
		}
	}
	return false
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
