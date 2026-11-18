package ui_test

import (
	"fmt"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// reorderModel is a list of names that reorders in place, with a handle per row.
type reorderModel struct {
	names []string
	state ui.ListState
	moves []string
}

func newReorderModel(names ...string) *reorderModel {
	m := &reorderModel{names: names}
	m.state.Key = func(i int) int { return i }
	m.state.Label = func(i int) string { return m.names[i] }
	m.state.Reorder = func(rows []int, to int) {
		m.moves = append(m.moves, fmt.Sprint(rows, to))
		run := append([]string(nil), m.names[rows[0]:rows[len(rows)-1]+1]...)
		rest := append(append([]string(nil), m.names[:rows[0]]...), m.names[rows[len(rows)-1]+1:]...)
		if to > rows[len(rows)-1] {
			to -= len(run)
		}
		m.names = append(append(append([]string(nil), rest[:to]...), run...), rest[to:]...)
	}
	return m
}

func (m *reorderModel) order() string {
	out := ""
	for _, name := range m.names {
		out += name
	}
	return out
}

// A list whose drags start on a handle starts them only there: a drag from the row
// body moves nothing, a drag from the handle does, and plain arrows on the focused
// handle reorder the row.
func TestListReorderHandleOnly(t *testing.T) {
	var tree gfx.Tree
	grips := tree.Sub(tree.Feature("page"), tree.Name("grip"))
	m := newReorderModel("a", "b", "c", "d")
	m.state.Handle = grips
	d := openPageWith(t, ui.PageConfig{Tree: &tree, Root: "page", View: func(c ui.C) {
		ui.List(c, &m.state, len(m.names), func(row ui.ListRow) {
			row.Element.Children(func() {
				ui.Slot(c).Size(24, 24).Shrink(0).HitWith(grips.At(row.Index), "grip "+m.names[row.Index], gfx.HitOptions{Role: gfx.RoleButton})
				ui.Text(c, m.names[row.Index]).Grow(1)
			})
		}).Height(200)
	}}, app.Config{Tree: &tree})
	d.repaint()

	// Plain arrows reorder from the focused handle, without Alt.
	d.click("grip b")
	if d.focused() != "grip b" {
		t.Fatalf("the handle took %q's focus", d.focused())
	}
	if err := d.driver.Key(gfx.KeyUp, 0); err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprint(m.moves); got != "[[1] 0]" || m.order() != "bacd" {
		t.Fatalf("Up on the handle moved %v, order %q", m.moves, m.order())
	}
	if got := d.focused(); got != "grip b" {
		t.Fatalf("the handle left the keyboard for %q", got)
	}

	// A drag from the row body is a click, not a reorder.
	row := d.hit("c").Bounds
	if err := d.driver.Press(row.X+row.Width-10, row.Y+row.Height/2, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if err := d.driver.Move(row.X+row.Width-10, row.Y+row.Height+60); err != nil {
		t.Fatal(err)
	}
	if err := d.driver.Release(row.X+row.Width-10, row.Y+row.Height+60, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprint(m.moves); got != "[[1] 0]" {
		t.Fatalf("a drag from the row body reordered: %v", m.moves)
	}

	// A drag from a handle does: a before d's lower half.
	grip := d.hit("grip a").Bounds
	target := d.hit("d").Bounds
	if err := d.driver.Press(grip.X+grip.Width/2, grip.Y+grip.Height/2, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if err := d.driver.Move(target.X+target.Width/2, target.Y+target.Height*0.9); err != nil {
		t.Fatal(err)
	}
	if err := d.driver.Release(target.X+target.Width/2, target.Y+target.Height*0.9, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprint(m.moves); got != "[[1] 0 [1] 4]" || m.order() != "bcda" {
		t.Fatalf("a drag from the handle moved %v, order %q", m.moves, m.order())
	}
	if got := d.focused(); got != "grip a" {
		t.Fatalf("after the drop the keyboard is on %q, want the moved row's handle", got)
	}
}
