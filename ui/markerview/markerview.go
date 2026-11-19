// Package markerview is the opt-in page control for a lightweight metadata
// marker: an icon and a label, a centered separator, a bottom-border row, or a
// progress row. It wraps components.Marker and components.MarkerProgress in a
// ui.Control, so a page declares the visible content with chained options:
//
//	markerview.View(c, "Updated 2 minutes ago", markerview.New().
//		Icon(m.clockIcon).Label("Updated 2 minutes ago"))
//
//	markerview.View(c, "October 2026", markerview.New().
//		Separator().Label("October 2026"))
//
//	phase = (phase + 1) % 8
//	markerview.View(c, "Thinking", markerview.New().
//		Progress(phase).Label("Thinking"))
//
// The caller owns the progress phase and the frames that advance it, as for
// components.MarkerProgress: an unchanged marker schedules no work.
//
// It is a separate package so an application that does not import it pays
// nothing for it, and it depends only on the public components, gfx and ui
// packages. A Marker belongs to one place on one page: keep one in the model
// per marker, make its handlers and draw function once, and Reset its
// declaration in each paint. Do not copy one after it has been painted.
package markerview

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

// Marker is one marker's configuration and its page control. Configure it with
// the chainable methods after Reset and before View.
type Marker struct {
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
func New() *Marker { return new(Marker) }

// Reset clears the label, icon, variant and progress state.
func (m *Marker) Reset() *Marker {
	m.label = ""
	m.icon = gfx.Icon{}
	m.variant = components.MarkerDefault
	m.progress = false
	m.phase = 0
	return m
}

// Label is the marker's muted content.
func (m *Marker) Label(label string) *Marker { m.label = label; return m }

// Icon paints a 16 pixel leading icon. A zero icon paints none. Progress takes
// its place when both are set.
func (m *Marker) Icon(icon gfx.Icon) *Marker { m.icon = icon; return m }

// Separator centers the label between two rules, as a date separator does. It
// replaces Border.
func (m *Marker) Separator() *Marker { m.variant = components.MarkerSeparator; return m }

// Border draws a rule below the row, as a section boundary does. It replaces
// Separator.
func (m *Marker) Border() *Marker { m.variant = components.MarkerBorder; return m }

// Plain selects the default form with no separator or border.
func (m *Marker) Plain() *Marker { m.variant = components.MarkerDefault; return m }

// Progress replaces the icon with the spinner frame, as work in progress does.
// Advancing the phase and scheduling the next frame are the caller's, as for
// components.MarkerProgress.
func (m *Marker) Progress(phase Phase) *Marker {
	m.progress, m.phase = true, phase
	return m
}

// View declares the marker at c. label names the row for assistive technology.
// The box is full width and as tall as the component's line, plus its rule for
// the border variant; a Width or Height after it overrides that.
//
// Declare the view on its own, not inside a Memo.
func View(c ui.C, label string, marker *Marker) ui.Box {
	if marker == nil {
		panic("markerview: View needs its Marker")
	}
	marker.prepare()
	return ui.Control(c, label, &marker.handlers).
		FillWidth().Height(marker.height(*c.Theme())).
		Draw(marker.drawFn)
}

// View declares the marker at c, for a page that keeps the Marker in its
// model.
func (m *Marker) View(c ui.C, label string) ui.Box { return View(c, label, m) }

func (m *Marker) prepare() {
	if m.prepared {
		return
	}
	m.prepared = true
	m.drawFn = m.draw
}

func (m *Marker) draw(d *ui.DrawContext) {
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
func (m *Marker) height(theme gfx.Theme) float32 {
	metrics := theme.Metrics
	line := metrics.FontSizeM + 7
	if m.variant == components.MarkerBorder {
		return line + 8*(metrics.ControlHeight/32) + 1
	}
	return line
}
