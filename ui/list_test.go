package ui_test

import (
	"strconv"
	"strings"
	"testing"

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
		ui.List(c, &m.state, len(m.names), func(i int) {
			ui.Row(c).Pad(4).Children(func() {
				ui.Text(c, m.names[i])
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
			ui.List(c, &m.state, len(m.names), func(i int) {
				row := ui.Column(c).Pad(4).Children(func() { ui.Text(c, m.names[i]) })
				if i%3 == 0 {
					row.Height(80)
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
	page, canvas := ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas()
	drawPage(page, canvas, m.view)
	b.ReportAllocs()
	for b.Loop() {
		drawPage(page, canvas, m.view)
	}
}
