//go:build !js

package popconfirm_test

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/popconfirm"
)

type model struct {
	pc                   *popconfirm.Popconfirm
	danger               bool
	confirmed, cancelled int
}

func (m *model) open(t *testing.T) *headless.Session {
	t.Helper()
	page := ui.NewPage(ui.PageConfig{View: func(c ui.C) {
		ui.Column(c).Pad(16).Children(func() {
			trigger := ui.Button(c, "Remove").Outline()
			if trigger.Clicked() {
				m.pc.Open()
			}
			m.pc.View(c, trigger, popconfirm.Spec{Title: "Remove build-01?", Description: "It cannot be undone.", OkLabel: "Yes", Danger: m.danger})
			if m.pc.Confirmed() {
				m.confirmed++
			}
			if m.pc.Cancelled() {
				m.cancelled++
			}
		})
	}})
	session, err := headless.Open(ui.Configure(page, app.Config{Title: "Pop", Width: 480, Height: 320}), nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(session.Close)
	return session
}

// The question shows only after the trigger opens it; confirming reports once and
// closes it.
func TestConfirmReportsOnceAndCloses(t *testing.T) {
	m := &model{pc: popconfirm.New()}
	session := m.open(t)
	if err := session.Expect("text", "Remove build-01?"); err == nil {
		t.Fatal("the question shows before it was opened")
	}
	if err := session.ClickLabel("Remove"); err != nil {
		t.Fatal(err)
	}
	if err := session.Expect("text", "Remove build-01?"); err != nil {
		t.Fatal(err)
	}
	if err := session.ClickLabel("Yes"); err != nil {
		t.Fatal(err)
	}
	if m.confirmed != 1 || m.cancelled != 0 {
		t.Fatalf("confirmed %d, cancelled %d after pressing Yes, want 1, 0", m.confirmed, m.cancelled)
	}
	if m.pc.IsOpen() {
		t.Fatal("the question stayed open after an answer")
	}
}

func TestCancelReportsAndCloses(t *testing.T) {
	m := &model{pc: popconfirm.New()}
	session := m.open(t)
	session.ClickLabel("Remove")
	if err := session.ClickLabel("Cancel"); err != nil {
		t.Fatal(err)
	}
	if m.cancelled != 1 || m.confirmed != 0 || m.pc.IsOpen() {
		t.Fatalf("cancelled %d, confirmed %d, open %v after Cancel", m.cancelled, m.confirmed, m.pc.IsOpen())
	}
}

// Escape closes the question without answering it.
func TestEscapeClosesWithoutAnswering(t *testing.T) {
	m := &model{pc: popconfirm.New()}
	session := m.open(t)
	session.ClickLabel("Remove")
	if err := session.Keys("escape"); err != nil {
		t.Fatal(err)
	}
	if m.pc.IsOpen() || m.confirmed != 0 || m.cancelled != 0 {
		t.Fatalf("open %v, confirmed %d, cancelled %d after Escape", m.pc.IsOpen(), m.confirmed, m.cancelled)
	}
}

func TestWarmPaintAllocatesNothing(t *testing.T) {
	pc := popconfirm.New()
	pc.Open()
	var c0 ui.C
	_ = c0
	view := func(c ui.C) {
		trigger := ui.Button(c, "Remove")
		pc.View(c, trigger, popconfirm.Spec{Title: "Remove?", Description: "No undo.", Danger: true})
	}
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: view, Theme: &theme})
	canvas := &gfx.Canvas{}
	canvas.SetReduceMotion(true)
	draw := func() {
		canvas.BeginFrame()
		canvas.Reset()
		page.Draw(canvas, gfx.Rect{Width: 480, Height: 320}, view)
		canvas.EndFrame()
	}
	draw()
	draw()
	if allocations := testing.AllocsPerRun(100, draw); allocations != 0 {
		t.Fatalf("warm paint allocated %v times, want 0", allocations)
	}
}
