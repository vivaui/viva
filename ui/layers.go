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
