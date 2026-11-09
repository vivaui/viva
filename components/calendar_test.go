package components

import (
	"slices"
	"testing"

	"github.com/vivaui/viva/gfx"
)

var calendarTestWeekdays = [7]string{"Su", "Mo", "Tu", "We", "Th", "Fr", "Sa"}

var calendarTestDays = [...]CalendarDay{
	{ID: gfx.Item(101), Label: "29", AccessibilityLabel: "September 29", Outside: true},
	{ID: gfx.Item(102), Label: "30", AccessibilityLabel: "September 30", Outside: true, Disabled: true},
	{ID: gfx.Item(103), Label: "1", AccessibilityLabel: "October 1", Today: true},
	{ID: gfx.Item(104), Label: "2", AccessibilityLabel: "October 2", Selected: true},
	{ID: gfx.Item(105), Label: "3", AccessibilityLabel: "October 3", RangeStart: true},
	{ID: gfx.Item(106), Label: "4", AccessibilityLabel: "October 4", RangeMiddle: true},
	{ID: gfx.Item(107), Label: "5", AccessibilityLabel: "October 5", RangeEnd: true},
	{ID: gfx.Item(108), Label: "6", AccessibilityLabel: "October 6", Value: "2026-10-06", Hint: "Available"},
}

func calendarTestOptions() CalendarOptions {
	return CalendarOptions{
		MonthTitle: "October 2026", Weekdays: calendarTestWeekdays, Days: calendarTestDays[:],
		PreviousID: gfx.Item(91), NextID: gfx.Item(92), PreviousLabel: "Previous month", NextLabel: "Next month",
		PreviousDisabled: true,
	}
}

func TestCalendarUsesSevenColumnGeometryAndCallerOwnedSnapshot(t *testing.T) {
	theme := gfx.LightTheme()
	var canvas gfx.Canvas
	calendar := BeginCalendar(&canvas, theme, gfx.Rect{X: 10, Y: 20, Width: 240}, calendarTestOptions())
	calendar.Paint()

	if got, want := calendar.Bounds(), (gfx.Rect{X: 10, Y: 20, Width: 240, Height: 160}); got != want {
		t.Fatalf("calendar bounds = %+v, want %+v", got, want)
	}
	if got, want := calendar.DayBounds(0), (gfx.Rect{X: 18, Y: 108, Width: 28, Height: 28}); got != want {
		t.Fatalf("first day bounds = %+v, want %+v", got, want)
	}
	if got, want := calendar.DayBounds(6), (gfx.Rect{X: 186, Y: 108, Width: 28, Height: 28}); got != want {
		t.Fatalf("seventh day bounds = %+v, want %+v", got, want)
	}
	if got, want := calendar.DayBounds(7), (gfx.Rect{X: 18, Y: 144, Width: 28, Height: 28}); got != want {
		t.Fatalf("eighth day bounds = %+v, want %+v", got, want)
	}
	// The navigation arrows are icons, not text: title, 7 weekdays, 8 days.
	if len(canvas.Texts()) != 16 {
		t.Fatalf("calendar texts = %d, want 16", len(canvas.Texts()))
	}
	if canvas.Texts()[0].Value != "October 2026" || canvas.Texts()[1].Value != "Su" || canvas.Texts()[8].Value != "29" {
		t.Fatalf("calendar text order = %+v", canvas.Texts())
	}
	if len(nonBlockHits(canvas.Hits())) != 10 {
		t.Fatalf("calendar hits = %d, want 10", len(nonBlockHits(canvas.Hits())))
	}
	if previous := nonBlockHits(canvas.Hits())[0]; previous.ID != gfx.Item(91) || previous.Role != gfx.RoleButton || !previous.Disabled || previous.Focusable {
		t.Fatalf("previous hit = %+v", previous)
	}
	if next := nonBlockHits(canvas.Hits())[1]; next.ID != gfx.Item(92) || next.Role != gfx.RoleButton || next.Disabled || !next.Focusable {
		t.Fatalf("next hit = %+v", next)
	}
	if disabledDay := nonBlockHits(canvas.Hits())[3]; disabledDay.Label != "September 30" || !disabledDay.Disabled || disabledDay.Focusable {
		t.Fatalf("disabled day hit = %+v", disabledDay)
	}
	if selected := nonBlockHits(canvas.Hits())[5]; !selected.Selected || selected.Role != gfx.RoleGridCell {
		t.Fatalf("selected day hit = %+v", selected)
	}
	if rangeMiddle := nonBlockHits(canvas.Hits())[7]; !rangeMiddle.Selected {
		t.Fatalf("range-middle hit = %+v", rangeMiddle)
	}
	if value := nonBlockHits(canvas.Hits())[9]; value.Value != "2026-10-06" || value.Hint != "Available" {
		t.Fatalf("day value semantics = %+v", value)
	}
}

func TestCalendarPaintsOfficialSingleRangeTodayAndOutsideStates(t *testing.T) {
	theme := gfx.LightTheme()
	var canvas gfx.Canvas
	BeginCalendar(&canvas, theme, gfx.Rect{Width: 240}, calendarTestOptions()).Paint()

	texts := canvas.Texts()
	if texts[8].Style.Color != theme.MutedForeground {
		t.Fatalf("outside foreground = %+v, want %+v", texts[8].Style.Color, theme.MutedForeground)
	}
	disabledForeground := theme.MutedForeground
	disabledForeground.Alpha *= 0.50
	if texts[9].Style.Color != disabledForeground {
		t.Fatalf("disabled foreground = %+v, want %+v", texts[9].Style.Color, disabledForeground)
	}
	if texts[10].Style.Color != theme.Foreground {
		t.Fatalf("today foreground = %+v, want %+v", texts[10].Style.Color, theme.Foreground)
	}
	if texts[11].Style.Color != theme.PrimaryForeground || texts[12].Style.Color != theme.PrimaryForeground || texts[14].Style.Color != theme.PrimaryForeground {
		t.Fatalf("selected endpoint foregrounds = %+v %+v %+v", texts[11].Style.Color, texts[12].Style.Color, texts[14].Style.Color)
	}
	if texts[13].Style.Color != theme.Foreground {
		t.Fatalf("range middle foreground = %+v, want %+v", texts[13].Style.Color, theme.Foreground)
	}

	dayRectangles := canvas.Rectangles()[2:]
	if dayRectangles[2].Color != theme.Muted || dayRectangles[2].Radius == 0 {
		t.Fatalf("today surface = %+v", dayRectangles[2])
	}
	if dayRectangles[3].Color != theme.Primary || dayRectangles[3].Radius == 0 {
		t.Fatalf("single selection surface = %+v", dayRectangles[3])
	}
	if dayRectangles[4].Bounds.Width != 14 || dayRectangles[4].Color != theme.Muted || dayRectangles[5].Color != theme.Primary {
		t.Fatalf("range start surfaces = %+v %+v", dayRectangles[4], dayRectangles[5])
	}
	if dayRectangles[6].Color != theme.Muted || dayRectangles[6].Radius != 0 {
		t.Fatalf("range middle surface = %+v", dayRectangles[6])
	}
	if dayRectangles[7].Bounds.Width != 14 || dayRectangles[7].Color != theme.Muted || dayRectangles[8].Color != theme.Primary {
		t.Fatalf("range end surfaces = %+v %+v", dayRectangles[7], dayRectangles[8])
	}
}

func TestCalendarFocusedDayUsesRingAndPreservesTypedSemantics(t *testing.T) {
	theme := gfx.DarkTheme()
	var canvas gfx.Canvas
	canvas.SetInteraction(gfx.ID{}, gfx.ID{}, gfx.Item(106), true)
	calendar := BeginCalendar(&canvas, theme, gfx.Rect{Width: 240}, calendarTestOptions())
	calendar.Paint()

	focusedBounds := calendar.DayBounds(5)
	wantRingBounds := gfx.Rect{X: focusedBounds.X - 3, Y: focusedBounds.Y - 3, Width: 34, Height: 34}
	foundRing := false
	for _, rectangle := range canvas.Rectangles() {
		if rectangle.Bounds == wantRingBounds {
			wantColor := theme.Ring
			wantColor.Alpha *= 0.50
			if rectangle.Color != wantColor {
				t.Fatalf("focus ring color = %+v, want %+v", rectangle.Color, wantColor)
			}
			foundRing = true
			break
		}
	}
	if !foundRing {
		t.Fatalf("focused day ring with bounds %+v not found", wantRingBounds)
	}
	focusedHit := nonBlockHits(canvas.Hits())[7]
	if focusedHit.ID != gfx.Item(106) || focusedHit.Role != gfx.RoleGridCell || !focusedHit.Selected || !focusedHit.Focusable {
		t.Fatalf("focused range day semantics = %+v", focusedHit)
	}
}

// A calendar sits on whatever surface holds it. Painting the page background
// itself would make a darker rectangle inside a popover or card in dark mode.
func TestCalendarPaintsNoSurfaceOfItsOwn(t *testing.T) {
	for _, theme := range []gfx.Theme{gfx.LightTheme(), gfx.DarkTheme()} {
		var canvas gfx.Canvas
		calendar := BeginCalendar(&canvas, theme, gfx.Rect{Width: 240}, calendarTestOptions())
		calendar.Paint()
		for _, rectangle := range canvas.Rectangles() {
			if rectangle.Bounds == calendar.Bounds() && rectangle.Color.Alpha != 0 {
				t.Fatalf("calendar filled its own bounds with %+v", rectangle.Color)
			}
		}
	}
}

func TestCalendarCommandForKeyMapsNavigationIntents(t *testing.T) {
	for _, test := range []struct {
		key       gfx.Key
		modifiers gfx.Modifiers
		want      CalendarCommand
		handled   bool
	}{
		{gfx.KeyLeft, 0, CalendarCommand{Kind: CalendarCommandMoveDays, Amount: -1}, true},
		{gfx.KeyRight, 0, CalendarCommand{Kind: CalendarCommandMoveDays, Amount: 1}, true},
		{gfx.KeyUp, 0, CalendarCommand{Kind: CalendarCommandMoveWeeks, Amount: -1}, true},
		{gfx.KeyDown, 0, CalendarCommand{Kind: CalendarCommandMoveWeeks, Amount: 1}, true},
		{gfx.KeyPageUp, 0, CalendarCommand{Kind: CalendarCommandMoveMonths, Amount: -1}, true},
		{gfx.KeyPageDown, 0, CalendarCommand{Kind: CalendarCommandMoveMonths, Amount: 1}, true},
		{gfx.KeyPageUp, gfx.ModifierShift, CalendarCommand{Kind: CalendarCommandMoveYears, Amount: -1}, true},
		{gfx.KeyPageDown, gfx.ModifierShift, CalendarCommand{Kind: CalendarCommandMoveYears, Amount: 1}, true},
		{gfx.KeyHome, 0, CalendarCommand{Kind: CalendarCommandWeekStart}, true},
		{gfx.KeyEnd, 0, CalendarCommand{Kind: CalendarCommandWeekEnd}, true},
		{gfx.KeyEnter, 0, CalendarCommand{}, false},
	} {
		got, handled := CalendarCommandForKey(test.key, test.modifiers)
		if got != test.want || handled != test.handled {
			t.Errorf("key %v modifiers %v = (%+v, %v), want (%+v, %v)", test.key, test.modifiers, got, handled, test.want, test.handled)
		}
	}
}

func TestCalendarRepeatedPaintIsStableAndAllocationFree(t *testing.T) {
	theme := gfx.DarkTheme()
	options := calendarTestOptions()
	var canvas gfx.Canvas
	paint := func() {
		canvas.Reset()
		BeginCalendar(&canvas, theme, gfx.Rect{Width: 240}, options).Paint()
	}
	paint()
	rectangles := slices.Clone(canvas.Rectangles())
	texts := slices.Clone(canvas.Texts())
	hits := slices.Clone(nonBlockHits(canvas.Hits()))
	paint()
	if !slices.Equal(rectangles, canvas.Rectangles()) || !slices.Equal(texts, canvas.Texts()) || !slices.Equal(hits, nonBlockHits(canvas.Hits())) {
		t.Fatal("identical calendar snapshot produced different command streams")
	}
	if allocations := testing.AllocsPerRun(100, paint); allocations != 0 {
		t.Fatalf("calendar paint allocations = %v", allocations)
	}
}

// Calendar focus movement skips disabled days, clamps at the snapshot edge, and
// reverses horizontal arrows in RTL.
func TestCalendarFocusForKeySkipsAndClamps(t *testing.T) {
	days := make([]CalendarDay, 14)
	for index := range days {
		days[index] = CalendarDay{ID: gfx.Item(index + 1), Label: "day"}
	}
	days[1].Disabled = true
	if id, ok := CalendarFocusForKey(gfx.KeyRight, LTR, days, gfx.Item(1)); !ok || id != gfx.Item(3) {
		t.Fatalf("Right over disabled = (%v, %v), want the next enabled day", id, ok)
	}
	if id, ok := CalendarFocusForKey(gfx.KeyLeft, LTR, days, gfx.Item(1)); !ok || id != gfx.Item(1) {
		t.Fatalf("Left at the start = (%v, %v)", id, ok)
	}
	if id, ok := CalendarFocusForKey(gfx.KeyRight, RTL, days, gfx.Item(3)); !ok || id != gfx.Item(1) {
		t.Fatalf("Right in RTL = (%v, %v), want the previous enabled day", id, ok)
	}
	if id, ok := CalendarFocusForKey(gfx.KeyEnd, LTR, days, gfx.Item(1)); !ok || id != gfx.Item(7) {
		t.Fatalf("End = (%v, %v), want the week end", id, ok)
	}
	if id, ok := CalendarFocusForKey(gfx.KeyDown, LTR, days, gfx.Item(7)); !ok || id != gfx.Item(14) {
		t.Fatalf("Down into the last week = (%v, %v)", id, ok)
	}
	if _, ok := CalendarFocusForKey(gfx.KeyPageDown, LTR, days, gfx.Item(1)); ok {
		t.Fatal("PageDown belongs to month navigation")
	}
}
