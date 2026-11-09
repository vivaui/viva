package ui_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/keymap"
	"github.com/vivaui/viva/ui"
)

// --- sections with a heading that stays at the top ---

type sectioned struct {
	list    ui.ListState
	sel     ui.Selection
	items   []sectionItem
	c       ui.C
	rowFn   func(ui.ListRow)
	width   float32
	changed int
}

type sectionItem struct {
	text   string
	header bool
}

func newSectioned(sections ...[]string) *sectioned {
	m := &sectioned{}
	for _, section := range sections {
		m.items = append(m.items, sectionItem{text: section[0], header: true})
		for _, name := range section[1:] {
			m.items = append(m.items, sectionItem{text: name})
		}
	}
	m.list.Header = func(i int) bool { return m.items[i].header }
	m.list.Label = func(i int) string { return m.items[i].text }
	m.list.Selection = &m.sel
	m.rowFn = m.row
	return m
}

func (m *sectioned) row(row ui.ListRow) {
	row.Element.Height(24).PadX(8)
	if row.Header {
		row.Element.Background(m.c.Theme().Muted)
	}
	row.Element.Children(func() { ui.Text(m.c, m.items[row.Index].text) })
}

func (m *sectioned) page(c ui.C) {
	m.c = c
	m.list.RowWidth = m.width
	ui.Column(c).Fill().Pad(8).Children(func() {
		ui.List(c, &m.list, len(m.items), m.rowFn).Label("Items").Grow(1)
	})
}

// manySections is five sections of ten rows each: "Section k", "k-0" ... "k-9".
func manySections() *sectioned {
	var sections [][]string
	for k := range 5 {
		section := []string{fmt.Sprintf("Section %d", k)}
		for j := range 10 {
			section = append(section, fmt.Sprintf("%d-%d", k, j))
		}
		sections = append(sections, section)
	}
	return newSectioned(sections...)
}

// textsNamed returns the bounds of every painted text with the value.
func (d *pageDriver) textsNamed(value string) []gfx.Rect {
	var bounds []gfx.Rect
	for _, text := range d.driver.Snapshot().Texts {
		if text.Value == value {
			bounds = append(bounds, text.Bounds)
		}
	}
	return bounds
}

// listArea is the rectangle of the listbox named label.
func (d *pageDriver) listArea(label string) gfx.Rect {
	d.t.Helper()
	for _, hit := range d.driver.Snapshot().Hits {
		if hit.Container && hit.Role == gfx.RoleListBox && hit.Label == label {
			return hit.Bounds
		}
	}
	d.t.Fatalf("no listbox %q", label)
	return gfx.Rect{}
}

// wheel scrolls the list under the pointer by exact distances.
func (d *pageDriver) wheel(area gfx.Rect, dx, dy float32, modifiers gfx.Modifiers) {
	d.t.Helper()
	event := gfx.ScrollEvent{X: area.X + 20, Y: area.Y + 20, DeltaX: dx, DeltaY: dy, Precise: true, Modifiers: modifiers}
	if err := d.driver.Scroll(event); err != nil {
		d.t.Fatal(err)
	}
	d.repaint()
}

// The heading of the section at the top stays there while its rows scroll under it,
// and the next heading pushes it up as it arrives.
func TestListHeaderStaysAtTheTop(t *testing.T) {
	m := manySections()
	d := openPage(t, m.page)
	d.repaint()
	area := d.listArea("Items")
	if got := d.textsNamed("Section 0"); len(got) != 1 {
		t.Fatalf("at the top Section 0 is painted %d times", len(got))
	}

	// Into section 1 (rows 11..21): its heading is above the top, and pinned there.
	d.wheel(area, 0, 15*24, 0)
	pinned := d.textsNamed("Section 1")
	if len(pinned) == 0 {
		t.Fatal("the heading of the section at the top is not painted")
	}
	top := pinned[len(pinned)-1]
	if top.Y < area.Y-1 || top.Y > area.Y+6 {
		t.Fatalf("Section 1 is pinned at y=%v, the list starts at %v", top.Y, area.Y)
	}
	if got := d.textsNamed("Section 0"); len(got) != 0 {
		t.Fatalf("the heading of the section above is still painted: %v", got)
	}

	// The next heading (row 22) arrives 10 px below the top: the pinned one is
	// pushed up by what it overlaps.
	d.wheel(area, 0, 22*24-10-15*24, 0)
	pushed := d.textsNamed("Section 1")
	if len(pushed) == 0 {
		t.Fatal("the pinned heading went before the next one took its place")
	}
	if got := pushed[len(pushed)-1].Y; got >= top.Y-8 {
		t.Fatalf("Section 1 was not pushed up: y=%v (pinned at %v)", got, top.Y)
	}
	next := d.textsNamed("Section 2")
	if len(next) == 0 || next[0].Y < area.Y+6 {
		t.Fatalf("Section 2 arrives at %v", next)
	}
}

// A heading is not a row: it is no option of the listbox, and the keys, typing,
// choosing all and a Shift range pass over it.
func TestListHeaderIsNotARow(t *testing.T) {
	m := newSectioned([]string{"Bakery", "bagel", "croissant"}, []string{"Dairy", "butter", "cheese"})
	d := openPage(t, m.page)
	d.repaint()
	if d.has("Bakery") || d.has("Dairy") {
		t.Fatal("a heading is a control of the list")
	}

	d.click("croissant")
	d.key(gfx.KeyDown)
	if d.focused() != "butter" {
		t.Fatalf("Down from the last row of a section went to %q, not past the heading to butter", d.focused())
	}
	d.key(gfx.KeyUp)
	if d.focused() != "croissant" {
		t.Fatalf("Up went to %q", d.focused())
	}
	d.key(gfx.KeyHome)
	if d.focused() != "bagel" || m.list.Selected() != 1 {
		t.Fatalf("Home went to %q (row %d), not the first row", d.focused(), m.list.Selected())
	}
	d.key(gfx.KeyUp)
	if d.focused() != "bagel" {
		t.Fatalf("Up from the first row went to %q", d.focused())
	}
	d.key(gfx.KeyEnd)
	if d.focused() != "cheese" {
		t.Fatalf("End went to %q", d.focused())
	}
	d.key(gfx.KeyDown)
	if d.focused() != "cheese" {
		t.Fatalf("Down from the last row went to %q", d.focused())
	}

	// Typing goes round past a heading whose name begins with the letter.
	d.click("butter")
	if _, err := d.driver.ShortcutChord(0, 'b'); err != nil {
		t.Fatal(err)
	}
	if d.focused() != "bagel" {
		t.Fatalf("typing b from butter went to %q, not round to bagel", d.focused())
	}

	// Primary+A and a Shift range choose rows, not headings.
	if _, err := d.driver.ShortcutChord(keymap.Primary, 'a'); err != nil {
		t.Fatal(err)
	}
	if m.sel.Len() != 4 {
		t.Fatalf("Primary+A chose %d, not the four rows", m.sel.Len())
	}
	d.click("bagel")
	d.withModifier(gfx.KeyShiftLeft, func() { d.click("butter") })
	if m.sel.Len() != 3 || !m.sel.Has(1) || !m.sel.Has(2) || !m.sel.Has(4) {
		t.Fatalf("Shift range across a heading chose %d rows", m.sel.Len())
	}
}

// A row the keys move to is scrolled out from under the heading pinned over the
// top of the list.
func TestListHeaderDoesNotCoverTheRowYouMoveTo(t *testing.T) {
	m := manySections()
	d := openPage(t, m.page)
	d.repaint()
	area := d.listArea("Items")
	d.wheel(area, 0, 600, 0) // rows from about 25 on, the top one under the pinned heading
	d.click("2-4")
	for range 4 {
		d.key(gfx.KeyUp)
	}
	if d.focused() != "2-0" {
		t.Fatalf("Up four times from 2-4 went to %q", d.focused())
	}
	d.repaint()
	heading := d.textsNamed("Section 2")
	if len(heading) == 0 {
		t.Fatal("the pinned heading is not painted")
	}
	covered := heading[len(heading)-1].Y + heading[len(heading)-1].Height
	if row := d.hit("2-0").Bounds; row.Y < covered-1 {
		t.Fatalf("row 2-0 at y=%v is under the heading, which ends at %v", row.Y, covered)
	}
}

// A pinned heading takes the pointer from the rows under it, and its rows scroll
// under it without allocating.
func TestListHeaderRepaintAllocates(t *testing.T) {
	m := manySections()
	d := openPage(t, m.page)
	d.repaint()
	d.wheel(d.listArea("Items"), 0, 15*24, 0)
	if allocs := testing.AllocsPerRun(100, d.repaint); allocs != 0 {
		t.Fatalf("%v allocations a repaint", allocs)
	}
}

// --- dragging rows to another place ---

type reordering struct {
	list   ui.ListState
	sel    ui.Selection
	names  []string
	moves  []move
	c      ui.C
	rowFn  func(ui.ListRow)
	clicks int
}

type move struct {
	rows []int
	to   int
}

func newReordering(names ...string) *reordering {
	m := &reordering{names: names}
	m.list.Label = func(i int) string { return m.names[i] }
	m.list.Selection = &m.sel
	m.list.Reorder = func(rows []int, to int) { m.moves = append(m.moves, move{append([]int(nil), rows...), to}) }
	m.rowFn = m.row
	return m
}

func (m *reordering) row(row ui.ListRow) {
	row.Element.Height(24).PadX(8).Children(func() { ui.Text(m.c, m.names[row.Index]) })
}

func (m *reordering) page(c ui.C) {
	m.c = c
	ui.Column(c).Fill().Pad(8).Children(func() {
		ui.List(c, &m.list, len(m.names), m.rowFn).Label("Items").Grow(1)
	})
}

// A row dragged onto the lower half of another goes after it, onto the upper half
// before it; the drop calls Reorder once, with the row and the one it goes before,
// and the release is not also a click.
func TestListReorderDrops(t *testing.T) {
	m := newReordering("a", "b", "c", "d", "e", "f")
	d := openPage(t, m.page)
	d.repaint()
	d.click("a")
	chosenBefore := m.list.Selected()

	target := d.hit("d").Bounds
	d.drag("b", target, 0.9)
	d.release(target, 0.9)
	if len(m.moves) != 1 || fmt.Sprint(m.moves[0]) != "{[1] 4}" {
		t.Fatalf("lower half of d: %v, want [1] before 4", m.moves)
	}
	if m.list.Selected() != chosenBefore {
		t.Fatalf("the drop was also a click: selected %d (was %d)", m.list.Selected(), chosenBefore)
	}

	target = d.hit("d").Bounds
	d.drag("b", target, 0.1)
	d.release(target, 0.1)
	if len(m.moves) != 2 || fmt.Sprint(m.moves[1]) != "{[1] 3}" {
		t.Fatalf("upper half of d: %v, want [1] before 3", m.moves)
	}

	end := d.hit("f").Bounds
	d.drag("c", end, 0.9)
	d.release(end, 0.9)
	if len(m.moves) != 3 || fmt.Sprint(m.moves[2]) != "{[2] 6}" {
		t.Fatalf("below the last row: %v, want [2] before 6", m.moves)
	}
}

// Dropping where the rows already are calls nothing, and a press that does not move
// is the click it always was.
func TestListReorderNoopAndClick(t *testing.T) {
	m := newReordering("a", "b", "c", "d")
	d := openPage(t, m.page)
	d.repaint()
	self := d.hit("b").Bounds
	d.drag("b", self, 0.9) // onto the lower half of itself: before c, where it is
	d.release(self, 0.9)
	if len(m.moves) != 0 {
		t.Fatalf("a drop that moves nothing called Reorder: %v", m.moves)
	}
	d.click("c")
	if m.list.Selected() != 2 {
		t.Fatalf("a click after a drag chose %d", m.list.Selected())
	}
}

// The rows chosen move together when the row dragged is one of them; a row that is
// not chosen moves alone.
func TestListReorderCarriesTheChoice(t *testing.T) {
	m := newReordering("a", "b", "c", "d", "e", "f")
	d := openPage(t, m.page)
	d.repaint()
	d.click("b")
	d.withModifier(primaryKey(), func() { d.click("d") })
	target := d.hit("f").Bounds
	d.drag("d", target, 0.9)
	d.release(target, 0.9)
	if len(m.moves) != 1 || fmt.Sprint(m.moves[0]) != "{[1 3] 6}" {
		t.Fatalf("dragging a chosen row: %v, want [1 3] before 6", m.moves)
	}
	if m.sel.Len() != 2 {
		t.Fatalf("the drag changed the choice: %d chosen", m.sel.Len())
	}

	d.drag("e", d.hit("a").Bounds, 0.1)
	d.release(d.hit("a").Bounds, 0.1)
	if len(m.moves) != 2 || fmt.Sprint(m.moves[1]) != "{[4] 0}" {
		t.Fatalf("dragging a row that is not chosen: %v, want [4] before 0", m.moves)
	}
}

// Escape lets go of the drag, and nothing is dropped.
func TestListReorderEscape(t *testing.T) {
	m := newReordering("a", "b", "c", "d")
	d := openPage(t, m.page)
	d.repaint()
	target := d.hit("d").Bounds
	d.drag("a", target, 0.9)
	d.key(gfx.KeyEscape)
	d.release(target, 0.9)
	if len(m.moves) != 0 {
		t.Fatalf("a drag let go by Escape dropped: %v", m.moves)
	}

	// Back on the row it was pressed on, the release is not a click either.
	a := d.hit("a").Bounds
	d.drag("a", target, 0.9)
	d.key(gfx.KeyEscape)
	if err := d.driver.Move(a.X+a.Width/2, a.Y+a.Height/2); err != nil {
		t.Fatal(err)
	}
	d.release(a, 0.5)
	if m.list.Selected() != -1 || len(m.moves) != 0 {
		t.Fatalf("the release after Escape was a click on a row: selected %d, moves %v", m.list.Selected(), m.moves)
	}
}

// A drag resting at the bottom edge scrolls the list, and frames are asked for only
// while it does.
func TestListReorderScrollsAtTheEdge(t *testing.T) {
	var names []string
	for i := range 100 {
		names = append(names, fmt.Sprintf("row %02d", i))
	}
	m := newReordering(names...)
	d := openPage(t, m.page)
	d.repaint()
	area := d.listArea("Items")
	start := d.hit("row 00").Bounds
	x := start.X + start.Width/2
	if err := d.driver.Press(x, start.Y+4, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if err := d.driver.Move(x, start.Y+40); err != nil {
		t.Fatal(err)
	}
	if err := d.driver.Move(x, area.Y+area.Height-4); err != nil {
		t.Fatal(err)
	}
	if err := d.driver.Advance(500 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if d.has("row 00") {
		t.Fatal("a drag at the bottom edge did not scroll the list")
	}
	if err := d.driver.Move(x, area.Y+area.Height/2); err != nil { // away from the edge
		t.Fatal(err)
	}
	if err := d.driver.Advance(100 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if d.driver.Transitioning() {
		t.Fatal("frames are still asked for with the drag away from the edge")
	}
	if err := d.driver.Release(x, area.Y+area.Height/2, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
}

// A drag repaints without allocating.
func TestListReorderRepaintAllocates(t *testing.T) {
	m := newReordering("a", "b", "c", "d")
	d := openPage(t, m.page)
	d.repaint()
	d.drag("a", d.hit("c").Bounds, 0.9)
	if allocs := testing.AllocsPerRun(100, d.repaint); allocs != 0 {
		t.Fatalf("%v allocations a repaint", allocs)
	}
}

// --- rows wider than the list ---

// widthModel has a name and, after a wide gap, a size in every row.
type widthModel struct {
	list  ui.ListState
	names []string
	c     ui.C
	rowFn func(ui.ListRow)
	head  func(ui.C)
}

func newWidthModel(rowWidth float32, names ...string) *widthModel {
	m := &widthModel{names: names}
	m.list.RowWidth = rowWidth
	m.list.Label = func(i int) string { return m.names[i] }
	m.rowFn = m.row
	return m
}

func (m *widthModel) row(row ui.ListRow) {
	row.Element.Height(24).PadX(8).Children(func() {
		ui.Text(m.c, m.names[row.Index]).Grow(1)
		ui.Text(m.c, "size").Width(60).TextEnd()
	})
}

func (m *widthModel) page(c ui.C) {
	m.c = c
	ui.Column(c).Fill().Pad(8).Children(func() {
		if m.head != nil {
			m.head(c)
		}
		ui.List(c, &m.list, len(m.names), m.rowFn).Label("Items").Grow(1)
	})
}

func firstX(d *pageDriver, value string) float32 {
	d.t.Helper()
	got := d.textsNamed(value)
	if len(got) == 0 {
		d.t.Fatalf("%q is not painted", value)
	}
	return got[0].X
}

// Rows wider than the list scroll sideways with the horizontal wheel and with the
// wheel and Shift, within what is left of them; a list wide enough takes neither.
func TestListRowWidthScrollsSideways(t *testing.T) {
	m := newWidthModel(800, "alpha", "beta", "gamma")
	d := openPage(t, m.page)
	d.repaint()
	area := d.listArea("Items")
	x0 := firstX(d, "size")
	if x0 < area.X+700 {
		t.Fatalf("the size is at %v: the rows are not 800 px wide, the size at their end", x0)
	}

	d.wheel(area, 100, 0, 0)
	if got := firstX(d, "size"); got != x0-100 || m.list.ScrollX() != 100 {
		t.Fatalf("after 100 px: size at %v (was %v), ScrollX %v", got, x0, m.list.ScrollX())
	}
	d.wheel(area, 0, 30, gfx.ModifierShift)
	if got := m.list.ScrollX(); got != 130 {
		t.Fatalf("Shift with the wheel: ScrollX %v, want 130", got)
	}
	if !d.has("alpha") {
		t.Fatal("Shift with the wheel scrolled the list down")
	}
	d.wheel(area, 100000, 0, 0)
	if room := 800 - area.Width; m.list.ScrollX() != room {
		t.Fatalf("scrolled to the end: ScrollX %v, want %v", m.list.ScrollX(), room)
	}
	d.wheel(area, -100000, 0, 0)
	if m.list.ScrollX() != 0 {
		t.Fatalf("scrolled back: ScrollX %v", m.list.ScrollX())
	}

	// Rows no wider than the list leave nothing to scroll.
	narrow := newWidthModel(300, "alpha", "beta")
	dn := openPage(t, narrow.page)
	dn.repaint()
	nArea := dn.listArea("Items")
	before := firstX(dn, "size")
	if before < nArea.X+nArea.Width-80 {
		t.Fatalf("rows narrower than the list did not grow to its width: size at %v, list %v", before, nArea)
	}
	dn.wheel(nArea, 100, 0, 0)
	if narrow.list.ScrollX() != 0 || firstX(dn, "size") != before {
		t.Fatalf("a list wide enough scrolled: ScrollX %v", narrow.list.ScrollX())
	}
}

// Widening the window gives the rows room: the offset is kept within what is left to
// scroll, instead of leaving the end of the rows short of the list's edge.
func TestListRowWidthOffsetFollowsTheWindow(t *testing.T) {
	m := newWidthModel(800, "alpha", "beta")
	d := openPage(t, m.page)
	d.repaint()
	d.wheel(d.listArea("Items"), 100000, 0, 0) // to the end
	if err := d.driver.Resize(700, 480); err != nil {
		t.Fatal(err)
	}
	d.repaint()
	d.repaint()
	area := d.listArea("Items")
	if room := 800 - area.Width; m.list.ScrollX() > room+0.5 {
		t.Fatalf("ScrollX %v is past the %v left to scroll in a list %v wide", m.list.ScrollX(), room, area.Width)
	}
	if end := firstX(d, "size"); end < area.X+area.Width-80 {
		t.Fatalf("the rows end short of the list: size at %v, list %v", end, area)
	}
}

// A header above the list goes with the rows, every column of it, and what is outside
// the list does not move; the rows' pointer regions stay inside the list.
func TestListRowWidthHeaderFollowsAndHitsStayInside(t *testing.T) {
	m := newWidthModel(800, "alpha", "beta")
	m.head = func(c ui.C) {
		ui.Row(c).Clip().Height(24).Children(func() {
			ui.Row(c).Width(800).PadX(8).ScrollWith(&m.list).Children(func() {
				ui.Text(c, "Name header").Grow(1)
				ui.Text(c, "End header").Width(60).TextEnd()
			})
		})
	}
	d := openPage(t, m.page)
	d.repaint()
	area := d.listArea("Items")
	h0 := firstX(d, "Name header")
	e0 := firstX(d, "End header")
	s0 := firstX(d, "size")
	d.wheel(area, 90, 0, 0)
	if got := firstX(d, "Name header"); got != h0-90 {
		t.Fatalf("the header at %v, want %v: it did not follow the rows", got, h0-90)
	}
	// A header column aligned to the end must follow too: a negative margin widens the
	// header row instead of moving it, which leaves its end columns behind.
	if got := firstX(d, "End header"); got != e0-90 {
		t.Fatalf("the header's end column at %v, want %v: it did not follow the rows", got, e0-90)
	}
	if got, want := firstX(d, "End header")-firstX(d, "size"), e0-s0; got != want {
		t.Fatalf("the header's end column is %v from the rows' end, want %v", got, want)
	}
	row := d.hit("alpha").Bounds
	if row.X < area.X-0.5 || row.X+row.Width > area.X+area.Width+0.5 {
		t.Fatalf("a row's pointer region %v is outside the list %v", row, area)
	}
}

// The pinned heading goes sideways with the rows, and the rows still repaint
// without allocating.
func TestListRowWidthWithAHeading(t *testing.T) {
	m := manySections()
	m.width = 800
	d := openPage(t, m.page)
	d.repaint()
	area := d.listArea("Items")
	d.wheel(area, 0, 15*24, 0)
	before := d.textsNamed("Section 1")
	if len(before) == 0 {
		t.Fatal("no pinned heading")
	}
	d.wheel(area, 60, 0, 0)
	after := d.textsNamed("Section 1")
	if len(after) == 0 || after[len(after)-1].X != before[len(before)-1].X-60 {
		t.Fatalf("the pinned heading at %v, want %v", after, before[len(before)-1].X-60)
	}
	if allocs := testing.AllocsPerRun(100, d.repaint); allocs != 0 {
		t.Fatalf("%v allocations a repaint", allocs)
	}
}
