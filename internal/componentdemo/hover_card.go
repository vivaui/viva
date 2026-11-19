package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/hovercard"
)

var hoverCardComponent = Component{
	ID: "hover-card", Title: "Hover Card", Category: "Overlays", Summary: "带延迟、可进入内容的页面锚点悬停预览卡。",
	paint: (*State).paintHoverCard, pointer: (*State).pointerHoverCard,
	Examples: []Example{
		{ID: "default", Title: "个人资料", Description: "页面 HoverCard 使用锚点几何，在指针停留后打开，并允许指针进入卡片。", Snippet: `anchor := ui.Button(c, "@viva-framework").Outline().Width(180)
hovercard.View(c, "@viva-framework", &hovercard.Card{
    Anchor:      anchor,
    Title:       "Viva Framework",
    Description: "Go-first, GPU-native desktop UI.",
    Body: func() {
        ui.Text(c, "Native • Retained • Allocation-aware").Muted()
    },
})`, PageSnippet: `anchor := ui.Button(c, "@viva-framework").Outline().Width(180)
hovercard.View(c, "@viva-framework", &hovercard.Card{
    Anchor:      anchor,
    Title:       "Viva Framework",
    Description: "Go-first, GPU-native desktop UI.",
    Body: func() {
        ui.Text(c, "Native • Retained • Allocation-aware").Muted()
    },
    Open: &m.hoverOpen,
})`, PageSnippetZh: `anchor := ui.Button(c, "@viva-framework").Outline().Width(180)
hovercard.View(c, "@viva-framework", &hovercard.Card{
    Anchor:      anchor,
    Title:       "Viva Framework",
    Description: "Go 优先、GPU 原生的桌面 UI。",
    Body: func() {
        ui.Text(c, "原生 • 保留式 • 关注分配").Muted()
    },
    Open: &m.hoverOpen,
})`},
		{ID: "sides", Title: "侧边放置", Description: "右侧放置并在窄视口内由页面弹层收进边缘。", Snippet: `anchor := ui.Button(c, "@viva-framework").Outline().Width(180)
card := &hovercard.Card{
    Anchor:      anchor,
    Title:       "Viva Framework",
    Description: "Placed right and clamped to the viewport.",
    Body: func() {
        ui.Text(c, "@viva-framework").Muted()
    },
}
card.Side(hovercard.Right).Align(hovercard.Center)
hovercard.View(c, "@viva-framework", card)`, PageSnippet: `anchor := ui.Button(c, "@viva-framework").Outline().Width(180)
card := &hovercard.Card{
    Anchor:      anchor,
    Title:       "Viva Framework",
    Description: "Placed right and clamped to the viewport.",
    Body: func() {
        ui.Text(c, "@viva-framework").Muted()
    },
    Open: &m.hoverOpen,
}
card.Side(hovercard.Right).Align(hovercard.Center)
hovercard.View(c, "@viva-framework", card)`, PageSnippetZh: `anchor := ui.Button(c, "@viva-framework").Outline().Width(180)
card := &hovercard.Card{
    Anchor:      anchor,
    Title:       "Viva Framework",
    Description: "放在右侧，并在视口边缘内收。",
    Body: func() {
        ui.Text(c, "@viva-framework").Muted()
    },
    Open: &m.hoverOpen,
}
card.Side(hovercard.Right).Align(hovercard.Center)
hovercard.View(c, "@viva-framework", card)`},
		{ID: "disabled", Title: "禁用触发器", Description: "禁用声明不打开预览卡。", Snippet: `anchor := ui.Button(c, "@viva-framework").Outline().Width(180)
hovercard.View(c, "@viva-framework", (&hovercard.Card{
    Anchor:      anchor,
    Title:       "Viva Framework",
    Description: "A disabled trigger shows no preview.",
}).Disabled())`, PageSnippet: `anchor := ui.Button(c, "@viva-framework").Outline().Width(180)
hovercard.View(c, "@viva-framework", (&hovercard.Card{
    Anchor:      anchor,
    Title:       "Viva Framework",
    Description: "A disabled trigger shows no preview.",
}).Disabled())`, PageSnippetZh: `anchor := ui.Button(c, "@viva-framework").Outline().Width(180)
hovercard.View(c, "@viva-framework", (&hovercard.Card{
    Anchor:      anchor,
    Title:       "Viva Framework",
    Description: "禁用的触发器不显示预览。",
}).Disabled())`},
	},
}

var (
	hoverCardUI     = tree.Feature("hover-card")
	hoverCardAnchor = tree.ID(hoverCardUI, "anchor")
)

// pointerHoverCard keeps the example's typed pointer state for the demo's
// event tests. The card's hover timing is owned by hovercard.View.
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

// hoverCardDemo is the demo's page-owned declaration: one card and one stable
// body function, so a warm paint reuses them instead of allocating a closure
// and a card every frame.
type hoverCardDemo struct {
	card     hovercard.Card
	body     func()
	bodyC    ui.C
	bodyText string
}

func (demo *hoverCardDemo) paintBody() {
	ui.Text(demo.bodyC, demo.bodyText).Muted()
}

func (state *State) paintHoverCard(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	state.drawPage(canvas, theme, size, 760, state.Text("Hover Card · 悬停预览", "Hover Card · Hover preview"), func(c ui.C) {
		anchor := ui.Button(c, "@viva-framework").Outline().Width(180).Self(ui.Start).Hit(hoverCardAnchor, "@viva-framework")
		demo := ui.State[hoverCardDemo](c, "hover-card-demo")
		if demo.body == nil {
			demo.body = demo.paintBody
			demo.card.Body = demo.body
		}
		demo.bodyC = c
		demo.bodyText = state.Text("原生 • 保留式 • 关注分配", "Native • Retained • Allocation-aware")
		demo.card.Anchor = anchor
		demo.card.Title = state.Text("Viva Framework", "Viva Framework")
		demo.card.Description = state.Text("Go 优先、GPU 原生的桌面 UI。", "Go-first, GPU-native desktop UI.")
		demo.card.Width(min(float32(280), size.Width-32))
		demo.card.Side(hovercard.Bottom).Align(hovercard.Center)
		demo.card.Disabled(state.exampleID == "disabled")
		if state.exampleID == "sides" {
			demo.card.Side(hovercard.Right)
		}
		hovercard.View(c, "@viva-framework", &demo.card)
	})
}
