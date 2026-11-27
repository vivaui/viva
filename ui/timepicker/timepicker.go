// Package timepicker is the opt-in page control for choosing a time of day: a
// button that shows the time and opens columns of hours and minutes, and
// seconds with Seconds, to choose it from. A page keeps a TimePicker in its model
// beside the time it edits:
//
//	m.start = timepicker.New().Placeholder("Pick a time").MinuteStep(5)
//	// in the page:
//	m.start.View(c, "Start", &m.startAt).Width(160)
//	if m.start.Changed() {
//		// the user chose a time; m.startAt holds it
//	}
//
// A Value is a time on the 24 hour clock; Valid is false while there is none.
// Choosing a number in a column sets that part of the time at once, so the
// value follows what the user points at. The wheel scrolls a column, Up and Down
// change the part the focus is in by one step and Left and Right move between the
// columns; Enter closes the columns and Escape does too.
//
// MinuteStep and SecondStep limit the minutes and seconds to multiples of a
// step, and a time the page sets between two steps is brought to the one below.
//
// It is a separate package so an application that does not import it pays
// nothing for it. Keep a TimePicker in the model: its handlers and draw function
// are made once, so a warm paint allocates nothing. Do not copy a TimePicker after
// it has been declared.
package timepicker

import (
	"strconv"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// Value is a time of day.
type Value struct {
	Hour, Minute, Second int
	// Valid is whether there is a time at all.
	Valid bool
}

var clockIcon = gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{
	gfx.MoveTo(12, 6), gfx.LineTo(12, 12), gfx.LineTo(16, 14),
	gfx.MoveTo(22, 12), gfx.CubicTo(22, 17.52, 17.52, 22, 12, 22), gfx.CubicTo(6.48, 22, 2, 17.52, 2, 12),
	gfx.CubicTo(2, 6.48, 6.48, 2, 12, 2), gfx.CubicTo(17.52, 2, 22, 6.48, 22, 12),
}}

// pad is the two digit words of the numbers of a column, made once.
var pad [60]string

func init() {
	for i := range pad {
		pad[i] = strconv.Itoa(i/10) + strconv.Itoa(i%10)
	}
}

const (
	columnWidth = 56
	rowHeight   = 32
	visibleRows = 7
	padding     = 8
	hourColumn  = 0
)

// TimePicker is one time field and its columns. Configure it with the chainable
// methods; they may be called before each View.
type TimePicker struct {
	handlers ui.Handlers
	drawFn   func(*ui.DrawContext)

	placeholder string
	minuteStep  int
	secondStep  int
	seconds     bool
	units       [3]string

	value       *Value
	open        bool
	active      int        // the column the keyboard is in
	offsets     [3]float32 // how far each column is scrolled, in pixels
	focusOnOpen bool
	changed     bool
	reported    bool

	shown     Value
	shownText string
}

// New returns a time field of hours and minutes in steps of one.
func New() *TimePicker {
	t := &TimePicker{minuteStep: 1, secondStep: 1, units: [3]string{"Hours", "Minutes", "Seconds"}}
	t.handlers.Key = t.key
	t.handlers.Pointer = t.pointer
	t.handlers.Scroll = t.scroll
	t.drawFn = t.draw
	return t
}

// Placeholder is the text shown while there is no time.
func (t *TimePicker) Placeholder(text string) *TimePicker {
	t.placeholder, t.shownText = text, ""
	return t
}

// MinuteStep limits the minutes to multiples of step; one by default.
func (t *TimePicker) MinuteStep(step int) *TimePicker {
	t.minuteStep = max(step, 1)
	return t
}

// SecondStep limits the seconds to multiples of step; one by default.
func (t *TimePicker) SecondStep(step int) *TimePicker {
	t.secondStep = max(step, 1)
	return t
}

// Seconds adds a column of seconds.
func (t *TimePicker) Seconds() *TimePicker {
	t.seconds = true
	t.shownText = ""
	return t
}

// Units are the names of the columns for assistive technology, "Hours",
// "Minutes" and "Seconds" by default, for a page in another language.
func (t *TimePicker) Units(hours, minutes, seconds string) *TimePicker {
	t.units = [3]string{hours, minutes, seconds}
	return t
}

// Changed reports whether the user chose a new time in the paint that delivered it.
func (t *TimePicker) Changed() bool { return t.reported }

func (t *TimePicker) columns() int {
	if t.seconds {
		return 3
	}
	return 2
}

// size is the number of rows of a column, and step the distance between them.
func (t *TimePicker) size(column int) (rows, step int) {
	switch column {
	case hourColumn:
		return 24, 1
	case 1:
		return (59 + t.minuteStep) / t.minuteStep, t.minuteStep
	}
	return (59 + t.secondStep) / t.secondStep, t.secondStep
}

// part is the number a column holds.
func (v Value) part(column int) int {
	switch column {
	case hourColumn:
		return v.Hour
	case 1:
		return v.Minute
	}
	return v.Second
}

// with is v with a part of it changed.
func (v Value) with(column, number int) Value {
	switch column {
	case hourColumn:
		v.Hour = number
	case 1:
		v.Minute = number
	default:
		v.Second = number
	}
	v.Valid = true
	return v
}

// tidy is v as a time the columns can show: in range, on the steps, with the
// seconds dropped when there is no column for them.
func (t *TimePicker) tidy(v Value) Value {
	if !v.Valid {
		return Value{}
	}
	v.Hour = min(max(v.Hour, 0), 23)
	v.Minute = min(max(v.Minute, 0), 59) / t.minuteStep * t.minuteStep
	v.Second = min(max(v.Second, 0), 59) / t.secondStep * t.secondStep
	if !t.seconds {
		v.Second = 0
	}
	return v
}

func (t *TimePicker) text(v Value) string {
	if !v.Valid {
		return t.placeholder
	}
	text := pad[v.Hour] + ":" + pad[v.Minute]
	if t.seconds {
		text += ":" + pad[v.Second]
	}
	return text
}

// choose sets a part of the value and scrolls the column to it.
func (t *TimePicker) choose(column, number int) {
	next := t.tidy(t.value.with(column, number))
	if !t.value.Valid {
		// A first choice makes a time: the other parts start at zero.
		next = t.tidy(Value{Valid: true}.with(column, number))
	}
	if next != *t.value {
		*t.value, t.changed = next, true
	}
	t.reveal(column)
}

// reveal scrolls a column so the number it holds is in the middle of its rows.
func (t *TimePicker) reveal(column int) {
	rows, step := t.size(column)
	index := t.value.part(column) / step
	total := float32(rows * rowHeight)
	view := float32(visibleRows * rowHeight)
	offset := float32(index)*rowHeight + rowHeight/2 - view/2
	t.offsets[column] = min(max(offset, 0), max(total-view, 0))
}

func (t *TimePicker) scroll(event gfx.ScrollEvent) gfx.EventResult {
	if t.value == nil {
		return gfx.EventResult{}
	}
	column, ok := event.Target.In(t.handlers.Parts())
	if !ok {
		return gfx.EventResult{}
	}
	column /= 64
	if column >= t.columns() {
		return gfx.EventResult{}
	}
	rows, _ := t.size(column)
	limit := max(float32(rows*rowHeight)-visibleRows*rowHeight, 0)
	step := event.DeltaY
	if event.Pages != 0 {
		step = event.StepYIn(rowHeight, visibleRows*rowHeight)
	}
	t.offsets[column] = min(max(t.offsets[column]-step, 0), limit)
	return gfx.EventResult{Handled: true, Repaint: true}
}

func (t *TimePicker) pointer(event gfx.PointerEvent) gfx.EventResult {
	if t.value == nil || event.Kind != gfx.PointerDown || event.Button != gfx.PointerButtonPrimary {
		return gfx.EventResult{}
	}
	index, ok := event.Target.In(t.handlers.Parts())
	if !ok {
		return gfx.EventResult{}
	}
	column, row := index/64, index%64
	_, step := t.size(column)
	t.active = column
	t.choose(column, row*step)
	return gfx.EventResult{Handled: true, Repaint: true, Focus: t.handlers.Parts().At(t.focusIndex())}
}

// focusIndex is the part the keyboard rests on: the chosen number of the active
// column.
func (t *TimePicker) focusIndex() int {
	_, step := t.size(t.active)
	return t.active*64 + t.value.part(t.active)/step
}

func (t *TimePicker) key(event gfx.KeyEvent) gfx.EventResult {
	if t.value == nil || event.Text != "" {
		return gfx.EventResult{}
	}
	rows, step := t.size(t.active)
	number := t.value.part(t.active)
	switch event.Key {
	case gfx.KeyUp:
		t.choose(t.active, (number/step+rows-1)%rows*step)
	case gfx.KeyDown:
		t.choose(t.active, (number/step+1)%rows*step)
	case gfx.KeyLeft:
		t.active = (t.active + t.columns() - 1) % t.columns()
	case gfx.KeyRight:
		t.active = (t.active + 1) % t.columns()
	case gfx.KeyEnter:
		t.open = false
		return gfx.EventResult{Handled: true, Repaint: true}
	default:
		return gfx.EventResult{}
	}
	return gfx.EventResult{Handled: true, Repaint: true, Focus: t.handlers.Parts().At(t.focusIndex()), FocusVisible: true}
}

// View declares the field at c, showing and editing *value. label names it for
// assistive technology. The button is one control high; give it a Width. Declare
// the view on its own, not inside a Memo.
func (t *TimePicker) View(c ui.C, label string, value *Value) ui.Box {
	if t == nil {
		panic("timepicker: View needs its TimePicker")
	}
	if value == nil {
		panic("timepicker: View needs the time")
	}
	t.reported, t.changed = t.changed, false
	t.value = value
	*value = t.tidy(*value)
	if *value != t.shown || t.shownText == "" {
		t.shown = *value
		t.shownText = t.text(*value)
	}
	trigger := ui.Button(c, t.shownText).Outline().Icon(clockIcon)
	if !value.Valid {
		trigger.Muted()
	}
	if trigger.Clicked() {
		t.open = !t.open
		if t.open {
			t.active, t.focusOnOpen = hourColumn, true
			for column := range t.columns() {
				t.reveal(column)
			}
		}
	}
	width := float32(t.columns()*columnWidth + 2*padding)
	ui.Popover(c, trigger, &t.open, ui.PopoverSpec{Align: ui.Start, Padding: padding, Width: width}, func() {
		ui.Control(c, label, &t.handlers).Size(float32(t.columns()*columnWidth), visibleRows*rowHeight).Shrink(0).Draw(t.drawFn)
	})
	return trigger
}

// draw paints the columns and declares each number as a part of the control.
func (t *TimePicker) draw(d *ui.DrawContext) {
	if t.value == nil {
		return
	}
	canvas, theme := d.Canvas, d.Theme
	parts := t.handlers.Parts()
	if t.focusOnOpen {
		t.focusOnOpen = false
		canvas.RequestFocus(parts.At(t.focusIndex()))
	}
	size := theme.Metrics.FontSizeM
	rest := t.focusIndex()
	for column := range t.columns() {
		area := gfx.Rect{X: d.Rect.X + float32(column*columnWidth), Y: d.Rect.Y, Width: columnWidth, Height: visibleRows * rowHeight}
		rows, step := t.size(column)
		chosen := t.value.part(column)
		canvas.Clip(area)
		first := int(t.offsets[column] / rowHeight)
		for row := first; row < min(rows, first+visibleRows+1); row++ {
			bounds := gfx.Rect{X: area.X, Y: area.Y + float32(row)*rowHeight - t.offsets[column], Width: columnWidth, Height: rowHeight}
			inner := gfx.Rect{X: bounds.X + 4, Y: bounds.Y + 2, Width: columnWidth - 8, Height: rowHeight - 4}
			index := column*64 + row
			number := row * step
			ink := theme.Foreground
			selected := t.value.Valid && number == chosen
			switch {
			case selected:
				canvas.Rounded(inner, theme.Metrics.RadiusControl, theme.Primary)
				ink = theme.PrimaryForeground
			case canvas.Hovered() == parts.At(index):
				canvas.Rounded(inner, theme.Metrics.RadiusControl, theme.Hover)
			}
			canvas.LabelCenter(pad[number], bounds, gfx.TextStyle{Size: size, Weight: gfx.WeightRegular, Color: ink})
			if canvas.FocusVisible() == parts.At(index) {
				canvas.RoundedBorder(inner, theme.Metrics.RadiusControl, 2, theme.Ring, gfx.Color{})
			}
			canvas.HitWithOptions(parts.At(index), bounds, pad[number], gfx.HitOptions{
				Role: gfx.RoleButton, Selected: selected, SkipFocus: index != rest, SkipTab: index != rest,
				Description: t.units[column],
			})
		}
		canvas.Unclip()
	}
}
