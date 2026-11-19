package calendar_test

import (
	"testing"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/calendar"
)

var (
	testWeekdays = [7]string{"Su", "Mo", "Tu", "We", "Th", "Fr", "Sa"}
	testPrevious = gfx.Item(91)
	testNext     = gfx.Item(92)
	testDays     = [...]components.CalendarDay{
		{ID: gfx.Item(101), Label: "29", AccessibilityLabel: "September 29", Outside: true},
		{ID: gfx.Item(102), Label: "30", AccessibilityLabel: "September 30", Disabled: true},
		{ID: gfx.Item(103), Label: "1", AccessibilityLabel: "October 1", Today: true},
		{ID: gfx.Item(104), Label: "2", AccessibilityLabel: "October 2", Selected: true},
		{ID: gfx.Item(105), Label: "3", AccessibilityLabel: "October 3"},
	}
)

func testOptions() calendar.Options {
	return calendar.Options{
		MonthTitle: "October 2026", Weekdays: testWeekdays, Days: testDays[:],
		PreviousID: testPrevious, NextID: testNext,
		PreviousLabel: "Previous month", NextLabel: "Next month",
	}
}

type model struct {
	view    calendar.State
	page    *ui.Page
	canvas  *gfx.Canvas
	target  gfx.ID
	keys    int
	scrolls int
}

func (m *model) wire() {
	m.view.Options = testOptions()
	m.view.Pointer = func(event gfx.PointerEvent) gfx.EventResult {
		m.target = event.Target
		return gfx.EventResult{Handled: true, Repaint: true}
	}
	m.view.Key = func(event gfx.KeyEvent) gfx.EventResult {
		m.keys++
		m.target = event.Target
		return gfx.EventResult{Handled: true}
	}
	m.view.Scroll = func(event gfx.ScrollEvent) gfx.EventResult {
		m.scrolls++
		m.target = event.Target
		return gfx.EventResult{Handled: true}
	}
}

func (m *model) viewPage(c ui.C) {
	calendar.View(c, "State", &m.view).Size(300, 260)
}

func (m *model) open(t testing.TB) {
	t.Helper()
	theme := gfx.LightTheme()
	m.page = ui.NewPage(ui.PageConfig{View: m.viewPage, Theme: &theme})
	m.canvas = new(gfx.Canvas)
	draw(m.page, m.canvas, m.viewPage)
}

func draw(page *ui.Page, canvas *gfx.Canvas, view func(ui.C)) {
	canvas.Reset()
	canvas.BeginFrame()
	page.Draw(canvas, gfx.Rect{Width: 360, Height: 320}, view)
	canvas.EndFrame()
}

func drawClick(page *ui.Page, canvas *gfx.Canvas, view func(ui.C), id gfx.ID) {
	canvas.BeginEventPass(id, gfx.ClickInfo{})
	draw(page, canvas, view)
	canvas.EndEventPass()
}

// hitLabelled returns the front-most hit with the label and a non-zero identity.
func hitLabelled(canvas *gfx.Canvas, label string) (gfx.Hit, bool) {
	hits := canvas.Hits()
	for i := len(hits) - 1; i >= 0; i-- {
		if hits[i].Label == label && !hits[i].ID.IsZero() {
			return hits[i], true
		}
	}
	return gfx.Hit{}, false
}

func hit(t *testing.T, canvas *gfx.Canvas, label string) gfx.Hit {
	t.Helper()
	got, ok := hitLabelled(canvas, label)
	if !ok {
		t.Fatalf("no hit labelled %q was painted", label)
	}
	return got
}

// The page declares the view, the component paints the caller's month snapshot
// with the caller's own identities, and the wrapper's callbacks take pointer,
// key and wheel input aimed at the control.
func TestViewPaintsCallerIdentitiesAndRoutesInput(t *testing.T) {
	m := new(model)
	m.wire()
	m.open(t)

	control := hit(t, m.canvas, "State")
	if control.Role != gfx.RoleGroup {
		t.Fatalf("control role = %v, want a group", control.Role)
	}
	if previous := hit(t, m.canvas, "Previous month"); previous.ID != testPrevious {
		t.Fatalf("previous hit = %v, want the caller's %v", previous.ID, testPrevious)
	}
	day := hit(t, m.canvas, "October 2")
	if day.ID != testDays[3].ID || day.Role != gfx.RoleGridCell || !day.Selected {
		t.Fatalf("day hit = %+v, want the caller's selected grid cell", day)
	}

	if result := m.page.Pointer(gfx.PointerEvent{
		Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, Target: control.ID,
		X: control.Bounds.X + 4, Y: control.Bounds.Y + 4,
	}); !result.Handled || !result.Repaint || m.target != control.ID {
		t.Fatalf("pointer Target = %v (result %+v), want %v", m.target, result, control.ID)
	}
	if result := m.page.Key(gfx.KeyEvent{Key: gfx.KeyRight, Target: control.ID}); !result.Handled || m.keys != 1 || m.target != control.ID {
		t.Fatalf("key Target = %v (result %+v after %d calls)", m.target, result, m.keys)
	}
	if result := m.page.Wheel(gfx.ScrollEvent{Target: control.ID, DeltaY: 1}); !result.Handled || m.scrolls != 1 || m.target != control.ID {
		t.Fatalf("wheel Target = %v (result %+v after %d calls)", m.target, result, m.scrolls)
	}
}

// A click on a day or navigation control is read with Clicked after View, and a
// different identity is not reported.
func TestViewReportsClicksForCallerIdentities(t *testing.T) {
	m := new(model)
	m.wire()
	m.open(t)

	drawClick(m.page, m.canvas, m.viewPage, testNext)
	if !m.view.Clicked(testNext) {
		t.Fatal("Clicked did not report the clicked next control")
	}
	if m.view.Clicked(testPrevious) || m.view.Clicked(testDays[3].ID) {
		t.Fatal("Clicked reported an identity that was not clicked")
	}
	// An ordinary paint is not an event pass and reports no click.
	draw(m.page, m.canvas, m.viewPage)
	if m.view.Clicked(testNext) {
		t.Fatal("Clicked reported a click in an ordinary paint")
	}
}

// A zero day identity stays unpainted, so it is not focusable or clickable,
// while the other days keep their caller identities.
func TestViewLeavesZeroIdentitiesUnpainted(t *testing.T) {
	options := testOptions()
	options.Days = options.Days[:2]
	options.Days[0].ID = gfx.ID{}
	var view calendar.State
	view.Options = options
	viewPage := func(c ui.C) { calendar.View(c, "State", &view).Size(300, 260) }
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: viewPage, Theme: &theme})
	canvas := new(gfx.Canvas)
	draw(page, canvas, viewPage)

	if _, ok := hitLabelled(canvas, "September 29"); ok {
		t.Fatal("a day with the zero identity was painted")
	}
	disabled := hit(t, canvas, "September 30")
	if disabled.ID != testDays[1].ID || !disabled.Disabled || disabled.Focusable {
		t.Fatalf("disabled day hit = %+v, want inert with its caller identity", disabled)
	}
}

// A warm repaint allocates nothing: the wrapper keeps its handlers and draw
// function, and the component keeps its own caches.
func TestWarmPaintAllocatesNothing(t *testing.T) {
	m := new(model)
	m.wire()
	m.open(t)
	if allocations := testing.AllocsPerRun(100, func() { draw(m.page, m.canvas, m.viewPage) }); allocations != 0 {
		t.Fatalf("warm paint allocated %v times, want 0", allocations)
	}
}

func BenchmarkViewPaint(b *testing.B) {
	m := new(model)
	m.wire()
	m.open(b)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		draw(m.page, m.canvas, m.viewPage)
	}
}
