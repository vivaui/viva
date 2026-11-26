// Package datepicker is the opt-in page control for choosing a date: a button
// that shows the date and opens a month to choose it from. A page keeps a
// DatePicker in its model beside the date it edits:
//
//	m.due = datepicker.New().Placeholder("Pick a date")
//	// in the page:
//	m.due.View(c, "Due date", &m.dueDate).Width(220)
//	if m.due.Changed() {
//		// the user chose a date; m.dueDate holds it
//	}
//
// The date is a time.Time at midnight in its own location; the zero time is no
// date. A date the page sets with a time of day is brought back to midnight.
// The month opens at the chosen date, or at today when there is none. Arrow
// keys move by a day or a week, PageUp and PageDown by a month, Home and End to
// the start and end of the week, and Enter or Space chooses the day; Escape
// closes the month. Min and Max limit the dates that can be chosen, and a day
// outside them is dimmed.
//
// The words of the month are English by default; Labels gives another
// language's, and Layout is the time.Format layout the date is shown in.
//
// It is a separate package so an application that does not import it pays
// nothing for it. Keep a DatePicker in the model: its handlers and draw function
// are made once, so a warm paint allocates nothing. Do not copy a DatePicker
// after it has been declared.
package datepicker

import (
	"strconv"
	"time"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// Labels are the words of the month and of its buttons.
type Labels struct {
	// Months are the names of the months, January first.
	Months [12]string
	// Weekdays are the short names of the days of the week, Sunday first.
	Weekdays [7]string
	// Previous and Next name the buttons that move to the month before and after.
	Previous, Next string
}

// English is the default words.
var English = Labels{
	Months:   [12]string{"January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"},
	Weekdays: [7]string{"Su", "Mo", "Tu", "We", "Th", "Fr", "Sa"},
	Previous: "Previous month", Next: "Next month",
}

var (
	calendarIcon = gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{
		gfx.MoveTo(8, 2), gfx.LineTo(8, 6), gfx.MoveTo(16, 2), gfx.LineTo(16, 6),
		gfx.MoveTo(5, 4), gfx.LineTo(19, 4), gfx.QuadTo(21, 4, 21, 6), gfx.LineTo(21, 20), gfx.QuadTo(21, 22, 19, 22),
		gfx.LineTo(5, 22), gfx.QuadTo(3, 22, 3, 20), gfx.LineTo(3, 6), gfx.QuadTo(3, 4, 5, 4),
		gfx.MoveTo(3, 10), gfx.LineTo(21, 10),
	}}
	previousIcon = gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{
		gfx.MoveTo(15, 18), gfx.LineTo(9, 12), gfx.LineTo(15, 6),
	}}
	nextIcon = gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{
		gfx.MoveTo(9, 18), gfx.LineTo(15, 12), gfx.LineTo(9, 6),
	}}
)

// numbers are the words of the days of a month, made once.
var numbers [32]string

func init() {
	for i := range numbers {
		numbers[i] = strconv.Itoa(i)
	}
}

const (
	cell    = 36 // the width and height of a day
	weeks   = 6  // the lines of the grid
	padding = 12 // the room around the month inside its panel
	panel   = 7*cell + 2*padding
)

// DatePicker is one date field and its month. Configure it with the chainable
// methods; they may be called before each View.
type DatePicker struct {
	handlers ui.Handlers
	drawFn   func(*ui.DrawContext)

	placeholder string
	layout      string
	labels      Labels
	min, max    time.Time
	first       time.Weekday
	now         func() time.Time

	value  *time.Time
	open   bool
	cursor time.Time // the day the keyboard rests on; its month is the one shown

	focusOnOpen bool
	changed     bool
	reported    bool

	// What the trigger and the title say, made when the date or the month changes.
	shownDate  time.Time
	shownText  string
	titleMonth time.Time
	titleText  string
}

// New returns a date field of English words that shows a date as 2006-01-02.
func New() *DatePicker {
	d := &DatePicker{layout: "2006-01-02", labels: English, now: time.Now}
	d.handlers.Key = d.key
	d.handlers.Pointer = d.pointer
	d.drawFn = d.draw
	return d
}

// Placeholder is the text shown while there is no date.
func (d *DatePicker) Placeholder(text string) *DatePicker {
	d.placeholder = text
	return d
}

// Layout is the time.Format layout the date is shown in; 2006-01-02 by default.
func (d *DatePicker) Layout(layout string) *DatePicker {
	d.layout, d.shownText = layout, ""
	return d
}

// Labels gives the words of the month and its buttons in another language.
func (d *DatePicker) Labels(labels Labels) *DatePicker {
	d.labels, d.titleText, d.shownText = labels, "", ""
	return d
}

// Min is the earliest date that can be chosen; the zero time sets none.
func (d *DatePicker) Min(min time.Time) *DatePicker {
	d.min = day(min)
	return d
}

// Max is the latest date that can be chosen; the zero time sets none.
func (d *DatePicker) Max(max time.Time) *DatePicker {
	d.max = day(max)
	return d
}

// FirstWeekday is the day the weeks of the month start on; Sunday by default.
func (d *DatePicker) FirstWeekday(weekday time.Weekday) *DatePicker {
	d.first = weekday
	return d
}

// Clock gives the time that decides which day is today, instead of the system's,
// for a test or a page that runs on its own clock.
func (d *DatePicker) Clock(now func() time.Time) *DatePicker {
	if now != nil {
		d.now = now
	}
	return d
}

// Changed reports whether the user chose a new date in the paint that
// delivered it.
func (d *DatePicker) Changed() bool { return d.reported }

// day is t at midnight of its own day and location; the zero time stays zero.
func day(t time.Time) time.Time {
	if t.IsZero() {
		return t
	}
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func (d *DatePicker) allowed(t time.Time) bool {
	return (d.min.IsZero() || !t.Before(d.min)) && (d.max.IsZero() || !t.After(d.max))
}

// clamp is t brought into the range that can be chosen.
func (d *DatePicker) clamp(t time.Time) time.Time {
	if !d.min.IsZero() && t.Before(d.min) {
		return d.min
	}
	if !d.max.IsZero() && t.After(d.max) {
		return d.max
	}
	return t
}

// monthStart is the first day of the month of t.
func monthStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
}

// gridStart is the first day of the grid that shows the month of t: the start of
// the week the month begins in.
func (d *DatePicker) gridStart(t time.Time) time.Time {
	first := monthStart(t)
	back := (int(first.Weekday()) - int(d.first) + 7) % 7
	return first.AddDate(0, 0, -back)
}

// index is where day t is in the grid of the month of the cursor.
func (d *DatePicker) index(t time.Time) int {
	start := d.gridStart(d.cursor)
	return int(day(t).Sub(start).Hours()/24 + 0.5)
}

// move puts the cursor on t, within the dates that can be chosen.
func (d *DatePicker) move(t time.Time) {
	d.cursor = d.clamp(day(t))
}

// shiftMonth moves the cursor by months, keeping its day of the month where the
// month is long enough.
func (d *DatePicker) shiftMonth(months int) {
	target := monthStart(d.cursor).AddDate(0, months, 0)
	last := monthStart(target).AddDate(0, 1, -1).Day()
	d.move(time.Date(target.Year(), target.Month(), min(d.cursor.Day(), last), 0, 0, 0, 0, target.Location()))
}

// choose makes t the chosen date and closes the month.
func (d *DatePicker) choose(t time.Time) {
	t = day(t)
	if !d.allowed(t) {
		return
	}
	if !t.Equal(*d.value) {
		*d.value, d.changed = t, true
	}
	d.open = false
}

func (d *DatePicker) pointer(event gfx.PointerEvent) gfx.EventResult {
	if d.value == nil || event.Kind != gfx.PointerDown || event.Button != gfx.PointerButtonPrimary {
		return gfx.EventResult{}
	}
	i, ok := event.Target.In(d.handlers.Parts())
	if !ok {
		return gfx.EventResult{}
	}
	t := d.gridStart(d.cursor).AddDate(0, 0, i)
	if !d.allowed(t) {
		return gfx.EventResult{Handled: true}
	}
	d.choose(t)
	return gfx.EventResult{Handled: true, Repaint: true}
}

func (d *DatePicker) key(event gfx.KeyEvent) gfx.EventResult {
	if d.value == nil || event.Text != "" {
		return gfx.EventResult{}
	}
	switch event.Key {
	case gfx.KeyLeft:
		d.move(d.cursor.AddDate(0, 0, -1))
	case gfx.KeyRight:
		d.move(d.cursor.AddDate(0, 0, 1))
	case gfx.KeyUp:
		d.move(d.cursor.AddDate(0, 0, -7))
	case gfx.KeyDown:
		d.move(d.cursor.AddDate(0, 0, 7))
	case gfx.KeyPageUp:
		d.shiftMonth(-1)
	case gfx.KeyPageDown:
		d.shiftMonth(1)
	case gfx.KeyHome:
		d.move(d.cursor.AddDate(0, 0, -((int(d.cursor.Weekday()) - int(d.first) + 7) % 7)))
	case gfx.KeyEnd:
		d.move(d.cursor.AddDate(0, 0, 6-((int(d.cursor.Weekday())-int(d.first)+7)%7)))
	case gfx.KeyEnter, gfx.KeySpace:
		d.choose(d.cursor)
		return gfx.EventResult{Handled: true, Repaint: true}
	default:
		return gfx.EventResult{}
	}
	return gfx.EventResult{Handled: true, Repaint: true, Focus: d.handlers.Parts().At(d.index(d.cursor)), FocusVisible: true}
}

// View declares the field at c, showing and editing *value. label names it for
// assistive technology. The button is one control high; give it a Width. Declare
// the view on its own, not inside a Memo.
func (d *DatePicker) View(c ui.C, label string, value *time.Time) ui.Box {
	if d == nil {
		panic("datepicker: View needs its DatePicker")
	}
	if value == nil {
		panic("datepicker: View needs the date")
	}
	d.reported, d.changed = d.changed, false
	d.value = value
	*value = day(*value)
	if !value.Equal(d.shownDate) || d.shownText == "" {
		d.shownDate = *value
		d.shownText = d.placeholder
		if !value.IsZero() {
			d.shownText = value.Format(d.layout)
		}
	}
	trigger := ui.Button(c, d.shownText).Outline().Icon(calendarIcon)
	if value.IsZero() {
		trigger.Muted()
	}
	if trigger.Clicked() {
		d.open = !d.open
		if d.open {
			d.cursor = d.clamp(day(d.now()))
			if !value.IsZero() {
				d.cursor = *value
			}
			d.focusOnOpen = true
		}
	}
	ui.Popover(c, trigger, &d.open, ui.PopoverSpec{Align: ui.Start, Padding: padding, Width: panel}, func() {
		d.month(c, label)
	})
	return trigger
}

// month declares the header, the days of the week and the days of the month.
func (d *DatePicker) month(c ui.C, label string) {
	if !monthStart(d.cursor).Equal(d.titleMonth) || d.titleText == "" {
		d.titleMonth = monthStart(d.cursor)
		d.titleText = d.labels.Months[d.cursor.Month()-1] + " " + strconv.Itoa(d.cursor.Year())
	}
	ui.Column(c).Gap(4).Children(func() {
		ui.Row(c).Items(ui.Center).Children(func() {
			previous := ui.Button(c, d.labels.Previous).Icon(previousIcon).IconOnly().Ghost()
			ui.Text(c, d.titleText).Grow(1).TextCenter().Weight(gfx.WeightMedium)
			next := ui.Button(c, d.labels.Next).Icon(nextIcon).IconOnly().Ghost()
			if previous.Clicked() {
				d.shiftMonth(-1)
			}
			if next.Clicked() {
				d.shiftMonth(1)
			}
		})
		ui.Row(c).Children(func() {
			for i := range 7 {
				name := d.labels.Weekdays[(int(d.first)+i)%7]
				ui.Row(c).Size(cell, cell*2/3).Shrink(0).Center().Children(func() {
					ui.Text(c, name).Muted().FontSize(c.Theme().Metrics.FontSizeS)
				})
			}
		})
		ui.Control(c, label, &d.handlers).Size(7*cell, weeks*cell).Shrink(0).Draw(d.drawFn)
	})
}

// draw paints the days of the month and declares each as a part of the control.
func (d *DatePicker) draw(dc *ui.DrawContext) {
	if d.value == nil {
		return
	}
	canvas, theme := dc.Canvas, dc.Theme
	parts := d.handlers.Parts()
	start := d.gridStart(d.cursor)
	today := day(d.now())
	cursor := d.index(d.cursor)
	if d.focusOnOpen {
		d.focusOnOpen = false
		canvas.RequestFocus(parts.At(cursor))
	}
	size := theme.Metrics.FontSizeM
	for i := range 7 * weeks {
		date := start.AddDate(0, 0, i)
		bounds := gfx.Rect{X: dc.Rect.X + float32(i%7)*cell, Y: dc.Rect.Y + float32(i/7)*cell, Width: cell, Height: cell}
		inner := gfx.Rect{X: bounds.X + 2, Y: bounds.Y + 2, Width: cell - 4, Height: cell - 4}
		allowed := d.allowed(date)
		chosen := date.Equal(*d.value)
		hovered := allowed && canvas.Hovered() == parts.At(i)
		ink := theme.Foreground
		switch {
		case chosen:
			canvas.Rounded(inner, theme.Metrics.RadiusControl, theme.Primary)
			ink = theme.PrimaryForeground
		case hovered:
			canvas.Rounded(inner, theme.Metrics.RadiusControl, theme.Hover)
		case date.Equal(today):
			canvas.RoundedBorder(inner, theme.Metrics.RadiusControl, 1, theme.Border, gfx.Color{})
		}
		if date.Month() != d.cursor.Month() && !chosen {
			ink = theme.MutedForeground
		}
		if !allowed {
			ink = theme.MutedForeground
			ink.Alpha *= 0.5
		}
		canvas.LabelCenter(numbers[date.Day()], bounds, gfx.TextStyle{Size: size, Weight: gfx.WeightRegular, Color: ink})
		if canvas.FocusVisible() == parts.At(i) {
			ring(canvas, theme, inner)
		}
		canvas.HitWithOptions(parts.At(i), bounds, numbers[date.Day()], gfx.HitOptions{
			Role: gfx.RoleButton, Disabled: !allowed, Selected: chosen, SkipFocus: i != cursor, SkipTab: i != cursor,
		})
	}
}

// ring paints the focus ring around a day.
func ring(canvas *gfx.Canvas, theme *gfx.Theme, b gfx.Rect) {
	canvas.RoundedBorder(b, theme.Metrics.RadiusControl, 2, theme.Ring, gfx.Color{})
}
