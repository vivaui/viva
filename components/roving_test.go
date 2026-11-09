package components

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// A roving group has exactly one Tab stop: the selection (or first enabled
// item), and an explicitly focused item takes it over. The arrow keys can still
// focus every other item because they remain focusable.
func TestRovingGroupsKeepOneTabStop(t *testing.T) {
	check := func(t *testing.T, canvas *gfx.Canvas, want gfx.ID) {
		t.Helper()
		tabs := 0
		for _, hit := range canvas.Hits() {
			if hit.Focusable && !hit.SkipTab {
				tabs++
				if hit.ID != want {
					t.Fatalf("Tab stop is %v, want %v (hits %+v)", hit.ID, want, canvas.Hits())
				}
			}
		}
		if tabs != 1 {
			t.Fatalf("%d Tab stops, want one", tabs)
		}
	}

	t.Run("radio", func(t *testing.T) {
		var canvas gfx.Canvas
		BeginRadioGroup(&canvas, gfx.LightTheme(), gfx.Rect{Width: 200, Height: 100}, []RadioOption{
			{ID: gfx.Item(1), Label: "One"}, {ID: gfx.Item(2), Label: "Two"}, {ID: gfx.Item(3), Label: "Three"},
		}, gfx.Item(2)).Paint()
		check(t, &canvas, gfx.Item(2))
	})

	t.Run("tabs", func(t *testing.T) {
		var canvas gfx.Canvas
		BeginTabs(&canvas, gfx.LightTheme(), gfx.Rect{Width: 300, Height: 32}, []TabOption{
			{ID: gfx.Item(1), Label: "One"}, {ID: gfx.Item(2), Label: "Two"}, {ID: gfx.Item(3), Label: "Three"},
		}, gfx.Item(2)).Paint()
		check(t, &canvas, gfx.Item(2))
	})

	t.Run("toggle group", func(t *testing.T) {
		var canvas gfx.Canvas
		BeginToggleGroup(&canvas, gfx.LightTheme(), gfx.Rect{Width: 300, Height: 32}, []ToggleGroupOption{
			{ID: gfx.Item(1), Label: "One"}, {ID: gfx.Item(2), Label: "Two", Active: true},
		}).Paint()
		check(t, &canvas, gfx.Item(2))
	})

	t.Run("segmented", func(t *testing.T) {
		var canvas gfx.Canvas
		BeginSegmentedControl(&canvas, gfx.LightTheme(), gfx.Rect{Width: 300, Height: 32}, []SegmentOption{
			{ID: gfx.Item(1), Label: "One"}, {ID: gfx.Item(2), Label: "Two"}, {ID: gfx.Item(3), Label: "Three"},
		}, gfx.Item(3)).Paint()
		check(t, &canvas, gfx.Item(3))
	})

	t.Run("calendar", func(t *testing.T) {
		var canvas gfx.Canvas
		days := make([]CalendarDay, 7)
		for index := range days {
			days[index] = CalendarDay{ID: gfx.Item(index + 1), Label: "1", Selected: index == 4}
		}
		BeginCalendar(&canvas, gfx.LightTheme(), gfx.Rect{Width: 300, Height: 240}, CalendarOptions{MonthTitle: "May", Days: days}).Paint()
		check(t, &canvas, gfx.Item(5))
	})

	t.Run("focused item takes the stop", func(t *testing.T) {
		var canvas gfx.Canvas
		canvas.SetInteraction(gfx.ID{}, gfx.ID{}, gfx.Item(3), true)
		BeginRadioGroup(&canvas, gfx.LightTheme(), gfx.Rect{Width: 200, Height: 100}, []RadioOption{
			{ID: gfx.Item(1), Label: "One"}, {ID: gfx.Item(2), Label: "Two"}, {ID: gfx.Item(3), Label: "Three"},
		}, gfx.Item(2)).Paint()
		check(t, &canvas, gfx.Item(3))
	})
}
