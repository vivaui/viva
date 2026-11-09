package components

import (
	"slices"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/input"
)

func TestInputOTPGroupedGeometryAndState(t *testing.T) {
	var canvas gfx.Canvas
	layout := PaintInputOTP(&canvas, gfx.LightTheme(), gfx.Rect{X: 10, Y: 12}, InputOTPSnapshot{
		ID: gfx.Item(8), Label: "Verification code", Value: "12三456", Hint: "Six digits", Slots: 6, GroupSize: 3,
		Focused: true, HasActiveSlot: true, ActiveSlot: 2, ShowCaret: true,
		HasSelection: true, SelectionStart: 0, SelectionEnd: 2,
		HasComposition: true, CompositionStart: 2, CompositionEnd: 3,
	})
	if layout.Bounds != (gfx.Rect{X: 10, Y: 12, Width: 224, Height: 32}) {
		t.Fatalf("bounds = %+v", layout.Bounds)
	}
	if got := layout.SlotBounds(3); got != (gfx.Rect{X: 138, Y: 12, Width: 32, Height: 32}) {
		t.Fatalf("slot 3 = %+v", got)
	}
	// The six characters are text (the active slot's last, so its ring can
	// reach over its neighbors); the separator between the groups is an icon.
	var values []string
	for _, text := range canvas.Texts() {
		values = append(values, text.Value)
	}
	slices.Sort(values)
	if !slices.Equal(values, []string{"1", "2", "4", "5", "6", "三"}) {
		t.Fatalf("texts = %+v", canvas.Texts())
	}
	if len(canvas.Triangles()) == 0 {
		t.Fatal("the separator between the groups painted no icon")
	}
	hit := canvas.Hits()[0]
	if hit.Role != gfx.RoleTextField || hit.Value != "12三456" || hit.Hint != "Six digits" {
		t.Fatalf("hit = %+v", hit)
	}
}

func TestInputOTPDisabledReadOnlySemantics(t *testing.T) {
	var canvas gfx.Canvas
	PaintInputOTP(&canvas, gfx.DarkTheme(), gfx.Rect{}, InputOTPSnapshot{
		ID: gfx.Item(1), Label: "Code", Value: "123", Slots: 4, Disabled: true, ReadOnly: true,
	})
	if hit := canvas.Hits()[0]; !hit.Disabled || !hit.ReadOnly || hit.Focusable {
		t.Fatalf("hit = %+v", hit)
	}
}

func TestInputOTPTypedSemanticsAndCommands(t *testing.T) {
	snapshot := InputOTPSnapshot{
		ID: gfx.Item(3), Label: "Code", Value: "1234", Hint: "Four digits", Slots: 4,
		Focused: true, Invalid: true, HasActiveSlot: true, ActiveSlot: 9,
		HasSelection: true, SelectionStart: 3, SelectionEnd: -2,
		HasComposition: true, CompositionStart: 2, CompositionEnd: 8,
	}
	want := InputOTPSemantics{
		ID: gfx.Item(3), Label: "Code", Value: "1234", Hint: "Four digits", Slots: 4,
		Focused: true, Invalid: true, ActiveSlot: 3, HasActiveSlot: true,
		SelectionStart: 0, SelectionEnd: 3, HasSelection: true,
		CompositionStart: 2, CompositionEnd: 4, HasComposition: true,
	}
	if got := snapshot.Semantics(); got != want {
		t.Fatalf("semantics = %+v, want %+v", got, want)
	}
	commands := [...]struct {
		key  input.Key
		want InputOTPCommand
	}{
		{input.KeyLeft, InputOTPCommandMovePrevious},
		{input.KeyRight, InputOTPCommandMoveNext},
		{input.KeyHome, InputOTPCommandMoveFirst},
		{input.KeyEnd, InputOTPCommandMoveLast},
		{input.KeyBackspace, InputOTPCommandDeleteBackward},
		{input.KeyDelete, InputOTPCommandDeleteForward},
	}
	for _, testCase := range commands {
		if got, handled := InputOTPCommandForKey(snapshot, testCase.key); got != testCase.want || !handled {
			t.Fatalf("key %v = (%v, %v), want %v", testCase.key, got, handled, testCase.want)
		}
	}
	if got, handled := InputOTPCommandForKey(InputOTPSnapshot{ReadOnly: true}, input.KeyLeft); got != InputOTPCommandNone || handled {
		t.Fatalf("read-only left = (%v, %v)", got, handled)
	}
	if allocations := testing.AllocsPerRun(100, func() {
		_ = snapshot.Semantics()
		InputOTPCommandForKey(snapshot, input.KeyLeft)
	}); allocations != 0 {
		t.Fatalf("OTP helpers allocations = %v", allocations)
	}
}

func TestInputOTPDisplayValueAndNativeEditing(t *testing.T) {
	var canvas gfx.Canvas
	PaintInputOTP(&canvas, gfx.LightTheme(), gfx.Rect{}, InputOTPSnapshot{
		ID: gfx.Item(5), Label: "Code", Value: "12", DisplayValue: "••", Slots: 2,
	})
	if texts := canvas.Texts(); len(texts) != 2 || texts[0].Value != "•" || texts[1].Value != "•" {
		t.Fatalf("masked texts = %+v", texts)
	}
	canvas.Reset()
	PaintInputOTP(&canvas, gfx.LightTheme(), gfx.Rect{}, InputOTPSnapshot{
		ID: gfx.Item(5), Label: "Code", Value: "12", DisplayValue: "••", Slots: 2,
		NativeEditing: true, Focused: true, HasActiveSlot: true, ActiveSlot: 1, ShowCaret: true,
	})
	if len(canvas.Texts()) != 0 {
		t.Fatalf("native editing painted duplicate slot text: %+v", canvas.Texts())
	}
	if hits := canvas.Hits(); len(hits) != 1 || hits[0].Value != "12" {
		t.Fatalf("native editing semantics = %+v", hits)
	}
}

func TestInputOTPPaintIsDeterministicAndAllocationFreeAfterWarmup(t *testing.T) {
	var canvas gfx.Canvas
	paint := func() {
		canvas.Reset()
		PaintInputOTP(&canvas, gfx.LightTheme(), gfx.Rect{X: 2, Y: 3}, InputOTPSnapshot{
			ID: gfx.Item(2), Label: "Code", Value: "123456", Slots: 6, GroupSize: 3,
			Focused: true, HasActiveSlot: true, ActiveSlot: 5, ShowCaret: true,
			HasSelection: true, SelectionStart: 1, SelectionEnd: 3,
		})
	}
	paint()
	rectangles := slices.Clone(canvas.Rectangles())
	texts := slices.Clone(canvas.Texts())
	hits := slices.Clone(canvas.Hits())
	paint()
	if !slices.Equal(rectangles, canvas.Rectangles()) || !slices.Equal(texts, canvas.Texts()) || !slices.Equal(hits, canvas.Hits()) {
		t.Fatal("identical OTP snapshots produced different streams")
	}
	if allocations := testing.AllocsPerRun(100, paint); allocations != 0 {
		t.Fatalf("OTP paint allocations = %v", allocations)
	}
}

// visibleAt returns the last rectangle painted over a point: what the user sees.
func visibleAt(canvas *gfx.Canvas, x, y float32) (gfx.Rectangle, bool) {
	var found gfx.Rectangle
	ok := false
	for _, rectangle := range canvas.Rectangles() {
		bounds := rectangle.Bounds
		if rectangle.Color.Alpha > 0 && x >= bounds.X && x < bounds.X+bounds.Width && y >= bounds.Y && y < bounds.Y+bounds.Height {
			found, ok = rectangle, true
		}
	}
	return found, ok
}

func TestInputOTPCaretMarksOnlyAnEmptyActiveSlot(t *testing.T) {
	caretIn := func(snapshot InputOTPSnapshot) bool {
		var canvas gfx.Canvas
		theme := gfx.LightTheme()
		layout := PaintInputOTP(&canvas, theme, gfx.Rect{}, snapshot)
		slot := layout.SlotBounds(snapshot.ActiveSlot)
		for _, rectangle := range canvas.Rectangles() {
			if rectangle.Bounds.Width == 1 && rectangle.Bounds.Height == 16 && rectangle.Color == theme.Foreground &&
				rectangle.Bounds.X > slot.X && rectangle.Bounds.X < slot.X+slot.Width {
				return true
			}
		}
		return false
	}
	base := InputOTPSnapshot{ID: gfx.Item(1), Label: "Code", Slots: 6, GroupSize: 3, Focused: true, HasActiveSlot: true, ShowCaret: true}

	filled := base
	filled.Value, filled.ActiveSlot = "482901", 0
	if caretIn(filled) {
		t.Fatal("a caret was drawn through the digit of a filled slot")
	}
	partial := base
	partial.Value, partial.ActiveSlot = "482", 3
	if !caretIn(partial) {
		t.Fatal("the empty slot that takes the next digit has no caret")
	}
	partial.ActiveSlot = 1
	if caretIn(partial) {
		t.Fatal("a caret was drawn in a slot that already holds a digit")
	}
	hidden := base
	hidden.Value, hidden.ActiveSlot, hidden.ShowCaret = "482", 3, false
	if caretIn(hidden) {
		t.Fatal("ShowCaret=false still drew a caret")
	}
}

func TestInputOTPNeighborsShareOneBorderLine(t *testing.T) {
	theme := gfx.LightTheme()
	var canvas gfx.Canvas
	layout := PaintInputOTP(&canvas, theme, gfx.Rect{X: 10, Y: 12}, InputOTPSnapshot{
		ID: gfx.Item(1), Label: "Code", Value: "482901", Slots: 6, GroupSize: 3,
	})
	border := theme.Input.Over(theme.Background)
	y := layout.Bounds.Y + layout.Bounds.Height/2

	// The border-colored pixel columns across the first group, left to right.
	var columns []int
	group := layout.SlotBounds(0)
	for column := 0; column < 3*int(layout.SlotSize); column++ {
		rectangle, ok := visibleAt(&canvas, group.X+float32(column)+0.5, y)
		if ok && rectangle.Color == border {
			columns = append(columns, column)
		}
	}
	size := int(layout.SlotSize)
	want := []int{0, size - 1, 2*size - 1, 3*size - 1}
	if !slices.Equal(columns, want) {
		t.Fatalf("border lines at columns %v, want exactly one per edge: %v", columns, want)
	}
}

func TestInputOTPOnlyTheOuterCornersOfAGroupAreRounded(t *testing.T) {
	theme := gfx.LightTheme()
	var canvas gfx.Canvas
	layout := PaintInputOTP(&canvas, theme, gfx.Rect{X: 10, Y: 12}, InputOTPSnapshot{
		ID: gfx.Item(1), Label: "Code", Value: "482901", Slots: 6, GroupSize: 3,
	})
	rounded := func(x, y float32) bool {
		rectangle, ok := visibleAt(&canvas, x, y)
		return ok && rectangle.Radius > 0
	}
	slot := func(index int) gfx.Rect { return layout.SlotBounds(index) }
	for group := 0; group < 2; group++ {
		first, middle, last := slot(group*3), slot(group*3+1), slot(group*3+2)
		top, bottom := first.Y+0.5, first.Y+first.Height-0.5
		corners := []struct {
			name string
			x, y float32
			want bool
		}{
			{"group top-left", first.X + 0.5, top, true},
			{"group bottom-left", first.X + 0.5, bottom, true},
			{"first slot top-right", first.X + first.Width - 0.5, top, false},
			{"first slot bottom-right", first.X + first.Width - 0.5, bottom, false},
			{"middle slot top-left", middle.X + 0.5, top, false},
			{"middle slot top-right", middle.X + middle.Width - 0.5, top, false},
			{"middle slot bottom-left", middle.X + 0.5, bottom, false},
			{"last slot top-left", last.X + 0.5, top, false},
			{"last slot bottom-left", last.X + 0.5, bottom, false},
			{"group top-right", last.X + last.Width - 0.5, top, true},
			{"group bottom-right", last.X + last.Width - 0.5, bottom, true},
		}
		for _, corner := range corners {
			if got := rounded(corner.x, corner.y); got != corner.want {
				t.Errorf("group %d %s: rounded = %v, want %v", group, corner.name, got, corner.want)
			}
		}
	}
}

func TestInputOTPActiveSlotBorderIsCompleteOnEverySide(t *testing.T) {
	theme := gfx.LightTheme()
	accent := theme.Primary.Over(theme.Background)
	for active := 0; active < 6; active++ {
		var canvas gfx.Canvas
		layout := PaintInputOTP(&canvas, theme, gfx.Rect{X: 10, Y: 12}, InputOTPSnapshot{
			ID: gfx.Item(1), Label: "Code", Value: "482901", Slots: 6, GroupSize: 3,
			Focused: true, HasActiveSlot: true, ActiveSlot: active,
		})
		slot := layout.SlotBounds(active)
		y := slot.Y + slot.Height/2
		// Inside a group the left edge is the line shared with the slot on the
		// left, which is the column just outside this slot's bounds.
		leftEdge := slot.X - 0.5
		if active%3 == 0 {
			leftEdge = slot.X + 0.5
		}
		probes := map[string][2]float32{
			"left":   {leftEdge, y},
			"right":  {slot.X + slot.Width - 0.5, y},
			"top":    {slot.X + slot.Width/2, slot.Y + 0.5},
			"bottom": {slot.X + slot.Width/2, slot.Y + slot.Height - 0.5},
		}
		for side, point := range probes {
			rectangle, ok := visibleAt(&canvas, point[0], point[1])
			if !ok || rectangle.Color != accent {
				t.Errorf("active slot %d: its %s edge shows %+v, want the accent", active, side, rectangle.Color)
			}
		}
		// The ring must not leak onto the neighbors' outer edges.
		if active > 0 && active%3 != 0 {
			rectangle, _ := visibleAt(&canvas, layout.SlotBounds(active-1).X+0.5, y)
			if rectangle.Color == accent {
				t.Errorf("active slot %d recolored the far edge of its left neighbor", active)
			}
		}
		if active < 5 && (active+1)%3 != 0 {
			rectangle, _ := visibleAt(&canvas, layout.SlotBounds(active+1).X+layout.SlotSize-0.5, y)
			if rectangle.Color == accent {
				t.Errorf("active slot %d recolored the far edge of its right neighbor", active)
			}
		}
	}
}

func TestInputOTPMovingFocusKeepsTheRectangleCountStable(t *testing.T) {
	theme := gfx.LightTheme()
	paint := func(active int) []gfx.Rectangle {
		var canvas gfx.Canvas
		PaintInputOTP(&canvas, theme, gfx.Rect{}, InputOTPSnapshot{
			ID: gfx.Item(1), Label: "Code", Value: "482901", Slots: 6, GroupSize: 3,
			Focused: true, HasActiveSlot: true, ActiveSlot: active,
		})
		return slices.Clone(canvas.Rectangles())
	}
	first, second := paint(1), paint(4)
	if len(first) != len(second) {
		t.Fatalf("moving focus changed the rectangle count from %d to %d", len(first), len(second))
	}
	changed := 0
	for index := range first {
		if first[index] != second[index] {
			changed++
		}
	}
	if changed == 0 {
		t.Fatal("moving focus changed nothing")
	}
}

func TestInputOTPSlotsPastTheTypedCodeAreEmpty(t *testing.T) {
	for _, test := range []struct {
		value string
		slots []string
	}{
		{"", []string{"", "", "", ""}},
		{"4", []string{"4", "", "", ""}},
		{"482", []string{"4", "8", "2", ""}},
		{"4829", []string{"4", "8", "2", "9"}},
		{"4829017", []string{"4", "8", "2", "9"}}, // longer than the slots: the rest is not drawn
		{"你好a", []string{"你", "好", "a", ""}},
	} {
		var canvas gfx.Canvas
		PaintInputOTP(&canvas, gfx.LightTheme(), gfx.Rect{}, InputOTPSnapshot{ID: gfx.Item(1), Label: "Code", Value: test.value, Slots: 4})
		var got []string
		for index := range test.slots {
			got = append(got, inputOTPCharacter(test.value, index))
		}
		if !slices.Equal(got, test.slots) {
			t.Errorf("value %q fills the slots as %q, want %q", test.value, got, test.slots)
		}
		painted := len(canvas.Texts())
		want := 0
		for _, slot := range test.slots {
			if slot != "" {
				want++
			}
		}
		if painted != want {
			t.Errorf("value %q painted %d characters, want %d", test.value, painted, want)
		}
	}
	if inputOTPCharacter("482", -1) != "" {
		t.Error("a negative slot index must be empty")
	}
}

// Focus and invalid change borders only: the active slot's, or every slot's
// when the code is invalid. No ring is painted around either.
func TestInputOTPFocusAndInvalidAddNoRing(t *testing.T) {
	theme := gfx.LightTheme()
	paint := func(snapshot InputOTPSnapshot) gfx.Canvas {
		var canvas gfx.Canvas
		snapshot.ID, snapshot.Label, snapshot.Value, snapshot.Slots, snapshot.GroupSize = gfx.Item(1), "Code", "4829", 6, 3
		PaintInputOTP(&canvas, theme, gfx.Rect{X: 10, Y: 12}, snapshot)
		return canvas
	}
	idle := paint(InputOTPSnapshot{})
	for name, snapshot := range map[string]InputOTPSnapshot{
		"active":  {Focused: true, HasActiveSlot: true, ActiveSlot: 1},
		"invalid": {Invalid: true},
		"both":    {Invalid: true, Focused: true, HasActiveSlot: true, ActiveSlot: 1},
	} {
		if got := paint(snapshot); len(got.Rectangles()) != len(idle.Rectangles()) {
			t.Errorf("%s painted %d rectangles, want the %d of a resting code: the state is a border color", name, len(got.Rectangles()), len(idle.Rectangles()))
		}
	}
}

func TestInputOTPFilterNormalizesTypedAndPastedCodes(t *testing.T) {
	for _, test := range []struct {
		filter InputOTPFilter
		text   string
		slots  int
		want   string
	}{
		{InputOTPDigits, "123-456", 6, "123456"},
		{InputOTPDigits, " 12 34 56 ", 6, "123456"},
		{InputOTPDigits, "1234567890", 6, "123456"},
		{InputOTPDigits, "ab12", 6, "12"},
		{InputOTPDigits, "١٢٣", 6, ""}, // only ASCII digits, like \d
		{InputOTPAlphanumeric, "a1-B2 c3", 6, "a1B2c3"},
		{InputOTPAlphanumeric, "é1", 6, "1"},
		{InputOTPAny, "a b\n-c", 6, "ab-c"},
		{InputOTPAny, "你好a", 2, "你好"},
		{InputOTPDigits, "", 6, ""},
		{InputOTPDigits, "12", 0, "12"}, // no slot count means the default of six
	} {
		if got := test.filter.Normalize(test.text, test.slots); got != test.want {
			t.Errorf("filter %d normalized %q (%d slots) to %q, want %q", test.filter, test.text, test.slots, got, test.want)
		}
	}
	if allocations := testing.AllocsPerRun(100, func() { InputOTPDigits.Normalize("123456", 6) }); allocations != 0 {
		t.Fatalf("normalizing a clean code allocated %v times", allocations)
	}
	if !InputOTPComplete("123456", 6) || InputOTPComplete("12345", 6) || !InputOTPComplete("1234", 4) {
		t.Fatal("InputOTPComplete disagrees with the slot count")
	}
}
