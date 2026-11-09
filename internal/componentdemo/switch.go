package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var switchComponent = Component{
	ID: "switch", Title: "Switch", Category: "Forms", Summary: "受控开关与有界的状态过渡。",
	paint: (*State).paintSwitch,
	Examples: []Example{
		{ID: "default", Title: "受控开关", Description: "点击改变目标状态，生产组件使用运行器的帧时钟过渡。", Snippet: `notify := f.Switch(tree.ID(page, "notifications"), enabled).Label("Desktop notifications")
if notify.Clicked() {
    enabled = !enabled
}`},
		{ID: "sizes", Title: "两种尺寸", Description: "默认与紧凑开关独立受控。", Snippet: `f.Row().Gap(12).Children(func() {
    if f.Switch(tree.ID(page, "default"), normal).Label("Default size").Clicked() {
        normal = !normal
    }
    if f.Switch(tree.ID(page, "small"), compact).Label("Compact size").Small().Clicked() {
        compact = !compact
    }
})`},
		{ID: "states", Title: "启用与禁用", Description: "一个可切换开关和开、关两种禁用状态。", Snippet: `if f.Switch(tree.ID(page, "notify"), notify).Label("Desktop notifications").Clicked() {
    notify = !notify
}
f.Switch(tree.ID(page, "off"), false).Label("Disabled · Off").Disabled(true)
f.Switch(tree.ID(page, "on"), true).Label("Disabled · On").Disabled(true)`},
	},
}

var (
	switchUI = tree.Feature("switch")
	// switchToggle's item i is the i-th switch of the example.
	switchToggle = tree.Sub(switchUI, tree.Name("toggle"))
)

type switchState struct{ checked uint8 }

func (state *State) paintSwitch(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	labels := []string{state.Text("桌面通知", "Desktop notifications")}
	switch state.exampleID {
	case "sizes":
		labels = []string{state.Text("默认尺寸", "Default size"), state.Text("紧凑尺寸", "Compact size")}
	case "states":
		labels = []string{
			state.Text("桌面通知", "Desktop notifications"),
			state.Text("禁用 · 关闭", "Disabled · Off"),
			state.Text("禁用 · 开启", "Disabled · On"),
		}
	}
	state.page(canvas, theme, size, 480, state.Text("Switch · 调用方持有状态", "Switch · Caller-owned state"), func(f *ui.Frame) {
		for index, label := range labels {
			checked := state.switches.checked&(1<<index) != 0
			disabled := state.exampleID == "states" && index > 0
			if disabled {
				checked = index == 2
			}
			f.Row().Gap(12).Items(ui.Center).PadX(12).MarginY(4).Children(func() {
				control := f.Switch(switchToggle.At(index), checked).Label(label).Disabled(disabled)
				if state.exampleID == "sizes" && index == 1 {
					control.Small()
				}
				if control.Clicked() {
					state.switches.checked ^= 1 << index
				}
				color := theme.Foreground
				if disabled {
					color.Alpha *= 0.50
				}
				f.Text(label).FontSize(theme.Metrics.FontSizeM).Color(color)
			})
		}
		caption := state.Text("首项状态：关闭", "First switch: off")
		if state.switches.checked&1 != 0 {
			caption = state.Text("首项状态：开启", "First switch: on")
		}
		f.Text(caption).FontSize(theme.Metrics.FontSizeS).Muted()
	})
}
