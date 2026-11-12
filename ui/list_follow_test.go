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
