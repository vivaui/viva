package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var progressRingComponent = Component{
	ID: "progress-ring", Title: "Progress Ring", Category: "Feedback", Summary: "环形进度：按比例顺时针填充，可着状态色，也可无限转动。",
	Source: "ui/leaves_paint.go", paint: (*State).paintProgressRing,
	Examples: []Example{
		{ID: "default", Title: "环形与状态色", Description: "Circle 把进度条画成圆环，Status 用成功、警告、危险色填充；Indeterminate 的环不断转动，系统要求减少动效时静止。", Snippet: `ui.Progress(c, m.done/m.total).Circle().Size(80, 80)
ui.Progress(c, 1).Circle().Status(ui.StatusSuccess)
ui.Progress(c, 0).Circle().Indeterminate()`, PageSnippet: `ui.Progress(c, m.done/m.total).Circle().Size(80, 80)
ui.Progress(c, 1).Circle().Status(ui.StatusSuccess)
ui.Progress(c, 0).Circle().Indeterminate()`, PageSnippetZh: `ui.Progress(c, m.done/m.total).Circle().Size(80, 80)
ui.Progress(c, 1).Circle().Status(ui.StatusSuccess)
ui.Progress(c, 0).Circle().Indeterminate()`},
	},
}

func (state *State) paintProgressRing(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	state.drawPage(canvas, theme, size, 480, state.Text("Progress · 环形", "Progress · Ring"), func(c ui.C) {
		ui.Row(c).Gap(20).Items(ui.Center).Children(func() {
			ui.Progress(c, 0.25).Circle().Label(state.Text("上传", "Upload"))
			ui.Progress(c, 0.7).Circle().Status(ui.StatusSuccess).Size(80, 80).Label(state.Text("同步", "Sync"))
			ui.Progress(c, 0.9).Circle().Status(ui.StatusWarning).Size(48, 48).Label(state.Text("磁盘", "Disk"))
			ui.Progress(c, 1).Circle().Status(ui.StatusDanger).Size(48, 48).Label(state.Text("失败", "Failed"))
		})
	})
}
