package ui

// Band is a compositing band for an application's own overlay. Viva's menus,
// dialogs, popovers, tooltips and toasts keep their own layers above every
// band, so an overlay can sit over the page without claiming a widget's
// dismissal, focus trap or backdrop.
//
// A band orders paint; hits and Tab follow the same order: a higher band is
// hit first and reached later by Tab, and within a band the last declared wins,
// as on the page itself (see Page.Draw). An overlay is declared before the page
// is laid out, like any element; its Bounds are page coordinates.
type Band uint16

const (
	// LayerContent is the page itself. An overlay declared with it is painted
	// in the page's layer; it is rarely what an application wants.
	LayerContent Band = iota
	// LayerPaneChrome is for app chrome over the page: a vertical tab sidebar
	// that expands over the workspace, a floating toolbar. It is below menus,
	// dialogs and popovers.
	LayerPaneChrome
	// LayerDrag is for a drag's ghost and drop indicator: over the page and its
	// chrome, below menus, dialogs and popovers.
	LayerDrag
)

// widgetLayerBase is the first layer Viva's own widgets paint on at the top of
// a page: a Dialog, Drawer, Popover or Menu defaults two above its parent
// layer, and a Select or Combobox list one above. The base must be above every
// application band, so a widget opened over an overlay is not under it.
const widgetLayerBase = 10

// widgetLayer is the layer a widget's children paint on: the widget's own
// layer when it is declared inside one, else the base above every app band.
func (f *frame) widgetLayer() uint16 { return max(f.widgets.layer, widgetLayerBase) }
