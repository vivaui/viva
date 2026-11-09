package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var tooltipComponent = Component{
	ID: "tooltip", Title: "Tooltip", Category: "Overlay", Summary: "由调用方决定开启时机的同步提示浮层。",
	paint: (*State).paintTooltip, pointer: (*State).pointerTooltip,
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
}`},
		{ID: "sides", Title: "四个方向", Description: "同屏展示上、右、下、左四种放置方向。", Snippet: `labels := [...]string{"Top", "Right", "Bottom", "Left"}
sides := [...]components.TooltipSide{
    components.TooltipTop, components.TooltipRight,
    components.TooltipBottom, components.TooltipLeft,
}
for index, anchor := range anchors {
    components.BeginTooltip(canvas, theme, viewport, anchor, components.TooltipOptions{
        Layer: uint16(60 + index), Side: sides[index], MaxWidth: 96,
    }).Paint(labels[index])
}`},
		{ID: "clamped", Title: "视口约束", Description: "靠近边缘时将提示内容限制在视口内。", Snippet: `anchor := f.Slot().Size(60, theme.Metrics.ControlHeight).Self(ui.Start).Rect()
anchor.X, anchor.Width = size.Width-76, 60 // pin the trigger to the window edge
components.BeginTooltip(canvas, theme, viewport, anchor, components.TooltipOptions{
    Layer: 60, Side: components.TooltipBottom, MaxWidth: 176,
}).Paint("Clamped to the viewport")`},
	},
}

var (
	tooltipUI      = tree.Feature("tooltip")
	tooltipTrigger = tree.ID(tooltipUI, "trigger")
)

func (state *State) pointerTooltip(event gfx.PointerEvent) gfx.EventResult {
	if event.Kind != gfx.PointerMove && event.Kind != gfx.PointerExit {
		return gfx.EventResult{}
	}
	open := event.Kind == gfx.PointerMove && event.Target == tooltipTrigger
	wasOpen := state.flags&2 != 0
	if open {
		state.flags |= 2
	} else {
		state.flags &^= 2
	}
	return gfx.EventResult{Handled: open || wasOpen, Repaint: open != wasOpen}
}

func (state *State) paintTooltip(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	viewport := gfx.Rect{Width: size.Width, Height: size.Height}
	var anchors [4]ui.Box
	state.page(canvas, theme, size, 520, state.Text("Tooltip · 同步浮层", "Tooltip · Synchronous overlay"), func(f *ui.Frame) {
		if state.exampleID == "sides" {
			// A cross: the anchor whose tip hangs below on top, the others in a row.
			f.Column().Gap(36).Items(ui.Center).Children(func() {
				anchors[2] = f.Slot().Size(56, theme.Metrics.ControlHeight)
				f.Row().Gap(48).Children(func() {
					anchors[1] = f.Slot().Size(56, theme.Metrics.ControlHeight)
					anchors[0] = f.Slot().Size(56, theme.Metrics.ControlHeight)
					anchors[3] = f.Slot().Size(56, theme.Metrics.ControlHeight)
				})
			})
			return
		}
		anchors[0] = f.Slot().Size(128, theme.Metrics.ControlHeight).Self(ui.Start)
		caption(f, state.Text("开启策略和延时由调用方决定。", "Open policy and delay are caller-owned."))
	})

	if state.exampleID == "sides" {
		sides := [...]components.TooltipSide{components.TooltipTop, components.TooltipRight, components.TooltipBottom, components.TooltipLeft}
		labels := [...]string{"Top", "Right", "Bottom", "Left"}
		for index, slot := range anchors {
			anchor := slot.Rect()
			canvas.RoundedBorder(anchor, theme.Metrics.RadiusControl, 1, theme.Border, theme.Background)
			canvas.LabelCenter(labels[index], anchor, gfx.TextStyle{Size: theme.Metrics.FontSizeS, Weight: gfx.WeightMedium, Color: theme.Foreground})
			components.BeginTooltip(canvas, theme, viewport, anchor, components.TooltipOptions{Layer: uint16(60 + index), Side: sides[index], MaxWidth: 96}).Paint(labels[index])
		}
		return
	}
	anchor := anchors[0].Rect()
	if state.exampleID == "clamped" {
		// The point of this example is an anchor at the window's edge.
		anchor.X, anchor.Width = size.Width-76, 60
	}
	canvas.RoundedBorder(anchor, theme.Metrics.RadiusControl, 1, theme.Border, theme.Background)
	canvas.LabelCenter(state.Text("悬停或点击", "Hover or click"), anchor, gfx.TextStyle{Size: theme.Metrics.FontSizeM, Weight: gfx.WeightMedium, Color: theme.Foreground})
	canvas.HitWithOptions(tooltipTrigger, anchor, state.Text("显示提示", "Show tooltip"), gfx.HitOptions{Role: gfx.RoleButton})
	if canvas.Clicked(tooltipTrigger) {
		state.flags ^= 1
	}
	if state.flags&3 != 0 || canvas.Hovered() == tooltipTrigger {
		side := components.TooltipTop
		if state.exampleID == "clamped" {
			side = components.TooltipBottom
		}
		components.BeginTooltip(canvas, theme, viewport, anchor, components.TooltipOptions{Layer: 60, Side: side, MaxWidth: 176}).Paint(state.Text("原生同步提示，无后台延时器", "Native synchronous tooltip, no background timer"))
	}
}
