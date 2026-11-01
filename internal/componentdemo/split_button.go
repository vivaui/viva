package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

var (
	splitButtonUI      = tree.Feature("split-button")
	splitButtonPrimary = tree.ID(splitButtonUI, "primary")
	splitButtonMenu    = tree.ID(splitButtonUI, "menu")
)

var splitButtonComponent = Component{
	ID: "split-button", Title: "Split Button", Category: "Actions",
	Summary: "主操作与菜单触发器保持独立点击、焦点和无障碍语义。",
	paint:   (*State).paintSplitButton,
	Examples: []Example{{
		ID: "default", Title: "保存与更多操作", Description: "点击左侧执行主操作，点击右侧展开状态；两个热区共享同一外观但保留独立语义。",
		Height: 210,
		Snippet: `result := components.BeginSplitButton(canvas, theme, bounds, components.SplitButtonOptions{
    PrimaryID: save, MenuID: more, Label: "Save",
    Variant: components.ButtonOutline, Open: open,
}).Paint()
primary, menu := result.Clicked()`,
	}},
}

func (state *State) paintSplitButton(canvas *ui.Canvas, theme ui.Theme, size ui.Size) {
	open := state.flags&1 != 0
	state.page(canvas, theme, size, 480, state.Text("Split Button · 主操作与菜单", "Split Button · Primary and menu"), func(f *flex.Frame) {
		var target flex.Box
		f.Row().Self(flex.Start).Children(func() {
			target = f.Slot().Width(196).Height(36)
		})
		result := components.BeginSplitButton(canvas, theme, target.Rect(), components.SplitButtonOptions{
			PrimaryID: splitButtonPrimary, MenuID: splitButtonMenu,
			Label:     state.Text("保存", "Save"),
			MenuLabel: state.Text("更多保存操作", "More save actions"),
			Variant:   components.ButtonOutline, Open: open,
		}).Paint()
		primary, menu := result.Clicked()
		if primary {
			state.values[0]++
		}
		if menu {
			state.flags ^= 1
		}
		status := state.Text("主操作：保存 · 菜单：收起", "Primary: Save · Menu: closed")
		if state.values[0] > 0 && open {
			status = state.Text("主操作已触发 · 菜单：展开", "Primary triggered · Menu: open")
		} else if state.values[0] > 0 {
			status = state.Text("主操作已触发 · 菜单：收起", "Primary triggered · Menu: closed")
		} else if open {
			status = state.Text("主操作：保存 · 菜单：展开", "Primary: Save · Menu: open")
		}
		f.Text(status).
			FontSize(theme.Metrics.FontSizeS).Muted()
		caption(f, state.Text(
			"主操作与菜单热区各自支持键盘焦点和独立可访问名称。",
			"The primary and menu targets have separate keyboard focus and accessible names.",
		))
	})
}
