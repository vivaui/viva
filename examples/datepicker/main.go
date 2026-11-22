// Command datepicker enumerates every DatePicker and DateRangePicker option so
// each one can be tried by hand. Run it with:
//
//	go run ./examples/datepicker
package main

import (
	"log"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/datepicker"
)

func day(year int, month time.Month, date int) time.Time {
	return time.Date(year, month, date, 0, 0, 0, 0, time.Local)
}

type singleRow struct {
	label  string
	note   string
	width  float32
	picker *datepicker.DatePicker
}

type rangeRow struct {
	label  string
	note   string
	width  float32
	picker *datepicker.DateRangePicker
}

type model struct {
	last    string
	singles []singleRow
	ranges  []rangeRow
}

// valueLayout is the format a row shows its current value with, so the status
// text and the field agree.
func valueLayout(granularity datepicker.Granularity, format string) string {
	if format != "" {
		return format
	}
	switch granularity {
	case datepicker.GranularityMonth:
		return "2006-01"
	case datepicker.GranularityYear:
		return "2006"
	}
	return "2006-01-02"
}

func singleStatus(picker *datepicker.DatePicker) string {
	if !picker.HasValue {
		return "（空）"
	}
	return picker.Value.Format(valueLayout(picker.Granularity, picker.Format))
}

func rangeStatus(picker *datepicker.DateRangePicker) string {
	layout := valueLayout(picker.Granularity, picker.Format)
	if pending, ok := picker.Pending(); ok {
		return "待选起点 " + pending.Format(layout) + " …"
	}
	if !picker.StartSet || !picker.EndSet {
		return "（空）"
	}
	return picker.Start.Format(layout) + " → " + picker.End.Format(layout)
}

func (m *model) add(s singleRow) *datepicker.DatePicker {
	m.singles = append(m.singles, s)
	return s.picker
}

func (m *model) addRange(r rangeRow) *datepicker.DateRangePicker {
	m.ranges = append(m.ranges, r)
	return r.picker
}

func newModel() *model {
	m := &model{last: "操作任意控件，这里显示最近一次回调"}
	chinese := datepicker.ChineseLabels()

	// 1. Empty day field: placeholder, and no clear button until a value is set.
	empty := datepicker.New(datepicker.GranularityDay)
	empty.Labels = chinese
	empty.OnChange = func(value time.Time, has bool) { m.last = "空日字段：" + singleStatus(empty) }
	m.add(singleRow{"日粒度 · 空值", "未选时显示占位符；点面板底部“今天”会选中今天并关闭。", 260, empty})

	// 2. A value with the caller's own format and Chinese labels.
	selected := datepicker.New(datepicker.GranularityDay)
	selected.Labels = chinese
	selected.Format = "2006年1月2日"
	selected.SetValue(day(2026, time.November, 22))
	selected.OnChange = func(value time.Time, has bool) { m.last = "日粒度：" + singleStatus(selected) }
	m.add(singleRow{"日粒度 · 已选 + 自定义格式", "Format 只影响显示；值是本地时区 00:00 的文明日（与 antd 的 dayjs 语义一致）。", 260, selected})

	// 3. Month and year granularities.
	month := datepicker.New(datepicker.GranularityMonth)
	month.Labels = chinese
	month.SetValue(day(2026, time.March, 1))
	m.add(singleRow{"月粒度", "面板是 3×4 的月份网格；值归一化到 1 日。", 230, month})

	year := datepicker.New(datepicker.GranularityYear)
	year.Labels = chinese
	year.SetValue(day(2024, time.January, 1))
	year.Min = day(2000, time.January, 1)
	year.Max = day(2035, time.December, 1)
	year.OnChange = func(value time.Time, has bool) { m.last = "年粒度：" + singleStatus(year) }
	m.add(singleRow{"年粒度 · min/max", "2000–2035 之外的年被禁用；十年段用箭头翻。", 200, year})

	// 4. Weekends disabled, and a week that starts on Sunday.
	weekend := datepicker.New(datepicker.GranularityDay)
	weekend.Labels = chinese
	weekend.DisabledDate = func(value time.Time) bool {
		return value.Weekday() == time.Saturday || value.Weekday() == time.Sunday
	}
	weekend.OnChange = func(value time.Time, has bool) { m.last = "周末禁用：" + singleStatus(weekend) }
	m.add(singleRow{"日粒度 · 禁用周末", "DisabledDate 同时影响点击与方向键（键盘会跳过禁用格）。", 260, weekend})

	sundayWeek := datepicker.New(datepicker.GranularityDay)
	sundayWeek.Labels = chinese
	sundayWeek.WeekStart = time.Sunday
	sundayWeek.SetValue(day(2026, time.October, 5))
	m.add(singleRow{"日粒度 · 周日开始", "WeekStart 决定星期行的首列，默认是周一；点“今天”看它跳回 11 月。", 260, sundayWeek})

	// 5. Sizes and states.
	small := datepicker.New(datepicker.GranularityDay)
	small.Labels = chinese
	small.Small = true
	small.AllowClear = false
	small.SetValue(day(2026, time.June, 18))
	m.add(singleRow{"日粒度 · 小尺寸 + 不可清除", "small 触发框 28 高、格 24；AllowClear=false 隐藏清除按钮。", 230, small})

	invalid := datepicker.New(datepicker.GranularityDay)
	invalid.Labels = chinese
	invalid.Invalid = true
	invalid.SetValue(day(2027, time.February, 28))
	m.add(singleRow{"日粒度 · 错误态", "Invalid 只改外观与无障碍语义，不阻止选值。", 230, invalid})

	disabled := datepicker.New(datepicker.GranularityDay)
	disabled.Labels = chinese
	disabled.Disabled = true
	disabled.SetValue(day(2026, time.June, 18))
	m.add(singleRow{"日粒度 · 禁用", "Disabled 时不可聚焦、不可打开、不画面板。", 230, disabled})

	// 6. Presets: chips along the panel's bottom, Today on the same row.
	presets := datepicker.New(datepicker.GranularityDay)
	presets.Labels = chinese
	presets.Presets = []datepicker.Preset{
		{Label: "圣诞", Value: day(2026, time.December, 25)},
		{Label: "元旦", Value: day(2027, time.January, 1)},
		{Label: "春节", Value: day(2027, time.February, 6)},
		{Label: "劳动节", Value: day(2027, time.May, 1)},
		{Label: "国庆节", Value: day(2027, time.October, 1)},
	}
	presets.OnChange = func(value time.Time, has bool) { m.last = "preset/选择：" + singleStatus(presets) }
	m.add(singleRow{"日粒度 · presets 换行", "presets 在面板底部；一行放不下会自动换行，今天固定在最后一行的右端。", 260, presets})

	// 7. English labels, for comparison with ChineseLabels.
	english := datepicker.New(datepicker.GranularityDay)
	english.Labels = datepicker.EnglishLabels()
	english.Format = "Jan 2, 2006"
	english.SetValue(day(2026, time.November, 22))
	m.add(singleRow{"日粒度 · 英文词表", "Labels 由调用方提供；空字段回退英文默认值。", 260, english})

	// 8. A day range with presets.
	rangeDay := datepicker.NewRange(datepicker.GranularityDay)
	rangeDay.Labels = chinese
	rangeDay.SetRange(day(2026, time.November, 3), day(2026, time.November, 9))
	rangeDay.Presets = []datepicker.RangePreset{
		{Label: "今天", Start: day(2026, time.November, 22), End: day(2026, time.November, 22)},
		{Label: "最近 7 天", Start: day(2026, time.November, 16), End: day(2026, time.November, 22)},
		{Label: "本月", Start: day(2026, time.November, 1), End: day(2026, time.November, 30)},
		{Label: "本季度", Start: day(2026, time.October, 1), End: day(2026, time.December, 31)},
		{Label: "今年", Start: day(2026, time.January, 1), End: day(2026, time.December, 31)},
		{Label: "去年", Start: day(2025, time.January, 1), End: day(2025, time.December, 31)},
	}
	rangeDay.OnChange = func(start, end time.Time, has bool) { m.last = "区间：" + rangeStatus(rangeDay) }
	m.addRange(rangeRow{"区间 · 日粒度", "闭区间；点第一端后悬停预览，第二端落定才回调；presets 一行放不下会换行。", 560, rangeDay})

	// 9. Month range: two years side by side.
	rangeMonth := datepicker.NewRange(datepicker.GranularityMonth)
	rangeMonth.Labels = chinese
	rangeMonth.Format = "2006年1月"
	rangeMonth.SetRange(day(2026, time.March, 1), day(2026, time.September, 1))
	rangeMonth.OnChange = func(start, end time.Time, has bool) { m.last = "月度区间：" + rangeStatus(rangeMonth) }
	m.addRange(rangeRow{"区间 · 月粒度", "两个面板分别是一年，各 12 个月；翻页时两端一起走。", 480, rangeMonth})

	// 10. Year range: two decades.
	rangeYear := datepicker.NewRange(datepicker.GranularityYear)
	rangeYear.Labels = chinese
	rangeYear.Min = day(2000, time.January, 1)
	rangeYear.Max = day(2035, time.December, 1)
	rangeYear.OnChange = func(start, end time.Time, has bool) { m.last = "年度区间：" + rangeStatus(rangeYear) }
	m.addRange(rangeRow{"区间 · 年粒度", "两个十年段面板；min/max 之外的年份禁用。", 430, rangeYear})

	// 11. Weekends disabled inside a bounded range, with a slash format.
	rangeBusy := datepicker.NewRange(datepicker.GranularityDay)
	rangeBusy.Labels = chinese
	rangeBusy.Format = "2006/01/02"
	rangeBusy.Min = day(2026, time.November, 1)
	rangeBusy.Max = day(2026, time.December, 31)
	rangeBusy.DisabledDate = func(value time.Time) bool {
		return value.Weekday() == time.Saturday || value.Weekday() == time.Sunday
	}
	rangeBusy.OnChange = func(start, end time.Time, has bool) { m.last = "工作日区间：" + rangeStatus(rangeBusy) }
	m.addRange(rangeRow{"区间 · 禁用周末 + min/max", "禁用格不可点、键盘跳过；越界值仍可显示但不参与选择。", 560, rangeBusy})

	// 12. English labels for a range.
	rangeEnglish := datepicker.NewRange(datepicker.GranularityDay)
	rangeEnglish.Labels = datepicker.EnglishLabels()
	rangeEnglish.Format = "Jan 2, 2006"
	rangeEnglish.OnChange = func(start, end time.Time, has bool) { m.last = "range: " + rangeStatus(rangeEnglish) }
	m.addRange(rangeRow{"区间 · 英文词表", "同一套组件配 EnglishLabels 的效果。", 520, rangeEnglish})

	// 13. States: small, invalid, disabled.
	rangeSmall := datepicker.NewRange(datepicker.GranularityDay)
	rangeSmall.Labels = chinese
	rangeSmall.Small = true
	rangeSmall.AllowClear = false
	m.addRange(rangeRow{"区间 · 小尺寸 + 不可清除", "触发框 28 高；两字段与分隔符同步收缩。", 480, rangeSmall})

	rangeInvalid := datepicker.NewRange(datepicker.GranularityDay)
	rangeInvalid.Labels = chinese
	rangeInvalid.Invalid = true
	rangeInvalid.SetRange(day(2026, time.December, 1), day(2026, time.November, 1))
	m.addRange(rangeRow{"区间 · 错误态", "Invalid 只改外观；SetRange 会把颠倒的两端自动交换。", 520, rangeInvalid})

	rangeDisabled := datepicker.NewRange(datepicker.GranularityDay)
	rangeDisabled.Labels = chinese
	rangeDisabled.Disabled = true
	rangeDisabled.SetRange(day(2026, time.November, 3), day(2026, time.November, 9))
	m.addRange(rangeRow{"区间 · 禁用", "不可聚焦、不可打开。", 520, rangeDisabled})

	return m
}

// row lays a label, the field and its current value on one line, with a note
// under it.
func row(c ui.C, label, note string, field func(ui.C) ui.Box, value string) {
	ui.Column(c).Gap(4).Children(func() {
		ui.Row(c).Gap(12).Children(func() {
			ui.Text(c, label).Width(190)
			field(c)
			ui.Text(c, value).Grow(1)
		})
		if note != "" {
			ui.Text(c, note).FillWidth()
		}
	})
}

func (m *model) page(c ui.C) {
	ui.Column(c).Fill().Pad(28).Gap(14).Children(func() {
		ui.Text(c, "DatePicker / DateRangePicker · 全用法").FillWidth()
		ui.Text(c, "页面可滚动。每行右侧是该控件的当前值；面板底部是调用方传入的 presets。").FillWidth()
		ui.Text(c, m.last).FillWidth()
		ui.Scroll(c).Grow(1).Gap(16).Scrollbars().Children(func() {
			ui.Text(c, "单值").FillWidth()
			for _, single := range m.singles {
				single := single
				row(c, single.label, single.note,
					func(c ui.C) ui.Box { return single.picker.View(c, single.label).Width(single.width) },
					singleStatus(single.picker))
			}
			ui.Text(c, "区间").FillWidth()
			for _, span := range m.ranges {
				span := span
				row(c, span.label, span.note,
					func(c ui.C) ui.Box { return span.picker.View(c, span.label).Width(span.width) },
					rangeStatus(span.picker))
			}
		})
	})
}

func main() {
	m := newModel()
	if err := ui.Run(app.Config{Title: "Viva · DatePicker", Width: 1160, Height: 860}, m.page); err != nil {
		log.Fatal(err)
	}
}
