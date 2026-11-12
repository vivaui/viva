//go:build darwin

package darwin

import (
	"reflect"
	"testing"

	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
)

func TestCompositeABI(t *testing.T) {
	rectangleSize, rectangleLayerOffset, textSize, textLayerOffset, commandSize := compositeABI()
	if rectangleSize != 80 || rectangleLayerOffset != 36 {
		t.Fatalf("viva_rectangle ABI = size %d layer offset %d, want 80 and 36", rectangleSize, rectangleLayerOffset)
	}
	if textSize != 264 || textLayerOffset != 56 {
		t.Fatalf("viva_text_label ABI = size %d layer offset %d, want 264 and 56", textSize, textLayerOffset)
	}
	if labelSize, runsOffset, runSize := labelABI(); labelSize != 264 || runsOffset != 64 || runSize != 36 {
		t.Fatalf("label ABI = size %d runs offset %d run size %d, want 264, 64 and 36", labelSize, runsOffset, runSize)
	}
	if commandSize != 32 {
		t.Fatalf("viva_composite_command size = %d, want 32", commandSize)
	}
}

func TestTriangleEdgeMaskUsesTheExistingInstanceSlot(t *testing.T) {
	native := makeNativeRectangle(render.Rectangle{Kind: scene.KindTriangle, EdgeMask: 5})
	if got := float32(native.reserved2); got != 6 {
		t.Fatalf("packed triangle marker = %v, want edge mask + 1 = 6", got)
	}
	native = makeNativeRectangle(render.Rectangle{Kind: scene.KindRectangle})
	if got := float32(native.reserved2); got != 0 {
		t.Fatalf("packed rectangle marker = %v, want 0", got)
	}
}

func TestShadedTrianglePacksItsThreeCornerColors(t *testing.T) {
	red, blue := scene.Color{Red: 1, Alpha: 1}, scene.Color{Blue: 1, Alpha: 0.5}
	native := makeNativeRectangle(render.Rectangle{
		Kind: scene.KindTriangle, Color: scene.Color{Green: 1, Alpha: 1},
		Shade: scene.Shade{On: true, Colors: [2]scene.Color{red, blue}},
	})
	if native.green != 1 || native.shade1_red != 1 || native.shade2_blue != 1 || native.shade2_alpha != 0.5 {
		t.Fatalf("packed corner colors = %+v", native)
	}
	flat := makeNativeRectangle(render.Rectangle{Kind: scene.KindTriangle, Color: red})
	if flat.shade1_red != 1 || flat.shade2_red != 1 || flat.shade1_alpha != 1 {
		t.Fatalf("a flat triangle must repeat its color at the other corners: %+v", flat)
	}
}

func TestPlanCompositeOrdersLayersAndPreservesSameLayerOrder(t *testing.T) {
	rectangles := []render.Rectangle{
		{Layer: 2},
		{Layer: 0},
		{Layer: 2},
		{Layer: 2},
		{Layer: 1},
	}
	labels := []TextLabel{
		{Layer: 2},
		{Layer: 0},
		{Layer: 1},
		{Layer: 2},
	}
	got, err := planComposite(rectangles, labels)
	if err != nil {
		t.Fatal(err)
	}
	want := []compositeCommand{
		{Layer: 0, Kind: 0, First: 1, Count: 1},
		{Layer: 0, Kind: 1, First: 1, Count: 1},
		{Layer: 1, Kind: 0, First: 4, Count: 1},
		{Layer: 1, Kind: 1, First: 2, Count: 1},
		{Layer: 2, Kind: 0, First: 0, Count: 1},
		{Layer: 2, Kind: 0, First: 2, Count: 2},
		{Layer: 2, Kind: 1, First: 0, Count: 1},
		{Layer: 2, Kind: 1, First: 3, Count: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("composite plan = %#v, want %#v", got, want)
	}
}

func TestPlanCompositeSplitsRectangleRunsOnClip(t *testing.T) {
	a := scene.Rect{X: 1, Y: 2, Width: 30, Height: 40}
	b := scene.Rect{X: 5, Y: 6, Width: 7, Height: 8}
	rectangles := []render.Rectangle{
		{},                                 // 0 unclipped
		{Clip: b},                          // 1 unclipped, stale clip: merges with 0
		{Clipped: true, Clip: a},           // 2
		{Clipped: true, Clip: a},           // 3 same clip: merges with 2
		{Clipped: true, Clip: b},           // 4 other clip: new run
		{},                                 // 5 unclipped again: new run
		{Layer: 1, Clipped: true, Clip: b}, // 6 other layer
	}
	got, err := planComposite(rectangles, []TextLabel{{Layer: 0}})
	if err != nil {
		t.Fatal(err)
	}
	want := []compositeCommand{
		{Layer: 0, Kind: 0, First: 0, Count: 2},
		{Layer: 0, Kind: 0, First: 2, Count: 2, Clipped: true, Clip: a},
		{Layer: 0, Kind: 0, First: 4, Count: 1, Clipped: true, Clip: b},
		{Layer: 0, Kind: 0, First: 5, Count: 1},
		{Layer: 0, Kind: 1, First: 0, Count: 1},
		{Layer: 1, Kind: 0, First: 6, Count: 1, Clipped: true, Clip: b},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("composite plan = %#v, want %#v", got, want)
	}
}

func TestPlanCompositeEmpty(t *testing.T) {
	got, err := planComposite(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("empty composite plan = %#v", got)
	}
}

// The native composite plan reads each instance's layer from reserved0, and
// the update paths no longer patch it from Go: the staged bytes must carry it
// for both kinds, or every instance silently composites at layer 0.
func TestStagedRectanglesCarryTheirLayer(t *testing.T) {
	rectangle := makeNativeRectangle(render.Rectangle{Layer: 7})
	if got := float32(rectangle.reserved0); got != 7 {
		t.Fatalf("staged rectangle layer = %v, want 7", got)
	}
	triangle := makeNativeRectangle(render.Rectangle{
		Kind: scene.KindTriangle, Layer: 5,
		Points: [3]scene.Point{{X: 1, Y: 2}, {X: 3, Y: 4}, {X: 5, Y: 6}},
	})
	if got := float32(triangle.reserved0); got != 5 {
		t.Fatalf("staged triangle layer = %v, want 5", got)
	}
}

// Label textures hold colors already multiplied by coverage, so text must
// blend with a One source factor. Blending them with SourceAlpha multiplies by
// alpha twice and renders glyphs about a fifth lighter than system text, which
// shows as soft, washed-out type, most visibly on 1x displays.
func TestTextBlendsPremultipliedAndRectanglesDoNot(t *testing.T) {
	if !fragmentBlendsPremultiplied("viva_text_fragment") {
		t.Fatal("text fragments must blend as premultiplied")
	}
	if fragmentBlendsPremultiplied("viva_rectangle_fragment") {
		t.Fatal("rectangle fragments output straight alpha and must blend with SourceAlpha")
	}
	if fragmentBlendsPremultiplied("") {
		t.Fatal("an unknown fragment must not be treated as premultiplied")
	}
}

func TestPlanLabelReuseFollowsContentAcrossIndexes(t *testing.T) {
	for _, test := range []struct {
		name      string
		old, next []uint64
		want      []int64
	}{
		{"nothing changed", []uint64{1, 2, 3}, []uint64{1, 2, 3}, []int64{0, 1, 2}},
		{"one row changed in place", []uint64{1, 2, 3}, []uint64{1, 9, 3}, []int64{0, -1, 2}},
		{"scrolled up by one", []uint64{1, 2, 3, 4}, []uint64{2, 3, 4, 5}, []int64{1, 2, 3, -1}},
		{"scrolled down by two", []uint64{1, 2, 3, 4}, []uint64{8, 9, 1, 2}, []int64{-1, -1, 0, 1}},
		{"list grew", []uint64{1, 2}, []uint64{1, 2, 3, 4}, []int64{0, 1, -1, -1}},
		{"list shrank", []uint64{1, 2, 3, 4}, []uint64{3, 4}, []int64{2, 3}},
		{"first from empty", nil, []uint64{5, 6}, []int64{-1, -1}},
		{"to empty", []uint64{5, 6}, nil, []int64{}},
		{"duplicates are used once each", []uint64{7, 7}, []uint64{7, 7, 7}, []int64{0, 1, -1}},
		{"duplicates shift", []uint64{7, 7, 8}, []uint64{8, 7, 7}, []int64{2, 1, 0}},
		{"the same-index match is taken first", []uint64{4, 4, 5}, []uint64{5, 4, 4}, []int64{2, 1, 0}},
	} {
		got, err := planLabelReuse(test.old, test.next)
		if err != nil {
			t.Fatalf("%s: %v", test.name, err)
		}
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("%s: plan = %v, want %v", test.name, got, test.want)
		}
	}
}

func TestPlanLabelReuseNeverSharesAnOldLabelTwice(t *testing.T) {
	old := make([]uint64, 200)
	next := make([]uint64, 300)
	for i := range old {
		old[i] = uint64(i % 17)
	}
	for i := range next {
		next[i] = uint64((i * 3) % 19)
	}
	source, err := planLabelReuse(old, next)
	if err != nil {
		t.Fatal(err)
	}
	used := map[int64]bool{}
	for index, from := range source {
		if from < 0 {
			continue
		}
		if used[from] {
			t.Fatalf("old label %d reused twice", from)
		}
		used[from] = true
		if old[from] != next[index] {
			t.Fatalf("new label %d took old label %d with a different hash", index, from)
		}
	}
}
