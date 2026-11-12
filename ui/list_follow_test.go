package ui_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/vivaui/viva/ui"
)

// followingRows is a log of rows of varying height in a List that follows its end.
func followingRows(m *logRows) func(ui.C) {
	return func(c ui.C) {
		ui.Column(c).Fill().Children(func() {
			ui.List(c, &m.list, m.n, func(row ui.ListRow) {
				row.Element.Height(m.height(row.Index)).Children(func() {
					ui.Text(row.C, "row "+strconv.Itoa(row.Index))
				})
			}).Label("Rows").Follow(true).Grow(1)
		})
	}
}

// A List that follows its end shows its last rows in the paint that adds rows, even
// when more rows arrive at once than the viewport holds: the rows are placed for the
// end the layout pins the area to, not for the offset the area had before.
func TestListFollowShowsTheEndAsRowsArrive(t *testing.T) {
	m := newLogRows(10)
	d := openPage(t, followingRows(m))
	d.repaint()
	for step := range 8 {
		m.n += 3000
		d.repaint()
		last := "row " + strconv.Itoa(m.n-1)
		if !d.has(last) || !m.list.AtEnd() {
			t.Fatalf("step %d: %s painted %v after %d rows arrived (Visible %v, at the end %v)", step, last, d.has(last), m.n, visibleOf(&m.list), m.list.AtEnd())
		}
	}
}

// followRowsModel is a log whose page declares the follow flag the model holds,
// so a test can stop it between paints as a scrollbar drag does.
type followRowsModel struct {
	rows   *logRows
	follow bool
	scroll *ui.ScrollState
}

func (m *followRowsModel) page(c ui.C) {
	ui.Column(c).Fill().Children(func() {
		list := ui.List(c, &m.rows.list, m.rows.n, func(row ui.ListRow) {
			row.Element.Height(m.rows.height(row.Index)).Children(func() {
				ui.Text(row.C, "row "+strconv.Itoa(row.Index))
			})
		}).Label("Rows")
		m.scroll = list.ScrollState()
		list.Follow(m.follow).Grow(1)
	})
}

// The rows that stream in while a scrollbar drag holds the thumb must not pin
// the list to its end. A List reads whether it follows when it is declared,
// before the Box it returns can say Follow(false), so the gesture stops it on
// the area's state between paints.
func TestListFollowCanBeStoppedBetweenPaints(t *testing.T) {
	m := &followRowsModel{rows: newLogRows(10), follow: true}
	d := openPage(t, m.page)
	d.repaint()
	m.scroll.Follow(false) // the user grabs the thumb
	m.follow = false
	before := m.scroll.Offset
	m.rows.n += 20
	d.repaint()
	if m.scroll.Offset != before {
		t.Fatalf("rows that arrived after the drag began moved the offset from %v to %v", before, m.scroll.Offset)
	}
	m.follow = true
	d.repaint()
	if !m.rows.list.AtEnd() {
		t.Fatalf("following again did not return to the end: offset %v max %v", m.scroll.Offset, m.scroll.Max())
	}
}

// A long List that starts following places its first rows before Follow is
// declared; it asks for the paint that shows its end instead of staying blank.
func TestListFollowReachesTheEndOfALongListItStartsWith(t *testing.T) {
	m := newLogRows(5000)
	d := openPage(t, followingRows(m))
	d.repaint()
	for frames := 0; d.driver.Transitioning(); frames++ {
		if frames == 10 {
			t.Fatal("the list asked for ten paints to reach its end")
		}
		if err := d.driver.Advance(16 * time.Millisecond); err != nil {
			t.Fatal(err)
		}
	}
	d.repaint()
	if !d.has("row 4999") || !m.list.AtEnd() {
		t.Fatalf("the last row painted %v (Visible %v, at the end %v)", d.has("row 4999"), visibleOf(&m.list), m.list.AtEnd())
	}
}
