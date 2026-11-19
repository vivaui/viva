// Package calendar is the opt-in page control for the month grid of package
// components. It wraps components.BeginCalendar in a ui.Control, so a page
// declares a calendar without calling the component itself:
//
//	month := &calendar.Calendar{Options: calendar.Options{
//		MonthTitle: title, Weekdays: weekdays, Days: days,
//		PreviousID: previous, NextID: next,
//		PreviousLabel: "Previous month", NextLabel: "Next month",
//	}}
//	...
//	calendar.View(c, "Calendar", month).FillWidth().MaxWidth(360)
//	if month.Clicked(previous) {
//		m.month = -1
//	} else if month.Clicked(next) {
//		m.month = 1
//	}
//
// The caller owns the month snapshot, the day identities and the navigation
// identities; the wrapper adds no date model and derives no dates. It paints
// through the component with exactly the identities the caller supplied, so a
// day or navigation click is read with Clicked after View, as for any canvas
// component. Pointer, keys and the wheel aimed at the control itself reach the
// optional callbacks.
//
// It is a separate package so an application that does not import it pays
// nothing for it. Keep a Calendar in the model, beside the data it presents:
// its handlers and draw function are made once, so a warm paint allocates
// nothing. Do not copy a Calendar after it has been declared.
package calendar

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// Options is the caller-owned month snapshot the view paints. It is
// components.CalendarOptions, named here so a page that declares a calendar
// reads through this package alone.
type Options = components.CalendarOptions

// Day is one caller-owned cell of the month grid.
type Day = components.CalendarDay

// Calendar is one calendar's month snapshot and page control. Fill Options from
// the caller's typed month data every paint; the callbacks stay as they are and
// are read when an event arrives.
type Calendar struct {
	// Options is the complete application-owned month snapshot painted next.
	// The wrapper does not modify it.
	Options components.CalendarOptions

	// Pointer receives every pointer event aimed at the control itself.
	Pointer func(gfx.PointerEvent) gfx.EventResult
	// Key receives the keys pressed while the control has the focus.
	Key func(gfx.KeyEvent) gfx.EventResult
	// Scroll receives the wheel over the control. A nil callback leaves it to
	// the page.
	Scroll func(gfx.ScrollEvent) gfx.EventResult

	handlers ui.Handlers
	drawFn   func(*ui.DrawContext)
	prepared bool
	clicked  gfx.ID
	hadClick bool
}

// View declares the calendar at c. label names the control for assistive
// technology. Give it a size; it is a ui.Control painted through the component.
//
// Declare the view on its own, not inside a Memo.
func View(c ui.C, label string, calendar *Calendar) ui.Box {
	if calendar == nil {
		panic("calendar: View needs its Calendar")
	}
	calendar.prepare()
	return ui.Control(c, label, &calendar.handlers).Draw(calendar.drawFn)
}

// Clicked reports whether the paint just made was an event pass delivering a
// click on id, one of the caller's own day or navigation identities. It is
// false in ordinary paints. Ask it after View, where the component's clicks
// have always been read.
func (c *Calendar) Clicked(id gfx.ID) bool {
	return c.hadClick && !id.IsZero() && c.clicked == id
}

func (c *Calendar) prepare() {
	if c.prepared {
		return
	}
	c.prepared = true
	c.handlers.Pointer = c.pointer
	c.handlers.Key = c.key
	c.handlers.Scroll = c.scroll
	c.drawFn = c.draw
}

func (c *Calendar) pointer(event gfx.PointerEvent) gfx.EventResult {
	if c.Pointer == nil {
		return gfx.EventResult{}
	}
	return c.Pointer(event)
}

func (c *Calendar) key(event gfx.KeyEvent) gfx.EventResult {
	if c.Key == nil {
		return gfx.EventResult{}
	}
	return c.Key(event)
}

func (c *Calendar) scroll(event gfx.ScrollEvent) gfx.EventResult {
	if c.Scroll == nil {
		return gfx.EventResult{}
	}
	return c.Scroll(event)
}

func (c *Calendar) draw(d *ui.DrawContext) {
	c.clicked, c.hadClick = d.Canvas.ClickTarget()
	components.BeginCalendar(d.Canvas, *d.Theme, d.Rect, c.Options).Paint()
}
