package ui_test

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

type listModel struct {
	state ui.ListState
	names []string
}

func newListModel(n int) *listModel {
	m := &listModel{names: make([]string, n)}
	for i := range m.names {
		m.names[i] = "row " + strconv.Itoa(i)
	}
	return m
}

func (m *listModel) view(c ui.C) {
	ui.Column(c).Fill().Children(func() {
		ui.List(c, &m.state, len(m.names), func(row ui.ListRow) {
			ui.Row(c).Pad(4).Children(func() {
				ui.Text(c, m.names[row.Index])
				ui.Button(c, "Open")
			})
		}).Grow(1)
	})
}

func paintedTexts(canvas *gfx.Canvas) map[string]bool {
	seen := map[string]bool{}
	for _, text := range canvas.Texts() {
		seen[text.Value] = true
	}
	return seen
}

// A list of a hundred thousand rows declares only the rows on screen.
func TestListDeclaresOnlyVisibleRows(t *testing.T) {
	m := newListModel(100_000)
	page, canvas := ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas()
	for range 2 {
		drawPage(page, canvas, m.view)
	}
	buttons := 0
	for _, hit := range canvas.Hits() {
		if hit.Label == "Open" {
			buttons++
		}
	}
	if buttons == 0 || buttons > 30 {
		t.Fatalf("%d rows declared for a 400 px list", buttons)
	}
	if allocs := testing.AllocsPerRun(20, func() { drawPage(page, canvas, m.view) }); allocs != 0 {
		t.Fatalf("a steady list paint allocates %v times", allocs)
	}
}

// Rows of different heights: scrolled deep into the list, it paints a run of
// consecutive rows that covers the viewport, the ones at that offset.
func TestListScrollsToTheRowsAtTheOffset(t *testing.T) {
	m := newListModel(5_000)
	page, canvas := ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas()
	view := func(c ui.C) {
		ui.Column(c).Fill().Children(func() {
			ui.List(c, &m.state, len(m.names), func(row ui.ListRow) {
				cell := ui.Column(c).Pad(4).Children(func() { ui.Text(c, m.names[row.Index]) })
				if row.Index%3 == 0 {
					cell.Height(80)
				}
			}).Grow(1)
		})
	}
	drawPage(page, canvas, view)
	for range 40 {
		page.Wheel(gfx.ScrollEvent{X: 100, Y: 100, DeltaY: 2000, Precise: true})
		drawPage(page, canvas, view)
	}
	rows := map[int]gfx.Rect{}
	low, high := -1, -1
	for _, text := range canvas.Texts() {
		var i int
		if _, err := fmtSscan(text.Value, &i); err != nil {
			continue
		}
		rows[i] = text.Bounds
		if low < 0 || i < low {
			low = i
		}
		high = max(high, i)
	}
	if low < 100 {
		t.Fatalf("after scrolling the first painted row is %d", low)
	}
	if high-low+1 != len(rows) {
		t.Fatalf("painted rows %d..%d are not consecutive (%d of them)", low, high, len(rows))
	}
	covered := false
	for _, r := range rows {
		if r.Y <= 1 && r.Y+r.Height > 0 || r.Y <= 0 {
			covered = true
		}
	}
	if !covered {
		t.Fatalf("no painted row covers the top of the viewport: %v", rows)
	}
}

// RowHeightAt reports the height the last paint gave each visible row, so an
// application-owned drag can use the same rows the List laid out.
func TestListRowHeightAtReportsLaidOutRows(t *testing.T) {
	heights := []float32{20, 34, 28}
	var state ui.ListState
	page, canvas := ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas()
	drawPage(page, canvas, func(c ui.C) {
		ui.Column(c).Fill().Children(func() {
			ui.List(c, &state, len(heights), func(row ui.ListRow) {
				row.Element.Height(heights[row.Index])
			}).Grow(1)
		})
	})
	for i, want := range heights {
		if got := state.RowHeightAt(i); got != want {
			t.Fatalf("row %d height = %v, want %v", i, got, want)
		}
	}
}

func fmtSscan(value string, i *int) (int, error) {
	n, err := strconv.Atoi(strings.TrimPrefix(value, "row "))
	*i = n
	if err != nil || !strings.HasPrefix(value, "row ") {
		return 0, strconv.ErrSyntax
	}
	return 1, nil
}

// --- a list whose rows are of different heights ---

// logRows is a list of n rows whose heights differ, as the lines of a log are: the
// height of row i comes from a table, so every run lays out the same.
type logRows struct {
	list ui.ListState
	n    int
}

func newLogRows(n int) *logRows {
	m := &logRows{n: n}
	m.list.Label = func(i int) string { return "row " + strconv.Itoa(i) }
	return m
}

func (m *logRows) height(i int) float32 { return float32(24 + (i*37)%53) }

func (m *logRows) page(c ui.C) {
	ui.Column(c).Fill().Children(func() {
		ui.List(c, &m.list, m.n, func(row ui.ListRow) {
			row.Element.Height(m.height(row.Index)).Children(func() {
				ui.Text(row.C, "row "+strconv.Itoa(row.Index))
			})
		}).Label("Rows").Grow(1)
	})
}

// One ScrollTo lands on a row the list has not measured, without the application
// asking again: the list keeps the request while the rows around the target are
// measured, asks for those paints itself, and stops when the offset settles.
func TestListScrollToLandsOnARowOfVaryingHeight(t *testing.T) {
	m := newLogRows(5000)
	d := openPage(t, m.page)
	d.repaint() // the first screen, which is all the list has measured
	const target = 4000
	m.list.ScrollTo(target, ui.ListStart)
	d.repaint() // the one request the application makes
	if !d.driver.Transitioning() {
		t.Fatal("the list did not ask for the paints an estimated target needs")
	}
	for frames := 0; d.driver.Transitioning(); frames++ {
		if frames == 10 {
			t.Fatal("the list asked for ten paints of one request")
		}
		if err := d.driver.Advance(16 * time.Millisecond); err != nil {
			t.Fatal(err)
		}
	}
	d.repaint() // a paint of the settled list: the row is still where it was asked
	label := "row " + strconv.Itoa(target)
	if !d.has(label) {
		t.Fatalf("row %d is not painted: Visible %v", target, visibleOf(&m.list))
	}
	area := d.listArea("Rows")
	if top := d.hit(label).Bounds.Y - area.Y; top < -2 || top > 2 {
		t.Fatalf("row %d is %v px from the top of the list, want it at the top", target, top)
	}
}

// ScrollToEnd settles the same way: the paints that measure the rows near the end
// bring the list there, and the last row is shown at the end of it.
func TestListScrollToEndLandsAtTheEnd(t *testing.T) {
	m := newLogRows(5000)
	d := openPage(t, m.page)
	d.repaint()
	m.list.ScrollToEnd()
	d.repaint()
	if !d.driver.Transitioning() {
		t.Fatal("the list did not ask for the paints an estimated end needs")
	}
	for frames := 0; d.driver.Transitioning(); frames++ {
		if frames == 10 {
			t.Fatal("the list asked for ten paints to reach its end")
		}
		if err := d.driver.Advance(16 * time.Millisecond); err != nil {
			t.Fatal(err)
		}
	}
	d.repaint()
	if !m.list.AtEnd() || !d.has("row 4999") {
		t.Fatalf("ScrollToEnd: at the end %v, Visible %v, the last row painted %v", m.list.AtEnd(), visibleOf(&m.list), d.has("row 4999"))
	}
}

// A request lets go when the user scrolls while it is still settling, instead of
// pulling the list back to the row it was asked to show.
func TestListScrollToLetsGoOfAUserScroll(t *testing.T) {
	m := newLogRows(5000)
	d := openPage(t, m.page)
	d.repaint()
	m.list.ScrollTo(4000, ui.ListStart)
	d.repaint()
	d.wheel(d.listArea("Rows"), 0, -50000, 0) // the user scrolls far up while it settles
	d.repaint()
	d.repaint()
	if d.has("row 4000") {
		t.Fatalf("the list went back to row 4000: the request did not let go of the user's scroll (Visible %v)", visibleOf(&m.list))
	}
}

// --- focus, names and type-ahead ---

// focusRows is a list with a control before and after it, so the focus can leave the
// list; focused is what ListState.HasFocus said in the last paint.
type focusRows struct {
	list    ui.ListState
	names   []string
	focused bool
}

func newFocusRows(names ...string) *focusRows {
	m := &focusRows{names: names}
	m.list.Label = func(i int) string { return m.names[i] }
	return m
}

func (m *focusRows) page(c ui.C) {
	ui.Column(c).Fill().Children(func() {
		ui.Button(c, "before")
		ui.List(c, &m.list, len(m.names), func(row ui.ListRow) {
			row.Element.Height(24).Children(func() { ui.Text(row.C, m.names[row.Index]) })
		}).Label("Rows").Grow(1)
		ui.Button(c, "after")
		m.focused = m.list.HasFocus()
	})
}

// HasFocus says the keyboard is on the list's rows, without a row at hand to ask.
func TestListHasFocus(t *testing.T) {
	m := newFocusRows("alpha", "beta", "gamma")
	d := openPage(t, m.page)
	d.repaint()
	if m.focused || m.list.HasFocus() {
		t.Fatal("a list nothing has focused reports focus")
	}
	d.click("beta")
	if d.focused() != "beta" {
		t.Fatalf("clicking beta focused %q", d.focused())
	}
	if !m.focused || !m.list.HasFocus() {
		t.Fatal("the list whose row has the keyboard does not report focus")
	}
	d.click("after")
	if m.focused || m.list.HasFocus() {
		t.Fatal("the list reports focus with the focus on the button after it")
	}
}

// namedRows names its rows twice: the texts type-ahead matches and the lines
// assistive technology reads.
type namedRows struct {
	list   ui.ListState
	labels []string
	names  []string
}

func newNamedRows() *namedRows {
	m := &namedRows{
		labels: []string{"disk full", "reconnected", "slow"},
		names:  []string{"12:00 ERROR disk full", "12:01 INFO reconnected", "12:02 WARN slow"},
	}
	m.list.Label = func(i int) string { return m.labels[i] }
	m.list.Name = func(i int) string { return m.names[i] }
	return m
}

func (m *namedRows) page(c ui.C) {
	ui.Column(c).Fill().Children(func() {
		ui.List(c, &m.list, len(m.labels), func(row ui.ListRow) {
			row.Element.Height(24).Children(func() { ui.Text(row.C, m.labels[row.Index]) })
		}).Label("Rows").Grow(1)
	})
}

// Name is the row's accessible name, so a row whose spoken name is more than the
// text typing matches has both; NoTypeahead leaves the keys to the page.
func TestListNameAndNoTypeahead(t *testing.T) {
	m := newNamedRows()
	d := openPage(t, m.page)
	d.repaint()
	if !d.has(m.names[0]) {
		t.Fatal("the row is not named by Name")
	}
	if d.has(m.labels[0]) {
		t.Fatal("the row is named by Label although Name is set")
	}

	d.click(m.names[1])
	if _, err := d.driver.ShortcutChord(0, 's'); err != nil {
		t.Fatal(err)
	}
	if d.focused() != m.names[2] || m.list.Selected() != 2 {
		t.Fatalf("typing s went to %q (row %d): Label is no longer the text type-ahead matches", d.focused(), m.list.Selected())
	}

	m.list.NoTypeahead = true
	d.click(m.names[0])
	if _, err := d.driver.ShortcutChord(0, 's'); err != nil {
		t.Fatal(err)
	}
	if d.focused() != m.names[0] || m.list.Selected() != 0 {
		t.Fatalf("typing with NoTypeahead went to %q (row %d)", d.focused(), m.list.Selected())
	}
}

func BenchmarkList100k(b *testing.B) {
	m := newListModel(100_000)
	page, canvas := ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas()
	drawPage(page, canvas, m.view)
	b.ReportAllocs()
	for b.Loop() {
		drawPage(page, canvas, m.view)
	}
}
