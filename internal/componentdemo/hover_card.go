package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var hoverCardComponent = Component{
	ID: "hover-card", Title: "Hover Card", Category: "Overlays", Summary: "无计时器的同步锚点预览卡。",
	paint: (*State).paintHoverCard, pointer: (*State).pointerHoverCard,
	Examples: []Example{
		{ID: "default", Title: "个人资料", Description: "生产 HoverCard 根据锚点和视口放置预览。", Snippet: `trigger := ui.Button(c, "@viva-framework").Outline().Width(180).Hit(anchorID, "@viva-framework")
if canvas.Hovered() == anchorID {
    card := components.BeginHoverCard(canvas, theme, viewport, trigger.Rect(), components.HoverCardOptions{
        Layer: 1, Side: components.HoverCardBottom, BodyHeight: 24,
    })
    layout := card.Begin("Viva Framework", "Go-first, GPU-native desktop UI.")
    canvas.Label("Native • Retained", layout.Body, labelStyle)
    card.End()
}`},
		{ID: "sides", Title: "侧边放置", Description: "右侧放置在窄视口会自动限制到边缘。", Snippet: `trigger := ui.Button(c, "@viva-framework").Outline().Width(180).Hit(anchorID, "@viva-framework")
card := components.BeginHoverCard(canvas, theme, viewport, trigger.Rect(), components.HoverCardOptions{
    Layer: 1, Side: components.HoverCardRight, Align: components.HoverCardAlignCenter,
    ContentSize: gfx.Size{Width: min(280, size.Width-32)}, BodyHeight: 24,
})
layout := card.Begin("Viva Framework", "Placed right and clamped to the viewport.")
canvas.Label("@viva-framework", layout.Body, labelStyle)
card.End()`},
		{ID: "disabled", Title: "禁用触发器", Description: "禁用声明不绘制预览表面。", Snippet: `trigger := ui.Button(c, "@viva-framework").Outline().Width(180).Hit(anchorID, "@viva-framework")
card := components.BeginHoverCard(canvas, theme, viewport, trigger.Rect(),
    components.HoverCardOptions{Layer: 1, Side: components.HoverCardBottom}).Disabled()
card.Paint("Viva Framework", "A disabled trigger shows no preview.")`},
	},
}

var (
	hoverCardUI     = tree.Feature("hover-card")
	hoverCardAnchor = tree.ID(hoverCardUI, "anchor")
)

func (state *State) pointerHoverCard(event gfx.PointerEvent) gfx.EventResult {
	if event.Kind != gfx.PointerMove && event.Kind != gfx.PointerExit {
		return gfx.EventResult{}
	}
	open := state.exampleID != "disabled" && event.Kind == gfx.PointerMove && event.Target == hoverCardAnchor
	wasOpen := state.flags&1 != 0
	if open {
		state.flags |= 1
	} else {
		state.flags &^= 1
	}
	return gfx.EventResult{Handled: open || wasOpen, Repaint: open != wasOpen}
}

func (state *State) paintHoverCard(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	var trigger ui.Box
	state.drawPage(canvas, theme, size, 760, state.Text("Hover Card · 悬停预览", "Hover Card · Hover preview"), func(c ui.C) {
		trigger = ui.Button(c, "@viva-framework").Outline().Width(180).Self(ui.Start).Hit(hoverCardAnchor, "@viva-framework")
	})
	anchor := trigger.Rect()
	viewport := gfx.Rect{Width: size.Width, Height: size.Height}
	side := components.HoverCardBottom
	align := components.HoverCardAlignStart
	if state.exampleID == "sides" {
		side = components.HoverCardRight
		align = components.HoverCardAlignCenter
	}
	card := components.BeginHoverCard(canvas, theme, viewport, anchor, components.HoverCardOptions{Layer: 1, Side: side, Align: align, ContentSize: gfx.Size{Width: min(float32(280), size.Width-32)}, BodyHeight: 24})
	if state.exampleID == "disabled" {
		card = card.Disabled()
	}
	if state.flags&1 == 0 && canvas.Hovered() != hoverCardAnchor {
		return
	}
	layout := card.Begin("Viva Framework", "Go-first, GPU-native desktop UI.")
	if layout.Body.Width > 0 {
		canvas.Label("Native • Retained • Allocation-aware", layout.Body, gfx.TextStyle{Size: theme.Metrics.FontSizeS, Color: theme.MutedForeground})
	}
	card.End()
}
