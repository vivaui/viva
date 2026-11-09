package components

import (
	"slices"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/input"
)

var tabsTestOptions = [...]TabOption{
	{ID: gfx.Item(31), Label: "Overview"},
	{ID: gfx.Item(32), Label: "Analytics"},
	{ID: gfx.Item(33), Label: "Reports", Disabled: true},
}

func TestTabsPaintActiveIndicatorAndStableHits(t *testing.T) {
	theme := gfx.LightTheme()
	var canvas gfx.Canvas
	BeginTabs(&canvas, theme, gfx.Rect{Width: 360}, tabsTestOptions[:], gfx.Item(32)).Line().Paint()
	if len(canvas.Rectangles()) != 7 || len(canvas.Texts()) != 3 {
		t.Fatalf("tab commands = %d rectangles, %d texts", len(canvas.Rectangles()), len(canvas.Texts()))
	}
	if canvas.Rectangles()[2].Color.Alpha != 0 {
		t.Fatalf("inactive indicator is visible: %+v", canvas.Rectangles()[2])
	}
	if canvas.Rectangles()[4].Color != theme.Foreground {
		t.Fatalf("active indicator = %+v, want foreground", canvas.Rectangles()[4].Color)
	}
	if len(canvas.Hits()) != 3 || canvas.Hits()[1].Role != gfx.RoleTab || !canvas.Hits()[1].Selected || !canvas.Hits()[2].Disabled {
		t.Fatalf("tab hits = %+v", canvas.Hits())
	}
	if canvas.Hits()[0].Bounds.Width != canvas.Hits()[1].Bounds.Width || canvas.Hits()[0].Bounds.Width*3 >= 360 {
		t.Fatalf("line tabs should split the list equally inside its padding: %+v", canvas.Hits())
	}
	// The indicator sits just under the active trigger, not a full list height below it.
	if gap := canvas.Rectangles()[4].Bounds.Y - (canvas.Hits()[1].Bounds.Y + canvas.Hits()[1].Bounds.Height); gap < 0 || gap > 6 {
		t.Fatalf("indicator is %v below its trigger", gap)
	}
}

func TestTabsDefaultAndVerticalVariants(t *testing.T) {
	theme := gfx.LightTheme()
	var defaultCanvas gfx.Canvas
	BeginTabs(&defaultCanvas, theme, gfx.Rect{Width: 300}, tabsTestOptions[:], gfx.Item(31)).Paint()
	if defaultCanvas.Rectangles()[0].Color != theme.Muted || defaultCanvas.Rectangles()[0].Bounds.Height != 36 || defaultCanvas.Rectangles()[1].Color != theme.Background || defaultCanvas.Rectangles()[2].Color != theme.Background {
		t.Fatalf("default tabs surfaces = %+v", defaultCanvas.Rectangles()[:2])
	}
	var verticalCanvas gfx.Canvas
	BeginTabs(&verticalCanvas, theme, gfx.Rect{Width: 140, Height: 96}, tabsTestOptions[:], gfx.Item(31)).Line().Vertical().Paint()
	indicator := verticalCanvas.Rectangles()[2]
	trigger := verticalCanvas.Hits()[0].Bounds
	if indicator.Bounds.Width != 2 || indicator.Bounds.X <= trigger.X+trigger.Width {
		t.Fatalf("vertical active indicator = %+v beside trigger %+v", indicator, trigger)
	}
}

func TestTabsDarkActiveTriggerIsLiftedAndBordered(t *testing.T) {
	theme := gfx.DarkTheme()
	var canvas gfx.Canvas
	BeginTabs(&canvas, theme, gfx.Rect{Width: 300}, tabsTestOptions[:], gfx.Item(31)).Paint()
	border, fill := canvas.Rectangles()[1], canvas.Rectangles()[2]
	if border.Color == theme.Muted || fill.Color == theme.Muted || border.Color == fill.Color {
		t.Fatalf("dark active trigger = border %+v fill %+v on %+v", border.Color, fill.Color, theme.Muted)
	}
}

func TestTabsFitSizesListToLabels(t *testing.T) {
	theme := gfx.LightTheme()
	options := [...]TabOption{{ID: gfx.Item(1), Label: "A"}, {ID: gfx.Item(2), Label: "A much longer label"}}
	var canvas gfx.Canvas
	result := BeginTabs(&canvas, theme, gfx.Rect{X: 10, Y: 20}, options[:], gfx.Item(1)).Fit().Paint()
	hits := canvas.Hits()
	if len(hits) != 2 || hits[1].Bounds.Width <= hits[0].Bounds.Width*2 {
		t.Fatalf("fit triggers should follow label widths: %+v", hits)
	}
	list := result.Bounds()
	if list.X != 10 || list.Width <= hits[0].Bounds.Width+hits[1].Bounds.Width || hits[1].Bounds.X+hits[1].Bounds.Width >= list.X+list.Width {
		t.Fatalf("fit list %+v does not wrap triggers %+v", list, hits)
	}
	if canvas.Rectangles()[0].Bounds != list {
		t.Fatalf("list surface %+v != reported bounds %+v", canvas.Rectangles()[0].Bounds, list)
	}
}

func TestTabsRepeatedPaintIsStableAndAllocationFree(t *testing.T) {
	theme := gfx.DarkTheme()
	var canvas gfx.Canvas
	paint := func() {
		canvas.Reset()
		BeginTabs(&canvas, theme, gfx.Rect{Width: 360}, tabsTestOptions[:], gfx.Item(31)).Paint()
	}
	paint()
	rectangles := slices.Clone(canvas.Rectangles())
	texts := slices.Clone(canvas.Texts())
	hits := slices.Clone(canvas.Hits())
	paint()
	if !slices.Equal(rectangles, canvas.Rectangles()) || !slices.Equal(texts, canvas.Texts()) || !slices.Equal(hits, canvas.Hits()) {
		t.Fatal("identical tabs produced different command streams")
	}
	if allocations := testing.AllocsPerRun(100, paint); allocations != 0 {
		t.Fatalf("tabs paint allocations = %v", allocations)
	}
}

func TestTabsKeyDown(t *testing.T) {
	options := [...]TabOption{
		{ID: gfx.Item(401)},
		{ID: gfx.Item(402), Disabled: true},
		{},
		{ID: gfx.Item(404)},
		{ID: gfx.Item(405)},
	}
	tests := []struct {
		name        string
		current     uint32
		orientation TabsOrientation
		direction   Direction
		key         input.Key
		want        uint32
		handled     bool
	}{
		{name: "horizontal right skips disabled and zero", current: 401, key: input.KeyRight, want: 404, handled: true},
		{name: "horizontal left wraps", current: 401, key: input.KeyLeft, want: 405, handled: true},
		{name: "horizontal rtl right moves backward", current: 401, direction: RTL, key: input.KeyRight, want: 405, handled: true},
		{name: "horizontal rtl left moves forward", current: 401, direction: RTL, key: input.KeyLeft, want: 404, handled: true},
		{name: "horizontal ignores up", current: 404, key: input.KeyUp, want: 404},
		{name: "horizontal ignores down", current: 404, key: input.KeyDown, want: 404},
		{name: "vertical up wraps", current: 401, orientation: TabsVertical, key: input.KeyUp, want: 405, handled: true},
		{name: "vertical down skips disabled and zero", current: 401, orientation: TabsVertical, key: input.KeyDown, want: 404, handled: true},
		{name: "vertical ignores left", current: 404, orientation: TabsVertical, key: input.KeyLeft, want: 404},
		{name: "vertical ignores right", current: 404, orientation: TabsVertical, key: input.KeyRight, want: 404},
		{name: "home", current: 405, key: input.KeyHome, want: 401, handled: true},
		{name: "end", current: 401, orientation: TabsVertical, key: input.KeyEnd, want: 405, handled: true},
		{name: "missing current forward", current: 999, key: input.KeyRight, want: 401, handled: true},
		{name: "missing current backward", current: 999, key: input.KeyLeft, want: 405, handled: true},
		{name: "unowned key", current: 404, key: input.KeyEnter, want: 404},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, handled := TabsKeyDown(gfx.Item(int(test.current)), options[:], test.orientation, test.direction, test.key)
			if got != gfx.Item(int(test.want)) || handled != test.handled {
				t.Fatalf("TabsKeyDown() = (%d, %v), want (%d, %v)", got, handled, test.want, test.handled)
			}
		})
	}
}

func TestTabsKeyDownNoAvailableOptionAndNoAllocations(t *testing.T) {
	options := [...]TabOption{{ID: gfx.Item(401), Disabled: true}, {}, {ID: gfx.Item(403), Disabled: true}}
	if got, handled := TabsKeyDown(gfx.Item(77), options[:], TabsHorizontal, LTR, input.KeyEnd); got != gfx.Item(77) || !handled {
		t.Fatalf("all-disabled End = (%d, %v), want (77, true)", got, handled)
	}
	if got, handled := TabsKeyDown(gfx.Item(77), nil, TabsVertical, LTR, input.KeyUp); got != gfx.Item(77) || !handled {
		t.Fatalf("empty Up = (%d, %v), want (77, true)", got, handled)
	}
	if allocations := testing.AllocsPerRun(1000, func() {
		TabsKeyDown(gfx.Item(31), tabsTestOptions[:], TabsHorizontal, RTL, input.KeyLeft)
	}); allocations != 0 {
		t.Fatalf("TabsKeyDown allocations = %v", allocations)
	}
}

var tabsKeyDownResult gfx.ID

func BenchmarkTabsKeyDown(b *testing.B) {
	var result gfx.ID
	b.ReportAllocs()
	for range b.N {
		result, _ = TabsKeyDown(gfx.Item(31), tabsTestOptions[:], TabsHorizontal, LTR, input.KeyRight)
	}
	tabsKeyDownResult = result
}

func TestTabListNumbersItsOwnTabsAndReportsTheClickedOne(t *testing.T) {
	var tree gfx.Tree
	tabs := tree.Sub(tree.Feature("page"), tree.Name("tabs"))
	labels := []string{"Account", "Password", "Team"}
	var canvas gfx.Canvas

	paint := func() (int, bool) {
		canvas.Reset()
		return BeginTabList(&canvas, gfx.LightTheme(), gfx.Rect{Width: 300, Height: 36}, tabs, labels, 1).Paint().Clicked()
	}
	if _, ok := paint(); ok {
		t.Fatal("an ordinary paint reported a click")
	}
	if len(canvas.Hits()) != 3 {
		t.Fatalf("tab hits = %d, want 3", len(canvas.Hits()))
	}
	for index, hit := range canvas.Hits() {
		if hit.ID != tabs.At(index) || hit.Label != labels[index] {
			t.Fatalf("tab %d = %+v, want item %d %q", index, hit, index, labels[index])
		}
	}
	if !canvas.Hits()[1].Selected || canvas.Hits()[0].Selected {
		t.Fatalf("selected = %v %v, want only the second", canvas.Hits()[0].Selected, canvas.Hits()[1].Selected)
	}

	canvas.BeginEventPass(tabs.At(2))
	index, ok := paint()
	canvas.EndEventPass()
	if !ok || index != 2 {
		t.Fatalf("event pass on tab 2 reported %d, %v", index, ok)
	}
	canvas.BeginEventPass(tree.ID(tree.Feature("page"), "elsewhere"))
	if _, ok := paint(); ok {
		t.Fatal("a click elsewhere was reported as a tab")
	}
	canvas.EndEventPass()
}

func TestTableRowAtReportsOnlyItsOwnRow(t *testing.T) {
	var tree gfx.Tree
	rows := tree.Sub(tree.Feature("page"), tree.Name("rows"))
	columns := []TableColumn{{Width: 100}, {}}
	var canvas gfx.Canvas
	paint := func() (clicked []int) {
		canvas.Reset()
		table := BeginTable(&canvas, gfx.LightTheme(), gfx.Rect{Width: 300, Height: 200}, columns)
		for i := range 3 {
			if table.RowAt(rows, i, []string{"row", "x"}, i == 0) {
				clicked = append(clicked, i)
			}
		}
		return clicked
	}
	if got := paint(); len(got) != 0 {
		t.Fatalf("ordinary paint clicked %v", got)
	}
	canvas.BeginEventPass(rows.At(1))
	got := paint()
	canvas.EndEventPass()
	if len(got) != 1 || got[0] != 1 {
		t.Fatalf("event pass on row 1 clicked %v, want [1]", got)
	}
}
