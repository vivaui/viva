package componentdemo

import (
	"strconv"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var buttonComponent = Component{
	ID: "button", Title: "Button", Category: "Actions", Summary: "按钮变体、禁用状态与点击计数。",
	paint: (*State).paintButton, initialize: (*State).initializeButton,
	Examples: []Example{
		{ID: "default", Title: "点击计数", Description: "点击或键盘激活按钮，更新调用方持有的计数。", Snippet: `button := tree.ID(page, "button")
if f.Button(button, "Click me").Self(ui.Start).Clicked() {
    state.clicks++
}`, Height: 180},
		{ID: "variants", Title: "六种变体", Description: "六种生产按钮外观共享同一个点击计数。", Snippet: `f.Grid(128).Gap(12).Children(func() {
    f.Button(tree.ID(page, "default"), "Default").Clicked()
    f.Button(tree.ID(page, "destructive"), "Destructive").Destructive().Clicked()
    f.Button(tree.ID(page, "outline"), "Outline").Outline().Clicked()
    f.Button(tree.ID(page, "secondary"), "Secondary").Secondary().Clicked()
    f.Button(tree.ID(page, "ghost"), "Ghost").Ghost().Clicked()
    f.Button(tree.ID(page, "link"), "Link").Link().Clicked()
})`, Height: 230},
		{ID: "disabled", Title: "禁用变体", Description: "六种变体保留禁用语义，不响应点击。", Snippet: `f.Grid(128).Gap(12).Children(func() {
    f.Button(tree.ID(page, "default"), "Default").Disabled(true).Clicked()
    f.Button(tree.ID(page, "destructive"), "Destructive").Destructive().Disabled(true).Clicked()
    f.Button(tree.ID(page, "outline"), "Outline").Outline().Disabled(true).Clicked()
    f.Button(tree.ID(page, "secondary"), "Secondary").Secondary().Disabled(true).Clicked()
    f.Button(tree.ID(page, "ghost"), "Ghost").Ghost().Disabled(true).Clicked()
    f.Button(tree.ID(page, "link"), "Link").Link().Disabled(true).Clicked()
})`, Height: 230},
	},
}

var (
	buttonUI = tree.Feature("button")
	// buttonPress's item i is the i-th button of the example.
	buttonPress = tree.Sub(buttonUI, tree.Name("press"))
)

type buttonState struct {
	count uint64
	label string
}

func (state *State) initializeButton() {
	state.button.label = state.buttonLabel()
}

func (state *State) buttonLabel() string {
	return state.Text("点击：", "Clicks: ") + strconv.FormatUint(state.button.count, 10)
}

func (state *State) countButtonClick() {
	state.button.count++
	state.button.label = state.buttonLabel()
}

func (state *State) paintButton(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	countLabel := state.button.label
	if countLabel == "" {
		countLabel = state.buttonLabel()
	}
	state.page(canvas, theme, size, 480, countLabel, func(f *ui.Frame) {
		if state.exampleID == "default" {
			if f.Button(buttonPress.At(0), "Click me").Self(ui.Start).Clicked() {
				state.countButtonClick()
			}
			f.Text(state.Text("点击按钮，或用 Tab / Enter 操作。", "Click the button, or use Tab / Enter.")).FontSize(theme.Metrics.FontSizeS).Muted()
			return
		}
		disabled := state.exampleID == "disabled"
		f.Grid(128).Gap(12).Children(func() {
			for index, label := range [...]string{"Default", "Destructive", "Outline", "Secondary", "Ghost", "Link"} {
				button := f.Button(buttonPress.At(index), label).Disabled(disabled)
				switch index {
				case 1:
					button.Destructive()
				case 2:
					button.Outline()
				case 3:
					button.Secondary()
				case 4:
					button.Ghost()
				case 5:
					button.Link()
				}
				if button.Clicked() {
					state.countButtonClick()
				}
			}
		})
		caption := state.Text("六种外观；每次激活都会计数。", "Six appearances; every activation counts.")
		if disabled {
			caption = state.Text("全部禁用；不会计数或获得焦点。", "All disabled; they neither count nor take focus.")
		}
		f.Text(caption).FontSize(theme.Metrics.FontSizeS).Muted()
	})
}
