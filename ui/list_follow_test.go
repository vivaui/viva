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
	Scroll *ui.ScrollState
}

func (m *followRowsModel) page(c ui.C) {
	ui.Column(c).Fill().Children(func() {
		list := ui.List(c, &m.rows.list, m.rows.n, func(row ui.ListRow) {
			row.Element.Height(m.rows.height(row.Index)).Children(func() {
				ui.Text(row.C, "row "+strconv.Itoa(row.Index))
			})
		}).Label("Rows")
		m.Scroll = list.ScrollState()
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
	m.Scroll.Follow(false) // the user grabs the thumb
	m.follow = false
	before := m.Scroll.Offset
	m.rows.n += 20
	d.repaint()
	if m.Scroll.Offset != before {
		t.Fatalf("rows that arrived after the drag began moved the offset from %v to %v", before, m.Scroll.Offset)
	}
	m.follow = true
	d.repaint()
	if !m.rows.list.AtEnd() {
		t.Fatalf("following again did not return to the end: offset %v max %v", m.Scroll.Offset, m.Scroll.Max())
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

// ScrollTo on a List that follows its end leaves the end, as a scroll by the user
// does: the request is applied when the list is declared, before the Box it returns
// says Follow(true) again, and the rows that arrive after it do not pull the list
// back to the end. ScrollToEnd follows the end again.
func TestListScrollToLeavesTheEndOfAFollowingList(t *testing.T) {
	m := newLogRows(400)
	d := openPage(t, followingRows(m))
	settle := func(step string) {
		t.Helper()
		for frames := 0; d.driver.Transitioning(); frames++ {
			if frames == 10 {
				t.Fatalf("%s: the list asked for ten paints", step)
			}
			if err := d.driver.Advance(16 * time.Millisecond); err != nil {
				t.Fatal(err)
			}
		}
		d.repaint()
	}
	d.repaint()
	settle("start")
	if !m.list.AtEnd() {
		t.Fatalf("the following list did not start at its end (Visible %v)", visibleOf(&m.list))
	}
	const target = 20
	label := "row " + strconv.Itoa(target)
	m.list.ScrollTo(target, ui.ListStart)
	d.repaint()
	settle("ScrollTo")
	if !d.has(label) || m.list.AtEnd() {
		t.Fatalf("after ScrollTo(%d) %s painted %v, at the end %v (Visible %v)", target, label, d.has(label), m.list.AtEnd(), visibleOf(&m.list))
	}
	area := d.ListArea("Rows")
	if top := d.hit(label).Bounds.Y - area.Y; top < -2 || top > 2 {
		t.Fatalf("row %d is %v px from the top of the list, want it at the top", target, top)
	}
	m.n += 5
	d.repaint()
	settle("rows arrived")
	if !d.has(label) || m.list.AtEnd() {
		t.Fatalf("rows that arrived after ScrollTo moved the list: %s painted %v, at the end %v (Visible %v)", label, d.has(label), m.list.AtEnd(), visibleOf(&m.list))
	}
	m.list.ScrollToEnd()
	d.repaint()
	settle("ScrollToEnd")
	m.n += 5
	d.repaint()
	settle("rows arrived at the end")
	last := "row " + strconv.Itoa(m.n-1)
	if !d.has(last) || !m.list.AtEnd() {
		t.Fatalf("after ScrollToEnd the list did not follow its end: %s painted %v, at the end %v (Visible %v)", last, d.has(last), m.list.AtEnd(), visibleOf(&m.list))
	}
}
