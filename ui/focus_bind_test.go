package ui_test

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/ui"
)

const (
	focusA = 1
	focusB = 2
)

type focusModel struct {
	focus int
	showB bool
	seen  int
}

func (m *focusModel) view(c ui.C) {
	ui.Column(c).Fill().Children(func() {
		ui.FocusBind(ui.Button(c, "A"), &m.focus, focusA)
		if m.showB {
			ui.FocusBind(ui.Button(c, "B"), &m.focus, focusB)
		}
		m.seen = ui.FocusedValue(c, &m.focus)
	})
}

func focusSettled(d *pageDriver, want string) bool {
	for range 4 {
		if d.focused() == want {
			return true
		}
		d.repaint()
	}
	return d.focused() == want
}

func TestFocusBindRequestsAndWritesBack(t *testing.T) {
	m := &focusModel{focus: focusB, showB: true}
	d := openPage(t, m.view)
	if !focusSettled(d, "B") {
		t.Fatalf("focus = %q, want B", d.focused())
	}
	d.repaint() // the paint after the one that requested the focus
	if m.seen != focusB {
		t.Fatalf("FocusedValue = %d, want %d", m.seen, focusB)
	}
	d.click("A")
	d.repaint()
	if d.focused() != "A" {
		t.Fatalf("focus after clicking A = %q", d.focused())
	}
	if m.focus != focusA {
		t.Fatalf("the field = %d after the user focused A, want %d", m.focus, focusA)
	}
	if m.seen != focusA {
		t.Fatalf("FocusedValue = %d, want %d", m.seen, focusA)
	}
	m.focus = focusB
	if !focusSettled(d, "B") {
		t.Fatalf("focus after setting the field to B = %q", d.focused())
	}
}

func TestFocusBindWaitsForAHiddenElement(t *testing.T) {
	m := &focusModel{focus: focusB, showB: false}
	d := openPage(t, m.view)
	d.repaint()
	if d.focused() == "B" {
		t.Fatal("B is focused while hidden")
	}
	m.showB = true
	if !focusSettled(d, "B") {
		t.Fatalf("focus after B appeared = %q", d.focused())
	}
}

func TestFocusBindAsksAgainWhenTheElementComesBack(t *testing.T) {
	m := &focusModel{focus: focusB, showB: true}
	d := openPage(t, m.view)
	if !focusSettled(d, "B") {
		t.Fatalf("focus = %q, want B", d.focused())
	}
	m.showB = false
	d.repaint()
	m.showB = true
	if !focusSettled(d, "B") {
		t.Fatalf("focus after B came back = %q", d.focused())
	}
}

func TestFocusBindNeedsAnIdentity(t *testing.T) {
	defer func() {
		if r := recover(); r == nil || !strings.Contains(r.(string), "identity") {
			t.Fatalf("recover() = %v, want an identity panic", r)
		}
	}()
	var focus int
	page := ui.NewPage(ui.PageConfig{})
	drawPage(page, benchCanvas(), func(c ui.C) {
		ui.FocusBind(ui.Slot(c).Size(10, 10), &focus, 1)
	})
}

func TestFocusBindInAMemoPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil || !strings.Contains(r.(string), "Memo") {
			t.Fatalf("recover() = %v, want a Memo panic", r)
		}
	}()
	var focus int
	page := ui.NewPage(ui.PageConfig{})
	drawPage(page, benchCanvas(), func(c ui.C) {
		ui.Memo(c, 1, func() {
			ui.FocusBind(ui.Button(c, "A"), &focus, 1)
		})
	})
}

func TestFocusBindSteadyPaintAllocatesNothing(t *testing.T) {
	m := &focusModel{focus: focusB, showB: true}
	page := ui.NewPage(ui.PageConfig{View: m.view})
	canvas := benchCanvas()
	for range 3 {
		drawPage(page, canvas, m.view)
	}
	if allocs := testing.AllocsPerRun(30, func() { drawPage(page, canvas, m.view) }); allocs != 0 {
		t.Fatalf("a steady focus-bound page allocates %v times", allocs)
	}
}
