package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/indicator"
)

var indicatorComponent = Component{
	ID: "indicator", Title: "Indicator", Category: "Data Display", Summary: "角标：贴在控件右上角的数字或小圆点，如未读数。",
	Source: "ui/indicator/indicator.go", paint: (*State).paintIndicator,
	Examples: []Example{
		{ID: "default", Title: "数字与圆点", Description: "Count 超过 Max 显示“Max+”，为 0 时不显示（ShowZero 除外）；Dot 改用小圆点。角标不占位，加上它控件不会移动。", Snippet: `indicator.View(c, "Inbox", indicator.Spec{Count: m.unread, Max: 99}, func() {
    ui.Button(c, "Inbox").Icon(m.inboxIcon).IconOnly().Ghost()
})`, PageSnippet: `indicator.View(c, "Inbox", indicator.Spec{Count: m.unread, Max: 99}, func() {
    ui.Button(c, "Inbox").Icon(m.inboxIcon).IconOnly().Ghost()
})`, PageSnippetZh: `indicator.View(c, "Inbox", indicator.Spec{Count: m.unread, Max: 99}, func() {
    ui.Button(c, "Inbox").Icon(m.inboxIcon).IconOnly().Ghost()
})`},
	},
}

// indicatorDemo keeps the closures that declare the marked buttons, made once, so a
// warm paint allocates nothing.
type indicatorDemo struct {
	c     ui.C
	state *State
	kids  [4]func()
	row   func()
}

var indicatorDemos = map[*State]*indicatorDemo{}

func indicatorDemoFor(state *State) *indicatorDemo {
	if demo := indicatorDemos[state]; demo != nil {
		return demo
	}
	demo := &indicatorDemo{state: state}
	for i, words := range [4][2]string{{"消息", "Messages"}, {"通知", "Alerts"}, {"更新", "Updates"}, {"完成", "Done"}} {
		demo.kids[i] = func() { ui.Button(demo.c, state.Text(words[0], words[1])).Outline() }
	}
	demo.row = func() {
		c := demo.c
		indicator.View(c, state.Text("消息", "Messages"), indicator.Spec{Count: 5}, demo.kids[0])
		indicator.View(c, state.Text("通知", "Alerts"), indicator.Spec{Count: 128}, demo.kids[1])
		indicator.View(c, state.Text("更新", "Updates"), indicator.Spec{Count: 1, Dot: true}, demo.kids[2])
		indicator.View(c, state.Text("完成", "Done"), indicator.Spec{Count: 3, Status: ui.StatusSuccess}, demo.kids[3])
	}
	indicatorDemos[state] = demo
	return demo
}

func (state *State) paintIndicator(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	demo := indicatorDemoFor(state)
	state.drawPage(canvas, theme, size, 420, state.Text("Indicator · 角标", "Indicator"), func(c ui.C) {
		demo.c = c
		ui.Row(c).Gap(32).Pad(8).Children(demo.row)
	})
}
