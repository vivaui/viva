package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var kbdComponent = Component{
	ID: "kbd", Title: "Kbd", Category: "Display", Summary: "按内容测量的键帽与快捷键组合。",
	paint: (*State).paintKbd,
	Examples: []Example{
		{ID: "default", Title: "快捷键", Description: "使用生产 KbdGroup 组合平台快捷键。", Snippet: `group := components.BeginKbdGroup(canvas, theme, bounds)
group.Add("⌘")
group.Add("⇧")
group.Add("P")`},
		{ID: "keys", Title: "常用按键", Description: "单键和长标签均按内容自然伸展。", Snippet: `x := bounds.X
for _, key := range []string{"Esc", "Enter", "Space", "⌥ Option"} {
    rect := gfx.Rect{X: x, Y: bounds.Y, Height: bounds.Height}
    keycap := components.Kbd(canvas, theme, rect, key)
    x += keycap.Width + 10
}`},
	},
}

func (state *State) paintKbd(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	var row ui.Box
	state.page(canvas, theme, size, 560, state.Text("Kbd · 系统快捷键", "Kbd · System shortcuts"), func(f *ui.Frame) {
		if state.exampleID == "default" {
			row = f.Slot().FillWidth().Height(theme.Metrics.FontSizeS + 9)
			f.Text(state.Text("打开命令面板", "Open the command palette")).FontSize(theme.Metrics.FontSizeS).Muted()
			return
		}
		row = f.Slot().FillWidth().Height(theme.Metrics.FontSizeS + 9)
	})
	bounds := row.Rect()
	if state.exampleID == "default" {
		group := components.BeginKbdGroup(canvas, theme, gfx.Rect{X: bounds.X, Y: bounds.Y, Height: bounds.Height})
		group.Add("⌘")
		group.Add("⇧")
		group.Add("P")
		return
	}
	x := bounds.X
	for _, key := range [...]string{"Esc", "Enter", "Space", "⌥ Option"} {
		keycap := components.Kbd(canvas, theme, gfx.Rect{X: x, Y: bounds.Y, Height: bounds.Height}, key)
		x += keycap.Width + 10
	}
}
