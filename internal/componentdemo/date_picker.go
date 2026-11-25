package componentdemo

import (
	"time"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/datepicker"
)

var datePickerComponent = Component{
	ID: "date-picker", Title: "Date Picker", Category: "Forms", Summary: "选择日期：按钮显示日期，点开月历选取，支持范围限制与键盘。",
	Source: "ui/datepicker/datepicker.go", paint: (*State).paintDatePicker,
	Examples: []Example{
		{ID: "default", Title: "选择日期", Description: "月历打开在已选日期（没有时是今天）；方向键移动，PageUp/PageDown 换月，回车选择。", Snippet: `m.due = datepicker.New().Placeholder("Pick a date")
// in the page:
m.due.View(c, "Due date", &m.dueDate).Width(220)
if m.due.Changed() {
    // m.dueDate holds the chosen day
}`, PageSnippet: `m.due = datepicker.New().Placeholder("Pick a date")
// in the page:
m.due.View(c, "Due date", &m.dueDate).Width(220)
if m.due.Changed() {
    // m.dueDate holds the chosen day
}`, PageSnippetZh: `m.due = datepicker.New().Placeholder("Pick a date")
// 在页面中：
m.due.View(c, "Due date", &m.dueDate).Width(220)
if m.due.Changed() {
    // m.dueDate 是选中的日期
}`},
		{ID: "range", Title: "限制范围", Description: "Min 与 Max 之外的日期变暗且不能选；FirstWeekday 设置每周起始日。", Snippet: `m.start = datepicker.New().
    Min(time.Now()).
    Max(time.Now().AddDate(0, 1, 0)).
    FirstWeekday(time.Monday)`, PageSnippet: `m.start = datepicker.New().
    Min(time.Now()).
    Max(time.Now().AddDate(0, 1, 0)).
    FirstWeekday(time.Monday)`, PageSnippetZh: `m.start = datepicker.New().
    Min(time.Now()).
    Max(time.Now().AddDate(0, 1, 0)).
    FirstWeekday(time.Monday)`},
	},
}

type datePickerDemo struct {
	due, window *datepicker.DatePicker
	a, b        time.Time
}

var datePickerDemos = map[*State]*datePickerDemo{}

func datePickerDemoFor(state *State) *datePickerDemo {
	if demo := datePickerDemos[state]; demo != nil {
		return demo
	}
	// A fixed day keeps the previews the same whenever they are looked at.
	today := time.Date(2026, 11, 25, 9, 0, 0, 0, time.Local)
	clock := func() time.Time { return today }
	demo := &datePickerDemo{
		due: datepicker.New().Clock(clock).Placeholder(state.Text("选择日期", "Pick a date")),
		window: datepicker.New().Clock(clock).Placeholder(state.Text("选择日期", "Pick a date")).
			Min(today).Max(today.AddDate(0, 1, 0)).FirstWeekday(time.Monday),
		a: time.Date(2026, 11, 18, 0, 0, 0, 0, time.Local),
	}
	if !state.english {
		labels := datepicker.Labels{
			Months:   [12]string{"一月", "二月", "三月", "四月", "五月", "六月", "七月", "八月", "九月", "十月", "十一月", "十二月"},
			Weekdays: [7]string{"日", "一", "二", "三", "四", "五", "六"},
			Previous: "上个月", Next: "下个月",
		}
		demo.due.Labels(labels)
		demo.window.Labels(labels)
	}
	datePickerDemos[state] = demo
	return demo
}

func (state *State) paintDatePicker(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	demo := datePickerDemoFor(state)
	state.drawPage(canvas, theme, size, 360, state.Text("Date Picker · 日期", "Date Picker"), func(c ui.C) {
		if state.exampleID == "range" {
			demo.window.View(c, state.Text("开始日期", "Start date"), &demo.b).Width(220)
			return
		}
		demo.due.View(c, state.Text("截止日期", "Due date"), &demo.a).Width(220)
	})
}
