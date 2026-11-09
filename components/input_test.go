package components

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/vivaui/viva/gfx"
)

func TestInputCapturesGeometryDisplayAndSemantics(t *testing.T) {
	theme := gfx.LightTheme()
	var canvas gfx.Canvas
	canvas.SetInteraction(gfx.ID{}, gfx.ID{}, gfx.Item(7), true)
	bounds := PaintInput(&canvas, theme, gfx.Rect{X: 10, Y: 20, Width: 180}, InputSnapshot{
		ID: gfx.Item(7), Label: "Email", Value: "user@example.com", DisplayValue: "user@example.com",
		Hint: "Work address", ReadOnly: true, Invalid: true, ShowCaret: true, Caret: 4,
		SelectionStart: 0, SelectionEnd: 4, CompositionStart: 5, CompositionEnd: 12,
	})
	if bounds != (gfx.Rect{X: 10, Y: 20, Width: 180, Height: 32}) {
		t.Fatalf("bounds = %+v", bounds)
	}
	// Invalid: the destructive border and the fill, then selection, composition
	// underline and caret.
	if len(canvas.Rectangles()) != 5 || canvas.Rectangles()[0].Color != theme.Danger {
		t.Fatalf("rectangles = %+v", canvas.Rectangles())
	}
	hit := canvas.Hits()[0]
	if hit.Role != gfx.RoleTextField || hit.Value != "user@example.com" || hit.Hint != "Work address" || !hit.ReadOnly || !hit.Focusable {
		t.Fatalf("hit = %+v", hit)
	}
	if len(canvas.Texts()) != 2 || canvas.Texts()[0].Value != "user@example.com" {
		t.Fatalf("texts = %+v", canvas.Texts())
	}
	// The line is centered in the field's 20 px text line box, not placed at the
	// field's top edge: the caret and the selection use the same vertical band.
	if got := canvas.Texts()[0].Bounds.Y; got != 26 {
		t.Errorf("the input line starts at y=%v, want the line box's 26", got)
	}
}

// Focus is the same however it arrived (pointer or keyboard): the border turns
// to the accent color. The family contract is in field_focus_test.go; this pins
// that Input honors the canvas's focus.
func TestInputFocusFromThePointerAndTheKeyboardTurnsTheBorder(t *testing.T) {
	theme := gfx.LightTheme()
	bounds := gfx.Rect{X: 10, Y: 20, Width: 180}
	snapshot := InputSnapshot{ID: gfx.Item(7), Label: "Email", Value: "hello@viva.dev"}
	var idle gfx.Canvas
	PaintInput(&idle, theme, bounds, snapshot)
	if idle.Rectangles()[0].Color != theme.Input {
		t.Fatalf("resting border = %+v, want the input token", idle.Rectangles()[0].Color)
	}
	for name, keyboard := range map[string]bool{"pointer": false, "keyboard": true} {
		t.Run(name, func(t *testing.T) {
			var focused gfx.Canvas
			focused.SetInteraction(gfx.ID{}, gfx.ID{}, gfx.Item(7), keyboard)
			PaintInput(&focused, theme, bounds, snapshot)
			if len(focused.Rectangles()) != len(idle.Rectangles()) {
				t.Fatalf("focus painted %d rectangles, want the %d of a resting field", len(focused.Rectangles()), len(idle.Rectangles()))
			}
			if focused.Rectangles()[0].Color != theme.Primary {
				t.Fatalf("focused border = %+v, want the accent color %+v", focused.Rectangles()[0].Color, theme.Primary)
			}
		})
	}
}

// RoundedBorder paints the border color as a solid shape under the fill, so a
// translucent token must be flattened or it tints the whole interior.
func TestInputSurfacePaintsOpaqueBorderAndFill(t *testing.T) {
	for name, theme := range map[string]gfx.Theme{"dark": gfx.DarkTheme(), "light": gfx.LightTheme()} {
		for _, state := range []InputSnapshot{{}, {Focused: true}, {Invalid: true}, {Disabled: true}} {
			var canvas gfx.Canvas
			PaintInput(&canvas, theme, gfx.Rect{Width: 160}, InputSnapshot{
				ID: gfx.Item(1), Label: "x", Value: "v", Focused: state.Focused, Invalid: state.Invalid, Disabled: state.Disabled,
			})
			rectangles := canvas.Rectangles()
			if rectangles[0].Radius != theme.Metrics.RadiusControl {
				rectangles = rectangles[1:] // the halo behind the field
			}
			border, fill := rectangles[0], rectangles[1]
			if border.Color.Alpha != 1 || fill.Color.Alpha != 1 {
				t.Fatalf("%s %+v: border alpha %v, fill alpha %v; both must be opaque", name, state, border.Color.Alpha, fill.Color.Alpha)
			}
		}
	}
}

func TestInputDisabledPlaceholderState(t *testing.T) {
	var canvas gfx.Canvas
	PaintInput(&canvas, gfx.DarkTheme(), gfx.Rect{Width: 120}, InputSnapshot{
		ID: gfx.Item(1), Label: "Search", Placeholder: "Search…", Disabled: true,
	})
	if text := canvas.Texts()[0]; text.Value != "Search…" || text.Style.Color.Alpha >= gfx.DarkTheme().MutedForeground.Alpha {
		t.Fatalf("placeholder = %+v", text)
	}
	if hit := canvas.Hits()[0]; !hit.Disabled || hit.Focusable || hit.Role != gfx.RoleTextField {
		t.Fatalf("hit = %+v", hit)
	}
}

func TestInputPlatformEditingLeavesTextToPlatformService(t *testing.T) {
	theme := gfx.LightTheme()
	var canvas gfx.Canvas
	PaintInput(&canvas, theme, gfx.Rect{Width: 220}, InputSnapshot{
		ID: gfx.Item(4), Label: "Email", Value: "hello@viva.dev", PlatformEditing: true,
	})
	if len(canvas.Texts()) != 0 {
		t.Fatalf("native editing painted duplicate text: %+v", canvas.Texts())
	}
	if hits := canvas.Hits(); len(hits) != 1 || hits[0].Value != "hello@viva.dev" {
		t.Fatalf("native editing semantics = %+v", hits)
	}
}

// HideEditorText keeps the platform's text and overlays invisible and paints
// the field from the same values a platform-independent field would: the line
// cannot differ between the two renderers because there is only one.
func TestInputCanHideThePlatformEditorAndPaintItsOwnText(t *testing.T) {
	theme := gfx.LightTheme()
	paint := func(snapshot InputSnapshot) (*gfx.Canvas, gfx.Hit) {
		var canvas gfx.Canvas
		canvas.SetInteraction(gfx.ID{}, gfx.ID{}, gfx.Item(4), false)
		snapshot.ID = gfx.Item(4)
		PaintInput(&canvas, theme, gfx.Rect{Width: 220}, snapshot)
		hits := canvas.Hits()
		if len(hits) != 1 {
			t.Fatalf("%d hits", len(hits))
		}
		return &canvas, hits[0]
	}
	_, shown := paint(InputSnapshot{Value: "hi", PlatformEditing: true})
	if shown.TextColor.Alpha == 0 {
		t.Fatal("the platform's text is invisible by default")
	}
	canvas, hidden := paint(InputSnapshot{
		Value: "hi", PlatformEditing: true, HideEditorText: true, ShowCaret: true, Caret: 2,
	})
	if hidden.TextColor.Alpha != 0 {
		t.Errorf("HideEditorText left the platform's text visible: %+v", hidden.TextColor)
	}
	if !hidden.HideNativeOverlays {
		t.Error("HideEditorText did not hide the platform caret and selection")
	}
	if len(canvas.Texts()) == 0 {
		t.Error("the component did not paint the text it hid from the platform")
	}
}

// A caret the component paints must keep the blink phase it would have had the
// platform draw it: the platform's caret is hidden, so nothing else blinks.
func TestInputHiddenEditorCaretBlinks(t *testing.T) {
	theme := gfx.LightTheme()
	var canvas gfx.Canvas
	now := time.Duration(0)
	canvas.SetClock(func() time.Duration { return now })
	var blink CaretBlink
	snapshot := InputSnapshot{
		ID: gfx.Item(4), Label: "Name", Value: "Gopher", HideEditorText: true,
		ShowCaret: true, Caret: 2, Blink: &blink,
	}
	paint := func() int {
		canvas.Reset()
		canvas.BeginFrame()
		canvas.SetInteraction(gfx.ID{}, gfx.ID{}, gfx.Item(4), false)
		PaintInput(&canvas, theme, gfx.Rect{Width: 200}, snapshot)
		return len(canvas.Rectangles())
	}
	lit := paint()
	now = CaretBlinkHalf + time.Millisecond
	if dark := paint(); dark != lit-1 {
		t.Fatalf("the dark phase painted %d rectangles, want one fewer than the %d of the lit phase", dark, lit)
	}
	now = 2*CaretBlinkHalf + time.Millisecond
	if again := paint(); again != lit {
		t.Fatalf("the next lit phase painted %d rectangles, want %d", again, lit)
	}
}

func TestInputPaintIsDeterministicAndAllocationFreeAfterWarmup(t *testing.T) {
	var canvas gfx.Canvas
	paint := func() {
		canvas.Reset()
		PaintInput(&canvas, gfx.LightTheme(), gfx.Rect{X: 2, Y: 3, Width: 160}, InputSnapshot{
			ID: gfx.Item(9), Label: "Name", Value: "Gopher", Focused: true, ShowCaret: true, Caret: 3,
			SelectionStart: 1, SelectionEnd: 3, CompositionStart: 3, CompositionEnd: 6,
		})
	}
	paint()
	rectangles := slices.Clone(canvas.Rectangles())
	texts := slices.Clone(canvas.Texts())
	hits := slices.Clone(canvas.Hits())
	paint()
	if !slices.Equal(rectangles, canvas.Rectangles()) || !slices.Equal(texts, canvas.Texts()) || !slices.Equal(hits, canvas.Hits()) {
		t.Fatal("identical input snapshots produced different streams")
	}
	if allocations := testing.AllocsPerRun(100, paint); allocations != 0 {
		t.Fatalf("input paint allocations = %v", allocations)
	}
}

func TestSecureInputMasksPaintAndNeverExposesTheValue(t *testing.T) {
	theme := gfx.DarkTheme()
	var canvas gfx.Canvas
	PaintInput(&canvas, theme, gfx.Rect{Width: 200}, InputSnapshot{
		ID: gfx.Item(3), Label: "Password", Value: "pä密ss", DisplayValue: "leak-through-display", Secure: true,
		SelectionStart: len("p"), SelectionEnd: len("pä密"), Caret: len("pä密"),
	})
	if hit := canvas.Hits()[0]; !hit.Secure || hit.Value != "" || hit.Role != gfx.RoleTextField {
		t.Fatalf("hit = %+v, want a secure text field without a value", hit)
	}
	// A selection paints the line twice (plain, then inverted over the
	// selected span); every pass must be the five bullets.
	if len(canvas.Texts()) == 0 {
		t.Fatal("secure input painted no text")
	}
	for _, text := range canvas.Texts() {
		if text.Value != "•••••" {
			t.Fatalf("painted %q, want exactly five bullets", text.Value)
		}
		for _, secret := range []string{"p", "ä", "密", "s", "leak"} {
			if strings.Contains(text.Value, secret) {
				t.Fatalf("painted text %q contains %q", text.Value, secret)
			}
		}
	}
}

func TestSecureInputMaskOffsetsFollowCharactersNotBytes(t *testing.T) {
	value := "a中😀b"
	for _, test := range []struct{ offset, want int }{
		{0, 0},
		{len("a"), 3},
		{len("a中"), 6},
		{len("a中😀"), 9},
		{len(value), 12},
		{len(value) + 50, 12},
		{len("a中") + 1, 6}, // inside a character: clamps to its start
	} {
		if got := inputMaskOffset(value, test.offset); got != test.want {
			t.Fatalf("offset %d maps to %d, want %d", test.offset, got, test.want)
		}
	}
	if got := inputMask(strings.Repeat("x", 1000)); len(got) != len(inputBullets) {
		t.Fatalf("long value masks to %d bytes, want the %d-byte cap", len(got), len(inputBullets))
	}
}

func TestSecureInputPaintDoesNotAllocateForMasking(t *testing.T) {
	var canvas gfx.Canvas
	snapshot := InputSnapshot{ID: gfx.Item(3), Label: "Password", Value: "correct horse", Secure: true}
	PaintInput(&canvas, gfx.DarkTheme(), gfx.Rect{Width: 200}, snapshot)
	if allocations := testing.AllocsPerRun(100, func() {
		canvas.Reset()
		PaintInput(&canvas, gfx.DarkTheme(), gfx.Rect{Width: 200}, snapshot)
	}); allocations != 0 {
		t.Fatalf("secure paint allocations = %v", allocations)
	}
}

// Hebrew in a field reads from the right: the text sits at the right end, and the caret at
// the start of the text is on its right edge and moves left.
func TestInputRightToLeftTextIsRightAlignedWithItsCaretOnTheRight(t *testing.T) {
	theme := gfx.LightTheme()
	caretX := func(value string, caret int) (caretX float32, text gfx.Text) {
		var canvas gfx.Canvas
		canvas.SetInteraction(gfx.ID{}, gfx.ID{}, gfx.Item(7), true)
		PaintInput(&canvas, theme, gfx.Rect{X: 10, Y: 20, Width: 240}, InputSnapshot{
			ID: gfx.Item(7), Value: value, DisplayValue: value, ShowCaret: true, Caret: caret,
		})
		for _, rect := range canvas.Rectangles() {
			if rect.Bounds.Width == 1 && rect.Bounds.Height == 16 {
				caretX = rect.Bounds.X
			}
		}
		return caretX, canvas.Texts()[0]
	}
	hebrew := "שלום עולם"
	atStart, text := caretX(hebrew, 0)
	atEnd, _ := caretX(hebrew, len(hebrew))
	if text.Bounds.X <= 10+12 {
		t.Errorf("Hebrew text starts at %v, want it pushed to the right end of the field", text.Bounds.X)
	}
	if !(atStart > atEnd) {
		t.Errorf("caret at the start of the text is at %v, at its end %v: it should move left", atStart, atEnd)
	}
	english, _ := caretX("hello world", 0)
	englishEnd, englishText := caretX("hello world", len("hello world"))
	if !(englishEnd > english) || englishText.Bounds.X > 10+12 {
		t.Errorf("English caret %v to %v, text at %v", english, englishEnd, englishText.Bounds.X)
	}
}

func TestInputPlaceholderThatDoesNotFitEndsInAnEllipsis(t *testing.T) {
	theme := gfx.LightTheme()
	for _, test := range []struct {
		placeholder string
		width       float32
		ellipsis    bool
	}{
		{"Search by host, type, fingerprint or comment", 120, true},
		{"Search", 240, false},
	} {
		var canvas gfx.Canvas
		PaintInput(&canvas, theme, gfx.Rect{Width: test.width}, InputSnapshot{ID: gfx.Item(1), Label: "Search", Placeholder: test.placeholder})
		shown := canvas.Texts()[0].Value
		if got := strings.HasSuffix(shown, "…"); got != test.ellipsis || test.ellipsis == (shown == test.placeholder) {
			t.Fatalf("%q in %v px shows %q", test.placeholder, test.width, shown)
		}
		if width := canvas.TextWidthWeight(shown, theme.Metrics.FontSizeM, gfx.WeightRegular); width > test.width {
			t.Fatalf("%q is %v wide in a field of %v", shown, width, test.width)
		}
	}
}

// Every editable text control declares the I-beam itself, including when it is read only
// (the text stays selectable); the driver only maps the declared cursor.
func TestTextEntryControlsDeclareTheTextCursor(t *testing.T) {
	theme := gfx.LightTheme()
	for name, paint := range map[string]func(*gfx.Canvas){
		"input": func(c *gfx.Canvas) {
			PaintInput(c, theme, gfx.Rect{Width: 160}, InputSnapshot{ID: gfx.Item(1), ReadOnly: true})
		},
		"textarea": func(c *gfx.Canvas) {
			PaintTextarea(c, theme, gfx.Rect{Width: 160, Height: 80}, TextareaSnapshot{ID: gfx.Item(1)})
		},
		"input group": func(c *gfx.Canvas) {
			PaintInputGroup(c, theme, gfx.Rect{Width: 160}, InputGroupSnapshot{ID: gfx.Item(1)})
		},
		"otp": func(c *gfx.Canvas) { PaintInputOTP(c, theme, gfx.Rect{}, InputOTPSnapshot{ID: gfx.Item(1), Slots: 6}) },
	} {
		var canvas gfx.Canvas
		paint(&canvas)
		hits := canvas.Hits()
		if len(hits) == 0 || hits[0].Cursor != gfx.PointerCursorText {
			t.Errorf("%s: hits = %+v", name, hits)
		}
	}
}
