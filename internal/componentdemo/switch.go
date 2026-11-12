package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var switchComponent = Component{
	ID: "switch", Title: "Switch", Category: "Forms", Summary: "受控开关与有界的状态过渡。",
	paint: (*State).paintSwitch,
	Examples: []Example{
		{ID: "default", Title: "受控开关", Description: "点击改变目标状态，生产组件使用运行器的帧时钟过渡。", Snippet: `ui.Row(c).Gap(12).Items(ui.Center).Children(func() {
    ui.Switch(c, &enabled, "").Hit(notifications, "Desktop notifications")
    ui.Text(c, "Desktop notifications").FontSize(theme.Metrics.FontSizeM)
})`},
		{ID: "sizes", Title: "两种尺寸", Description: "默认与紧凑开关独立受控。", Snippet: `ui.Row(c).Gap(12).Children(func() {
    ui.Switch(c, &normal, "").Hit(defaultID, "Default size")
    ui.Switch(c, &compact, "").Hit(smallID, "Compact size").Small()
})`},
		{ID: "states", Title: "启用与禁用", Description: "一个可切换开关和开、关两种禁用状态。", Snippet: `ui.Switch(c, &notify, "").Hit(notifyID, "Desktop notifications")
ui.Switch(c, &off, "").Hit(offID, "Disabled · Off").Disabled(true)
ui.Switch(c, &on, "").Hit(onID, "Disabled · On").Disabled(true)`},
	},
}

var (
	switchUI = tree.Feature("switch")
	// switchToggle's item i is the i-th switch of the example.
	switchToggle = tree.Sub(switchUI, tree.Name("toggle"))
)

type switchState struct {
	checked uint8
	// toggles are the values a Page Switch binds, one per example row: a stable
	// address keeps the page's bound names from allocating in every paint.
	toggles [3]bool
}

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
	state.drawPage(canvas, theme, size, 480, state.Text("Switch · 调用方持有状态", "Switch · Caller-owned state"), func(c ui.C) {
		for index, label := range labels {
			checked := state.switches.checked&(1<<index) != 0
			disabled := state.exampleID == "states" && index > 0
			if disabled {
				checked = index == 2
			}
			ui.Row(c).Gap(12).Items(ui.Center).PadX(12).MarginY(4).Children(func() {
				state.switches.toggles[index] = checked
				control := ui.Switch(c, &state.switches.toggles[index], "").Hit(switchToggle.At(index), label).Disabled(disabled)
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
				ui.Text(c, label).FontSize(theme.Metrics.FontSizeM).Color(color)
			})
		}
		caption := state.Text("首项状态：关闭", "First switch: off")
		if state.switches.checked&1 != 0 {
			caption = state.Text("首项状态：开启", "First switch: on")
		}
		ui.Text(c, caption).FontSize(theme.Metrics.FontSizeS).Muted()
	})
}
