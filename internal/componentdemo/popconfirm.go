package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/popconfirm"
)

var popconfirmComponent = Component{
	ID: "popconfirm", Title: "Popconfirm", Category: "Overlay", Summary: "挨着按钮的轻量确认：一句问题、可选说明和两个答案。",
	Source: "ui/popconfirm/popconfirm.go", paint: (*State).paintPopconfirm,
	Examples: []Example{
		{ID: "default", Title: "确认删除", Description: "按钮打开问题；确认或取消都会关闭，Esc 与点击外部关闭但不作答。不可撤销的后果仍应使用 AlertDialog。", Snippet: `m.remove = popconfirm.New()
// in the page:
trigger := ui.Button(c, "Remove").Outline()
if trigger.Clicked() {
    m.remove.Open()
}
m.remove.View(c, trigger, popconfirm.Spec{Title: "Remove build-01?", Danger: true, OkLabel: "Remove"})
if m.remove.Confirmed() {
    m.removeHost()
}`, PageSnippet: `m.remove = popconfirm.New()
// in the page:
trigger := ui.Button(c, "Remove").Outline()
if trigger.Clicked() {
    m.remove.Open()
}
m.remove.View(c, trigger, popconfirm.Spec{Title: "Remove build-01?", Danger: true, OkLabel: "Remove"})
if m.remove.Confirmed() {
    m.removeHost()
}`, PageSnippetZh: `m.remove = popconfirm.New()
// 在页面中：
trigger := ui.Button(c, "Remove").Outline()
if trigger.Clicked() {
    m.remove.Open()
}
m.remove.View(c, trigger, popconfirm.Spec{Title: "Remove build-01?", Danger: true, OkLabel: "Remove"})
if m.remove.Confirmed() {
    m.removeHost()
}`},
	},
}

type popconfirmDemo struct {
	pc *popconfirm.Popconfirm
}

var popconfirmDemos = map[*State]*popconfirmDemo{}

func popconfirmDemoFor(state *State) *popconfirmDemo {
	if demo := popconfirmDemos[state]; demo != nil {
		return demo
	}
	demo := &popconfirmDemo{pc: popconfirm.New()}
	popconfirmDemos[state] = demo
	return demo
}

func (state *State) paintPopconfirm(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	demo := popconfirmDemoFor(state)
	spec := popconfirm.Spec{
		Title: state.Text("移除 build-01？", "Remove build-01?"), Description: state.Text("主机及其保存的设置都会被删除。", "The host and its saved settings are deleted."),
		OkLabel: state.Text("移除", "Remove"), CancelLabel: state.Text("取消", "Cancel"), Danger: true,
	}
	state.drawPage(canvas, theme, size, 420, state.Text("Popconfirm · 轻量确认", "Popconfirm"), func(c ui.C) {
		trigger := ui.Button(c, state.Text("移除主机", "Remove host")).Outline()
		if trigger.Clicked() {
			demo.pc.Open()
		}
		demo.pc.View(c, trigger, spec)
		if demo.pc.Confirmed() {
			state.flags = 1
		} else if demo.pc.Cancelled() {
			state.flags = 2
		}
		switch state.flags {
		case 1:
			ui.Text(c, state.Text("已确认", "Confirmed")).Muted()
		case 2:
			ui.Text(c, state.Text("已取消", "Cancelled")).Muted()
		}
	})
}
