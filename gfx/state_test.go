package gfx

import "testing"

func TestStateOfReadsTheCanvasForTheControlsIdentity(t *testing.T) {
	a, b := Item(1), Item(2)
	var canvas Canvas
	canvas.SetInteraction(a, a, a, true)
	if got := canvas.StateOf(a); got != (State{Hovered: true, Pressed: true, Focused: true, FocusVisible: true}) {
		t.Errorf("state of the control the canvas points at = %+v", got)
	}
	if got := canvas.StateOf(b); got != (State{}) {
		t.Errorf("state of another control = %+v, want the zero state", got)
	}
	if got := canvas.StateOf(ID{}); got != (State{}) {
		t.Errorf("state of no control = %+v, want the zero state", got)
	}
}

// A click gives focus without making it visible: the ring is for the keyboard.
func TestStateOfFocusedByPointerIsNotFocusVisible(t *testing.T) {
	a := Item(1)
	var canvas Canvas
	canvas.SetInteraction(ID{}, ID{}, a, false)
	if got := canvas.StateOf(a); !got.Focused || got.FocusVisible {
		t.Errorf("state = %+v, want focused and not focus-visible", got)
	}
}

func TestStateOfNilCanvasIsZero(t *testing.T) {
	var canvas *Canvas
	if got := canvas.StateOf(Item(1)); got != (State{}) {
		t.Errorf("state = %+v", got)
	}
}

func TestStateInteracting(t *testing.T) {
	if (State{Checked: true, Invalid: true, Disabled: true}).Interacting() {
		t.Error("declared state is not interaction")
	}
	for _, s := range []State{{Hovered: true}, {Pressed: true}, {Focused: true}} {
		if !s.Interacting() {
			t.Errorf("%+v should be interacting", s)
		}
	}
}

func BenchmarkStateOf(b *testing.B) {
	a := Item(1)
	var canvas Canvas
	canvas.SetInteraction(a, ID{}, a, true)
	b.ReportAllocs()
	for range b.N {
		_ = canvas.StateOf(a)
	}
}

// A fixed-pitch text is every character on a cell, so the style measures of the canvas (width,
// caret, selection, hit test) are whole cells of its face and agree with each other; a style that
// is not fixed-pitch is measured as before.
func TestStyleMeasuresOfAMonoTextAreWholeCellsOfItsFace(t *testing.T) {
	var canvas Canvas
	canvas.SetMeasurer(func(value string, size, weight float32) float32 { return float32(len(value)) * size / 3 })
	canvas.SetCellMeasurer(func(size float32, face MonoFace) (float32, float32) { return size / 2, size })
	mono := TextStyle{Size: 20, Mono: true, MonoFace: MonoClassic}
	const cell = 10

	if got := canvas.TextWidthStyle("héllo", mono); got != 5*cell {
		t.Errorf("width of five characters = %v, want %v (bytes are not cells)", got, 5*cell)
	}
	if got := canvas.TextCaretStyle("héllo", mono, len("hé")); got != 2*cell {
		t.Errorf("caret after two characters = %v, want %v", got, 2*cell)
	}
	spans := canvas.TextSelectionStyle(nil, "héllo", mono, len("h"), len("hél"))
	if len(spans) != 1 || spans[0].X != cell || spans[0].Width != 2*cell {
		t.Errorf("selection of two characters from the second = %+v, want X %v width %v", spans, cell, 2*cell)
	}
	// A press lands on the nearest boundary between cells and never inside a character.
	for _, test := range []struct {
		x    float32
		want int
	}{{0, 0}, {4, 0}, {6, len("h")}, {16, len("hé")}, {1000, len("héllo")}} {
		if got := canvas.TextOffsetAtStyle("héllo", mono, test.x); got != test.want {
			t.Errorf("offset at x=%v = %d, want %d", test.x, got, test.want)
		}
	}
	// Another face has its own cell; a proportional style is unchanged.
	canvas.SetCellMeasurer(func(size float32, face MonoFace) (float32, float32) {
		if face == MonoClassic {
			return size, size
		}
		return size / 2, size
	})
	if got := canvas.TextWidthStyle("ab", mono); got != 2*20 {
		t.Errorf("width in the classic face = %v, want two 20 pixel cells", got)
	}
	if got := canvas.TextWidthStyle("abc", TextStyle{Size: 20}); got != canvas.TextWidthWeight("abc", 20, 0) {
		t.Errorf("a proportional style measures %v, want what TextWidthWeight does", got)
	}
}
