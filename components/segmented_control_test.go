package components

import (
	"slices"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/input"
)

var segmentedControlTestOptions = [...]SegmentOption{
	{ID: gfx.Item(11), Label: "Day"},
	{ID: gfx.Item(12), Label: "Week"},
	{ID: gfx.Item(13), Label: "Month"},
}

func TestSegmentedControlPaintsStableOptionsAndSelection(t *testing.T) {
	theme := gfx.LightTheme()
	var canvas gfx.Canvas
	BeginSegmentedControl(
		&canvas,
		theme,
		gfx.Rect{X: 6, Y: 8, Width: 180},
		segmentedControlTestOptions[:],
		gfx.Item(12),
	).Paint()

	if len(canvas.Hits()) != 3 || len(canvas.Texts()) != 3 {
		t.Fatalf("hits=%d texts=%d, want one of each per option", len(canvas.Hits()), len(canvas.Texts()))
	}
	for index, hit := range canvas.Hits() {
		option := segmentedControlTestOptions[index]
		if hit.ID != option.ID || hit.Label != option.Label {
			t.Fatalf("hit %d = %+v, want option %+v", index, hit, option)
		}
		if hit.Bounds.Width != 60 || hit.Bounds.Height != theme.Metrics.ControlHeight {
			t.Fatalf("hit %d bounds = %+v", index, hit.Bounds)
		}
	}
	if got := canvas.Rectangles()[3].Color; got != theme.Card {
		t.Fatalf("selected surface = %+v, want neutral Card %+v", got, theme.Card)
	}
	if selected, resting := canvas.Texts()[1].Style, canvas.Texts()[0].Style; selected.Weight != resting.Weight || selected.Size != resting.Size {
		t.Fatalf("selected label style = %+v, want the resting style %+v so selecting does not resize it", selected, resting)
	}
}

func TestSegmentedControlPointerFeedbackKeepsCommandTopology(t *testing.T) {
	theme := gfx.DarkTheme()
	bounds := gfx.Rect{Width: 180, Height: 32}

	paint := func(hovered, pressed uint32) []gfx.Rectangle {
		var canvas gfx.Canvas
		canvas.SetInteraction(gfx.Item(int(hovered)), gfx.Item(int(pressed)), gfx.ID{}, false)
		BeginSegmentedControl(&canvas, theme, bounds, segmentedControlTestOptions[:], gfx.Item(12)).Paint()
		return canvas.Rectangles()
	}
	resting := paint(0, 0)
	hovered := paint(11, 0)
	pressed := paint(0, 11)
	if len(resting) != len(hovered) || len(resting) != len(pressed) {
		t.Fatalf("rectangle topology changed: resting=%d hovered=%d pressed=%d", len(resting), len(hovered), len(pressed))
	}
	if hovered[2].Color != theme.Hover {
		t.Fatalf("hover surface = %+v, want Hover", hovered[2].Color)
	}
	if pressed[2].Color != theme.Border {
		t.Fatalf("pressed surface = %+v, want Border", pressed[2].Color)
	}
}

func TestSegmentedControlDeclaresRadiosAndKeepsDisabledOnesInTheTree(t *testing.T) {
	options := segmentedControlTestOptions
	options[1].Disabled = true
	var partial gfx.Canvas
	BeginSegmentedControl(&partial, gfx.LightTheme(), gfx.Rect{Width: 180}, options[:], gfx.Item(12)).Paint()
	hits := partial.Hits()
	if len(hits) != 3 {
		t.Fatalf("hits = %+v, want one per option", hits)
	}
	for index, hit := range hits {
		if hit.Role != gfx.RoleRadio || hit.Checked != (index == 1) || hit.Selected != (index == 1) {
			t.Fatalf("hit %d = %+v, want a radio checked only for the selected option", index, hit)
		}
	}
	if hits[0].Disabled || !hits[0].Focusable || !hits[1].Disabled || hits[1].Focusable {
		t.Fatalf("disabled flags = %+v", hits)
	}

	var disabled gfx.Canvas
	BeginSegmentedControl(&disabled, gfx.LightTheme(), gfx.Rect{Width: 180}, options[:], gfx.Item(12)).Disabled().Paint()
	if len(disabled.Hits()) != 3 {
		t.Fatalf("disabled control declared %d hits, want 3", len(disabled.Hits()))
	}
	for _, hit := range disabled.Hits() {
		if !hit.Disabled || hit.Focusable {
			t.Fatalf("disabled control hit = %+v", hit)
		}
	}
	for _, text := range disabled.Texts() {
		if text.Style.Color != gfx.LightTheme().MutedForeground {
			t.Fatalf("disabled label color = %+v", text.Style.Color)
		}
	}
}

// The focus ring belongs to the segment that holds the keyboard, not to the whole
// control: it lies within that segment's own bounds, give or take its 3px halo.
func TestSegmentedControlFocusRingIsAroundTheFocusedSegment(t *testing.T) {
	theme := gfx.DarkTheme()
	var canvas gfx.Canvas
	canvas.SetInteraction(gfx.ID{}, gfx.ID{}, gfx.Item(13), true)
	bounds := gfx.Rect{X: 4, Y: 5, Width: 180, Height: 32}
	BeginSegmentedControl(&canvas, theme, bounds, segmentedControlTestOptions[:], gfx.Item(12)).Paint()
	var ring gfx.Rectangle
	for _, rectangle := range canvas.Rectangles() {
		if rectangle.Color == scaledAlpha(theme.Ring, 0.50) {
			ring = rectangle
		}
	}
	segment := canvas.Hits()[2].Bounds
	if ring.Bounds.Width == 0 || ring.Bounds.X <= canvas.Hits()[1].Bounds.X || ring.Bounds.X+ring.Bounds.Width > segment.X+segment.Width+3 {
		t.Fatalf("focus ring %+v is not around the focused segment %+v", ring.Bounds, segment)
	}
}

func TestSegmentedControlClickedReportsTheClickedOption(t *testing.T) {
	var canvas gfx.Canvas
	canvas.BeginEventPass(gfx.Item(13))
	defer canvas.EndEventPass()
	id, ok := BeginSegmentedControl(&canvas, gfx.LightTheme(), gfx.Rect{Width: 180}, segmentedControlTestOptions[:], gfx.Item(12)).Paint().Clicked()
	if !ok || id != gfx.Item(13) {
		t.Fatalf("Clicked() = (%v, %v)", id, ok)
	}
}

func TestSegmentedControlRepeatedPaintIsStable(t *testing.T) {
	var canvas gfx.Canvas
	paint := func() ([]gfx.Rectangle, []gfx.Text, []gfx.Hit) {
		canvas.Reset()
		canvas.SetInteraction(gfx.Item(11), gfx.ID{}, gfx.ID{}, false)
		BeginSegmentedControl(&canvas, gfx.LightTheme(), gfx.Rect{Width: 181}, segmentedControlTestOptions[:], gfx.Item(12)).Paint()
		return slices.Clone(canvas.Rectangles()), slices.Clone(canvas.Texts()), slices.Clone(canvas.Hits())
	}
	firstRectangles, firstTexts, firstHits := paint()
	secondRectangles, secondTexts, secondHits := paint()
	if !slices.Equal(firstRectangles, secondRectangles) ||
		!slices.Equal(firstTexts, secondTexts) ||
		!slices.Equal(firstHits, secondHits) {
		t.Fatal("identical segmented control paints produced different command lists")
	}
}

func TestSegmentedControlHitsUseRuntimeKeyboardActivation(t *testing.T) {
	var canvas gfx.Canvas
	BeginSegmentedControl(&canvas, gfx.LightTheme(), gfx.Rect{Width: 180}, segmentedControlTestOptions[:], gfx.Item(11)).Paint()
	regions := make([]input.Region, len(canvas.Hits()))
	for index, hit := range canvas.Hits() {
		regions[index] = input.Region{Target: input.Target(hit.ID), Focusable: true}
	}
	var router input.Router
	router.SetRegions(regions)
	if !router.Focus(gfx.Item(12)) {
		t.Fatal("runtime could not focus the second segment")
	}
	for _, key := range []input.Key{input.KeyEnter, input.KeySpace} {
		result := router.KeyDown(input.KeyEvent{Key: key})
		if !result.Activated || result.Target != gfx.Item(12) {
			t.Fatalf("key %d result = %+v", key, result)
		}
	}
}

func BenchmarkSegmentedControlPaint(b *testing.B) {
	var canvas gfx.Canvas
	theme := gfx.DarkTheme()
	bounds := gfx.Rect{Width: 240, Height: 32}
	BeginSegmentedControl(&canvas, theme, bounds, segmentedControlTestOptions[:], gfx.Item(12)).Paint()

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		canvas.Reset()
		BeginSegmentedControl(&canvas, theme, bounds, segmentedControlTestOptions[:], gfx.Item(12)).Paint()
	}
}

func TestSegmentedControlKeyDown(t *testing.T) {
	options := [...]SegmentOption{
		{ID: gfx.Item(201)},
		{ID: gfx.Item(202), Disabled: true},
		{},
		{ID: gfx.Item(204)},
		{ID: gfx.Item(205)},
	}
	tests := []struct {
		name      string
		current   uint32
		direction Direction
		key       input.Key
		want      uint32
		handled   bool
	}{
		{name: "right skips disabled and zero", current: 201, key: input.KeyRight, want: 204, handled: true},
		{name: "left wraps", current: 201, key: input.KeyLeft, want: 205, handled: true},
		{name: "rtl right moves backward", current: 201, direction: RTL, key: input.KeyRight, want: 205, handled: true},
		{name: "rtl left moves forward", current: 201, direction: RTL, key: input.KeyLeft, want: 204, handled: true},
		{name: "disabled current advances", current: 202, key: input.KeyRight, want: 204, handled: true},
		{name: "home", current: 205, key: input.KeyHome, want: 201, handled: true},
		{name: "end", current: 201, key: input.KeyEnd, want: 205, handled: true},
		{name: "missing current forward", current: 999, key: input.KeyRight, want: 201, handled: true},
		{name: "missing current backward", current: 999, key: input.KeyLeft, want: 205, handled: true},
		{name: "up is not owned", current: 204, key: input.KeyUp, want: 204},
		{name: "down is not owned", current: 204, key: input.KeyDown, want: 204},
		{name: "enter is not owned", current: 204, key: input.KeyEnter, want: 204},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, handled := SegmentedControlKeyDown(gfx.Item(int(test.current)), options[:], test.direction, test.key)
			if got != gfx.Item(int(test.want)) || handled != test.handled {
				t.Fatalf("SegmentedControlKeyDown() = (%d, %v), want (%d, %v)", got, handled, test.want, test.handled)
			}
		})
	}
}

func TestSegmentedControlKeyDownNoAvailableOptionAndNoAllocations(t *testing.T) {
	options := [...]SegmentOption{{ID: gfx.Item(201), Disabled: true}, {}, {ID: gfx.Item(203), Disabled: true}}
	if got, handled := SegmentedControlKeyDown(gfx.Item(77), options[:], LTR, input.KeyEnd); got != gfx.Item(77) || !handled {
		t.Fatalf("all-disabled End = (%d, %v), want (77, true)", got, handled)
	}
	if got, handled := SegmentedControlKeyDown(gfx.Item(77), nil, LTR, input.KeyLeft); got != gfx.Item(77) || !handled {
		t.Fatalf("empty Left = (%d, %v), want (77, true)", got, handled)
	}
	if allocations := testing.AllocsPerRun(1000, func() {
		SegmentedControlKeyDown(gfx.Item(11), segmentedControlTestOptions[:], RTL, input.KeyLeft)
	}); allocations != 0 {
		t.Fatalf("SegmentedControlKeyDown allocations = %v", allocations)
	}
}

var segmentedControlKeyDownResult gfx.ID

func BenchmarkSegmentedControlKeyDown(b *testing.B) {
	var result gfx.ID
	b.ReportAllocs()
	for range b.N {
		result, _ = SegmentedControlKeyDown(gfx.Item(11), segmentedControlTestOptions[:], LTR, input.KeyRight)
	}
	segmentedControlKeyDownResult = result
}

// The selected choice must read as raised in a dark theme too, where the card
// color is darker than the track and would sink the pill into it.
func TestSegmentedControlSelectedPillIsLighterThanTheTrackInDarkThemes(t *testing.T) {
	for _, test := range []struct {
		name  string
		theme gfx.Theme
	}{
		{"dark", gfx.DarkTheme()},
		{"light", gfx.LightTheme()},
	} {
		var canvas gfx.Canvas
		BeginSegmentedControl(&canvas, test.theme, gfx.Rect{X: 6, Y: 8, Width: 180}, segmentedControlTestOptions[:], gfx.Item(12)).Paint()
		rectangles := canvas.Rectangles()
		// Two for the bordered track, then one pill per option.
		rested, selected := rectangles[2].Color, rectangles[3].Color
		if rested != test.theme.Muted {
			t.Errorf("%s: an unselected choice is %v, want the track %v", test.name, rested, test.theme.Muted)
		}
		if gap := segmentedControlLuminance(selected) - segmentedControlLuminance(rested); gap < 0.03 {
			t.Errorf("%s: selected %v is only %.3f lighter than the track %v", test.name, selected, gap, rested)
		}
	}
}

func TestSegmentedControlWidthKeepsEveryLabelWholeInItsSegment(t *testing.T) {
	theme := gfx.LightTheme()
	options := []SegmentOption{
		{ID: gfx.Item(1), Label: "Off"},
		{ID: gfx.Item(2), Label: "Numbers and the time of day"},
		{ID: gfx.Item(3), Label: "Time"},
	}
	var canvas gfx.Canvas
	width := SegmentedControlWidth(&canvas, theme, options)
	BeginSegmentedControl(&canvas, theme, gfx.Rect{Width: width}, options, options[0].ID).Paint()
	segment := width / float32(len(options))
	for _, text := range canvas.Texts() {
		if measured := canvas.TextWidthWeight(text.Value, theme.Metrics.FontSizeM, gfx.WeightMedium); measured+2*theme.Metrics.PadControlX > segment+0.01 {
			t.Fatalf("%q is %v wide in a segment of %v", text.Value, measured, segment)
		}
	}
	if narrow := SegmentedControlWidth(&canvas, theme, options[:1]); narrow >= width {
		t.Fatalf("one short option needs %v, no less than %v for three", narrow, width)
	}
}
