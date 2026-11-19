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
		{ID: "default", Title: "接受条款", Description: "点击切换勾选；语义状态与可见反馈同步。", Snippet: `ui.Row(c).Gap(12).Items(ui.Center).Children(func() {
    ui.Checkbox(c, &accepted, "").Hit(terms, "Accept terms of use")
    ui.Text(c, "Accept terms of use").FontSize(theme.Metrics.FontSizeM)
})`, PageSnippet: `ui.Checkbox(c, &m.accepted, "Accept terms of use")`, PageSnippetZh: `ui.Checkbox(c, &m.accepted, "接受使用条款")`},
		{ID: "selection", Title: "多项选择", Description: "三项桌面偏好独立选择，不模拟生产 API 尚未提供的混合态。", Snippet: `boxes := tree.Sub(page, tree.Name("box"))
for index, label := range [...]string{"Show line numbers", "Format on save", "Restore last window"} {
    checked := selected&(1<<index) != 0
    ui.Row(c).Gap(12).Items(ui.Center).Children(func() {
        if ui.Checkbox(c, &checked, "").Hit(boxes.At(index), label).Clicked() {
            selected ^= 1 << index
        }
        ui.Text(c, label).FontSize(theme.Metrics.FontSizeM)
    })
}`, PageSnippet: `labels := [...]string{"Show line numbers", "Format on save", "Restore last window"}
for index := range labels {
    ui.Checkbox(c, &m.preferences[index], labels[index])
}`, PageSnippetZh: `labels := [...]string{"显示行号", "保存时格式化", "恢复上次窗口"}
for index := range labels {
    ui.Checkbox(c, &m.preferences[index], labels[index])
}`},
		{ID: "states", Title: "校验与禁用", Description: "必选项未选时显示校验状态，另有两个禁用样本。", Snippet: `boxes := tree.Sub(page, tree.Name("box"))
box := components.BeginCheckbox(canvas, theme, boxes.At(0), bounds, false).
    Label("Required: accept terms of use").Invalid().Paint()
if box.Clicked() {
    selected ^= 1
}
ui.Checkbox(c, &off, "Disabled · Unchecked").Hit(boxes.At(1), "Disabled · Unchecked").Disabled(true)
ui.Checkbox(c, &on, "Disabled · Checked").Hit(boxes.At(2), "Disabled · Checked").Disabled(true)`, PageSnippet: `m.on = true
ui.Checkbox(c, &m.required, "Required: accept terms of use")
ui.Checkbox(c, &m.off, "Disabled · Unchecked").Disabled(true)
ui.Checkbox(c, &m.on, "Disabled · Checked").Disabled(true)`, PageSnippetZh: `m.on = true
ui.Checkbox(c, &m.required, "必选：接受使用条款")
ui.Checkbox(c, &m.off, "禁用 · 未选择").Disabled(true)
ui.Checkbox(c, &m.on, "禁用 · 已选择").Disabled(true)`},
	},
}

var (
	checkboxUI = tree.Feature("checkbox")
	// checkboxBox's item i is the i-th checkbox of the example.
	checkboxBox = tree.Sub(checkboxUI, tree.Name("box"))
)

type checkboxState struct {
	selected uint8
	// boxes are the values a Page Checkbox binds, one per example row: a stable
	// address keeps the page's bound names from allocating in every paint.
	boxes [3]bool
}

func (state *State) paintCheckbox(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	if state.exampleID == "states" {
		state.paintCheckboxStates(canvas, theme, size)
		return
	}
	labels := []string{state.Text("接受使用条款", "Accept terms of use")}
	if state.exampleID == "selection" {
		labels = []string{
			state.Text("显示行号", "Show line numbers"),
			state.Text("保存时格式化", "Format on save"),
			state.Text("恢复上次窗口", "Restore last window"),
		}
	}
	caption := state.Text("点击方框，或用 Tab / Space 切换。", "Click the box, or press Tab / Space to toggle.")
	if state.exampleID == "default" && state.checkbox.boxes[0] {
		caption = state.Text("已接受使用条款。", "Terms accepted.")
	}
	state.drawPage(canvas, theme, size, 480, state.Text("Checkbox · 显式选择", "Checkbox · Explicit selection"), func(c ui.C) {
		for index, label := range labels {
			ui.Row(c).Gap(12).Items(ui.Center).Children(func() {
				ui.Checkbox(c, &state.checkbox.boxes[index], "").Hit(checkboxBox.At(index), label)
				ui.Text(c, label).FontSize(theme.Metrics.FontSizeM)
			})
		}
		ui.Text(c, caption).FontSize(theme.Metrics.FontSizeS).Muted()
	})
}

// paintCheckboxStates keeps the validation example on the canvas API: the page
// Checkbox has no invalid option, and the components checkbox paints a required
// box that is still empty in its invalid look into a Slot.
func (state *State) paintCheckboxStates(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	labels := []string{
		state.Text("必选：接受使用条款", "Required: accept terms of use"),
		state.Text("禁用 · 未选择", "Disabled · Unchecked"),
		state.Text("禁用 · 已选择", "Disabled · Checked"),
	}
	var invalid ui.Box
	hasInvalid := false
	state.drawPage(canvas, theme, size, 480, state.Text("Checkbox · 显式选择", "Checkbox · Explicit selection"), func(c ui.C) {
		for index, label := range labels {
			checked := state.checkbox.selected&(1<<index) != 0
			disabled := index > 0
			if disabled {
				checked = index == 2
			}
			ui.Row(c).Gap(12).Items(ui.Center).PadX(12).MarginY(2).Children(func() {
				state.checkbox.boxes[index] = checked
				if index == 0 && !checked {
					side := components.CheckboxExtent(theme)
					invalid, hasInvalid = ui.Slot(c).Size(side, side), true
				} else if ui.Checkbox(c, &state.checkbox.boxes[index], "").Hit(checkboxBox.At(index), label).Disabled(disabled).Clicked() {
					state.checkbox.selected ^= 1 << index
				}
				color := theme.Foreground
				if disabled {
					color.Alpha *= 0.50
				}
				ui.Text(c, label).FontSize(theme.Metrics.FontSizeM).Color(color)
			})
		}
		ui.Text(c, state.Text("点击方框，或用 Tab / Space 切换。", "Click the box, or press Tab / Space to toggle.")).
			FontSize(theme.Metrics.FontSizeS).Muted()
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
