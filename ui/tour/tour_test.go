package tour_test

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/tour"
)

var steps = []tour.Step{
	{Title: "Save", Description: "Saves the file."},
	{Title: "Sync", Description: "Sends it to the host."},
	{Title: "Done", Description: "That is all."},
}

type model struct {
	t        *tour.Tour
	hideSync bool
	finished int
}

func (m *model) page(c ui.C) {
	ui.Column(c).Fill().Pad(16).Gap(12).Children(func() {
		save := ui.Button(c, "Save File")
		m.t.Target(0, save)
		if !m.hideSync {
			sync := ui.Button(c, "Sync Now")
			m.t.Target(1, sync)
		}
		m.t.View(c, tour.Spec{Steps: steps})
		if m.t.Finished() {
			m.finished++
		}
	})
}

type driver struct {
	t *testing.T
	*app.Driver
	wake *app.Invalidator
}

func open(t *testing.T, m *model) driver {
	t.Helper()
	m.t = tour.New()
	page := ui.NewPage(ui.PageConfig{View: m.page})
	wake := new(app.Invalidator)
	d, err := app.NewDriver(ui.Configure(page, app.Config{Width: 480, Height: 360, Invalidator: wake}), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(d.Close)
	return driver{t, d, wake}
}

func (d driver) repaint() {
	d.t.Helper()
	d.wake.Invalidate()
	if err := d.Pump(); err != nil {
		d.t.Fatal(err)
	}
}

func (d driver) shown(text string) bool {
	for _, v := range d.Snapshot().Texts {
		if v.Value == text {
			return true
		}
	}
	return false
}

func (d driver) click(label string) {
	d.t.Helper()
	hits := d.Snapshot().Hits
	for i := len(hits) - 1; i >= 0; i-- {
		if h := hits[i]; h.Label == label && !h.IsLabel && !h.ID.IsZero() {
			if err := d.Click(h.Bounds.X+h.Bounds.Width/2, h.Bounds.Y+h.Bounds.Height/2); err != nil {
				d.t.Fatal(err)
			}
			return
		}
	}
	d.t.Fatalf("no control labeled %q", label)
}

func TestClosedTourDeclaresNothing(t *testing.T) {
	d := open(t, &model{})
	if d.shown("1 / 3") {
		t.Fatal("a closed tour shows its counter")
	}
}

func TestTourStepsForwardAndBackAndFinishes(t *testing.T) {
	m := &model{}
	d := open(t, m)
	m.t.Start()
	d.repaint()
	if !d.shown("Save") || !d.shown("1 / 3") {
		t.Fatal("the first step is not shown")
	}
	d.click("Next")
	if !d.shown("2 / 3") || !d.shown("Sync") {
		t.Fatal("Next did not show the second step")
	}
	d.click("Previous")
	if !d.shown("1 / 3") {
		t.Fatal("Previous did not go back")
	}
	d.click("Next")
	d.click("Next")
	if !d.shown("3 / 3") {
		t.Fatal("the last step is not shown")
	}
	d.click("Finish")
	d.repaint()
	if m.t.IsOpen() {
		t.Fatal("Finish left the tour open")
	}
	if m.finished != 1 {
		t.Fatalf("Finished was reported %d times, want 1", m.finished)
	}
}

func TestSkipLeavesWithoutFinishing(t *testing.T) {
	m := &model{}
	d := open(t, m)
	m.t.Start()
	d.repaint()
	d.click("Skip")
	d.repaint()
	if m.t.IsOpen() || m.finished != 0 {
		t.Fatalf("after Skip: open %v, finished %d", m.t.IsOpen(), m.finished)
	}
}

func TestStepWithoutATargetIsStillShown(t *testing.T) {
	m := &model{hideSync: true}
	d := open(t, m)
	m.t.Start()
	d.repaint()
	d.click("Next")
	if !d.shown("Sync") || !d.shown("2 / 3") {
		t.Fatal("a step whose control is not on the page is not shown")
	}
}

func TestClosedWarmPaintDoesNotAllocate(t *testing.T) {
	m := &model{t: tour.New()}
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: m.page, Theme: &theme})
	canvas := &gfx.Canvas{}
	canvas.SetReduceMotion(true)
	draw := func() {
		canvas.BeginFrame()
		canvas.Reset()
		page.Draw(canvas, gfx.Rect{Width: 480, Height: 360}, m.page)
		canvas.EndFrame()
	}
	draw()
	draw()
	if n := testing.AllocsPerRun(100, draw); n != 0 {
		t.Fatalf("a warm paint allocates %v times", n)
	}
}
