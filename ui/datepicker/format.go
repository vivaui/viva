package datepicker

import "time"

// normalize turns any instant into the picker's value for a granularity: a
// civil date at 00:00 in the local zone, with the fields the granularity does
// not name zeroed. The wall-clock fields of t are taken as they are, whatever
// Location t carries, so time.Now() is the user's own calendar day. In a zone
// where local midnight does not exist on that date (a DST change), time.Date
// moves to the first existing instant, as antd's dayjs does.
func normalize(t time.Time, granularity Granularity) time.Time {
	if t.IsZero() {
		return time.Time{}
	}
	year, month, day := t.Date()
	switch granularity {
	case GranularityMonth:
		day = 1
	case GranularityYear:
		month, day = time.January, 1
	}
	return time.Date(year, month, day, 0, 0, 0, 0, time.Local)
}

// pageStart is the first value of the page that holds t at a granularity: the
// month, the year or the decade.
func pageStart(t time.Time, granularity Granularity) time.Time {
	if t.IsZero() {
		return time.Time{}
	}
	year, month, _ := t.Date()
	switch granularity {
	case GranularityMonth:
		month = time.January
	case GranularityYear:
		year, month = year/10*10, time.January
	}
	return time.Date(year, month, 1, 0, 0, 0, 0, time.Local)
}

// shiftPage moves a page start by whole pages.
func shiftPage(page time.Time, granularity Granularity, pages int) time.Time {
	switch granularity {
	case GranularityMonth:
		return page.AddDate(pages, 0, 0)
	case GranularityYear:
		return page.AddDate(pages*10, 0, 0)
	}
	return page.AddDate(0, pages, 0)
}

// withinPage reports whether t is on the page page starts.
func withinPage(t, page time.Time, granularity Granularity) bool {
	switch granularity {
	case GranularityMonth:
		return t.Year() == page.Year()
	case GranularityYear:
		return t.Year()/10 == page.Year()/10
	}
	return t.Year() == page.Year() && t.Month() == page.Month()
}

// Labels holds every string the picker shows. Empty fields fall back to the
// English defaults, as AvatarEditor's do; use ChineseLabels for the Chinese
// table. The picker never looks a locale up by itself.
type Labels struct {
	Placeholder      string
	RangePlaceholder [2]string
	RangeSeparator   string
	Previous         string
	Next             string
	PreviousYear     string
	NextYear         string
	PreviousDecade   string
	NextDecade       string
	Today            string
	Clear            string
	Dismiss          string
	ShowMonth        string
	ShowYear         string
	// Months is January to December.
	Months [12]string
	// Weekdays is Sunday to Saturday, short names; the grid rotates them by
	// WeekStart.
	Weekdays [7]string
	// Layouts format a value for the trigger; index 0 day, 1 month, 2 year.
	Layouts [3]string
	// TitleLayouts format a page's title.
	TitleLayouts [3]string
	// A11yLayouts name a cell for assistive technology.
	A11yLayouts [3]string
}

func dayLayout() string   { return "2006-01-02" }
func monthLayout() string { return "2006-01" }
func yearLayout() string  { return "2006" }

// EnglishLabels is the reference English table.
func EnglishLabels() Labels {
	return Labels{
		Placeholder:      "Select date",
		RangePlaceholder: [2]string{"Start date", "End date"},
		RangeSeparator:   "→",
		Previous:         "Previous month",
		Next:             "Next month",
		PreviousYear:     "Previous year",
		NextYear:         "Next year",
		PreviousDecade:   "Previous decade",
		NextDecade:       "Next decade",
		Today:            "Today",
		Clear:            "Clear",
		Dismiss:          "Dismiss calendar",
		ShowMonth:        "Show month",
		ShowYear:         "Show year",
		Months: [12]string{
			"January", "February", "March", "April", "May", "June",
			"July", "August", "September", "October", "November", "December",
		},
		Weekdays:     [7]string{"Su", "Mo", "Tu", "We", "Th", "Fr", "Sa"},
		Layouts:      [3]string{dayLayout(), monthLayout(), yearLayout()},
		TitleLayouts: [3]string{"January 2006", "2006", ""},
		A11yLayouts:  [3]string{dayLayout(), monthLayout(), yearLayout()},
	}
}

// ChineseLabels is a complete Chinese table, for applications that do not bring
// their own words.
func ChineseLabels() Labels {
	return Labels{
		Placeholder:      "选择日期",
		RangePlaceholder: [2]string{"开始日期", "结束日期"},
		RangeSeparator:   "→",
		Previous:         "上个月",
		Next:             "下个月",
		PreviousYear:     "上一年",
		NextYear:         "下一年",
		PreviousDecade:   "上一个十年",
		NextDecade:       "下一个十年",
		Today:            "今天",
		Clear:            "清除",
		Dismiss:          "关闭日历",
		ShowMonth:        "选择月份",
		ShowYear:         "选择年份",
		Months: [12]string{
			"一月", "二月", "三月", "四月", "五月", "六月",
			"七月", "八月", "九月", "十月", "十一月", "十二月",
		},
		Weekdays:     [7]string{"日", "一", "二", "三", "四", "五", "六"},
		Layouts:      [3]string{dayLayout(), monthLayout(), yearLayout()},
		TitleLayouts: [3]string{"2006年1月", "2006年", ""},
		A11yLayouts:  [3]string{"2006年1月2日", "2006年1月", "2006年"},
	}
}

// resolveLabels fills every empty field from EnglishLabels. Arrays are taken
// whole: a caller that sets one month name sets them all.
func resolveLabels(labels Labels) Labels {
	defaults := EnglishLabels()
	resolve := func(value, fallback string) string {
		if value == "" {
			return fallback
		}
		return value
	}
	labels.Placeholder = resolve(labels.Placeholder, defaults.Placeholder)
	labels.RangeSeparator = resolve(labels.RangeSeparator, defaults.RangeSeparator)
	labels.RangePlaceholder[0] = resolve(labels.RangePlaceholder[0], defaults.RangePlaceholder[0])
	labels.RangePlaceholder[1] = resolve(labels.RangePlaceholder[1], defaults.RangePlaceholder[1])
	labels.Previous = resolve(labels.Previous, defaults.Previous)
	labels.Next = resolve(labels.Next, defaults.Next)
	labels.PreviousYear = resolve(labels.PreviousYear, defaults.PreviousYear)
	labels.NextYear = resolve(labels.NextYear, defaults.NextYear)
	labels.PreviousDecade = resolve(labels.PreviousDecade, defaults.PreviousDecade)
	labels.NextDecade = resolve(labels.NextDecade, defaults.NextDecade)
	labels.Today = resolve(labels.Today, defaults.Today)
	labels.Clear = resolve(labels.Clear, defaults.Clear)
	labels.Dismiss = resolve(labels.Dismiss, defaults.Dismiss)
	labels.ShowMonth = resolve(labels.ShowMonth, defaults.ShowMonth)
	labels.ShowYear = resolve(labels.ShowYear, defaults.ShowYear)
	if labels.Months == [12]string{} {
		labels.Months = defaults.Months
	}
	if labels.Weekdays == [7]string{} {
		labels.Weekdays = defaults.Weekdays
	}
	for index := range labels.Layouts {
		labels.Layouts[index] = resolve(labels.Layouts[index], defaults.Layouts[index])
	}
	for index := range labels.TitleLayouts {
		labels.TitleLayouts[index] = resolve(labels.TitleLayouts[index], defaults.TitleLayouts[index])
	}
	for index := range labels.A11yLayouts {
		labels.A11yLayouts[index] = resolve(labels.A11yLayouts[index], defaults.A11yLayouts[index])
	}
	return labels
}
