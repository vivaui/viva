package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/steps"
)

var stepsComponent = Component{
	ID: "steps", Title: "Steps", Category: "Navigation", Summary: "按顺序展示任务阶段：已完成、进行中、未开始与出错。",
	Source: "ui/steps/steps.go", paint: (*State).paintSteps,
	Examples: []Example{
		{ID: "default", Title: "横向步骤", Description: "已完成的步骤打勾，当前步骤高亮；可点击的步骤条用点击或键盘切换当前步骤。", Snippet: `// m.stages is a []steps.Step kept in the model.
steps.View(c, "Checkout", &m.step, steps.Spec{
    Steps:     m.stages,
    Clickable: true,
}).FillWidth()`, PageSnippet: `// m.stages is a []steps.Step kept in the model.
steps.View(c, "Checkout", &m.step, steps.Spec{
    Steps:     m.stages,
    Clickable: true,
}).FillWidth()`, PageSnippetZh: `// m.stages 是保存在模型里的 []steps.Step。
steps.View(c, "Checkout", &m.step, steps.Spec{
    Steps:     m.stages,
    Clickable: true,
}).FillWidth()`},
		{ID: "vertical", Title: "纵向、圆点与出错", Description: "纵向排列，圆点标记，当前步骤失败时用危险色标出。", Snippet: `steps.View(c, "Deploy", &m.step, steps.Spec{
    Steps:     m.stages,
    Direction: steps.Vertical,
    Type:      steps.Dot,
    Status:    steps.Error,
})`, PageSnippet: `steps.View(c, "Deploy", &m.step, steps.Spec{
    Steps:     m.stages,
    Direction: steps.Vertical,
    Type:      steps.Dot,
    Status:    steps.Error,
})`, PageSnippetZh: `steps.View(c, "Deploy", &m.step, steps.Spec{
    Steps:     m.stages,
    Direction: steps.Vertical,
    Type:      steps.Dot,
    Status:    steps.Error,
})`},
	},
}

type stepsDemo struct {
	stages        []steps.Step
	current, fail int
	small         int
}

var stepsDemos = map[*State]*stepsDemo{}

func stepsDemoFor(state *State) *stepsDemo {
	if demo := stepsDemos[state]; demo != nil {
		return demo
	}
	demo := &stepsDemo{current: 1, fail: 2, stages: []steps.Step{
		{Title: state.Text("账号", "Account"), Description: state.Text("创建登录名", "Create a login")},
		{Title: state.Text("资料", "Profile"), Description: state.Text("填写基本信息", "Tell us about you")},
		{Title: state.Text("完成", "Done")},
	}}
	stepsDemos[state] = demo
	return demo
}

func (state *State) paintSteps(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	demo := stepsDemoFor(state)
	state.drawPage(canvas, theme, size, 620, state.Text("Steps · 步骤条", "Steps · Progress"), func(c ui.C) {
		if state.exampleID == "vertical" {
			steps.View(c, state.Text("部署", "Deploy"), &demo.fail, steps.Spec{
				Steps: demo.stages, Direction: steps.Vertical, Type: steps.Dot, Status: steps.Error,
			})
			return
		}
		steps.View(c, state.Text("结账", "Checkout"), &demo.current, steps.Spec{Steps: demo.stages, Clickable: true}).FillWidth()
		steps.View(c, state.Text("紧凑", "Compact"), &demo.current, steps.Spec{Steps: demo.stages, Size: steps.Small, Clickable: true}).FillWidth()
	})
}
