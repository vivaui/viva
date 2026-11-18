package ui

import "github.com/vivaui/viva/base"

// Band is a compositing band for an application's own overlay; package base
// holds its definition and ui re-exports it.
type Band = base.Band

const (
	LayerContent    = base.LayerContent
	LayerPaneChrome = base.LayerPaneChrome
	LayerDrag       = base.LayerDrag
)

// widgetLayerBase is the first layer Viva's own widgets paint on at the top of
// a page, from package base.
const widgetLayerBase = base.WidgetLayerBase

// widgetLayer is the layer a widget's children paint on: the widget's own
// layer when it is declared inside one, else the base above every app band.
func (f *frame) WidgetLayer() uint16 { return max(widgetsOf(f).layer, widgetLayerBase) }
