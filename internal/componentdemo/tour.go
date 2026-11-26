package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/tour"
)

var tourComponent = Component{
	ID: "tour", Title: "Tour", Category: "Overlays", Summary: "引导：逐步指向页面上的控件，其余窗口变暗。",
	Source: "ui/tour/tour.go", paint: (*State).paintTour,
	Examples: []Example{
		{ID: "default", Title: "逐步引导", Description: "页面在声明控件时用 Target 说明每一步指向哪个控件，最后声明 View；Finish 才算完成，Skip、Esc 与点击外部只是离开。", Snippet: `m.tour = tour.New()
save := ui.Button(c, "Save")
m.tour.Target(0, save)
m.tour.View(c, tour.Spec{Steps: m.steps})
if m.tour.Finished() {
    // remember that the tour was seen
}`, PageSnippet: `m.tour = tour.New()
save := ui.Button(c, "Save")
m.tour.Target(0, save)
m.tour.View(c, tour.Spec{Steps: m.steps})
if m.tour.Finished() {
    // remember that the tour was seen
}`, PageSnippetZh: `m.tour = tour.New()
save := ui.Button(c, "Save")
m.tour.Target(0, save)
m.tour.View(c, tour.Spec{Steps: m.steps})
if m.tour.Finished() {
    // 记住用户已经看过引导
}`},
	},
}

type tourDemo struct {
	t     *tour.Tour
	steps []tour.Step
}

var tourDemos = map[*State]*tourDemo{}

func tourDemoFor(state *State) *tourDemo {
	if demo := tourDemos[state]; demo != nil {
		return demo
	}
	demo := &tourDemo{t: tour.New(), steps: []tour.Step{
		{Title: state.Text("保存", "Save"), Description: state.Text("把改动写回文件。", "Writes your changes back to the file.")},
		{Title: state.Text("同步", "Sync"), Description: state.Text("把文件发送到远程主机。", "Sends the file to the remote host.")},
		{Title: state.Text("完成", "All set"), Description: state.Text("随时可以重新开始引导。", "You can start the tour again at any time.")},
	}}
	tourDemos[state] = demo
	return demo
}

func (state *State) paintTour(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	demo := tourDemoFor(state)
	state.drawPage(canvas, theme, size, 520, state.Text("Tour · 引导", "Tour"), func(c ui.C) {
		ui.Row(c).Gap(12).Children(func() {
			start := ui.Button(c, state.Text("开始引导", "Start tour")).Outline()
			if start.Clicked() {
				demo.t.Start()
			}
			save := ui.Button(c, state.Text("保存", "Save"))
			demo.t.Target(0, save)
			sync := ui.Button(c, state.Text("同步", "Sync")).Secondary()
			demo.t.Target(1, sync)
		})
		demo.t.View(c, tour.Spec{Steps: demo.steps, Labels: [4]string{
			state.Text("上一步", "Previous"), state.Text("下一步", "Next"), state.Text("完成", "Finish"), state.Text("跳过", "Skip"),
		}})
	})
}
