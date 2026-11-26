//go:build !js

package datepicker_test

import (
	"testing"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/datepicker"
)

func date(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.Local) }

var today = func() time.Time { return time.Date(2026, 11, 25, 15, 30, 0, 0, time.Local) }

// changes counts the paints that reported a change, as a page reads Changed.
var changes int

// open runs a page holding one date field over *value, headlessly.
func open(t *testing.T, d *datepicker.DatePicker, value *time.Time) *headless.Session {
	t.Helper()
	changes = 0
	page := ui.NewPage(ui.PageConfig{View: func(c ui.C) {
		ui.Column(c).Pad(16).Children(func() {
			d.View(c, "Due", value).Width(220)
			if d.Changed() {
				changes++
			}
		})
	}})
	session, err := headless.Open(ui.Configure(page, app.Config{Title: "Date", Width: 420, Height: 460}), nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(session.Close)
	return session
}

// A click on a day chooses it and closes the month.
func TestClickingADayChoosesIt(t *testing.T) {
	value := date(2026, 11, 18)
	session := open(t, datepicker.New().Clock(today), &value)
	if err := session.ClickLabel("2026-11-18"); err != nil {
		t.Fatal(err)
	}
	if err := session.ClickLabel("button:24"); err != nil {
		t.Fatal(err)
	}
	if !value.Equal(date(2026, 11, 24)) {
		t.Fatalf("value = %v after a click on the 24th, want 2026-11-24", value)
	}
	if changes != 1 {
		t.Fatalf("Changed was reported in %d paints, want 1", changes)
	}
	if _, err := session.Locate("Next month"); err == nil {
		t.Fatal("the month stayed open after a day was chosen")
	}
}

// The month opens on the chosen date, and the arrow keys move from there: a day
// at a time, a week at a time, and Enter chooses the day the cursor rests on.
func TestKeysMoveThroughTheMonth(t *testing.T) {
	value := date(2026, 11, 18)
	session := open(t, datepicker.New().Clock(today), &value)
	if err := session.ClickLabel("2026-11-18"); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"right", "right", "down"} {
		if err := session.Keys(key); err != nil {
			t.Fatal(err)
		}
	}
	if err := session.Keys("enter"); err != nil {
		t.Fatal(err)
	}
	if !value.Equal(date(2026, 11, 27)) {
		t.Fatalf("value = %v after Right, Right, Down and Enter from the 18th, want 2026-11-27", value)
	}
}

// PageUp and PageDown move by a month and keep the day where the month is long
// enough, and the end of a short month otherwise.
func TestPagesMoveByAMonth(t *testing.T) {
	value := date(2026, 3, 31)
	session := open(t, datepicker.New().Clock(today), &value)
	if err := session.ClickLabel("2026-03-31"); err != nil {
		t.Fatal(err)
	}
	if err := session.Keys("pageup"); err != nil {
		t.Fatal(err)
	}
	if err := session.Keys("enter"); err != nil {
		t.Fatal(err)
	}
	if !value.Equal(date(2026, 2, 28)) {
		t.Fatalf("value = %v after PageUp from March 31, want 2026-02-28", value)
	}
}

// A day outside Min and Max cannot be chosen: the click does nothing and the
// month stays open.
func TestDaysOutsideTheRangeCannotBeChosen(t *testing.T) {
	value := date(2026, 11, 18)
	session := open(t, datepicker.New().Clock(today).Min(date(2026, 11, 5)).Max(date(2026, 11, 28)), &value)
	if err := session.ClickLabel("2026-11-18"); err != nil {
		t.Fatal(err)
	}
	session.ClickLabel("button:3")
	session.ClickLabel("button:29")
	if !value.Equal(date(2026, 11, 18)) {
		t.Fatalf("value = %v after a click on a day out of the range, want it kept", value)
	}
	if err := session.ClickLabel("button:28"); err != nil {
		t.Fatal(err)
	}
	if !value.Equal(date(2026, 11, 28)) {
		t.Fatalf("value = %v after a click on the last day of the range, want 2026-11-28", value)
	}
}

// The month opens on today when there is no date, and a date with a time of day
// is brought back to midnight.
func TestTodayOpensTheMonthAndDatesAreMidnight(t *testing.T) {
	var value time.Time
	session := open(t, datepicker.New().Clock(today).Placeholder("Pick a date"), &value)
	if err := session.ClickLabel("Pick a date"); err != nil {
		t.Fatal(err)
	}
	if err := session.Keys("enter"); err != nil {
		t.Fatal(err)
	}
	if !value.Equal(date(2026, 11, 25)) {
		t.Fatalf("value = %v after Enter on a fresh month, want today 2026-11-25", value)
	}

	late := time.Date(2026, 11, 18, 23, 59, 0, 0, time.Local)
	open(t, datepicker.New().Clock(today), &late)
	if !late.Equal(date(2026, 11, 18)) {
		t.Fatalf("a date with a time of day became %v, want midnight", late)
	}
}

func TestWarmPaintAllocatesNothing(t *testing.T) {
	value := date(2026, 11, 18)
	d := datepicker.New().Clock(today)
	view := func(c ui.C) { d.View(c, "Due", &value).Width(220) }
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: view, Theme: &theme})
	canvas := &gfx.Canvas{}
	canvas.SetReduceMotion(true)
	draw := func() {
		canvas.BeginFrame()
		canvas.Reset()
		page.Draw(canvas, gfx.Rect{Width: 420, Height: 460}, view)
		canvas.EndFrame()
	}
	draw()
	if allocations := testing.AllocsPerRun(100, draw); allocations != 0 {
		t.Fatalf("warm paint allocated %v times, want 0", allocations)
	}
}

// An open month repaints without allocating too: the numbers and the title are
// made once, and the days are values.
func TestOpenMonthAllocatesNothing(t *testing.T) {
	value := date(2026, 11, 18)
	session := open(t, datepicker.New().Clock(today), &value)
	if err := session.ClickLabel("2026-11-18"); err != nil {
		t.Fatal(err)
	}
	session.Frame()
	if allocations := testing.AllocsPerRun(50, func() { session.Frame() }); allocations != 0 {
		t.Fatalf("an open month allocated %v times a frame, want 0", allocations)
	}
}

func BenchmarkViewPaint(b *testing.B) {
	value := date(2026, 11, 18)
	d := datepicker.New().Clock(today)
	view := func(c ui.C) { d.View(c, "Due", &value).Width(220) }
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: view, Theme: &theme})
	canvas := &gfx.Canvas{}
	canvas.SetReduceMotion(true)
	b.ReportAllocs()
	for range b.N {
		canvas.BeginFrame()
		canvas.Reset()
		page.Draw(canvas, gfx.Rect{Width: 420, Height: 460}, view)
		canvas.EndFrame()
	}
}
