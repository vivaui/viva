package ui_test

import (
	"fmt"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

type scrollModel struct {
	n        int
	labels   []string
	goTo     float32 // asks the next paint to scroll there when positive
	end      bool
	offset   float32
	content  float32
	viewport float32
	atEnd    bool
}

func newScrollModel(n int) *scrollModel {
	m := &scrollModel{n: n}
	for i := range n {
		m.labels = append(m.labels, fmt.Sprintf("Row %d", i))
	}
	return m
}

func (m *scrollModel) page(c ui.C) {
	ui.Column(c).Fill().Pad(8).Children(func() {
		area := ui.Scroll(c).Grow(1).Gap(4)
		state := area.ScrollState()
		if m.goTo > 0 {
			state.ScrollTo(m.goTo)
			m.goTo = 0
		}
		if m.end {
			state.ScrollToEnd()
			m.end = false
		}
		m.offset, m.content, m.viewport, m.atEnd = state.Offset, state.Content, state.Viewport, state.AtEnd()
		area.Children(func() {
			for i := range m.n {
				ui.Button(c.KeyInt(i), m.labels[i]).Outline()
			}
		})
	})
}

// The state of a scroll area is readable from the page, shows what the user scrolled
// to, and moves the area by program.
func TestScrollStateReadsAndMovesAPageScrollArea(t *testing.T) {
	m := newScrollModel(60)
	d := openPage(t, m.page)
	d.repaint() // the layout of the first paint is what the second reads
	if m.content <= m.viewport || m.viewport <= 0 {
		t.Fatalf("content %v, viewport %v: the layout was not reported", m.content, m.viewport)
	}
	if m.offset != 0 || d.has("Row 40") {
		t.Fatalf("offset %v at the start, row 40 shown %v", m.offset, d.has("Row 40"))
	}

	first := d.hit("Row 0").Bounds
	step := d.hit("Row 1").Bounds.Y - first.Y
	if err := d.driver.Scroll(gfx.ScrollEvent{X: first.X + 4, Y: first.Y + 4, DeltaY: 3}); err != nil {
		t.Fatal(err)
	}
	d.repaint()
	if m.offset <= 0 {
		t.Fatalf("the wheel left the offset at %v", m.offset)
	}

	m.goTo = 40 * step
	d.repaint()
	d.repaint()
	if !d.has("Row 40") || d.has("Row 0") {
		t.Fatalf("ScrollTo did not bring row 40 into view: row 40 %v, row 0 %v", d.has("Row 40"), d.has("Row 0"))
	}

	m.end = true
	d.repaint()
	d.repaint()
	if !m.atEnd || !d.has("Row 59") {
		t.Fatalf("ScrollToEnd: at end %v, row 59 shown %v", m.atEnd, d.has("Row 59"))
	}
}

// ScrollState is for scroll areas; a container is not one.
func TestScrollStateRefusesAContainer(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("ScrollState on a Column did not panic")
		}
	}()
	d := openPage(t, func(c ui.C) { ui.Column(c).ScrollState() })
	_ = d
}

// Asking for the state of a scroll area in each paint allocates nothing.
func TestScrollStateRepaintAllocates(t *testing.T) {
	m := newScrollModel(60)
	d := openPage(t, m.page)
	if allocs := testing.AllocsPerRun(100, d.repaint); allocs != 0 {
		t.Fatalf("%v allocations a repaint", allocs)
	}
}
