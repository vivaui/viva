package ui_test

import (
	"fmt"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// folder is a tree of the application's: folders with files in them.
type folder struct {
	name  string
	open  bool
	files []string
}

type treeModel struct {
	folders []folder
	visible []treeEntry // the rows that show, flattened
	state   ui.TreeState
	label   func(int) string // m.name, made once
	opened  string
}

type treeEntry struct {
	folder, file int // file -1 for the folder's own row
}

func newTreeModel(folders []folder) *treeModel {
	m := &treeModel{folders: folders}
	m.state.Node, m.label = m.node, m.name
	m.state.Key = func(i int) int { e := m.visible[i]; return e.folder<<16 | (e.file + 1) }
	m.flatten()
	return m
}

func (m *treeModel) flatten() {
	m.visible = m.visible[:0]
	for i, f := range m.folders {
		m.visible = append(m.visible, treeEntry{i, -1})
		if f.open {
			for j := range f.files {
				m.visible = append(m.visible, treeEntry{i, j})
			}
		}
	}
}

func (m *treeModel) node(i int) ui.TreeNode {
	e := m.visible[i]
	if e.file < 0 {
		return ui.TreeNode{Branch: true, Expanded: m.folders[e.folder].open}
	}
	return ui.TreeNode{Depth: 1}
}

func (m *treeModel) name(i int) string {
	e := m.visible[i]
	if e.file < 0 {
		return m.folders[e.folder].name
	}
	return m.folders[e.folder].files[e.file]
}

func (m *treeModel) page(c ui.C) {
	ui.Column(c).Fill().Pad(8).Children(func() {
		ui.Tree(c, &m.state, len(m.visible), m.label, nil).Grow(1)
		if i, ok := m.state.Toggled(); ok {
			f := &m.folders[m.visible[i].folder]
			f.open = !f.open
			c.After(m.flatten)
		}
		if i, ok := m.state.Chosen(); ok {
			m.opened = m.name(i)
		}
	})
}

// The chevron opens and closes a branch; the arrows move, open and close; Enter
// chooses.
func TestTreeOpensMovesAndChooses(t *testing.T) {
	m := newTreeModel([]folder{{name: "etc", files: []string{"hosts", "passwd"}}, {name: "var", files: []string{"log"}}})
	d := openPage(t, m.page)
	if d.has("hosts") {
		t.Fatal("a closed folder shows its files")
	}
	d.click("etc") // the row: chosen, focused
	if m.opened != "etc" {
		t.Fatalf("chose %q", m.opened)
	}
	d.key(gfx.KeyRight) // opens it
	if !m.folders[0].open || !d.has("hosts") {
		t.Fatal("Right did not open the branch")
	}
	d.key(gfx.KeyRight) // to its first child
	if d.focused() != "hosts" {
		t.Fatalf("Right on an open branch went to %q", d.focused())
	}
	d.key(gfx.KeyDown)
	d.key(gfx.KeyEnter)
	if m.opened != "passwd" {
		t.Fatalf("Down and Enter chose %q", m.opened)
	}
	d.key(gfx.KeyLeft) // to the parent
	if d.focused() != "etc" {
		t.Fatalf("Left on a leaf went to %q", d.focused())
	}
	d.key(gfx.KeyLeft) // closes it
	if m.folders[0].open || d.has("hosts") {
		t.Fatal("Left did not close the branch")
	}
	for _, hit := range d.driver.Snapshot().Hits {
		if hit.Expandable && hit.Label == "var" {
			if err := d.driver.Click(center(hit.Bounds)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if !m.folders[1].open || m.opened != "passwd" {
		t.Fatalf("the chevron: open %v, chose %q", m.folders[1].open, m.opened)
	}
}

// A tree of thousands of rows declares the ones on screen, and End reaches the last.
func TestTreeLarge(t *testing.T) {
	folders := make([]folder, 2000)
	for i := range folders {
		folders[i] = folder{name: fmt.Sprintf("f%04d", i), files: []string{"a"}}
	}
	m := newTreeModel(folders)
	d := openPage(t, m.page)
	rows := 0
	for _, hit := range d.driver.Snapshot().Hits {
		if hit.Role == gfx.RoleButton && !hit.Expandable && len(hit.Label) == 5 {
			rows++
		}
	}
	if rows == 0 || rows > 40 {
		t.Fatalf("%d rows declared for a window of about 15", rows)
	}
	d.click("f0000")
	d.key(gfx.KeyEnd)
	if d.focused() != "f1999" {
		t.Fatalf("End went to %q", d.focused())
	}
}

func BenchmarkTree(b *testing.B) {
	folders := make([]folder, 500)
	for i := range folders {
		folders[i] = folder{name: fmt.Sprintf("f%04d", i), open: i%2 == 0, files: []string{"a", "b"}}
	}
	m := newTreeModel(folders)
	page, canvas := ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas()
	view := m.page
	b.ReportAllocs()
	for b.Loop() {
		drawPage(page, canvas, view)
	}
}
