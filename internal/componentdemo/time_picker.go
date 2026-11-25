package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/timepicker"
)

var timePickerComponent = Component{
	ID: "time-picker", Title: "Time Picker", Category: "Forms", Summary: "选择时间：时、分（可加秒）列表，点击或方向键即时生效。",
	Source: "ui/timepicker/timepicker.go", paint: (*State).paintTimePicker,
	Examples: []Example{
		{ID: "default", Title: "选择时间", Description: "滚轮滚动列，点击数字选中；上下键改变当前列，左右键换列，回车关闭。", Snippet: `m.start = timepicker.New().Placeholder("Pick a time").MinuteStep(5)
// in the page:
m.start.View(c, "Start", &m.startAt).Width(160)
if m.start.Changed() {
    // m.startAt holds the chosen time
}`, PageSnippet: `m.start = timepicker.New().Placeholder("Pick a time").MinuteStep(5)
// in the page:
m.start.View(c, "Start", &m.startAt).Width(160)
if m.start.Changed() {
    // m.startAt holds the chosen time
}`, PageSnippetZh: `m.start = timepicker.New().Placeholder("Pick a time").MinuteStep(5)
// 在页面中：
m.start.View(c, "Start", &m.startAt).Width(160)
if m.start.Changed() {
    // m.startAt 是选中的时间
}`},
		{ID: "seconds", Title: "带秒", Description: "Seconds 增加一列；SecondStep 限制秒的步长。", Snippet: `m.exact = timepicker.New().Seconds().MinuteStep(5)`, PageSnippet: `m.exact = timepicker.New().Seconds().MinuteStep(5)`, PageSnippetZh: `m.exact = timepicker.New().Seconds().MinuteStep(5)`},
	},
}

type timePickerDemo struct {
	basic, exact *timepicker.TimePicker
	a, b         timepicker.Value
}

var timePickerDemos = map[*State]*timePickerDemo{}

func timePickerDemoFor(state *State) *timePickerDemo {
	if demo := timePickerDemos[state]; demo != nil {
		return demo
	}
	demo := &timePickerDemo{
		basic: timepicker.New().Placeholder(state.Text("选择时间", "Pick a time")).MinuteStep(5),
		exact: timepicker.New().Seconds().MinuteStep(5),
		a:     timepicker.Value{Hour: 9, Minute: 30, Valid: true},
		b:     timepicker.Value{Hour: 9, Minute: 30, Valid: true},
	}
	if !state.english {
		demo.basic.Units("时", "分", "秒")
		demo.exact.Units("时", "分", "秒")
	}
	timePickerDemos[state] = demo
	return demo
}

func (state *State) paintTimePicker(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	demo := timePickerDemoFor(state)
	state.drawPage(canvas, theme, size, 320, state.Text("Time Picker · 时间", "Time Picker"), func(c ui.C) {
		if state.exampleID == "seconds" {
			demo.exact.View(c, state.Text("精确时间", "Exact time"), &demo.b).Width(200)
			return
		}
		demo.basic.View(c, state.Text("开始时间", "Start"), &demo.a).Width(200)
	})
}
