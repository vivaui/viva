package inputotp_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/inputotp"
	"github.com/vivaui/viva/window"
)

func testTheme() gfx.Theme { return gfx.LightTheme() }

func draw(page *ui.Page, canvas *gfx.Canvas, view func(ui.C)) {
	canvas.Reset()
	canvas.BeginFrame()
	page.Draw(canvas, gfx.Rect{Width: 560, Height: 240}, view)
	canvas.EndFrame()
}

func open(t testing.TB, view func(ui.C)) (*ui.Page, *gfx.Canvas) {
	t.Helper()
	theme := testTheme()
	page := ui.NewPage(ui.PageConfig{View: view, Theme: &theme})
	canvas := new(gfx.Canvas)
	draw(page, canvas, view)
	return page, canvas
}

func hit(t *testing.T, canvas *gfx.Canvas, label string) gfx.Hit {
	t.Helper()
	hits := canvas.Hits()
	for index := len(hits) - 1; index >= 0; index-- {
		if hits[index].Label == label {
			return hits[index]
		}
	}
	t.Fatalf("no hit labelled %q was painted", label)
	return gfx.Hit{}
}

func hasText(canvas *gfx.Canvas, value string) bool {
	for _, label := range canvas.Texts() {
		if label.Value == value {
			return true
		}
	}
	return false
}

func caretCount(canvas *gfx.Canvas) int {
	count := 0
	for _, rectangle := range canvas.Rectangles() {
		if rectangle.Bounds.Width == 1 && rectangle.Bounds.Height == 16 {
			count++
		}
	}
	return count
}

type model struct {
	editor *text.Editor
	field  ui.Field
	code   inputotp.State
}

func (m *model) init() {
	m.editor = text.NewEditor("482901")
	m.field.Bind(m.editor)
	m.code = *inputotp.New().
		Bind(&m.field, m.editor).
		Slots(6).GroupSize(3).Filter(inputotp.Digits)
}

func (m *model) view(c ui.C) { m.code.View(c, "Verification code") }

// The slots paint the editor's value, and the platform text service gets the
// same editor a ui.TextField would.
func TestViewPaintsEditorAndRoutesText(t *testing.T) {
	m := new(model)
	m.init()
	page, canvas := open(t, m.view)
	control := hit(t, canvas, "Verification code")
	if control.ID.IsZero() || control.Role != gfx.RoleTextField {
		t.Fatalf("hit = %+v, want the control's text field", control)
	}
	for _, digit := range []string{"4", "8", "2", "9", "0", "1"} {
		if !hasText(canvas, digit) {
			t.Fatalf("digit %q was not painted", digit)
		}
	}
	input := page.TextInput(window.TextInputContext{ID: control.ID})
	if input == nil || input.Editor != m.editor {
		t.Fatalf("the platform text service got %+v, want the slot editor", input)
	}
}

// A press moves the editor's caret to the slot under the pointer.
func TestPointerSelectsSlot(t *testing.T) {
	m := new(model)
	m.init()
	page, canvas := open(t, m.view)
	control := hit(t, canvas, "Verification code")
	layout := m.code.Layout()
	slot := layout.SlotBounds(1)
	result := page.Pointer(gfx.PointerEvent{
		Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, Target: control.ID,
		X: slot.X + slot.Width/2, Y: slot.Y + slot.Height/2,
	})
	if !result.Handled {
		t.Fatal("the press was not handled")
	}
	if m.editor.Caret() != 1 {
		t.Fatalf("caret = %d, want slot 1", m.editor.Caret())
	}
}

// Keys that move between slots are the control's, and go through the same
// editor.
func TestKeysMoveAndDelete(t *testing.T) {
	m := new(model)
	m.init()
	page, canvas := open(t, m.view)
	control := hit(t, canvas, "Verification code")
	if result := page.Key(gfx.KeyEvent{Key: gfx.KeyHome, Target: control.ID}); !result.Handled {
		t.Fatal("Home was not handled")
	}
	if m.editor.Caret() != 0 {
		t.Fatalf("caret = %d after Home, want 0", m.editor.Caret())
	}
	if result := page.Key(gfx.KeyEvent{Key: gfx.KeyEnd, Target: control.ID}); !result.Handled {
		t.Fatal("End was not handled")
	}
	if m.editor.Caret() != m.editor.RuneCount() {
		t.Fatalf("caret = %d after End, want %d", m.editor.Caret(), m.editor.RuneCount())
	}
	if result := page.Key(gfx.KeyEvent{Key: gfx.KeyBackspace, Target: control.ID}); !result.Handled {
		t.Fatal("Backspace was not handled")
	}
	if m.editor.RuneCount() != 5 {
		t.Fatalf("the code has %d digits after Backspace, want 5", m.editor.RuneCount())
	}
}

// A static snapshot paints the same slots without an editor, including the
// active slot's caret and the states example's selection and composition.
func TestStaticSnapshot(t *testing.T) {
	theme := testTheme()
	page := ui.NewPage(ui.PageConfig{Theme: &theme})
	canvas := new(gfx.Canvas)
	var code inputotp.State
	view := func(c ui.C) {
		code = *inputotp.New().
			Value("482").Slots(6).GroupSize(3).
			Focused().ShowCaret().ActiveSlot(3)
		code.View(c, "Verification code")
	}
	draw(page, canvas, view)
	if !hasText(canvas, "4") || !hasText(canvas, "8") || !hasText(canvas, "2") {
		t.Fatal("the static code was not painted")
	}
	if hasText(canvas, "9") {
		t.Fatal("a digit beyond the code was painted")
	}
	if caret := caretCount(canvas); caret != 1 {
		t.Fatalf("%d carets painted, want one in the active empty slot", caret)
	}

	view = func(c ui.C) {
		code = *inputotp.New().
			Value("482901").Slots(6).GroupSize(3).
			Invalid().Focused().ActiveSlot(0).Select(1, 3).Composition(3, 5)
		code.View(c, "Verification code")
	}
	draw(page, canvas, view)
	if caret := caretCount(canvas); caret != 0 {
		t.Fatalf("%d carets painted through a complete code, want 0", caret)
	}
}

func TestWarmPaintAllocatesNothing(t *testing.T) {
	m := new(model)
	m.init()
	page, canvas := open(t, m.view)
	if allocations := testing.AllocsPerRun(100, func() { draw(page, canvas, m.view) }); allocations != 0 {
		t.Fatalf("warm paint allocated %v times, want 0", allocations)
	}
}

func BenchmarkViewPaint(b *testing.B) {
	m := new(model)
	m.init()
	page, canvas := open(b, m.view)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		draw(page, canvas, m.view)
	}
}
