package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var collapsibleComponent = Component{
	ID: "collapsible", Title: "Collapsible", Category: "Disclosure", Summary: "无装饰触发器与调用方绘制内容的折叠原语。",
	paint: (*State).paintCollapsible, initialize: initializeCollapsible,
	Examples: []Example{
		{ID: "default", Title: "构建详情", Description: "点击标题展开或收起调用方内容。", Snippet: `trigger := tree.ID(page, "trigger")
collapsible := components.BeginCollapsible(canvas, theme, trigger, bounds)
content := collapsible.Paint("3 build artifacts", open, 112)
if canvas.Clicked(trigger) {
    open = !open
}
canvas.Rounded(content, theme.Metrics.RadiusSurface, theme.Muted)`},
		{ID: "disabled", Title: "禁用状态", Description: "保持展开快照但禁止触发。", Snippet: `trigger := tree.ID(page, "trigger")
collapsible := components.BeginCollapsible(canvas, theme, trigger, bounds).Disabled()
content := collapsible.Paint("3 build artifacts", open, 112)
if content.Height > 0 {
    canvas.Rounded(content, theme.Metrics.RadiusSurface, theme.Muted)
}`},
	},
}

var (
	collapsibleUI      = tree.Feature("collapsible")
	collapsibleTrigger = tree.ID(collapsibleUI, "trigger")
)

func initializeCollapsible(state *State) { state.flags = 1 }

func (state *State) paintCollapsible(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	open := state.flags&1 != 0
	const contentHeight = 112
	var slot ui.Box
	state.drawPage(canvas, theme, size, 520, state.Text("Collapsible · 构建输出", "Collapsible · Build output"), func(c ui.C) {
		height := theme.Metrics.ControlHeight
		if open {
			height += contentHeight
		}
		slot = ui.Slot(c).FillWidth().Height(height)
	})
	collapsible := components.BeginCollapsible(canvas, theme, collapsibleTrigger, slot.Rect())
	disabled := state.exampleID == "disabled"
	if disabled {
		collapsible = collapsible.Disabled()
	}
	content := collapsible.Paint(state.Text("3 个构建产物", "3 build artifacts"), open, contentHeight)
	if !disabled && canvas.Clicked(collapsibleTrigger) {
		state.flags ^= 1
	}
	if content.Height == 0 {
		return
	}
	canvas.Rounded(content, theme.Metrics.RadiusSurface, theme.Muted)
	for index, line := range [...]string{
		"viva-darwin-arm64      3.8 MiB", "viva-windows-amd64     4.1 MiB", "viva-linux-amd64       3.9 MiB",
	} {
		canvas.Label(line, gfx.Rect{X: content.X + 14, Y: content.Y + 12 + float32(index)*30, Width: content.Width - 28, Height: 22}, gfx.TextStyle{Size: theme.Metrics.FontSizeM, Color: theme.Foreground})
	}
}
