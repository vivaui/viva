package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var markerComponent = Component{
	ID: "marker", Title: "Marker", Category: "Display", Summary: "用于分组和时间线的轻量元数据标记。",
	paint: (*State).paintMarker,
	Examples: []Example{
		{ID: "default", Title: "元数据", Description: "图标与文本组成的标准标记。", Snippet: `// clockIcon is a caller-owned gfx.Icon; a nil icon paints none.
components.Marker(canvas, theme, bounds, components.MarkerDefault,
    &clockIcon, "Updated 2 minutes ago")`},
		{ID: "variants", Title: "三种形式", Description: "普通、居中分隔和底部边框形式。", Snippet: `// The separator centers its content between rules; the border draws one below.
components.Marker(canvas, theme, bounds[0], components.MarkerDefault,
    &checkIcon, "Default")
components.Marker(canvas, theme, bounds[1], components.MarkerSeparator,
    nil, "October 2026")
components.Marker(canvas, theme, bounds[2], components.MarkerBorder,
    &clockIcon, "Release channel")`},
		{ID: "progress", Title: "进行中", Description: "用 Spinner 代替图标表示正在进行的工作。", Snippet: `// Advance the phase from an animation frame and request the next paint.
phase = (phase + 1) % 8
components.MarkerProgress(canvas, theme, bounds, components.MarkerDefault, phase, "Thinking")`},
	},
}

// Lucide's check and clock, compiled to the gfx.Icon form.
var (
	markerCheckIcon = gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{
		gfx.MoveTo(20, 6.5), gfx.LineTo(9, 17.5), gfx.LineTo(4, 12.5),
	}}
	markerClockIcon = gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: append(demoCircle(12, 12, 10),
		gfx.MoveTo(12, 6), gfx.LineTo(12, 12), gfx.LineTo(16, 14),
	)}
)

// demoCircle is a circle of radius r as four cubic curves.
func demoCircle(cx, cy, r float32) []gfx.PathCommand {
	k := r * 0.5523
	return []gfx.PathCommand{
		gfx.MoveTo(cx+r, cy),
		gfx.CubicTo(cx+r, cy+k, cx+k, cy+r, cx, cy+r),
		gfx.CubicTo(cx-k, cy+r, cx-r, cy+k, cx-r, cy),
		gfx.CubicTo(cx-r, cy-k, cx-k, cy-r, cx, cy-r),
		gfx.CubicTo(cx+k, cy-r, cx+r, cy-k, cx+r, cy),
	}
}

func (state *State) paintMarker(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	var rows [3]ui.Box
	state.drawPage(canvas, theme, size, 560, state.Text("Marker · 内容分组", "Marker · Content grouping"), func(c ui.C) {
		count := 3
		if state.exampleID != "variants" {
			count = 1
		}
		rowHeight := theme.Metrics.FontSizeM + 7
		for index := range count {
			height := rowHeight
			if state.exampleID == "variants" && index == 2 {
				height += 8*(theme.Metrics.ControlHeight/32) + 1 // MarkerBorder's rule
			}
			rows[index] = ui.Slot(c).FillWidth().Height(height)
		}
	})
	switch state.exampleID {
	case "default":
		components.Marker(canvas, theme, rows[0].Rect(), components.MarkerDefault, &markerClockIcon, "Updated 2 minutes ago")
	case "progress":
		// A fixed frame: the demo does not schedule frames, an application turns the phase.
		components.MarkerProgress(canvas, theme, rows[0].Rect(), components.MarkerDefault, 2, "Thinking")
	default:
		components.Marker(canvas, theme, rows[0].Rect(), components.MarkerDefault, &markerCheckIcon, "Default")
		components.Marker(canvas, theme, rows[1].Rect(), components.MarkerSeparator, nil, "October 2026")
		components.Marker(canvas, theme, rows[2].Rect(), components.MarkerBorder, &markerClockIcon, "Release channel")
	}
}
