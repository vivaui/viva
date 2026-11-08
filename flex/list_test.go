package flex_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

type listModel struct {
	state flex.ListState
	names []string
}

func newListModel(n int) *listModel {
	m := &listModel{names: make([]string, n)}
	for i := range m.names {
		m.names[i] = "row " + strconv.Itoa(i)
	}
	return m
}

func (m *listModel) view(c flex.C) {
	flex.Column(c).Fill().Children(func() {
		flex.List(c, &m.state, len(m.names), func(i int) {
			flex.Row(c).Pad(4).Children(func() {
				flex.Text(c, m.names[i])
				flex.Button(c, "Open")
			})
		}).Grow(1)
	})
}

func paintedTexts(canvas *ui.Canvas) map[string]bool {
	seen := map[string]bool{}
	for _, text := range canvas.Texts() {
		seen[text.Value] = true
	}
	return seen
}

// A list of a hundred thousand rows declares only the rows on screen.
func TestListDeclaresOnlyVisibleRows(t *testing.T) {
	m := newListModel(100_000)
	page, canvas := flex.NewPage(flex.PageConfig{Theme: &pageTheme}), benchCanvas()
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
	page, canvas := flex.NewPage(flex.PageConfig{Theme: &pageTheme}), benchCanvas()
	view := func(c flex.C) {
		flex.Column(c).Fill().Children(func() {
			flex.List(c, &m.state, len(m.names), func(i int) {
				row := flex.Column(c).Pad(4).Children(func() { flex.Text(c, m.names[i]) })
				if i%3 == 0 {
					row.Height(80)
				}
			}).Grow(1)
		})
	}
	drawPage(page, canvas, view)
	for range 40 {
		page.Wheel(ui.ScrollEvent{X: 100, Y: 100, DeltaY: 2000, Precise: true})
		drawPage(page, canvas, view)
	}
	rows := map[int]ui.Rect{}
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

func fmtSscan(value string, i *int) (int, error) {
	n, err := strconv.Atoi(strings.TrimPrefix(value, "row "))
	*i = n
	if err != nil || !strings.HasPrefix(value, "row ") {
		return 0, strconv.ErrSyntax
	}
	return 1, nil
}

func BenchmarkList100k(b *testing.B) {
	m := newListModel(100_000)
	page, canvas := flex.NewPage(flex.PageConfig{Theme: &pageTheme}), benchCanvas()
	drawPage(page, canvas, m.view)
	b.ReportAllocs()
	for b.Loop() {
		drawPage(page, canvas, m.view)
	}
}
