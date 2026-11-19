// Package marker is the opt-in page control for a lightweight metadata
// marker: an icon and a label, a centered separator, a bottom-border row, or a
// progress row. It wraps components.Marker and components.MarkerProgress in a
// ui.Control, so a page declares the visible content with chained options:
//
//	marker.View(c, "Updated 2 minutes ago", marker.New().
//		Icon(m.clockIcon).Label("Updated 2 minutes ago"))
//
//	marker.View(c, "October 2026", marker.New().
//		Separator().Label("October 2026"))
//
//	phase = (phase + 1) % 8
//	marker.View(c, "Thinking", marker.New().
//		Progress(phase).Label("Thinking"))
//
// The caller owns the progress phase and the frames that advance it, as for
// components.MarkerProgress: an unchanged marker schedules no work.
//
// It is a separate package so an application that does not import it pays
// nothing for it, and it depends only on the public components, gfx and ui
// packages. A State belongs to one place on one page: keep one in the model
// per marker, make its handlers and draw function once, and Reset its
// declaration in each paint. Do not copy one after it has been painted.
package marker

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// Variant is the component's marker form: plain, centered separator or
// bottom-border.
type Variant = components.MarkerVariant

// Phase is the spinner frame of a progress marker.
type Phase = components.SpinnerPhase

// State is one marker's configuration and its page control. Configure it with
// the chainable methods after Reset and before View.
type State struct {
	label    string
	icon     gfx.Icon
	variant  Variant
	progress bool
	phase    Phase

	handlers ui.Handlers
	drawFn   func(*ui.DrawContext)
	prepared bool
}

// New returns an empty plain marker.
func New() *State { return new(State) }

// Reset clears the label, icon, variant and progress state.
func (m *State) Reset() *State {
	m.label = ""
	m.icon = gfx.Icon{}
	m.variant = components.MarkerDefault
	m.progress = false
	m.phase = 0
	return m
}

// Label is the marker's muted content.
func (m *State) Label(label string) *State { m.label = label; return m }

// Icon paints a 16 pixel leading icon. A zero icon paints none. Progress takes
// its place when both are set.
func (m *State) Icon(icon gfx.Icon) *State { m.icon = icon; return m }

// Separator centers the label between two rules, as a date separator does. It
// replaces Border.
func (m *State) Separator() *State { m.variant = components.MarkerSeparator; return m }

// Border draws a rule below the row, as a section boundary does. It replaces
// Separator.
func (m *State) Border() *State { m.variant = components.MarkerBorder; return m }

// Plain selects the default form with no separator or border.
func (m *State) Plain() *State { m.variant = components.MarkerDefault; return m }

// Progress replaces the icon with the spinner frame, as work in progress does.
// Advancing the phase and scheduling the next frame are the caller's, as for
// components.MarkerProgress.
func (m *State) Progress(phase Phase) *State {
	m.progress, m.phase = true, phase
	return m
}

// View declares the marker at c. label names the row for assistive technology.
// The box is full width and as tall as the component's line, plus its rule for
// the border variant; a Width or Height after it overrides that.
//
// Declare the view on its own, not inside a Memo.
func View(c ui.C, label string, marker *State) ui.Box {
	if marker == nil {
		panic("marker: View needs its State")
	}
	marker.prepare()
	return ui.Control(c, label, &marker.handlers).
		FillWidth().Height(marker.height(*c.Theme())).
		Draw(marker.drawFn)
}

// View declares the marker at c, for a page that keeps the State in its
// model.
func (m *State) View(c ui.C, label string) ui.Box { return View(c, label, m) }

func (m *State) prepare() {
	if m.prepared {
		return
	}
	m.prepared = true
	m.drawFn = m.draw
}

func (m *State) draw(d *ui.DrawContext) {
	if m.progress {
		components.MarkerProgress(d.Canvas, *d.Theme, d.Rect, m.variant, m.phase, m.label)
		return
	}
	var icon *gfx.Icon
	if m.icon.ViewBox > 0 {
		icon = &m.icon
	}
	components.Marker(d.Canvas, *d.Theme, d.Rect, m.variant, icon, m.label)
}

// height is the line the component paints, plus MarkerBorder's padding and
// rule.
func (m *State) height(theme gfx.Theme) float32 {
	metrics := theme.Metrics
	line := metrics.FontSizeM + 7
	if m.variant == components.MarkerBorder {
		return line + 8*(metrics.ControlHeight/32) + 1
	}
	return line
}
