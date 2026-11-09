package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var checkboxComponent = Component{
	ID: "checkbox", Title: "Checkbox", Category: "Forms", Summary: "由调用方保存选择的复选框。",
	paint: (*State).paintCheckbox,
	Examples: []Example{
		{ID: "default", Title: "接受条款", Description: "点击切换勾选；语义状态与可见反馈同步。", Snippet: `f.Row().Gap(12).Items(ui.Center).Children(func() {
    checked := accepted
    if f.Checkbox(tree.ID(page, "terms"), checked).Label("Accept terms of use").Clicked() {
        accepted = !checked
    }
    f.Text("Accept terms of use").FontSize(theme.Metrics.FontSizeM)
})`},
		{ID: "selection", Title: "多项选择", Description: "三项桌面偏好独立选择，不模拟生产 API 尚未提供的混合态。", Snippet: `boxes := tree.Sub(page, tree.Name("box"))
for index, label := range [...]string{"Show line numbers", "Format on save", "Restore last window"} {
    checked := selected&(1<<index) != 0
    f.Row().Gap(12).Items(ui.Center).Children(func() {
        if f.Checkbox(boxes.At(index), checked).Label(label).Clicked() {
            selected ^= 1 << index
        }
        f.Text(label).FontSize(theme.Metrics.FontSizeM)
    })
}`},
		{ID: "states", Title: "校验与禁用", Description: "必选项未选时显示校验状态，另有两个禁用样本。", Snippet: `boxes := tree.Sub(page, tree.Name("box"))
box := components.BeginCheckbox(canvas, theme, boxes.At(0), bounds, false).
    Label("Required: accept terms of use").Invalid().Paint()
if box.Clicked() {
    selected ^= 1
}
f.Checkbox(boxes.At(1), false).Label("Disabled · Unchecked").Disabled(true)
f.Checkbox(boxes.At(2), true).Label("Disabled · Checked").Disabled(true)`},
	},
}

var (
	checkboxUI = tree.Feature("checkbox")
	// checkboxBox's item i is the i-th checkbox of the example.
	checkboxBox = tree.Sub(checkboxUI, tree.Name("box"))
)

type checkboxState struct{ selected uint8 }

func (state *State) paintCheckbox(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	labels := []string{state.Text("接受使用条款", "Accept terms of use")}
	switch state.exampleID {
	case "selection":
		labels = []string{
			state.Text("显示行号", "Show line numbers"),
			state.Text("保存时格式化", "Format on save"),
			state.Text("恢复上次窗口", "Restore last window"),
		}
	case "states":
		labels = []string{
			state.Text("必选：接受使用条款", "Required: accept terms of use"),
			state.Text("禁用 · 未选择", "Disabled · Unchecked"),
			state.Text("禁用 · 已选择", "Disabled · Checked"),
		}
	}
	// A required box that is still empty is drawn invalid, which ui's checkbox
	// has no option for: its square is a Slot the component paints into.
	var invalid ui.Box
	hasInvalid := false
	state.page(canvas, theme, size, 480, state.Text("Checkbox · 显式选择", "Checkbox · Explicit selection"), func(f *ui.Frame) {
		for index, label := range labels {
			checked := state.checkbox.selected&(1<<index) != 0
			disabled := state.exampleID == "states" && index > 0
			if disabled {
				checked = index == 2
			}
			f.Row().Gap(12).Items(ui.Center).PadX(12).MarginY(2).Children(func() {
				if state.exampleID == "states" && index == 0 && !checked {
					side := components.CheckboxExtent(theme)
					invalid, hasInvalid = f.Slot().Size(side, side), true
				} else if f.Checkbox(checkboxBox.At(index), checked).Label(label).Disabled(disabled).Clicked() {
					state.checkbox.selected ^= 1 << index
				}
				color := theme.Foreground
				if disabled {
					color.Alpha *= 0.50
				}
				f.Text(label).FontSize(theme.Metrics.FontSizeM).Color(color)
			})
		}
		caption := state.Text("点击方框，或用 Tab / Space 切换。", "Click the box, or press Tab / Space to toggle.")
		if state.exampleID == "default" && state.checkbox.selected != 0 {
			caption = state.Text("已接受使用条款。", "Terms accepted.")
		}
		f.Text(caption).FontSize(theme.Metrics.FontSizeS).Muted()
	})
	if hasInvalid {
		bounds := invalid.Rect()
		box := components.BeginCheckbox(canvas, theme, checkboxBox.At(0), gfx.Rect{X: bounds.X, Y: bounds.Y}, false).
			Label(labels[0]).Invalid().Paint()
		if box.Clicked() {
			state.checkbox.selected ^= 1
		}
	}
}
