package ui_test

import (
	"runtime"
	"strconv"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/keymap"
	"github.com/vivaui/viva/ui"
)

// selecting is a list of names the tests click, move through and type into.
type selecting struct {
	list      ui.ListState
	selection ui.Selection
	names     []string
	changed   int
	submitted int
	rowFn     func(ui.ListRow)
	c         ui.C
}

func newSelecting(names ...string) *selecting {
	m := &selecting{names: names, submitted: -1}
	m.list.Label = func(i int) string { return names[i] }
	m.list.Selection = &m.selection
	m.rowFn = m.row
	return m
}

func (m *selecting) row(row ui.ListRow) {
	row.Element.Height(24).PadX(8).Children(func() {
		ui.Text(m.c, m.names[row.Index])
	})
}

func (m *selecting) page(c ui.C) {
	m.c = c
	ui.Column(c).Fill().Pad(8).Children(func() {
		ui.List(c, &m.list, len(m.names), m.rowFn).Label("Names").Grow(1)
	})
	if m.list.Changed() {
		m.changed++
	}
	if i, ok := m.list.Submitted(); ok {
		m.submitted = i
	}
}

// withModifier holds a key down around fn, as a person does.
func (d *pageDriver) withModifier(key gfx.Key, fn func()) {
	d.t.Helper()
	if err := d.driver.KeyDown(key, ""); err != nil {
		d.t.Fatal(err)
	}
	fn()
	if err := d.driver.KeyUp(key, ""); err != nil {
		d.t.Fatal(err)
	}
}

// Command on macOS, Control elsewhere: the click that adds a row to the choice,
// and the key that moves without choosing.
func primaryKey() gfx.Key {
	if runtime.GOOS == "darwin" {
		return gfx.KeyMetaLeft
	}
	return gfx.KeyControlLeft
}

// A click chooses a row and focuses it; Changed reports it once; the row is an
// option of a listbox with its position in it.
func TestListClickChoosesAndReports(t *testing.T) {
	m := newSelecting("alpha", "beta", "gamma")
	d := openPage(t, m.page)
	d.repaint()
	named := false
	for _, hit := range d.driver.Snapshot().Hits {
		if hit.Container && hit.Role == gfx.RoleListBox && hit.Label == "Names" {
			named = true
		}
	}
	if !named {
		t.Fatal("the list is not a named listbox")
	}
	d.click("beta")
	if m.list.Selected() != 1 || !m.list.Chosen(1) || m.changed != 1 {
		t.Fatalf("click: selected %d, changed %d", m.list.Selected(), m.changed)
	}
	if d.focused() != "beta" {
		t.Fatalf("the click focused %q", d.focused())
	}
	if hit := d.hit("gamma"); hit.Role != gfx.RoleOption || hit.PosInSet != 3 || hit.SetSize != 3 {
		t.Fatalf("gamma is role %v at %d of %d", hit.Role, hit.PosInSet, hit.SetSize)
	}
	d.repaint()
	if m.changed != 1 {
		t.Fatalf("Changed reported again on the next paint: %d", m.changed)
	}
}

// Shift-click chooses the rows from the last chosen one to the clicked one, and
// Primary-click adds a row to the choice or takes it out.
func TestListModifierClicks(t *testing.T) {
	m := newSelecting("alpha", "beta", "gamma", "delta")
	d := openPage(t, m.page)
	d.click("alpha")
	d.withModifier(gfx.KeyShiftLeft, func() { d.click("gamma") })
	if m.list.Selected() != 2 || m.selection.Len() != 3 || !m.selection.Has(0) || !m.selection.Has(1) || !m.selection.Has(2) {
		t.Fatalf("Shift-click: selected %d, %d chosen", m.list.Selected(), m.selection.Len())
	}
	d.withModifier(primaryKey(), func() { d.click("beta") })
	if m.selection.Has(1) || m.selection.Len() != 2 {
		t.Fatalf("Primary-click on beta: %d chosen", m.selection.Len())
	}
	d.withModifier(primaryKey(), func() { d.click("beta") })
	if !m.selection.Has(1) || m.selection.Len() != 3 {
		t.Fatalf("Primary-click again: %d chosen", m.selection.Len())
	}
}

// An embedded page reads the click its host delivers, not a bare one: a List
// painted through gfx.BeginEmbedded still sees the modifier keys of a
// Shift-click and the count of a double click.
func TestEmbeddedListReadsTheClicksModifiers(t *testing.T) {
	m := newSelecting("alpha", "beta", "gamma", "delta")
	embedded := ui.NewPage(ui.PageConfig{View: m.page, Theme: &pageTheme})
	var host, view gfx.Canvas
	tree := host.Tree()
	mount := tree.Sub(tree.Feature("pane"), tree.Name("one"))
	paint := func() {
		host.Reset()
		host.BeginFrame()
		view.BeginEmbedded(&host, mount)
		embedded.Draw(&view, gfx.Rect{Width: 400, Height: 400}, m.page)
		view.EndFrame()
		host.Embed(&view, gfx.Point{}, mount)
		host.EndFrame()
	}
	click := func(label string, info gfx.ClickInfo) {
		t.Helper()
		var id gfx.ID
		for _, hit := range host.Hits() {
			if hit.Label == label && !hit.IsLabel && !hit.ID.IsZero() {
				id = hit.ID
			}
		}
		if id.IsZero() {
			t.Fatalf("no row is labeled %q", label)
		}
		host.BeginEventPass(id, info)
		paint()
		host.EndEventPass()
		paint()
	}
	paint()
	click("alpha", gfx.ClickInfo{})
	click("gamma", gfx.ClickInfo{Modifiers: gfx.ModifierShift, Clicks: 1})
	if m.list.Selected() != 2 || m.selection.Len() != 3 || !m.selection.Has(0) || !m.selection.Has(1) || !m.selection.Has(2) {
		t.Fatalf("Shift-click in an embedded page: selected %d, %d chosen", m.list.Selected(), m.selection.Len())
	}
	click("beta", gfx.ClickInfo{Clicks: 2})
	if m.submitted != 1 {
		t.Fatalf("double click in an embedded page submitted %d", m.submitted)
	}
}

// The arrows move the focus and the choice, Shift extends from the anchor, and
// Primary moves without choosing; Home and End go to the rows.
func TestListKeysMoveAndChoose(t *testing.T) {
	m := newSelecting("alpha", "beta", "gamma", "delta", "epsilon")
	d := openPage(t, m.page)
	d.click("alpha")
	d.key(gfx.KeyDown)
	if d.focused() != "beta" || m.list.Selected() != 1 {
		t.Fatalf("Down: focused %q, selected %d", d.focused(), m.list.Selected())
	}
	d.key(gfx.KeyEnd)
	if d.focused() != "epsilon" || m.list.Selected() != 4 {
		t.Fatalf("End: focused %q, selected %d", d.focused(), m.list.Selected())
	}
	d.key(gfx.KeyHome)
	if d.focused() != "alpha" || m.list.Selected() != 0 {
		t.Fatalf("Home: focused %q, selected %d", d.focused(), m.list.Selected())
	}

	// A plain move chooses one row; Shift extends from it.
	d.key(gfx.KeyDown) // beta
	if err := d.driver.Key(gfx.KeyDown, gfx.ModifierShift); err != nil {
		t.Fatal(err)
	}
	if m.list.Selected() != 2 || m.selection.Len() != 2 || !m.selection.Has(1) || !m.selection.Has(2) {
		t.Fatalf("Shift+Down: selected %d, %d chosen", m.list.Selected(), m.selection.Len())
	}
	// Primary moves the keys without choosing.
	if err := d.driver.Key(gfx.KeyDown, keymap.Primary); err != nil {
		t.Fatal(err)
	}
	if m.list.Selected() != 3 || m.selection.Len() != 2 || m.selection.Has(3) {
		t.Fatalf("Primary+Down: selected %d, %d chosen", m.list.Selected(), m.selection.Len())
	}
}

// Enter submits the row the keys are on; a double click submits the row clicked.
func TestListSubmitted(t *testing.T) {
	m := newSelecting("alpha", "beta", "gamma")
	d := openPage(t, m.page)
	d.click("beta")
	d.key(gfx.KeyEnter)
	if m.submitted != 1 {
		t.Fatalf("Enter submitted %d", m.submitted)
	}
	d.repaint()
	if m.submitted != 1 {
		t.Fatalf("Submitted reported again: %d", m.submitted)
	}
	r := d.hit("gamma").Bounds
	if err := d.driver.DoubleClick(r.X+8, r.Y+4); err != nil {
		t.Fatal(err)
	}
	if m.submitted != 2 {
		t.Fatalf("double click submitted %d", m.submitted)
	}
}

// Typing the first letters of a name moves to that row; the same letter again goes
// to the next row it begins; a pause starts the word anew.
func TestListTypeahead(t *testing.T) {
	m := newSelecting("alpha", "beta", "gamma", "grape", "delta")
	d := openPage(t, m.page)
	d.click("alpha")
	typed := func(letter rune, want string) {
		t.Helper()
		if _, err := d.driver.ShortcutChord(0, letter); err != nil {
			t.Fatal(err)
		}
		if d.focused() != want || m.list.Selected() != indexOfName(m.names, want) {
			t.Fatalf("%q went to %q, selected %d", letter, d.focused(), m.list.Selected())
		}
	}
	typed('g', "gamma")
	typed('g', "grape")
	if err := d.driver.Advance(1e9); err != nil { // a second: a new word
		t.Fatal(err)
	}
	typed('d', "delta")
}

func indexOfName(names []string, name string) int {
	for i, candidate := range names {
		if candidate == name {
			return i
		}
	}
	return -1
}

// A list that chooses several keeps its choice when Space is pressed on a row (it
// must not activate it), and Control+Space toggles it on Linux and Windows.
func TestListSpaceKeepsTheChoice(t *testing.T) {
	m := newSelecting("alpha", "beta", "gamma")
	d := openPage(t, m.page)
	d.click("alpha")
	d.withModifier(primaryKey(), func() { d.click("gamma") })
	chosen, on := m.selection.Len(), m.list.Selected() // alpha and gamma, the keys on gamma
	d.key(gfx.KeySpace)
	if m.selection.Len() != chosen || m.list.Selected() != on {
		t.Fatalf("Space changed the choice: %d chosen (was %d), selected %d (was %d)", m.selection.Len(), chosen, m.list.Selected(), on)
	}
	if runtime.GOOS == "darwin" {
		return // Control+Space opens Spotlight on macOS
	}
	if err := d.driver.Key(gfx.KeySpace, gfx.ModifierControl); err != nil {
		t.Fatal(err)
	}
	if m.selection.Has(2) || m.selection.Len() != chosen-1 {
		t.Fatalf("Control+Space did not take gamma out: %d chosen", m.selection.Len())
	}
}

// Primary+A chooses every row.
func TestListSelectAll(t *testing.T) {
	m := newSelecting("alpha", "beta", "gamma", "delta")
	d := openPage(t, m.page)
	d.click("beta")
	if _, err := d.driver.ShortcutChord(keymap.Primary, 'a'); err != nil {
		t.Fatal(err)
	}
	if m.selection.Len() != 4 {
		t.Fatalf("Primary+A chose %d of 4", m.selection.Len())
	}
}

// ScrollTo, ScrollIntoView and ScrollToEnd move the list in the one paint that applies
// them, without waiting for another, and Visible reports what it showed.
func TestListScrollToARow(t *testing.T) {
	var names []string
	for i := range 1000 {
		names = append(names, "row "+strconv.Itoa(i))
	}
	m := newSelecting(names...)
	d := openPage(t, m.page)
	d.repaint()
	if first, last := m.list.Visible(); first != 0 || last < 0 {
		t.Fatalf("at the top, Visible is %d..%d", first, last)
	}
	m.list.ScrollTo(900, ui.ListStart)
	d.repaint()
	if !d.has("row 900") {
		t.Fatalf("ScrollTo(900) did not show it: Visible %v", visibleOf(&m.list))
	}
	m.list.ScrollToEnd()
	d.repaint()
	if !m.list.AtEnd() || !d.has("row 999") {
		t.Fatalf("ScrollToEnd: at end %v, Visible %v", m.list.AtEnd(), visibleOf(&m.list))
	}
	m.list.ScrollIntoView(10)
	d.repaint()
	if !d.has("row 10") {
		t.Fatalf("ScrollIntoView(10) did not show it: Visible %v", visibleOf(&m.list))
	}
	// The middle of the last row is past where the list can scroll to: the list stops at
	// its end, in the same paint.
	m.list.ScrollTo(999, ui.ListCenter)
	d.repaint()
	if !d.has("row 999") || !d.has("row 990") {
		t.Fatalf("ScrollTo(999, ListCenter) did not show the end: Visible %v", visibleOf(&m.list))
	}
}

func visibleOf(s *ui.ListState) [2]int {
	first, last := s.Visible()
	return [2]int{first, last}
}

// Choosing a row repaints without allocating.
func TestListSelectionRepaintAllocates(t *testing.T) {
	m := newSelecting("alpha", "beta", "gamma")
	d := openPage(t, m.page)
	d.click("beta")
	if allocs := testing.AllocsPerRun(100, d.repaint); allocs != 0 {
		t.Fatalf("%v allocations a repaint", allocs)
	}
}

// The rows of a list are one stop of Tab: it reaches the row the keys are on, and
// the next Tab leaves the list.
func TestListIsOneTabStop(t *testing.T) {
	m := newSelecting("alpha", "beta", "gamma")
	d := openPage(t, func(c ui.C) {
		ui.Button(c, "before")
		m.page(c)
		ui.Button(c, "after")
	})
	d.click("beta")
	d.click("before")
	d.key(gfx.KeyTab)
	if d.focused() != "beta" {
		t.Fatalf("Tab into the list reached %q, not the row the keys are on", d.focused())
	}
	d.key(gfx.KeyTab)
	if d.focused() != "after" {
		t.Fatalf("Tab out of the list reached %q: the other rows are stops too", d.focused())
	}
}
