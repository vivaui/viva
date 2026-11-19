package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/markerview"
)

var markerComponent = Component{
	ID: "marker", Title: "Marker", Category: "Display", Summary: "用于分组和时间线的轻量元数据标记。",
	paint: (*State).paintMarker,
	Examples: []Example{
		{ID: "default", Title: "元数据", Description: "图标与文本组成的标准标记。", Snippet: `// m.clockIcon is a caller-owned gfx.Icon; a nil icon paints none.
components.Marker(canvas, theme, bounds, components.MarkerDefault,
    &m.clockIcon, "Updated 2 minutes ago")`, PageSnippet: `m.marker.Reset().Icon(m.clockIcon).Label("Updated 2 minutes ago")
m.marker.View(c, "Updated 2 minutes ago")`, PageSnippetZh: `m.marker.Reset().Icon(m.clockIcon).Label("Updated 2 minutes ago")
m.marker.View(c, "Updated 2 minutes ago")`},
		{ID: "variants", Title: "三种形式", Description: "普通、居中分隔和底部边框形式。", Snippet: `// The separator centers its content between rules; the border draws one below.
components.Marker(canvas, theme, bounds[0], components.MarkerDefault,
    &m.checkIcon, "Default")
components.Marker(canvas, theme, bounds[1], components.MarkerSeparator,
    nil, "October 2026")
components.Marker(canvas, theme, bounds[2], components.MarkerBorder,
    &m.clockIcon, "Release channel")`, PageSnippet: `m.markers[0].Reset().Icon(m.checkIcon).Label("Default")
m.markers[0].View(c, "Default")
m.markers[1].Reset().Separator().Label("October 2026")
m.markers[1].View(c, "October 2026")
m.markers[2].Reset().Border().Icon(m.clockIcon).Label("Release channel")
m.markers[2].View(c, "Release channel")`, PageSnippetZh: `m.markers[0].Reset().Icon(m.checkIcon).Label("Default")
m.markers[0].View(c, "Default")
m.markers[1].Reset().Separator().Label("October 2026")
m.markers[1].View(c, "October 2026")
m.markers[2].Reset().Border().Icon(m.clockIcon).Label("Release channel")
m.markers[2].View(c, "Release channel")`},
		{ID: "progress", Title: "进行中", Description: "Spinner 取代图标表示正在进行的工作；调用方拥有相位和帧。", Snippet: `// Advance the phase from an animation frame and request the next paint.
phase = (phase + 1) % 8
components.MarkerProgress(canvas, theme, bounds, components.MarkerDefault, phase, "Thinking")`, PageSnippet: `m.marker.Reset().Progress(m.phase).Label("Thinking")
m.marker.View(c, "Thinking")`, PageSnippetZh: `m.marker.Reset().Progress(m.phase).Label("Thinking")
m.marker.View(c, "Thinking")`},
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

// markerDemo is the demo's reusable marker declaration. State cannot hold it
// without widening the preview model, so the demo keeps configured views and
// resets them in every paint; their handlers are made once. A view belongs to
// one place, so the variants example keeps one per row. The progress example
// shows a fixed frame: the demo schedules no frames, an application advances
// the phase.
var (
	markerDemo     = markerview.New()
	markerVariants [3]markerview.Marker
)

func (state *State) paintMarker(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	state.drawPage(canvas, theme, size, 560, state.Text("Marker · 内容分组", "Marker · Content grouping"), func(c ui.C) {
		switch state.exampleID {
		case "default":
			markerDemo.Reset().Icon(markerClockIcon).Label("Updated 2 minutes ago").View(c, "Updated 2 minutes ago")
		case "progress":
			markerDemo.Reset().Progress(2).Label("Thinking").View(c, "Thinking")
		default:
			markerVariants[0].Reset().Icon(markerCheckIcon).Label("Default").View(c, "Default")
			markerVariants[1].Reset().Separator().Label("October 2026").View(c, "October 2026")
			markerVariants[2].Reset().Border().Icon(markerClockIcon).Label("Release channel").View(c, "Release channel")
		}
	})
}
