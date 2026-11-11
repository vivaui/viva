package ui_test

import (
	"fmt"
	"testing"
	"time"

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

// A row's own height comes from the same Node description that names it, so a
// tree of compact and full rows needs no hidden per-row state.
func TestTreeRowsCanHaveTheirOwnHeights(t *testing.T) {
	heights := []float32{20, 40, 30}
	labels := []string{"compact", "full", "medium"}
	var state ui.TreeState
	state.Node = func(i int) ui.TreeNode { return ui.TreeNode{Height: heights[i]} }
	state.Label = func(i int) string { return labels[i] }
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Fill().Children(func() {
			ui.Tree(c, &state, len(labels), state.Label, nil).Grow(1)
		})
	})
	for i, label := range labels {
		if got := d.hit(label).Bounds.Height; got != heights[i] {
			t.Fatalf("%s height = %v, want %v", label, got, heights[i])
		}
		if got := state.RowHeightAt(i); got != heights[i] {
			t.Fatalf("RowHeightAt(%d) = %v, want %v", i, got, heights[i])
		}
	}
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

// dndModel is treeModel whose files can be dragged into another folder, and whose
// folders can be reordered; nothing goes into a file.
type dndModel struct {
	*treeModel
	moves []ui.TreeMove
}

func newDndModel() *dndModel {
	m := &dndModel{treeModel: newTreeModel([]folder{
		{name: "src", open: true, files: []string{"main.go", "page.go"}},
		{name: "docs", files: []string{"guide.md"}},
		{name: "test", files: []string{"a_test.go"}},
	})}
	m.state.CanDrop = func(from, to int, where ui.TreeDrop) bool {
		f, t := m.visible[from], m.visible[to]
		if f.file >= 0 { // a file: into another folder
			return t.file < 0 && where == ui.TreeDropInside && t.folder != f.folder
		}
		return t.file < 0 && where != ui.TreeDropInside // a folder: beside another
	}
	return m
}

func (m *dndModel) page(c ui.C) {
	m.treeModel.page(c)
	if move, ok := m.state.Dropped(); ok {
		m.moves = append(m.moves, move)
		f, t := m.visible[move.From], m.visible[move.To]
		c.After(func() { // the rows change after the paint that declared them
			if f.file >= 0 {
				from := &m.folders[f.folder]
				to := &m.folders[t.folder]
				to.files = append(to.files, from.files[f.file])
				from.files = append(from.files[:f.file], from.files[f.file+1:]...)
			}
			m.flatten()
		})
	}
}

func (d *pageDriver) drag(from string, to gfx.Rect, fraction float32) {
	d.t.Helper()
	start := d.hit(from).Bounds
	x, y := start.X+start.Width/2, start.Y+start.Height/2
	if err := d.driver.Press(x, y, gfx.PointerButtonPrimary); err != nil {
		d.t.Fatal(err)
	}
	tx, ty := to.X+to.Width/2, to.Y+to.Height*fraction
	for _, step := range []float32{0.5, 1} { // past the threshold, then onto the row
		if err := d.driver.Move(x+(tx-x)*step, y+(ty-y)*step); err != nil {
			d.t.Fatal(err)
		}
	}
}

func (d *pageDriver) release(at gfx.Rect, fraction float32) {
	d.t.Helper()
	if err := d.driver.Release(at.X+at.Width/2, at.Y+at.Height*fraction, gfx.PointerButtonPrimary); err != nil {
		d.t.Fatal(err)
	}
}

// A file dragged onto a folder goes into it, and the release is not also a click; a
// row CanDrop refuses takes nothing, and Escape lets go of a drag.
func TestTreeDragIntoAFolder(t *testing.T) {
	m := newDndModel()
	d := openPage(t, m.page)
	docs := d.hit("docs").Bounds
	d.drag("page.go", docs, 0.5)
	if !m.state.Dragging() {
		t.Fatal("the press that moved is not a drag")
	}
	d.release(docs, 0.5)
	want := ui.TreeMove{From: 2, To: 3, Where: ui.TreeDropInside}
	if len(m.moves) != 1 || m.moves[0] != want {
		t.Fatalf("drops %v, want [%v]", m.moves, want)
	}
	if m.opened != "" {
		t.Fatalf("the drag ended in a click on %q", m.opened)
	}
	if got := m.folders[1].files; len(got) != 2 || got[1] != "page.go" {
		t.Fatalf("docs holds %v", got)
	}

	// Onto a file: CanDrop takes nothing there.
	main := d.hit("main.go").Bounds
	d.drag("test", main, 0.5)
	d.release(main, 0.5)
	if len(m.moves) != 1 {
		t.Fatalf("a refused drop was reported: %v", m.moves[1:])
	}

	// Escape in the middle of a drag.
	d.drag("main.go", docs, 0.5)
	d.key(gfx.KeyEscape)
	if m.state.Dragging() {
		t.Fatal("Escape did not let go of the drag")
	}
	d.release(docs, 0.5)
	if len(m.moves) != 1 {
		t.Fatalf("a drag let go by Escape dropped: %v", m.moves[1:])
	}
}

// The top of a row drops before it and its bottom after it, and a press that does
// not move is still a click.
func TestTreeDragBeforeAndAfter(t *testing.T) {
	m := newDndModel()
	d := openPage(t, m.page)
	src := d.hit("src").Bounds
	d.drag("test", src, 0.1)
	d.release(src, 0.1)
	test := d.hit("test").Bounds
	d.drag("src", test, 0.9)
	d.release(test, 0.9)
	// The rows: src, main.go, page.go, docs, test.
	want := []ui.TreeMove{{From: 4, To: 0, Where: ui.TreeDropBefore}, {From: 0, To: 4, Where: ui.TreeDropAfter}}
	if len(m.moves) != 2 || m.moves[0] != want[0] || m.moves[1] != want[1] {
		t.Fatalf("drops %v, want %v", m.moves, want)
	}
	d.click("main.go")
	if m.opened != "main.go" || len(m.moves) != 2 {
		t.Fatalf("a click: opened %q, drops %v", m.opened, m.moves)
	}
}

// Typing the start of a row's name moves to the next row it begins, and a pause
// starts the word anew.
func TestTreeTypeahead(t *testing.T) {
	m := newDndModel()
	d := openPage(t, m.page)
	d.click("src")
	typed := func(letter rune, want string) {
		t.Helper()
		if _, err := d.driver.ShortcutChord(0, letter); err != nil {
			t.Fatal(err)
		}
		if d.focused() != want {
			t.Fatalf("%q went to %q, not %q", letter, d.focused(), want)
		}
	}
	typed('t', "test")
	if err := d.driver.Advance(time.Second); err != nil {
		t.Fatal(err)
	}
	typed('p', "page.go")
	typed('a', "page.go") // "pa"
	if err := d.driver.Advance(time.Second); err != nil {
		t.Fatal(err)
	}
	typed('d', "docs")
}

// A drag that rests at the bottom edge of a tree scrolls it, and only while it rests
// there are frames asked for.
func TestTreeDragScrollsAtTheEdge(t *testing.T) {
	var folders []folder
	for i := range 40 {
		folders = append(folders, folder{name: fmt.Sprintf("group %02d", i)})
	}
	m := newTreeModel(folders)
	m.state.CanDrop = func(from, to int, where ui.TreeDrop) bool { return where != ui.TreeDropInside }
	d := openPage(t, m.page)
	start := d.hit("group 00").Bounds
	x := start.X + start.Width/2
	if err := d.driver.Press(x, start.Y+4, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	bottom := float32(480 - 12) // the window's height less the column's padding, inside the last row
	if err := d.driver.Move(x, start.Y+40); err != nil {
		t.Fatal(err)
	}
	if err := d.driver.Move(x, bottom); err != nil {
		t.Fatal(err)
	}
	if err := d.driver.Advance(500 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if d.has("group 00") {
		t.Fatal("a drag at the bottom edge did not scroll the tree")
	}
	if err := d.driver.Move(x, bottom-160); err != nil { // away from the edge
		t.Fatal(err)
	}
	if err := d.driver.Advance(100 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if d.driver.Transitioning() {
		t.Fatal("frames are still asked for with the drag away from the edge")
	}
	if err := d.driver.Release(x, bottom-160, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
}

// A drag repaints without allocating.
func TestTreeDragRepaintAllocates(t *testing.T) {
	m := newDndModel()
	d := openPage(t, m.page)
	d.drag("page.go", d.hit("docs").Bounds, 0.5)
	if allocs := testing.AllocsPerRun(100, d.repaint); allocs != 0 {
		t.Fatalf("%v allocations a repaint", allocs)
	}
}

// A drag that comes back to the row it started on and is let go there drops nothing
// and is not a click on it; with both taken, a row's top is before it and its
// middle into it.
func TestTreeDragBackAndZones(t *testing.T) {
	m := newDndModel()
	m.state.CanDrop = func(from, to int, where ui.TreeDrop) bool { return m.visible[to].file < 0 && where != ui.TreeDropAfter }
	d := openPage(t, m.page)
	main := d.hit("main.go").Bounds
	d.drag("main.go", d.hit("docs").Bounds, 0.5)
	if err := d.driver.Move(main.X+main.Width/2, main.Y+main.Height/2); err != nil {
		t.Fatal(err)
	}
	d.release(main, 0.5)
	if m.opened != "" || len(m.moves) != 0 {
		t.Fatalf("a drag back to its row: opened %q, drops %v", m.opened, m.moves)
	}
	docs := d.hit("docs").Bounds
	d.drag("main.go", docs, 0.1)
	d.release(docs, 0.1)
	d.drag("page.go", d.hit("test").Bounds, 0.5)
	d.release(d.hit("test").Bounds, 0.5)
	if len(m.moves) != 2 || m.moves[0].Where != ui.TreeDropBefore || m.moves[1].Where != ui.TreeDropInside {
		t.Fatalf("drops %v: want before docs, then into test", m.moves)
	}
}

// A row's and a chevron's identity are the ones the paint names, so the
// application can focus a row or find its hit without scanning the canvas.
func TestTreeRowAndDisclosureIDs(t *testing.T) {
	m := newTreeModel([]folder{{name: "etc", files: []string{"hosts"}}, {name: "var"}})
	if !m.state.RowID(0).IsZero() {
		t.Fatal("RowID names a row before the tree is declared")
	}
	d := openPage(t, m.page)
	var row, chevron bool
	for _, hit := range d.driver.Snapshot().Hits {
		if hit.ID == m.state.RowID(0) && hit.Label == "etc" {
			row = true
		}
		if hit.ID == m.state.DisclosureID(0) && hit.Expandable {
			chevron = true
		}
	}
	if !row || !chevron {
		t.Fatalf("hits do not carry RowID(0) (%v) and DisclosureID(0) (%v)", row, chevron)
	}
	if m.state.RowID(0) == m.state.RowID(1) || m.state.RowID(0) == m.state.DisclosureID(0) {
		t.Fatal("identities collide")
	}
}

func TestListRowID(t *testing.T) {
	var state ui.ListState
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Fill().Children(func() {
			ui.List(c, &state, 3, func(row ui.ListRow) {
				ui.Text(row.C, fmt.Sprint("row ", row.Index))
			}).Grow(1)
		})
	})
	found := false
	for _, hit := range d.driver.Snapshot().Hits {
		found = found || hit.ID == state.RowID(1)
	}
	if !found || state.RowID(1) == state.RowID(2) {
		t.Fatal("RowID(1) is not the identity of the second row's hit")
	}
}

// TreeState.SelectedColor is the selected row's background, as Indent is the tree's
// step: the theme's Accent unless the application chooses another.
func TestTreeSelectedRowColourIsTheStates(t *testing.T) {
	selected := gfx.Color{Red: 0.2, Green: 0.5, Blue: 0.4, Alpha: 1}
	labels := []string{"one", "two"}
	var state ui.TreeState
	state.SelectedColor = selected
	state.Node = func(i int) ui.TreeNode { return ui.TreeNode{Selected: i == 0} }
	state.Label = func(i int) string { return labels[i] }
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Fill().Children(func() {
			ui.Tree(c, &state, len(labels), state.Label, nil).Grow(1)
		})
	})
	row := d.hit("one")
	found := false
	for _, p := range d.driver.Snapshot().Primitives {
		if p.Color == selected && p.Bounds.X == row.Bounds.X && p.Bounds.Y == row.Bounds.Y &&
			p.Bounds.Width == row.Bounds.Width && p.Bounds.Height == row.Bounds.Height {
			found = true
		}
	}
	if !found {
		t.Fatalf("the selected row %+v painted no background in %+v", row.Bounds, selected)
	}
}
