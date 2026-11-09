package components

import (
	"slices"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/input"
)

var toggleGroupTestOptions = [...]ToggleGroupOption{
	{ID: gfx.Item(51), Label: "B", Active: true},
	{ID: gfx.Item(52), Label: "I"},
	{ID: gfx.Item(53), Label: "U", Active: true, Disabled: true},
}

func TestToggleGroupSupportsMultipleActiveOptions(t *testing.T) {
	theme := gfx.LightTheme()
	var canvas gfx.Canvas
	BeginToggleGroup(&canvas, theme, gfx.Rect{Width: 120}, toggleGroupTestOptions[:]).Paint()
	if len(canvas.Rectangles()) != 3 {
		t.Fatalf("toggle group rectangles = %d, want one surface per spaced item", len(canvas.Rectangles()))
	}
	if canvas.Rectangles()[0].Color != theme.Muted {
		t.Fatalf("first active surface = %+v", canvas.Rectangles()[0].Color)
	}
	if len(canvas.Hits()) != 3 || !canvas.Hits()[0].Checked || !canvas.Hits()[0].Selected || !canvas.Hits()[2].Disabled {
		t.Fatalf("toggle group hits = %+v", canvas.Hits())
	}
	if canvas.Hits()[1].Bounds.X-canvas.Hits()[0].Bounds.X-canvas.Hits()[0].Bounds.Width != 8 {
		t.Fatalf("default toggle group spacing = %+v", canvas.Hits())
	}
}

func TestToggleGroupJoinedOutlineAndVerticalVariants(t *testing.T) {
	theme := gfx.LightTheme()
	var joined gfx.Canvas
	BeginToggleGroup(&joined, theme, gfx.Rect{Width: 120}, toggleGroupTestOptions[:]).Outline().Spacing(0).Paint()
	if len(joined.Rectangles()) != 7 {
		t.Fatalf("joined outline rectangles = %d", len(joined.Rectangles()))
	}
	var vertical gfx.Canvas
	BeginToggleGroup(&vertical, theme, gfx.Rect{Width: 48}, toggleGroupTestOptions[:]).Vertical().Small().Paint()
	if vertical.Hits()[1].Bounds.Y <= vertical.Hits()[0].Bounds.Y || vertical.Hits()[0].Bounds.Height != 28 {
		t.Fatalf("vertical toggle hits = %+v", vertical.Hits())
	}
}

func TestToggleGroupStableAndAllocationFree(t *testing.T) {
	var canvas gfx.Canvas
	theme := gfx.DarkTheme()
	paint := func() {
		canvas.Reset()
		BeginToggleGroup(&canvas, theme, gfx.Rect{Width: 120}, toggleGroupTestOptions[:]).Paint()
	}
	paint()
	rectangles := slices.Clone(canvas.Rectangles())
	texts := slices.Clone(canvas.Texts())
	hits := slices.Clone(canvas.Hits())
	paint()
	if !slices.Equal(rectangles, canvas.Rectangles()) || !slices.Equal(texts, canvas.Texts()) || !slices.Equal(hits, canvas.Hits()) {
		t.Fatal("identical toggle groups produced different command streams")
	}
	if allocations := testing.AllocsPerRun(100, paint); allocations != 0 {
		t.Fatalf("toggle group paint allocations = %v", allocations)
	}
}

func TestToggleGroupKeyDown(t *testing.T) {
	options := [...]ToggleGroupOption{
		{ID: gfx.Item(301)},
		{ID: gfx.Item(302), Disabled: true},
		{},
		{ID: gfx.Item(304)},
		{ID: gfx.Item(305)},
	}
	tests := []struct {
		name        string
		current     uint32
		orientation ToggleGroupOrientation
		direction   Direction
		key         input.Key
		want        uint32
		handled     bool
	}{
		{name: "horizontal right skips disabled and zero", current: 301, key: input.KeyRight, want: 304, handled: true},
		{name: "horizontal left wraps", current: 301, key: input.KeyLeft, want: 305, handled: true},
		{name: "horizontal rtl right moves backward", current: 301, direction: RTL, key: input.KeyRight, want: 305, handled: true},
		{name: "horizontal rtl left moves forward", current: 301, direction: RTL, key: input.KeyLeft, want: 304, handled: true},
		{name: "horizontal ignores up", current: 304, key: input.KeyUp, want: 304},
		{name: "horizontal ignores down", current: 304, key: input.KeyDown, want: 304},
		{name: "vertical up wraps", current: 301, orientation: ToggleGroupVertical, key: input.KeyUp, want: 305, handled: true},
		{name: "vertical down skips disabled and zero", current: 301, orientation: ToggleGroupVertical, key: input.KeyDown, want: 304, handled: true},
		{name: "vertical ignores left", current: 304, orientation: ToggleGroupVertical, key: input.KeyLeft, want: 304},
		{name: "vertical ignores right", current: 304, orientation: ToggleGroupVertical, key: input.KeyRight, want: 304},
		{name: "home", current: 305, key: input.KeyHome, want: 301, handled: true},
		{name: "end", current: 301, orientation: ToggleGroupVertical, key: input.KeyEnd, want: 305, handled: true},
		{name: "missing current forward", current: 999, key: input.KeyRight, want: 301, handled: true},
		{name: "missing current backward", current: 999, key: input.KeyLeft, want: 305, handled: true},
		{name: "unowned key", current: 304, key: input.KeyEnter, want: 304},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, handled := ToggleGroupKeyDown(gfx.Item(int(test.current)), options[:], test.orientation, test.direction, test.key)
			if got != gfx.Item(int(test.want)) || handled != test.handled {
				t.Fatalf("ToggleGroupKeyDown() = (%d, %v), want (%d, %v)", got, handled, test.want, test.handled)
			}
		})
	}
}

func TestToggleGroupKeyDownNoAvailableOptionAndNoAllocations(t *testing.T) {
	options := [...]ToggleGroupOption{{ID: gfx.Item(301), Disabled: true}, {}, {ID: gfx.Item(303), Disabled: true}}
	if got, handled := ToggleGroupKeyDown(gfx.Item(77), options[:], ToggleGroupHorizontal, LTR, input.KeyHome); got != gfx.Item(77) || !handled {
		t.Fatalf("all-disabled Home = (%d, %v), want (77, true)", got, handled)
	}
	if got, handled := ToggleGroupKeyDown(gfx.Item(77), nil, ToggleGroupVertical, LTR, input.KeyDown); got != gfx.Item(77) || !handled {
		t.Fatalf("empty Down = (%d, %v), want (77, true)", got, handled)
	}
	if allocations := testing.AllocsPerRun(1000, func() {
		ToggleGroupKeyDown(gfx.Item(51), toggleGroupTestOptions[:], ToggleGroupHorizontal, RTL, input.KeyLeft)
	}); allocations != 0 {
		t.Fatalf("ToggleGroupKeyDown allocations = %v", allocations)
	}
}

var toggleGroupKeyDownResult gfx.ID

func BenchmarkToggleGroupKeyDown(b *testing.B) {
	var result gfx.ID
	b.ReportAllocs()
	for range b.N {
		result, _ = ToggleGroupKeyDown(gfx.Item(51), toggleGroupTestOptions[:], ToggleGroupHorizontal, LTR, input.KeyRight)
	}
	toggleGroupKeyDownResult = result
}

// Joined, an item is rounded only at the group's outer corners: the items in
// between are square, so neighbouring active items meet without a notch.
func TestToggleGroupJoinedItemsAreRoundedOnlyAtTheGroupsEnds(t *testing.T) {
	var canvas gfx.Canvas
	BeginToggleGroup(&canvas, gfx.LightTheme(), gfx.Rect{Width: 120}, toggleGroupTestOptions[:]).Spacing(0).Paint()
	rectangles := canvas.Rectangles()
	if len(rectangles) != 3 {
		t.Fatalf("joined default group painted %d rectangles, want one surface per item", len(rectangles))
	}
	first, middle, last := rectangles[0], rectangles[1], rectangles[2]
	if first.Radius == 0 || !first.HasClip || middle.Radius != 0 || last.Radius == 0 || !last.HasClip {
		t.Fatalf("joined surfaces = %+v", rectangles)
	}
}

func TestToggleGroupClickedReportsTheClickedOption(t *testing.T) {
	var canvas gfx.Canvas
	canvas.BeginEventPass(gfx.Item(52))
	defer canvas.EndEventPass()
	id, ok := BeginToggleGroup(&canvas, gfx.LightTheme(), gfx.Rect{Width: 120}, toggleGroupTestOptions[:]).Paint().Clicked()
	if !ok || id != gfx.Item(52) {
		t.Fatalf("Clicked() = (%v, %v)", id, ok)
	}
}

func TestToggleGroupSizeToFitSizesItemsToTheirLabels(t *testing.T) {
	options := [...]ToggleGroupOption{
		{ID: gfx.Item(1), Label: "B"},
		{ID: gfx.Item(2), Label: "Underline"},
	}
	var canvas gfx.Canvas
	canvas.SetMeasurer(func(value string, size, weight float32) float32 { return float32(len(value)) * 8 })
	group := BeginToggleGroup(&canvas, gfx.LightTheme(), gfx.Rect{X: 5}, options[:]).SizeToFit()
	group.Paint()
	hits := canvas.Hits()
	// "B" is narrower than the group is tall, so it keeps the square minimum.
	if hits[0].Bounds.Width != 32 || hits[1].Bounds.Width != 9*8+20 {
		t.Fatalf("item widths = %v and %v", hits[0].Bounds.Width, hits[1].Bounds.Width)
	}
	if hits[1].Bounds.X != hits[0].Bounds.X+hits[0].Bounds.Width+8 || group.BoundsWidth() != 32+8+9*8+20 {
		t.Fatalf("item positions %+v, group width %v", hits, group.BoundsWidth())
	}
}
