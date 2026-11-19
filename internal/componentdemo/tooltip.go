package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var tooltipComponent = Component{
	ID: "tooltip", Title: "Tooltip", Category: "Overlay", Summary: "由调用方决定开启时机的同步提示浮层。",
	paint: (*State).paintTooltip,
	Examples: []Example{
		{ID: "default", Title: "悬停提示", Description: "悬停或点击触发器显示顶部提示。", Snippet: `trigger := tree.ID(page, "trigger")
canvas.HitWithOptions(trigger, anchor, "Show tooltip", gfx.HitOptions{Role: gfx.RoleButton})
if canvas.Clicked(trigger) {
    open = !open
}
if open || canvas.Hovered() == trigger {
    components.BeginTooltip(canvas, theme, viewport, anchor,
        components.TooltipOptions{Layer: 60, Side: components.TooltipTop, MaxWidth: 176}).
        Paint("Native synchronous tooltip")
}`, PageSnippet: `ui.Button(c, "Hover or click").
    Tooltip("Native synchronous tooltip, no background timer").
    TooltipOn(ui.TooltipTop)`, PageSnippetZh: `ui.Button(c, "悬停或点击").
    Tooltip("原生同步提示，无后台延时器").
    TooltipOn(ui.TooltipTop)`},
		{ID: "sides", Title: "四个方向", Description: "同屏展示上、右、下、左四种放置方向。", Height: 340, Snippet: `labels := [...]string{"Top", "Right", "Bottom", "Left"}
sides := [...]components.TooltipSide{
    components.TooltipTop, components.TooltipRight,
    components.TooltipBottom, components.TooltipLeft,
}
for index, anchor := range anchors {
    components.BeginTooltip(canvas, theme, viewport, anchor, components.TooltipOptions{
        Layer: uint16(60 + index), Side: sides[index], MaxWidth: 96,
    }).Paint(labels[index])
}`, PageSnippet: `ui.Button(c, "Top").Tooltip("Top").TooltipOn(ui.TooltipTop)
ui.Button(c, "Right").Tooltip("Right").TooltipOn(ui.TooltipRight)
ui.Button(c, "Bottom").Tooltip("Bottom").TooltipOn(ui.TooltipBottom)
ui.Button(c, "Left").Tooltip("Left").TooltipOn(ui.TooltipLeft)`, PageSnippetZh: `ui.Button(c, "Top").Tooltip("Top").TooltipOn(ui.TooltipTop)
ui.Button(c, "Right").Tooltip("Right").TooltipOn(ui.TooltipRight)
ui.Button(c, "Bottom").Tooltip("Bottom").TooltipOn(ui.TooltipBottom)
ui.Button(c, "Left").Tooltip("Left").TooltipOn(ui.TooltipLeft)`},
		{ID: "clamped", Title: "视口约束", Description: "靠近边缘时将提示内容限制在视口内。", Snippet: `anchor := ui.Slot(c).Size(60, theme.Metrics.ControlHeight).Self(ui.Start).Rect()
anchor.X, anchor.Width = size.Width-76, 60 // pin the trigger to the window edge
components.BeginTooltip(canvas, theme, viewport, anchor, components.TooltipOptions{
    Layer: 60, Side: components.TooltipBottom, MaxWidth: 176,
}).Paint("Clamped to the viewport")`, PageSnippet: `ui.Button(c, "Hover or click").Self(ui.End).
    Tooltip("Clamped to the viewport").
    TooltipOn(ui.TooltipBottom)`, PageSnippetZh: `ui.Button(c, "悬停或点击").Self(ui.End).
    Tooltip("限制在视口内").
    TooltipOn(ui.TooltipBottom)`},
	},
}

var (
	tooltipUI      = tree.Feature("tooltip")
	tooltipTrigger = tree.ID(tooltipUI, "trigger")
)

func (state *State) paintTooltip(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	state.drawPage(canvas, theme, size, 520, state.Text("Tooltip · 同步浮层", "Tooltip · Synchronous overlay"), func(c ui.C) {
		if state.exampleID == "sides" {
			// A cross with room between the controls: each tip opens away from
			// the cross instead of over a neighbouring button.
			ui.Column(c).Gap(44).Items(ui.Center).Children(func() {
				ui.Button(c, "Top").Tooltip("Top").TooltipOn(ui.TooltipTop)
				ui.Row(c).Gap(180).Children(func() {
					ui.Button(c, "Left").Tooltip("Left").TooltipOn(ui.TooltipLeft)
					ui.Button(c, "Right").Tooltip("Right").TooltipOn(ui.TooltipRight)
				})
				ui.Button(c, "Bottom").Tooltip("Bottom").TooltipOn(ui.TooltipBottom)
			})
			return
		}
		label := state.Text("悬停或点击", "Hover or click")
		if state.exampleID == "clamped" {
			// The point of this example is a trigger at the window's edge; the
			// page clamps the tip inside the viewport.
			ui.Button(c, label).Self(ui.End).Hit(tooltipTrigger, label).
				Tooltip(state.Text("限制在视口内", "Clamped to the viewport")).TooltipOn(ui.TooltipBottom)
			return
		}
		ui.Button(c, label).Self(ui.Start).Hit(tooltipTrigger, label).
			Tooltip(state.Text("原生同步提示，无后台延时器", "Native synchronous tooltip, no background timer")).
			TooltipOn(ui.TooltipTop)
		caption(c, state.Text("开启策略和延时由调用方决定。", "Open policy and delay are caller-owned."))
	})
}
