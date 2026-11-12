package ui_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// shortcutModel is a window with a page-wide Save, a list whose Delete acts while
// the focus is in it, a field, and a popover.
type shortcutModel struct {
	saves, deletes, outerDeletes, clears int
	rows                                 []string
	query                                string
	menu                                 bool
}

func (m *shortcutModel) page(c ui.C) {
	if ui.Shortcut(c, ui.Primary, "s") {
		m.saves++
	}
	if ui.Shortcut(c, 0, "delete") { // page-wide, without a modifier
		m.clears++
	}
	ui.Column(c).Pad(16).Gap(8).Children(func() {
		ui.TextField(c, &m.query, "Search", "").Width(240)
		outer := ui.Column(c).Gap(4).Children(func() {
			list := ui.Column(c).Gap(4).Children(func() {
				for _, row := range m.rows {
					ui.Button(c.Key(row), row)
				}
			})
			if list.Shortcut(0, "delete") {
				m.deletes++
			}
		})
		if outer.Shortcut(0, "delete") {
			m.outerDeletes++
		}
		more := ui.Button(c, "More")
		if more.Clicked() {
			m.menu = !m.menu
		}
		ui.Popover(c, more, &m.menu, func() { ui.Button(c, "Rename") })
	})
}

func TestShortcutActsWhereTheFocusIs(t *testing.T) {
	m := &shortcutModel{rows: []string{"a", "b"}}
	d := openPage(t, m.page)

	chord := func(text rune) {
		t.Helper()
		if _, err := d.driver.ShortcutChord(ui.Primary, text); err != nil {
			t.Fatal(err)
		}
	}
	chord('s')
	if m.saves != 1 {
		t.Fatalf("Primary+S with nothing focused: saves %d", m.saves)
	}
	d.click("Search")
	chord('s')
	if m.saves != 2 {
		t.Fatalf("Primary+S in a field: saves %d", m.saves)
	}
	d.typeText("s") // a plain key types in the field
	if m.query != "s" || m.saves != 2 {
		t.Fatalf("typing reached %q, saves %d", m.query, m.saves)
	}
	d.key(gfx.KeyDelete) // a key without Primary edits the field it is typed in
	if m.deletes != 0 || m.outerDeletes != 0 || m.clears != 0 {
		t.Fatalf("Delete in the field acted: lists %d, %d, page %d", m.deletes, m.outerDeletes, m.clears)
	}

	d.click("b")
	d.key(gfx.KeyDelete)
	if m.deletes != 1 || m.outerDeletes != 0 || m.clears != 0 {
		t.Fatalf("Delete in the inner list: inner %d, outer %d, page %d (the innermost acts)", m.deletes, m.outerDeletes, m.clears)
	}
	m.rows = nil // the list is gone, and with it the focus
	d.repaint()
	d.key(gfx.KeyDelete)
	if m.deletes != 1 || m.clears != 1 {
		t.Fatalf("Delete with no list: inner %d, page %d", m.deletes, m.clears)
	}
}

// While an overlay has the keyboard, a page-wide Shortcut does not act behind it,
// and Escape still closes it.
func TestShortcutWaitsForAnOverlay(t *testing.T) {
	m := &shortcutModel{}
	d := openPage(t, m.page)
	d.click("More")
	if !m.menu {
		t.Fatal("the popover did not open")
	}
	if _, err := d.driver.ShortcutChord(ui.Primary, 's'); err != nil {
		t.Fatal(err)
	}
	if m.saves != 0 {
		t.Fatal("a page-wide Shortcut acted behind an open popover")
	}
	d.key(gfx.KeyEscape)
	if m.menu {
		t.Fatal("Escape did not close the popover")
	}
}

func BenchmarkShortcutPaint(b *testing.B) {
	m := &shortcutModel{rows: []string{"a", "b", "c"}}
	page, canvas := ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas()
	b.ReportAllocs()
	for b.Loop() {
		drawPage(page, canvas, m.page)
	}
}

// A key without Primary, Control or Alt that reaches the page from a field being
// typed in is the field's: Up in a TextArea moves to the row above, though the page
// has a Shortcut for Up.
func TestShortcutLeavesPlainKeysToAField(t *testing.T) {
	notes, ups := "", 0
	d := openPage(t, func(c ui.C) {
		if ui.Shortcut(c, 0, "up") {
			ups++
		}
		ui.Column(c).Pad(16).Children(func() {
			ui.TextArea(c, &notes, "Notes", "").Width(300).Height(120)
		})
	})
	d.click("Notes")
	d.typeText("one")
	d.key(gfx.KeyEnter)
	d.typeText("two")
	d.key(gfx.KeyUp)
	d.typeText("!")
	if ups != 0 || notes != "one!\ntwo" {
		t.Fatalf("ups %d, notes %q", ups, notes)
	}
}

// A Tree's rows are inside the box Tree returns, as a List's are, so its Shortcut
// acts while one of them has the keyboard focus.
func TestTreeShortcutActsWhileARowHasFocus(t *testing.T) {
	rows := []string{"alpha", "beta"}
	var state ui.TreeState
	state.Node = func(int) ui.TreeNode { return ui.TreeNode{} }
	state.Label = func(i int) string { return rows[i] }
	var opens int
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Fill().Pad(16).Children(func() {
			tree := ui.Tree(c, &state, len(rows), state.Label, nil).Grow(1)
			if tree.Shortcut(0, "f4") {
				opens++
			}
		})
	})
	d.click("alpha")
	if got := d.focused(); got != "alpha" {
		t.Fatalf("focus = %q, want the row under the pointer", got)
	}
	d.key(gfx.KeyF4)
	if opens != 1 {
		t.Fatalf("the Tree's F4 acted %d times with a row focused", opens)
	}
}

// A List's rows are inside the box List returns, so its Shortcut acts while one
// of them has the keyboard focus.
func TestListShortcutActsWhileARowHasFocus(t *testing.T) {
	rows := []string{"alpha", "beta"}
	var state ui.ListState
	state.Label = func(i int) string { return rows[i] }
	var deletes int
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Fill().Pad(16).Children(func() {
			list := ui.List(c, &state, len(rows), func(row ui.ListRow) {
				ui.Text(row.C, rows[row.Index])
			}).Grow(1)
			if list.Shortcut(0, "delete") {
				deletes++
			}
		})
	})
	d.click("alpha")
	if got := d.focused(); got != "alpha" {
		t.Fatalf("focus = %q, want the row under the pointer", got)
	}
	d.key(gfx.KeyDelete)
	if deletes != 1 {
		t.Fatalf("the List's Delete acted %d times with a row focused", deletes)
	}
}
