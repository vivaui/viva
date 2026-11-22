// Package datepicker adds a date field and a closed date range field to a
// page: a read-only trigger, a popup panel with day, month and year grids, and
// the keyboard and accessibility wiring a form needs. It is an opt-in page
// package, imported only by pages that use it.
//
//	field := datepicker.New(datepicker.GranularityDay)
//	field.Labels = datepicker.ChineseLabels()
//	field.OnChange = func(value time.Time, hasValue bool) { m.date, m.hasDate = value, hasValue }
//	field.View(c, "Start date").Width(240)
//
// Values are civil dates in the local zone, as antd's dayjs values are: a
// time.Time at 00:00 local time with the fields the granularity does not name
// zeroed (the first of the month, January 1st). SetValue takes the wall-clock
// fields of whatever time it is given and normalizes them, and values compare
// with time.Time.Equal. A zone whose local midnight is skipped by a DST change
// shifts that day's value to the first existing instant.
//
// The trigger is read-only and typing is not supported: values come from the
// panel, and Format only formats what it shows. The picker owns no hidden
// state: Value, HasValue and Open are fields the application can read and
// write, and OnChange/OnOpenChange report the user's choices. The panel traps
// the keyboard while it is open, closes on Escape or a press outside, and
// returns the keyboard to the field.
//
// Labels carries every user-visible string and falls back to English when a
// field is empty; ChineseLabels is a complete Chinese table. WeekStart decides
// the first column of the day grid and defaults to Monday.
package datepicker
