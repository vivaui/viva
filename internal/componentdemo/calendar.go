package componentdemo

import (
	"strconv"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/calendar"
)

var calendarComponent = Component{
	ID: "calendar", Title: "Calendar", Category: "Date & Time", Summary: "应用持有月份快照和选择状态的七列日历。",
	paint: (*State).paintCalendar, initialize: (*State).initializeCalendar,
	Examples: []Example{
		{ID: "month", Title: "单日选择", Description: "展示月导航、今天、外部日期和所选日期。", Snippet: `days[15].Selected, days[15].Today = true, true
previous, next := tree.ID(page, "previous"), tree.ID(page, "next")
// value selects the visible month snapshot; monthTitle and days follow it.
components.BeginCalendar(canvas, theme, bounds, components.CalendarOptions{
    MonthTitle: monthTitle, Weekdays: weekdays, Days: days,
    PreviousID: previous, NextID: next,
}).Paint()
if canvas.Clicked(previous) {
    value = -1
} else if canvas.Clicked(next) {
    value = 1
}`, PageSnippet: `days := []calendar.Day{
    {Label: "29", Outside: true},
    {Label: "30", Outside: true},
    {Label: "1"}, {Label: "2"}, {Label: "3"}, {Label: "4"}, {Label: "5"},
    {Label: "6"}, {Label: "7"}, {Label: "8"}, {Label: "9"}, {Label: "10"},
    {Label: "11"}, {Label: "12"}, {Label: "13"}, {Label: "14", Today: true, Selected: true},
    {Label: "15"}, {Label: "16"}, {Label: "17"}, {Label: "18"}, {Label: "19"},
    {Label: "20"}, {Label: "21"}, {Label: "22"}, {Label: "23"}, {Label: "24"},
    {Label: "25"}, {Label: "26"}, {Label: "27"}, {Label: "28"}, {Label: "29"},
    {Label: "30"}, {Label: "31"},
    {Label: "1", Outside: true}, {Label: "2", Outside: true},
}
previous, next := m.previous, m.next
cal := &calendar.Calendar{Options: calendar.Options{
    MonthTitle: "October 2026",
    Weekdays:   [7]string{"Mo", "Tu", "We", "Th", "Fr", "Sa", "Su"},
    Days:       days,
    PreviousID: previous, NextID: next,
    PreviousLabel: "Previous month", NextLabel: "Next month",
}}
calendar.View(c, "Calendar", cal).FillWidth().MaxWidth(360)
if cal.Clicked(previous) {
    m.month = -1
} else if cal.Clicked(next) {
    m.month = 1
}`, PageSnippetZh: `days := []calendar.Day{
    {Label: "29", Outside: true},
    {Label: "30", Outside: true},
    {Label: "1"}, {Label: "2"}, {Label: "3"}, {Label: "4"}, {Label: "5"},
    {Label: "6"}, {Label: "7"}, {Label: "8"}, {Label: "9"}, {Label: "10"},
    {Label: "11"}, {Label: "12"}, {Label: "13"}, {Label: "14", Today: true, Selected: true},
    {Label: "15"}, {Label: "16"}, {Label: "17"}, {Label: "18"}, {Label: "19"},
    {Label: "20"}, {Label: "21"}, {Label: "22"}, {Label: "23"}, {Label: "24"},
    {Label: "25"}, {Label: "26"}, {Label: "27"}, {Label: "28"}, {Label: "29"},
    {Label: "30"}, {Label: "31"},
    {Label: "1", Outside: true}, {Label: "2", Outside: true},
}
previous, next := m.previous, m.next
cal := &calendar.Calendar{Options: calendar.Options{
    MonthTitle: "2026 年 10 月",
    Weekdays:   [7]string{"一", "二", "三", "四", "五", "六", "日"},
    Days:       days,
    PreviousID: previous, NextID: next,
    PreviousLabel: "上个月", NextLabel: "下个月",
}}
calendar.View(c, "日历", cal).FillWidth().MaxWidth(360)
if cal.Clicked(previous) {
    m.month = -1
} else if cal.Clicked(next) {
    m.month = 1
}`},
		{ID: "range", Title: "日期范围", Description: "连续三天使用范围起点、中段和终点样式。", Snippet: `previous, next := tree.ID(page, "previous"), tree.ID(page, "next")
days[22].RangeStart = true
days[23].RangeMiddle = true
days[24].RangeEnd = true
components.BeginCalendar(canvas, theme, bounds, components.CalendarOptions{
    MonthTitle: monthTitle, Weekdays: weekdays, Days: days,
    PreviousID: previous, NextID: next,
}).Paint()`, PageSnippet: `days := []calendar.Day{
    {Label: "29", Outside: true},
    {Label: "30", Outside: true},
    {Label: "1"}, {Label: "2"}, {Label: "3"}, {Label: "4"}, {Label: "5"},
    {Label: "6"}, {Label: "7"}, {Label: "8"}, {Label: "9"}, {Label: "10"},
    {Label: "11"}, {Label: "12"}, {Label: "13"}, {Label: "14", Today: true},
    {Label: "15"}, {Label: "16"}, {Label: "17"}, {Label: "18"}, {Label: "19"},
    {Label: "20"}, {Label: "21", RangeStart: true},
    {Label: "22", RangeMiddle: true}, {Label: "23", RangeEnd: true},
    {Label: "24"}, {Label: "25"}, {Label: "26"}, {Label: "27"}, {Label: "28"},
    {Label: "29"}, {Label: "30"}, {Label: "31"},
    {Label: "1", Outside: true}, {Label: "2", Outside: true},
}
cal := &calendar.Calendar{Options: calendar.Options{
    MonthTitle: "October 2026",
    Weekdays:   [7]string{"Mo", "Tu", "We", "Th", "Fr", "Sa", "Su"},
    Days:       days,
    PreviousID: m.previous, NextID: m.next,
    PreviousLabel: "Previous month", NextLabel: "Next month",
}}
calendar.View(c, "Calendar", cal).FillWidth().MaxWidth(360)`, PageSnippetZh: `days := []calendar.Day{
    {Label: "29", Outside: true},
    {Label: "30", Outside: true},
    {Label: "1"}, {Label: "2"}, {Label: "3"}, {Label: "4"}, {Label: "5"},
    {Label: "6"}, {Label: "7"}, {Label: "8"}, {Label: "9"}, {Label: "10"},
    {Label: "11"}, {Label: "12"}, {Label: "13"}, {Label: "14", Today: true},
    {Label: "15"}, {Label: "16"}, {Label: "17"}, {Label: "18"}, {Label: "19"},
    {Label: "20"}, {Label: "21", RangeStart: true},
    {Label: "22", RangeMiddle: true}, {Label: "23", RangeEnd: true},
    {Label: "24"}, {Label: "25"}, {Label: "26"}, {Label: "27"}, {Label: "28"},
    {Label: "29"}, {Label: "30"}, {Label: "31"},
    {Label: "1", Outside: true}, {Label: "2", Outside: true},
}
cal := &calendar.Calendar{Options: calendar.Options{
    MonthTitle: "2026 年 10 月",
    Weekdays:   [7]string{"一", "二", "三", "四", "五", "六", "日"},
    Days:       days,
    PreviousID: m.previous, NextID: m.next,
    PreviousLabel: "上个月", NextLabel: "下个月",
}}
calendar.View(c, "日历", cal).FillWidth().MaxWidth(360)`},
	},
}

var (
	calendarUI       = tree.Feature("calendar")
	calendarPrevious = tree.ID(calendarUI, "previous")
	calendarNext     = tree.ID(calendarUI, "next")
	// calendarDay's item i is the i-th cell of the month grid.
	calendarDay = tree.Sub(calendarUI, tree.Name("day"))
)

var (
	calendarWeekdays        = [7]string{"一", "二", "三", "四", "五", "六", "日"}
	calendarWeekdaysEnglish = [7]string{"Mo", "Tu", "We", "Th", "Fr", "Sa", "Su"}
)

var calendarMonthNamesEnglish = [12]string{
	"January", "February", "March", "April", "May", "June",
	"July", "August", "September", "October", "November", "December",
}

// calendarMonthDays is the October 2026 grid: two days of September, the 31
// days of October and two of November, five weeks in all. The English snapshot
// is built the same way, so painting the localized screen-reader date allocates
// nothing in either language.
var (
	calendarMonthDays        = newCalendarMonthDays(false)
	calendarEnglishMonthDays = newCalendarMonthDays(true)
	calendarRangeDays        = newCalendarRangeDays(calendarMonthDays)
	calendarEnglishRangeDays = newCalendarRangeDays(calendarEnglishMonthDays)
)

func newCalendarMonthDays(english bool) [35]components.CalendarDay {
	var days [35]components.CalendarDay
	for index := range days {
		month, day := 10, index-1
		outside := false
		switch {
		case index < 2:
			month, day, outside = 9, index+29, true
		case index > 32:
			month, day, outside = 11, index-32, true
		}
		label := strconv.Itoa(day)
		accessibilityLabel := "2026年" + strconv.Itoa(month) + "月" + label + "日"
		if english {
			accessibilityLabel = calendarMonthNamesEnglish[month-1] + " " + label + ", 2026"
		}
		days[index] = components.CalendarDay{
			ID: calendarDay.At(index), Label: label, Outside: outside,
			AccessibilityLabel: accessibilityLabel,
		}
	}
	days[15].Today, days[15].Selected = true, true
	return days
}

func newCalendarRangeDays(month [35]components.CalendarDay) [35]components.CalendarDay {
	days := month
	days[15].Selected = false
	days[22].RangeStart = true
	days[23].RangeMiddle = true
	days[24].RangeEnd = true
	return days
}

// calendarDemo is one calendar example's page control. The wrapper's handlers
// are made once and point at this value, so it is kept beside the State rather
// than rebuilt in each paint.
type calendarDemo struct {
	view calendar.Calendar
}

// calendarDemos is keyed by State because the constrained demo model has no
// field for this example; a State is painted on one thread at a time.
var calendarDemos = map[*State]*calendarDemo{}

func (state *State) initializeCalendar() {
	calendarDemos[state] = &calendarDemo{}
}

func calendarDemoFor(state *State) *calendarDemo {
	if demo := calendarDemos[state]; demo != nil {
		return demo
	}
	demo := &calendarDemo{}
	calendarDemos[state] = demo
	return demo
}

func (state *State) paintCalendar(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	monthTitle := state.Text("2026 年 10 月", "October 2026")
	if state.values[0] < 0 {
		monthTitle = state.Text("2026 年 9 月", "September 2026")
	} else if state.values[0] > 0 {
		monthTitle = state.Text("2026 年 11 月", "November 2026")
	}
	days := calendarMonthDays[:]
	if state.exampleID == "range" {
		days = calendarRangeDays[:]
	}
	if state.english {
		days = calendarEnglishMonthDays[:]
		if state.exampleID == "range" {
			days = calendarEnglishRangeDays[:]
		}
	}
	var weekdays [7]string
	for index, label := range calendarWeekdays {
		weekdays[index] = state.Text(label, calendarWeekdaysEnglish[index])
	}
	demo := calendarDemoFor(state)
	demo.view.Options = calendar.Options{
		MonthTitle: monthTitle, Weekdays: weekdays, Days: days,
		PreviousID: calendarPrevious, NextID: calendarNext,
		PreviousLabel: state.Text("上个月", "Previous month"), NextLabel: state.Text("下个月", "Next month"),
	}
	// The calendar is the whole demo: it takes the stage, at most 360 wide.
	state.drawStage(canvas, theme, size, func(c ui.C) {
		ui.Column(c).Fill().Pad(18).Items(ui.Center).Children(func() {
			calendar.View(c, state.Text("日历", "Calendar"), &demo.view).
				FillWidth().MaxWidth(360).FillHeight()
		})
	})
	if demo.view.Clicked(calendarPrevious) {
		state.values[0] = -1
	} else if demo.view.Clicked(calendarNext) {
		state.values[0] = 1
	}
}
