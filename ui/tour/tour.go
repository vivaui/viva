// Package tour is the opt-in page control for a guided tour: a note beside one
// control of the page at a time, with the rest of the window dimmed around it,
// that the user steps through. A page keeps a Tour in its model, names the
// controls the steps point at as it declares them, and declares the tour last:
//
//	m.tour = tour.New()
//	// in the page:
//	save := ui.Button(c, "Save")
//	m.tour.Target(0, save)
//	sync := ui.Button(c, "Sync")
//	m.tour.Target(1, sync)
//	m.tour.View(c, tour.Spec{Steps: m.steps})
//	if m.tour.Finished() {
//		m.settings.TourSeen = true
//	}
//
// Start opens the tour at its first step. The note has Previous and Next
// buttons, Finish on the last step, and Skip to leave early; Escape and a press
// outside the note leave it too. Only Finish counts as finishing: Finished
// reports it, so a page can remember that the tour was seen. A step whose
// target was not declared in the paint is shown without a pointer at a
// control and without a hole in the dimming, so a tour survives a page whose
// controls come and go.
//
// It is a separate package so an application that does not import it pays
// nothing for it. Keep a Tour in the model; a warm paint allocates nothing
// while it is closed. Do not copy a Tour after it has been declared.
package tour

import (
	"strconv"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// Step is one note of the tour.
type Step struct {
	// Title heads the note and Description is the text under it; either may be
	// empty.
	Title, Description string
}

// Spec declares the tour.
type Spec struct {
	// Steps are the notes, in order. Keep the slice in the model.
	Steps []Step
	// Labels are the words of the buttons: Previous, Next, Finish and Skip;
	// "Previous", "Next", "Finish" and "Skip" when empty.
	Labels [4]string
	// NoMask leaves the window undimmed.
	NoMask bool
	// Width is the note's width, 288 when zero.
	Width float32
}

// Tour is one tour and where it is.
type Tour struct {
	open     bool
	step     int
	targets  []ui.Box // the controls of this paint, by step; a zero Box is none
	declared []bool
	finished bool
	pending  bool // Finish was pressed, reported by the next paint
	counter  struct {
		step, total int
		text        string
	}
	mask   func(*ui.DrawContext)
	note   func()
	spec   Spec
	c      ui.C
	target ui.Box
	has    bool
}

// New returns a closed tour.
func New() *Tour {
	t := &Tour{}
	t.mask = t.drawMask
	t.note = t.draw
	return t
}

// Start opens the tour at its first step.
func (t *Tour) Start() { t.open, t.step = true, 0 }

// Close leaves the tour.
func (t *Tour) Close() { t.open = false }

// IsOpen reports whether the tour is shown.
func (t *Tour) IsOpen() bool { return t.open }

// Step is the index of the step shown.
func (t *Tour) Step() int { return t.step }

// Finished reports whether the user pressed Finish since the paint before this
// one.
func (t *Tour) Finished() bool { return t.finished }

// Target says which control step i points at: the Box its declaration returned,
// in the paint being declared.
func (t *Tour) Target(step int, anchor ui.Box) {
	if step < 0 {
		panic("tour: Target needs a step index of zero or more")
	}
	for len(t.targets) <= step {
		t.targets = append(t.targets, ui.Box{})
		t.declared = append(t.declared, false)
	}
	t.targets[step], t.declared[step] = anchor, true
}

func (s *Spec) label(i int) string {
	if s.Labels[i] != "" {
		return s.Labels[i]
	}
	return [4]string{"Previous", "Next", "Finish", "Skip"}[i]
}

func (t *Tour) counterText(step, total int) string {
	if t.counter.text == "" || t.counter.step != step || t.counter.total != total {
		t.counter.step, t.counter.total = step, total
		t.counter.text = strconv.Itoa(step+1) + " / " + strconv.Itoa(total)
	}
	return t.counter.text
}

// View declares the tour at c, after the controls its steps point at. It
// declares nothing while the tour is closed. Declare it on its own, not inside a
// Memo.
func (t *Tour) View(c ui.C, spec Spec) {
	if t == nil {
		panic("tour: View needs its Tour")
	}
	t.finished, t.pending = t.pending, false
	defer t.forget()
	if !t.open || len(spec.Steps) == 0 {
		return
	}
	t.step = min(max(t.step, 0), len(spec.Steps)-1)
	t.has = t.step < len(t.declared) && t.declared[t.step]
	t.target = ui.Box{}
	if t.has {
		t.target = t.targets[t.step]
	}
	t.spec, t.c = spec, c
	if t.has && !spec.NoMask {
		ui.Overlay(c, ui.OverlaySpec{Band: ui.LayerPaneChrome, Bounds: c.Bounds()}).Children(func() {
			ui.Column(c).Fill().Draw(t.mask)
		})
	}
	width := spec.Width
	if width <= 0 {
		width = 288
	}
	step := spec.Steps[t.step]
	pop := ui.PopoverSpec{Align: ui.Center, Width: width, Title: step.Title, Description: step.Description}
	if !t.has {
		// No control to hang the note on: it is a note in the middle of the window.
		ui.Modal(c, &t.open, orDefault(step.Title, "Tour"), ui.ModalSpec{Description: step.Description, Width: width}, nil, t.note)
		return
	}
	ui.Popover(c, t.target, &t.open, pop, t.note)
}

func orDefault(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

// forget drops the controls of this paint; the next declares its own.
func (t *Tour) forget() {
	clear(t.targets)
	clear(t.declared)
}

// drawMask dims the window around the control the step points at.
func (t *Tour) drawMask(d *ui.DrawContext) {
	hole := t.target.Rect()
	if hole.Width <= 0 || hole.Height <= 0 {
		return
	}
	const gap = 4
	hole = gfx.Rect{X: hole.X - gap, Y: hole.Y - gap, Width: hole.Width + 2*gap, Height: hole.Height + 2*gap}
	dim := gfx.Color{Alpha: 0.5}
	whole := d.Rect
	canvas := d.Canvas
	canvas.Fill(gfx.Rect{X: whole.X, Y: whole.Y, Width: whole.Width, Height: max(hole.Y-whole.Y, 0)}, dim)
	canvas.Fill(gfx.Rect{X: whole.X, Y: hole.Y + hole.Height, Width: whole.Width, Height: max(whole.Y+whole.Height-hole.Y-hole.Height, 0)}, dim)
	canvas.Fill(gfx.Rect{X: whole.X, Y: hole.Y, Width: max(hole.X-whole.X, 0), Height: hole.Height}, dim)
	canvas.Fill(gfx.Rect{X: hole.X + hole.Width, Y: hole.Y, Width: max(whole.X+whole.Width-hole.X-hole.Width, 0), Height: hole.Height}, dim)
	canvas.Border(hole, 2, d.Theme.Primary, gfx.Color{})
}

// draw declares the buttons of the note.
func (t *Tour) draw() {
	c, spec := t.c, &t.spec
	metrics := c.Theme().Metrics
	last := t.step == len(spec.Steps)-1
	ui.Row(c).FillWidth().Items(ui.Center).Gap(metrics.GapItem).Children(func() {
		ui.Text(c, t.counterText(t.step, len(spec.Steps))).FontSize(metrics.FontSizeS).Muted().Grow(1)
		if !last {
			if ui.Button(c, spec.label(3)).Ghost().Small().Clicked() {
				t.open = false
			}
		}
		if t.step > 0 {
			if ui.Button(c, spec.label(0)).Outline().Small().Clicked() {
				t.step--
			}
		}
		if last {
			if ui.Button(c, spec.label(2)).Small().Clicked() {
				t.open, t.pending = false, true
			}
		} else if ui.Button(c, spec.label(1)).Small().Clicked() {
			t.step++
		}
	})
}
