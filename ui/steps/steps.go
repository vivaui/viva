// Package steps is the opt-in page control for a step bar: the stages of a
// task laid out in order, with the finished ones behind the current one and the
// ones to come after it. A page keeps the current step in its model and
// declares the stages with a Spec:
//
//	steps.View(c, "Checkout", &m.step, steps.Spec{Steps: m.stages, Clickable: true})
//	// m.stages is a []steps.Step kept in the model, so a warm paint allocates
//	// nothing: {Title: "Account", Description: "Create a login"}, ...
//
// current is the zero-based index of the step being worked on: the steps before
// it are finished and the ones after it wait. Spec.Status says how the current
// step stands, Process while it is under way or Error when it failed. With
// Clickable a click, or Enter or Space on a focused step, makes it the current
// one; without it the bar only shows where the task is.
//
// The step bar is horizontal by default and vertical for Direction: Vertical;
// Type: Dot replaces the numbered markers with dots, and Size: Small makes the
// markers and the text smaller. It composes the page's own primitives, so it
// takes the page's theme and density.
//
// It is a separate package so an application that does not import it pays
// nothing for it, and it depends only on the public gfx and ui packages.
package steps

import (
	"strconv"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// Status is how a step stands.
type Status uint8

const (
	// Process is the step under way: the zero value, and what the current step
	// shows unless Spec.Status says otherwise.
	Process Status = iota
	// Wait is a step not reached yet.
	Wait
	// Finish is a step done.
	Finish
	// Error is a step that failed.
	Error
)

// Type is the look of a step's marker.
type Type uint8

const (
	// Numbered marks each step with a circle holding its number, a check once
	// it is finished and a cross when it failed. It is the zero value.
	Numbered Type = iota
	// Dot marks each step with a small dot.
	Dot
)

// Direction is the way the steps run.
type Direction uint8

const (
	// Horizontal lays the steps out in a row, joined by lines. It is the zero
	// value.
	Horizontal Direction = iota
	// Vertical stacks the steps in a column.
	Vertical
)

// Size is how large the markers and the text are.
type Size uint8

const (
	// Default is the zero value.
	Default Size = iota
	// Small is the compact bar.
	Small
)

// Step is one stage of the task.
type Step struct {
	// Title names the stage.
	Title string
	// Description is an optional line under the title.
	Description string
	// Icon replaces the number of a numbered marker while the step is not
	// finished or failed. A zero icon keeps the number.
	Icon gfx.Icon
}

// Spec declares the step bar. Its zero value is a horizontal, default sized bar
// of numbered markers that only shows where the task is.
type Spec struct {
	// Steps are the stages, in order. Keep the slice in the model: a slice made in
	// each paint allocates in each paint.
	Steps []Step
	// Status is how the current step stands; Process by default.
	Status Status
	// Type is the look of the markers.
	Type Type
	// Direction is the way the steps run.
	Direction Direction
	// Size is how large the markers and the text are.
	Size Size
	// Clickable lets a click on a step make it the current one.
	Clickable bool
}

var (
	checkIcon = gfx.Icon{ViewBox: 24, Stroke: 2.5, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{
		gfx.MoveTo(20, 6), gfx.LineTo(9, 17), gfx.LineTo(4, 12),
	}}
	crossIcon = gfx.Icon{ViewBox: 24, Stroke: 2.5, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{
		gfx.MoveTo(18, 6), gfx.LineTo(6, 18), gfx.MoveTo(6, 6), gfx.LineTo(18, 18),
	}}
)

// numbers are the words of the markers of the first steps, made once.
var numbers [20]string

func init() {
	for i := range numbers {
		numbers[i] = strconv.Itoa(i + 1)
	}
}

func number(i int) string {
	if i < len(numbers) {
		return numbers[i]
	}
	return strconv.Itoa(i + 1)
}

// View declares the step bar at c. label names it for assistive technology;
// current is the index of the current step, and a click on a Clickable bar sets
// it. The bar is as tall as its steps and, horizontally, as wide as the room it
// is given: give it FillWidth. Declare the view on its own, not inside a Memo.
func View(c ui.C, label string, current *int, spec Spec) ui.Box {
	if c == nil {
		panic("steps: View needs a page")
	}
	if current == nil {
		panic("steps: View needs the current step")
	}
	var root ui.Box
	if spec.Direction == Vertical {
		root = ui.Column(c)
	} else {
		root = ui.Row(c).Items(ui.Start)
	}
	if spec.Clickable {
		// The steps are the control's hits, and a hit over them would overlap theirs.
		root = root.Label(label)
	} else {
		root = root.HitWith(ui.ID(c), label, gfx.HitOptions{Role: gfx.RoleGroup, SkipFocus: true})
	}
	look := newLook(c, spec)
	last := len(spec.Steps) - 1
	root.Children(func() {
		for i := range spec.Steps {
			status := statusOf(i, *current, spec.Status)
			var step ui.Box
			if spec.Direction == Vertical {
				step = ui.Row(c).Items(ui.Stretch).Gap(look.gap)
			} else {
				step = ui.Row(c).Items(ui.Start).Gap(look.gap).Shrink(0)
			}
			if spec.Clickable {
				step.HitWith(ui.ID(c), spec.Steps[i].Title, gfx.HitOptions{Role: gfx.RoleButton})
			}
			step.Children(func() {
				if spec.Direction == Vertical {
					look.rail(c, spec.Steps[i], i, status, i < last, i < *current)
					look.text(c, spec.Steps[i], status, i < last)
				} else {
					look.marker(c, spec.Steps[i], i, status)
					look.text(c, spec.Steps[i], status, false)
				}
			})
			if spec.Clickable && step.Clicked() {
				*current = i
			}
			if spec.Direction != Vertical && i < last {
				look.line(c, i < *current)
			}
		}
	})
	return root
}

// statusOf is how step i stands with the current one at current.
func statusOf(i, current int, now Status) Status {
	switch {
	case i < current:
		return Finish
	case i == current:
		return now
	}
	return Wait
}

// look is what a bar paints with: the page's theme and the spec's size and type.
type look struct {
	theme      *gfx.Theme
	markerSize float32
	dot        float32
	gap        float32
	title      float32
	detail     float32
	numbered   bool
	vertical   bool
}

func newLook(c ui.C, spec Spec) look {
	theme := c.Theme()
	metrics := theme.Metrics
	l := look{
		theme: theme, markerSize: 24, dot: 8, gap: metrics.GapItem,
		title: metrics.FontSizeM, detail: metrics.FontSizeS,
		numbered: spec.Type == Numbered, vertical: spec.Direction == Vertical,
	}
	if spec.Size == Small {
		l.markerSize, l.dot, l.title = 20, 6, metrics.FontSizeS
	}
	return l
}

// tone is the colors of a marker in a status: its fill, and what is drawn on it.
func (l look) tone(status Status) (fill, ink gfx.Color) {
	switch status {
	case Finish:
		return l.theme.Secondary, l.theme.Primary
	case Process:
		return l.theme.Primary, l.theme.PrimaryForeground
	case Error:
		return l.theme.Danger, l.theme.DangerForeground
	}
	return l.theme.Muted, l.theme.MutedForeground
}

// marker declares the circle or the dot of a step.
func (l look) marker(c ui.C, step Step, i int, status Status) ui.Box {
	fill, ink := l.tone(status)
	if !l.numbered {
		// A dot is centered on the first line of the title, where a number sits.
		slot := ui.Row(c).Size(l.dot, l.markerSize).Shrink(0).Justify(ui.Center).Items(ui.Center)
		slot.Children(func() {
			ui.Spacer(c).Size(l.dot, l.dot).Shrink(0).Radius(l.dot / 2).Background(dotFill(l.theme, status))
		})
		return slot
	}
	circle := ui.Row(c).Size(l.markerSize, l.markerSize).Shrink(0).Radius(l.markerSize / 2).Background(fill).Center()
	circle.Children(func() {
		glyph := l.markerSize * 0.55
		switch {
		case status == Finish:
			ui.Icon(c, checkIcon).Size(glyph, glyph).Color(ink)
		case status == Error:
			ui.Icon(c, crossIcon).Size(glyph, glyph).Color(ink)
		case step.Icon.ViewBox > 0:
			ui.Icon(c, step.Icon).Size(glyph, glyph).Color(ink)
		default:
			ui.Text(c, number(i)).FontSize(l.detail).Weight(gfx.WeightMedium).Color(ink)
		}
	})
	return circle
}

// dotFill is the color of a dot in a status.
func dotFill(theme *gfx.Theme, status Status) gfx.Color {
	switch status {
	case Finish, Process:
		return theme.Primary
	case Error:
		return theme.Danger
	}
	return theme.Border
}

// text declares the title and the description of a step.
func (l look) text(c ui.C, step Step, status Status, padded bool) {
	column := ui.Column(c).Gap(2)
	if padded {
		column = column.PadBottom(l.gap * 2)
	}
	column.Children(func() {
		title := ui.Text(c, step.Title).FontSize(l.title).LineHeight(l.markerSize)
		switch status {
		case Wait:
			title.Muted()
		case Error:
			title.Color(l.theme.Danger).Weight(gfx.WeightMedium)
		case Process:
			title.Weight(gfx.WeightMedium)
		}
		if step.Description != "" {
			ui.Text(c, step.Description).FontSize(l.detail).Muted()
		}
	})
}

// line declares the line that joins two steps of a horizontal bar; done is
// whether the step it leaves is behind the current one.
func (l look) line(c ui.C, done bool) {
	color := l.theme.Border
	if done {
		color = l.theme.Primary
	}
	holder := ui.Row(c).Grow(1).Height(l.markerSize).MinWidth(16).Items(ui.Center)
	holder.MarginSides(0, l.gap, 0, l.gap)
	holder.Children(func() {
		ui.Spacer(c).FillWidth().Height(1).Background(color)
	})
}

// rail declares the marker of a step of a vertical bar and the line that runs
// down from it to the next step.
func (l look) rail(c ui.C, step Step, i int, status Status, joined, done bool) {
	rail := ui.Column(c).Items(ui.Center).Shrink(0).Width(l.markerSize)
	rail.Children(func() {
		l.marker(c, step, i, status)
		if joined {
			color := l.theme.Border
			if done {
				color = l.theme.Primary
			}
			ui.Spacer(c).Width(1).Grow(1).MinHeight(8).Margin(4).Background(color)
		}
	})
}
