package style

import "github.com/vivaui/viva/gfx"

// A tab strip's metrics: the list's height and padding, its triggers' height and
// padding, the gap between line-variant triggers, and the line indicator that the
// line variant draws just past the active trigger. They are Vega's numbers.
const (
	TabsListHeight     = 36
	TabsListPadding    = 3
	TabsTriggerHeight  = 30
	TabsTriggerPadding = 9
	TabsLineGap        = 4
	TabsIndicator      = 2
	TabsLineDrop       = 3
)

// TabsGeometry is what a theme's metrics give a tab strip: its height, the
// padding inside the list, a trigger's height and padding either side of its
// content, the gap between line-variant triggers, and the size and weight of a
// trigger's label.
func TabsGeometry(m gfx.Metrics) (listHeight, listPadding, triggerHeight, triggerPadding, lineGap, fontSize, weight float32) {
	return TabsListHeight, TabsListPadding, TabsTriggerHeight, TabsTriggerPadding, TabsLineGap, m.FontSizeM, gfx.WeightMedium
}

// TabsListRadius is the corner radius of the strip behind its triggers, and
// TabsTriggerRadius the radius of one trigger's surface.
func TabsListRadius(m gfx.Metrics) float32    { return m.RadiusControl + 2 }
func TabsTriggerRadius(m gfx.Metrics) float32 { return m.RadiusControl }
