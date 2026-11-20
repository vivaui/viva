package buttongroup_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/buttongroup"
)

var groupIcon = gfx.Icon{ViewBox: 24, Stroke: 2, Commands: []gfx.PathCommand{
	gfx.MoveTo(4, 12), gfx.LineTo(20, 12),
}}

type iconGroupModel struct {
	group    buttongroup.Group
	selected int
	ids      [3]gfx.ID
}

func newIconGroupModel() *iconGroupModel {
	var tree gfx.Tree
	scope := tree.Feature("mode-group")
	m := &iconGroupModel{ids: [3]gfx.ID{tree.ID(scope, "source"), tree.ID(scope, "split"), tree.ID(scope, "preview")}, selected: 2}
	m.group.OnClick(func(index int, _ gfx.ID) { m.selected = index })
	return m
}

func (m *iconGroupModel) view(c ui.C) {
	g := &m.group
	g.Reset()
	for i, label := range [...]string{"Source", "Split", "Preview"} {
		g.Item(m.ids[i], label).Icon(groupIcon).IconOnly().Outline()
	}
	g.Selected(m.selected).View(c, "View mode").Size(96, 28)
}

// An icon-only group keeps names, tooltips and caller-owned selection, not
// visible text labels. Its items share edges and stay independently actionable.
func TestIconOnlyItemsKeepNamesAndSelection(t *testing.T) {
	m := newIconGroupModel()
	page, canvas := open(t, m.view)
	check := func() {
		t.Helper()
		hits := actions(canvas)
		if len(hits) != len(m.ids) {
			t.Fatalf("action hits = %d", len(hits))
		}
		for i, h := range hits {
			if h.ID != m.ids[i] || h.Label == "" || h.Hint != h.Label || h.Selected != (i == m.selected) {
				t.Fatalf("item %d: %+v", i, h)
			}
			if i > 0 && !near(hits[i-1].Bounds.Right(), h.Bounds.X) {
				t.Fatal("items do not share their edges")
			}
		}
		if len(canvas.Texts()) != 0 {
			t.Fatal("icon-only items painted visible labels")
		}
		if len(canvas.Triangles()) == 0 {
			t.Fatal("icon-only items painted no glyphs")
		}
	}
	check()
	drawClick(page, canvas, m.view, m.ids[0])
	draw(page, canvas, m.view)
	if m.selected != 0 {
		t.Fatal("the first item did not activate")
	}
	check()
}

func TestIconOnlyGroupWarmPaintAllocatesNothing(t *testing.T) {
	m := newIconGroupModel()
	page, canvas := open(t, m.view)
	for range 3 {
		draw(page, canvas, m.view)
	}
	if n := testing.AllocsPerRun(100, func() { draw(page, canvas, m.view) }); n != 0 {
		t.Fatalf("allocs = %v", n)
	}
}

func BenchmarkIconOnlyViewPaint(b *testing.B) {
	m := newIconGroupModel()
	page, canvas := open(b, m.view)
	b.ReportAllocs()
	for b.Loop() {
		draw(page, canvas, m.view)
	}
}
