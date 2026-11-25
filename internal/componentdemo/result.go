package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/result"
)

var resultComponent = Component{
	ID: "result", Title: "Result", Category: "Feedback", Summary: "任务结果页：大状态图标、标题、说明与后续操作。",
	Source: "ui/result/result.go", paint: (*State).paintResult,
	Examples: []Example{
		{ID: "default", Title: "成功", Description: "状态决定图标与颜色；操作区放下一步按钮。", Snippet: `result.View(c, result.Spec{
    Status:      ui.StatusSuccess,
    Title:       "Key imported",
    Description: "The key is now in your vault.",
}, func() {
    ui.Button(c, "Done")
}).Fill()`, PageSnippet: `result.View(c, result.Spec{
    Status:      ui.StatusSuccess,
    Title:       "Key imported",
    Description: "The key is now in your vault.",
}, func() {
    ui.Button(c, "Done")
}).Fill()`, PageSnippetZh: `result.View(c, result.Spec{
    Status:      ui.StatusSuccess,
    Title:       "Key imported",
    Description: "The key is now in your vault.",
}, func() {
    ui.Button(c, "Done")
}).Fill()`},
		{ID: "failure", Title: "失败与警告", Description: "危险与警告沿用同一套 ui.Status。", Snippet: `result.View(c, result.Spec{
    Status:      ui.StatusDanger,
    Title:       "Import failed",
    Description: "The file is not a private key.",
}, func() {
    ui.Button(c, "Try again")
}).Fill()`, PageSnippet: `result.View(c, result.Spec{
    Status:      ui.StatusDanger,
    Title:       "Import failed",
    Description: "The file is not a private key.",
}, func() {
    ui.Button(c, "Try again")
}).Fill()`, PageSnippetZh: `result.View(c, result.Spec{
    Status:      ui.StatusDanger,
    Title:       "Import failed",
    Description: "The file is not a private key.",
}, func() {
    ui.Button(c, "Try again")
}).Fill()`},
	},
}

// resultDemo keeps the closure the result calls for its actions, made once.
type resultDemo struct {
	c       ui.C
	label   string
	actions func()
}

var resultDemos = map[*State]*resultDemo{}

func resultDemoFor(state *State) *resultDemo {
	if demo := resultDemos[state]; demo != nil {
		return demo
	}
	demo := &resultDemo{}
	demo.actions = func() { ui.Button(demo.c, demo.label) }
	resultDemos[state] = demo
	return demo
}

func (state *State) paintResult(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	demo := resultDemoFor(state)
	spec := result.Spec{
		Status: ui.StatusSuccess, Title: state.Text("密钥已导入", "Key imported"),
		Description: state.Text("密钥已保存在你的保险库中。", "The key is now in your vault."),
	}
	demo.label = state.Text("完成", "Done")
	if state.exampleID == "failure" {
		spec.Status = ui.StatusDanger
		spec.Title, spec.Description = state.Text("导入失败", "Import failed"), state.Text("该文件不是私钥。", "The file is not a private key.")
		demo.label = state.Text("重试", "Try again")
	}
	state.drawStage(canvas, theme, size, func(c ui.C) {
		demo.c = c
		result.View(c, spec, demo.actions).Fill()
	})
}
