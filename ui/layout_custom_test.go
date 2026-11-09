package ui_test

import (
	"fmt"
	"testing"

	"github.com/vivaui/viva/ui"
)

// masonry is the acceptance custom layout: it balances cards of known heights
// across columns, then stacks each column by the heights the page measured.
type masonry struct {
	columns int
	gap     float32
	heights []float32 // the heights its cards want, known by the application

	columnHeight []float32
	columnOf     []int
	placedHeight []float32
}

func newMasonry(columns int, gap float32, heights ...float32) *masonry {
	return &masonry{
		columns: columns, gap: gap, heights: heights,
		columnHeight: make([]float32, columns),
		columnOf:     make([]int, len(heights)),
		placedHeight: make([]float32, columns),
	}
}

func (m *masonry) MeasureWidth(*ui.LayoutContext) float32 { return 0 } // FillWidth decides

func (m *masonry) PlaceWidth(ctx *ui.LayoutContext, bounds ui.Rect) {
	for i := range m.columnHeight {
		m.columnHeight[i] = 0
	}
	width := (bounds.Width - m.gap*float32(m.columns-1)) / float32(m.columns)
	for i := 0; i < ctx.Count(); i++ {
		column := 0
		for c := 1; c < m.columns; c++ {
			if m.columnHeight[c] < m.columnHeight[column] {
				column = c
			}
		}
		m.columnOf[i] = column
		m.columnHeight[column] += m.heights[i] + m.gap
		ctx.SetWidth(i, width)
		ctx.SetX(i, bounds.X+float32(column)*(width+m.gap))
	}
}

func (m *masonry) MeasureHeight(ctx *ui.LayoutContext) float32 {
	for i := range m.columnHeight {
		m.columnHeight[i] = 0
	}
	for i := 0; i < ctx.Count(); i++ {
		column := m.columnOf[i]
		m.columnHeight[column] += ctx.Height(i) + m.gap
	}
	var tallest float32
	for _, height := range m.columnHeight {
		tallest = max(tallest, height-m.gap)
	}
	return tallest
}

func (m *masonry) PlaceHeight(ctx *ui.LayoutContext, bounds ui.Rect) {
	for i := range m.placedHeight {
		m.placedHeight[i] = 0
	}
	for i := 0; i < ctx.Count(); i++ {
		column := m.columnOf[i]
		height := ctx.Height(i)
		ctx.SetHeight(i, height)
		ctx.SetY(i, bounds.Y+m.placedHeight[column])
		m.placedHeight[column] += height + m.gap
	}
}

type masonryModel struct {
	layout    *masonry
	heights   []float32
	labels    []string
	overlay   bool
	container ui.Box
	view      func(ui.C)
}

func newMasonryModel(columns int, heights ...float32) *masonryModel {
	m := &masonryModel{heights: heights, layout: newMasonry(columns, 8, heights...)}
	m.labels = make([]string, len(heights))
	for i := range heights {
		m.labels[i] = fmt.Sprintf("card %d", i)
	}
	m.view = m.page
	return m
}

func (m *masonryModel) page(c ui.C) {
	ui.Column(c).Fill().Pad(10).Children(func() {
		m.container = ui.CustomLayout(c, m.layout).FillWidth()
		m.container.Children(func() {
			for i, height := range m.heights {
				ui.Column(c).Height(height).Hit(ui.ID(c), m.labels[i]).Children(func() {
					ui.Text(c, m.labels[i])
				})
			}
			if m.overlay {
				ui.Row(c).Absolute().Inset(0).Hit(ui.ID(c), "overlay").Children(func() {})
			}
		})
	})
}

// A custom layout measures its children, balances them into columns, and places
// them without the page's flow; identities, hits and dimensions survive.
func TestCustomLayoutMasonry(t *testing.T) {
	m := newMasonryModel(2, 40, 60, 30, 50, 20)
	d := openPage(t, m.view)
	container := m.container.Rect()
	gap := m.layout.gap
	columnWidth := (container.Width - gap) / 2
	if columnWidth <= 0 {
		t.Fatalf("container width %v leaves no card width", container.Width)
	}
	bounds := make([]ui.Rect, len(m.heights))
	for i, label := range m.labels {
		bounds[i] = d.hit(label).Bounds
		if bounds[i].Width < columnWidth-0.5 || bounds[i].Width > columnWidth+0.5 {
			t.Fatalf("%s width %v, want the column width %v", label, bounds[i].Width, columnWidth)
		}
		if bounds[i].X < container.X-0.5 || bounds[i].X+bounds[i].Width > container.X+container.Width+0.5 {
			t.Fatalf("%s at %v is outside the container %v", label, bounds[i], container)
		}
		if bounds[i].Y < container.Y-0.5 || bounds[i].Y+bounds[i].Height > container.Y+container.Height+0.5 {
			t.Fatalf("%s at %v is outside the container %v", label, bounds[i], container)
		}
	}
	// The greedy balance with heights 40, 60, 30, 50, 20 puts cards 0, 2 and 4 in
	// the first column and 1 and 3 in the second.
	for _, pair := range [][2]int{{0, 2}, {2, 4}, {1, 3}} {
		a, b := bounds[pair[0]], bounds[pair[1]]
		if a.X != b.X {
			t.Fatalf("cards %d and %d are in different columns: %v, %v", pair[0], pair[1], a, b)
		}
	}
	if bounds[0].X == bounds[1].X {
		t.Fatalf("cards 0 and 1 did not go to different columns: %v", bounds[0])
	}
	for _, column := range [][]int{{0, 2, 4}, {1, 3}} {
		for j := 1; j < len(column); j++ {
			above, below := bounds[column[j-1]], bounds[column[j]]
			if got := below.Y - (above.Y + above.Height); got < gap-0.5 || got > gap+0.5 {
				t.Fatalf("cards %d and %d are %v apart, want %v", column[j-1], column[j], got, gap)
			}
		}
	}
	// The container is as tall as the tallest column.
	var tallest float32
	for _, b := range bounds {
		tallest = max(tallest, b.Y+b.Height)
	}
	if got := container.Y + container.Height; got < tallest-0.5 || got > tallest+0.5 {
		t.Fatalf("container bottom %v, tallest column %v", got, tallest)
	}
}

// The layout callbacks and the page paint allocate nothing once warm.
func TestCustomLayoutAllocatesNothing(t *testing.T) {
	m := newMasonryModel(3, 40, 60, 30, 50, 20, 70)
	page, canvas := ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas()
	drawPage(page, canvas, m.view)
	if allocs := testing.AllocsPerRun(100, func() { drawPage(page, canvas, m.view) }); allocs != 0 {
		t.Fatalf("a warm custom layout allocates %v times a frame, want 0", allocs)
	}
}

func BenchmarkCustomLayoutMasonry(b *testing.B) {
	m := newMasonryModel(3, 40, 60, 30, 50, 20, 70)
	page, canvas := ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas()
	drawPage(page, canvas, m.view)
	b.ReportAllocs()
	for b.Loop() {
		drawPage(page, canvas, m.view)
	}
}

// An absolute child is the page's, not the layout's: it takes no place in the
// layout's count or height, and fills the container after the layout places the
// in-flow children.
func TestCustomLayoutLeavesAbsoluteChildrenToThePage(t *testing.T) {
	m := newMasonryModel(2, 40, 60, 30)
	m.overlay = true
	d := openPage(t, m.view)
	container := m.container.Rect()
	overlay := d.hit("overlay").Bounds
	if overlay != container {
		t.Fatalf("the absolute child at %v does not fill the container %v", overlay, container)
	}
	if m.container.Rect().Height != 40+8+30 {
		t.Fatalf("the absolute child changed the layout height: %v", m.container.Rect())
	}
}

// A custom layout inside Scroll contributes its content height and scrolls as one.
func TestCustomLayoutInScroll(t *testing.T) {
	m := newMasonryModel(2, 40, 60, 30, 50)
	var area ui.Box
	page := func(c ui.C) {
		ui.Column(c).Fill().Pad(10).Children(func() {
			area = ui.ScrollAt(c, 5).Width(220).Height(50)
			area.Children(func() {
				ui.CustomLayout(c, m.layout).FillWidth().Children(func() {
					for i, height := range m.heights {
						ui.Column(c).Height(height).Hit(ui.ID(c), m.labels[i]).Children(func() {})
					}
				})
			})
		})
	}
	openPage(t, page)
	state := area.ScrollState()
	if state.Content < 90-0.5 {
		t.Fatalf("scroll content %v, want the custom layout's height", state.Content)
	}
	if state.Offset != 5 {
		t.Fatalf("scroll offset %v, want 5", state.Offset)
	}
	if state.Max() <= 0 {
		t.Fatalf("scroll area with content %v and viewport %v does not scroll", state.Content, state.Viewport)
	}
}

// A CustomLayout is not declared in a Memo: its children are not copied declarations.
func TestCustomLayoutInAMemoPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("a CustomLayout in a Memo did not panic")
		}
	}()
	page, canvas := ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas()
	drawPage(page, canvas, func(c ui.C) {
		ui.Memo(c, 1, func() { ui.CustomLayout(c, newMasonry(2, 8, 10)) })
	})
}

// The context refuses a placement on the wrong axis: X is fixed before heights are
// measured, so a layout cannot move it from PlaceHeight.
func TestCustomLayoutRejectsTheWrongPhase(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("a layout setting a height in PlaceWidth did not panic")
		}
	}()
	page, canvas := ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas()
	drawPage(page, canvas, func(c ui.C) {
		ui.CustomLayout(c, wrongPhase{}).Children(func() { ui.Row(c) })
	})
}

type wrongPhase struct{}

func (wrongPhase) MeasureWidth(*ui.LayoutContext) float32 { return 0 }
func (wrongPhase) PlaceWidth(ctx *ui.LayoutContext, _ ui.Rect) {
	ctx.SetHeight(0, 1)
}
func (wrongPhase) MeasureHeight(*ui.LayoutContext) float32 { return 0 }
func (wrongPhase) PlaceHeight(*ui.LayoutContext, ui.Rect)  {}
