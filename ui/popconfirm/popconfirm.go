// Package popconfirm is the opt-in page control for a small confirmation next to
// what it confirms: a popover over the button that was pressed, with a question,
// an optional line of detail and the two answers. It is for a cheap, local
// question; a consequence that cannot be undone, or one the user must not click
// through, belongs in ui.AlertDialog. A page keeps a Popconfirm in its model and
// opens it from the button:
//
//	m.remove = popconfirm.New()
//	// in the page:
//	trigger := ui.Button(c, "Remove").Outline()
//	if trigger.Clicked() {
//		m.remove.Open()
//	}
//	m.remove.View(c, trigger, popconfirm.Spec{Title: "Remove build-01?", Danger: true, OkLabel: "Remove"})
//	if m.remove.Confirmed() {
//		m.removeHost()
//	}
//
// Confirmed is true in the paint that delivers a press on the confirming button,
// Cancelled in the one that delivers the other; both close it. Escape and a press
// outside close it without either, as they do a ui.Popover. Danger paints the
// confirming button destructive. The popover opens below the button and flips
// above it when there is no room.
//
// It is a separate package so an application that does not import it pays
// nothing for it. Keep a Popconfirm in the model: it holds the open state, so a
// warm paint allocates nothing. Do not copy a Popconfirm after it has been
// declared.
package popconfirm

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// Spec declares the question.
type Spec struct {
	// Title is the question; Description is the line under it.
	Title, Description string
	// OkLabel and CancelLabel are the answers, "OK" and "Cancel" when empty.
	OkLabel, CancelLabel string
	// Danger paints the confirming button destructive, for a question about
	// removing something.
	Danger bool
	// Width is the popover's width, 280 when zero.
	Width float32
}

// Popconfirm is one question and what the user answered.
type Popconfirm struct {
	open      bool
	confirmed bool
	cancelled bool
	// spec and the closures are kept so the body is made once: a closure made in
	// each paint allocates in each paint.
	spec Spec
	c    ui.C
	body func()
}

// New returns a closed Popconfirm.
func New() *Popconfirm {
	p := &Popconfirm{}
	p.body = p.declare
	return p
}

// Open shows the question. Call it from the press on the button it belongs to.
func (p *Popconfirm) Open() { p.open = true }

// Close hides it without answering.
func (p *Popconfirm) Close() { p.open = false }

// IsOpen reports whether the question is showing.
func (p *Popconfirm) IsOpen() bool { return p.open }

// Confirmed reports whether the confirming button was pressed in the paint just made.
func (p *Popconfirm) Confirmed() bool { return p.confirmed }

// Cancelled reports whether the cancelling button was pressed in the paint just made.
func (p *Popconfirm) Cancelled() bool { return p.cancelled }

// View declares the question over anchor, the button it belongs to, while it is open.
// Declare the view on its own, not inside a Memo.
func (p *Popconfirm) View(c ui.C, anchor ui.Box, spec Spec) {
	if p == nil {
		panic("popconfirm: View needs its Popconfirm")
	}
	p.confirmed, p.cancelled = false, false
	if spec.OkLabel == "" {
		spec.OkLabel = "OK"
	}
	if spec.CancelLabel == "" {
		spec.CancelLabel = "Cancel"
	}
	width := spec.Width
	if width <= 0 {
		width = 280
	}
	p.spec, p.c = spec, c
	ui.Popover(c, anchor, &p.open, ui.PopoverSpec{Align: ui.Start, Width: width}, p.body)
}

// declare is the body of the popover: the words and the two answers.
func (p *Popconfirm) declare() {
	c, spec := p.c, p.spec
	metrics := c.Theme().Metrics
	ui.Column(c).Gap(metrics.GapItem).Children(func() {
		ui.Text(c, spec.Title).Weight(gfx.WeightSemibold)
		if spec.Description != "" {
			ui.Wrap(c, spec.Description).FontSize(metrics.FontSizeS).Muted()
		}
		ui.Row(c).Justify(ui.End).Gap(metrics.GapItem).MarginSides(metrics.GapItem, 0, 0, 0).Children(func() {
			if ui.Button(c, spec.CancelLabel).Outline().Small().Clicked() {
				p.cancelled, p.open = true, false
			}
			ok := ui.Button(c, spec.OkLabel).Small()
			if spec.Danger {
				ok = ok.Destructive()
			}
			if ok.Clicked() {
				p.confirmed, p.open = true, false
			}
		})
	})
}
