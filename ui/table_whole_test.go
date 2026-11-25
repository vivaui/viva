package ui_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

type wholeRows struct{}

func (wholeRows) Len() int { return 3 }
func (wholeRows) Cell(row, column int) string {
	return []string{"alpha", "beta", "gamma"}[row] + []string{"", ":22"}[column]
}

// A Table is a whole control as declared: with only Headers its columns share its
// width, a click selects the row, and the arrow keys move the selection without
// the application handing the keys to it.
func TestATableIsWholeAsDeclared(t *testing.T) {
	var state ui.TableState
	chosen := -1
	d := openPage(t, func(c ui.C) {
		ui.Table(c, &state, ui.TableSpec{Source: wholeRows{}, Columns: titledColumns(nil, []string{"Host", "Port"}, nil)}).Width(400).Height(200)
		if state.Changed() {
			chosen = state.Selected()
		}
	})
	at := map[string]float32{}
	for _, label := range d.driver.Snapshot().Labels {
		at[label.Value] = label.Bounds.X
	}
	host, hasHost := at["alpha"]
	port, hasPort := at["alpha:22"]
	if !hasHost || !hasPort || port < host+150 {
		t.Fatalf("columns from the Headers alone: Host at %v (%v), Port at %v (%v)", host, hasHost, port, hasPort)
	}
	d.click("beta")
	if state.Selected() != 1 || chosen != 1 {
		t.Fatalf("a click on a row selected %d; the page saw %d chosen", state.Selected(), chosen)
	}
	chosen = -1
	d.repaint()
	if chosen != -1 {
		t.Fatal("Changed stayed true in the paint after the click")
	}
	d.key(gfx.KeyDown)
	if state.Selected() != 2 || d.Focused() != "gamma" || chosen != 2 {
		t.Fatalf("Down selected %d with the focus on %q; the page saw %d chosen", state.Selected(), d.Focused(), chosen)
	}
	d.key(gfx.KeyHome)
	if state.Selected() != 0 {
		t.Fatalf("Home selected %d", state.Selected())
	}
}
