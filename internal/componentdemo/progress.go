package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var progressComponent = Component{
	ID: "progress", Title: "Progress", Category: "Feedback", Summary: "确定型进度条及边界值夹取。",
	paint: (*State).paintProgress,
	Examples: []Example{
		{ID: "default", Title: "上传进度", Description: "生产进度条展示一个调用方提供的确定值。", Snippet: `f.Column().Gap(8).Children(func() {
    f.Text("Uploading assets · 64%").FontSize(theme.Metrics.FontSizeM)
    f.Progress(0.64)
})`},
		{ID: "states", Title: "进度状态", Description: "同时展示空、进行中与完成状态。", Snippet: `labels := [...]string{"Queued", "Compiling", "Complete"}
values := [...]float32{0, 0.42, 1}
for index, label := range labels {
    f.Column().Gap(8).Children(func() {
        f.Text(label).FontSize(theme.Metrics.FontSizeM)
        f.Progress(values[index])
    })
}`},
	},
}

func (state *State) paintProgress(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	state.page(canvas, theme, size, 520, "Deployment", func(f *ui.Frame) {
		if state.exampleID == "default" {
			f.Text("Uploading assets · 64%").FontSize(theme.Metrics.FontSizeM)
			f.Progress(0.64)
			caption(f, "Caller-owned normalized fraction.")
			return
		}
		labels := [...]string{"Queued", "Compiling", "Complete"}
		values := [...]float32{0, 0.42, 1}
		for index, label := range labels {
			f.Column().Gap(8).Children(func() {
				f.Text(label).FontSize(theme.Metrics.FontSizeM)
				f.Progress(values[index])
			})
		}
	})
}
