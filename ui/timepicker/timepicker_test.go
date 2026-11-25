package timepicker_test

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/timepicker"
)

// changes counts the paints that reported a change, as a page reads Changed.
var changes int

// open runs a page holding one time field over *value, headlessly.
func open(t *testing.T, p *timepicker.TimePicker, value *timepicker.Value) *headless.Session {
	t.Helper()
	changes = 0
	page := ui.NewPage(ui.PageConfig{View: func(c ui.C) {
		ui.Column(c).Pad(16).Children(func() {
			p.View(c, "Start", value).Width(220)
			if p.Changed() {
				changes++
			}
		})
	}})
	session, err := headless.Open(ui.Configure(page, app.Config{Title: "Time", Width: 420, Height: 420}), nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(session.Close)
	return session
}

func at(hour, minute, second int) timepicker.Value {
	return timepicker.Value{Hour: hour, Minute: minute, Second: second, Valid: true}
}

// A click on a number sets that part of the time at once.
func TestClickingANumberSetsThatPart(t *testing.T) {
	value := at(9, 30, 0)
	session := open(t, timepicker.New().MinuteStep(5), &value)
	if err := session.ClickLabel("09:30"); err != nil {
		t.Fatal(err)
	}
	if err := session.ClickLabel("45"); err != nil {
		t.Fatal(err)
	}
	if value != at(9, 45, 0) {
		t.Fatalf("value = %+v after a click on 45, want 09:45", value)
	}
	if changes != 1 {
		t.Fatalf("Changed was reported in %d paints, want 1", changes)
	}
}

// Up and Down change the part the focus is in, wrapping at the ends, and Left
// and Right move between the columns; Enter closes the columns.
func TestKeysChangeTheActivePart(t *testing.T) {
	value := at(23, 55, 0)
	session := open(t, timepicker.New().MinuteStep(5), &value)
	if err := session.ClickLabel("23:55"); err != nil {
		t.Fatal(err)
	}
	for _, step := range []struct {
		key  string
		want timepicker.Value
	}{
		{"down", at(0, 55, 0)}, // the hours wrap
		{"right", at(0, 55, 0)},
		{"down", at(0, 0, 0)}, // the minutes wrap in steps of five
		{"up", at(0, 55, 0)},
		{"left", at(0, 55, 0)},
		{"up", at(23, 55, 0)},
	} {
		if err := session.Keys(step.key); err != nil {
			t.Fatal(err)
		}
		if value != step.want {
			t.Fatalf("value = %+v after %q, want %+v", value, step.key, step.want)
		}
	}
	if err := session.Keys("enter"); err != nil {
		t.Fatal(err)
	}
	if _, err := session.Locate("Start"); err == nil {
		t.Fatal("the columns stayed open after Enter")
	}
}

// With no time yet, the first choice makes one and the other parts start at zero.
func TestFirstChoiceMakesATime(t *testing.T) {
	var value timepicker.Value
	session := open(t, timepicker.New().Placeholder("Pick a time"), &value)
	if err := session.ClickLabel("Pick a time"); err != nil {
		t.Fatal(err)
	}
	if err := session.Keys("down"); err != nil {
		t.Fatal(err)
	}
	if value != at(1, 0, 0) {
		t.Fatalf("value = %+v after the first Down, want 01:00", value)
	}
}

// A time the page sets between two steps is brought to the one below, and seconds
// are dropped when there is no column for them.
func TestTimesAreKeptOnTheSteps(t *testing.T) {
	value := at(9, 33, 41)
	open(t, timepicker.New().MinuteStep(5), &value)
	if value != at(9, 30, 0) {
		t.Fatalf("value = %+v, want 09:30 with no seconds column", value)
	}
	value = at(9, 33, 41)
	open(t, timepicker.New().Seconds().SecondStep(10), &value)
	if value != at(9, 33, 40) {
		t.Fatalf("value = %+v, want 09:33:40 on steps of ten seconds", value)
	}
	value = at(31, 70, 0)
	open(t, timepicker.New(), &value)
	if value != at(23, 59, 0) {
		t.Fatalf("value = %+v, want an out of range time brought to 23:59", value)
	}
}

func TestWarmPaintAllocatesNothing(t *testing.T) {
	value := at(9, 30, 0)
	p := timepicker.New().Seconds()
	view := func(c ui.C) { p.View(c, "Start", &value).Width(220) }
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: view, Theme: &theme})
	canvas := &gfx.Canvas{}
	canvas.SetReduceMotion(true)
	draw := func() {
		canvas.BeginFrame()
		canvas.Reset()
		page.Draw(canvas, gfx.Rect{Width: 420, Height: 420}, view)
		canvas.EndFrame()
	}
	draw()
	if allocations := testing.AllocsPerRun(100, draw); allocations != 0 {
		t.Fatalf("warm paint allocated %v times, want 0", allocations)
	}
}

// Open columns repaint without allocating too.
func TestOpenColumnsAllocateNothing(t *testing.T) {
	value := at(9, 30, 0)
	session := open(t, timepicker.New().Seconds(), &value)
	if err := session.ClickLabel("09:30:00"); err != nil {
		t.Fatal(err)
	}
	session.Frame()
	if allocations := testing.AllocsPerRun(50, func() { session.Frame() }); allocations != 0 {
		t.Fatalf("open columns allocated %v times a frame, want 0", allocations)
	}
}

func BenchmarkViewPaint(b *testing.B) {
	value := at(9, 30, 0)
	p := timepicker.New().Seconds()
	view := func(c ui.C) { p.View(c, "Start", &value).Width(220) }
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: view, Theme: &theme})
	canvas := &gfx.Canvas{}
	canvas.SetReduceMotion(true)
	b.ReportAllocs()
	for range b.N {
		canvas.BeginFrame()
		canvas.Reset()
		page.Draw(canvas, gfx.Rect{Width: 420, Height: 420}, view)
		canvas.EndFrame()
	}
}
