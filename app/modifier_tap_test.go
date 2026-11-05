//go:build darwin || windows || linux

package app

import (
	"testing"

	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

type tapFixture struct {
	driver *Driver
	taps   []ui.ModifierTapEvent
	keys   []ui.KeyEvent
}

func newTapFixture(t *testing.T) *tapFixture {
	t.Helper()
	f := &tapFixture{}
	editor := text.NewEditor("")
	driver, err := NewDriver(Config{
		Width: 320, Height: 180,
		Paint: func(canvas *ui.Canvas, _ ui.Size) {
			canvas.HitWithOptions(ui.Item(1), ui.Rect{X: 10, Y: 10, Width: 200, Height: 32}, "Field", ui.HitOptions{Role: ui.RoleTextField})
			canvas.Hit(ui.Item(2), ui.Rect{X: 10, Y: 100, Width: 100, Height: 32})
		},
		TextInput: func(TextInputContext) *TextInput { return &TextInput{Editor: editor} },
		OnModifierTap: func(event ui.ModifierTapEvent) ui.EventResult {
			f.taps = append(f.taps, event)
			return ui.EventResult{}
		},
		OnKey: func(event ui.KeyEvent) ui.EventResult { f.keys = append(f.keys, event); return ui.EventResult{} },
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	f.driver = driver
	return f
}

func (f *tapFixture) tap(t *testing.T, key ui.Key) {
	t.Helper()
	if err := f.driver.KeyDown(key, ""); err != nil {
		t.Fatal(err)
	}
	if err := f.driver.KeyUp(key, ""); err != nil {
		t.Fatal(err)
	}
}

func TestModifierTapIsReportedForALoneModifier(t *testing.T) {
	f := newTapFixture(t)
	f.tap(t, ui.KeyShiftLeft)
	f.tap(t, ui.KeyShiftLeft)
	if len(f.taps) != 2 || f.taps[0].Key != ui.KeyShiftLeft {
		t.Fatalf("taps = %+v", f.taps)
	}
	// A window that did not ask for the whole keyboard never shows OnKey a
	// modifier: the tap is the only way an application learns of one.
	for _, event := range f.keys {
		if event.Key.IsModifier() {
			t.Fatalf("OnKey saw a modifier: %+v", event)
		}
	}
}

func TestModifierTapIsCancelledByAnythingBetweenPressAndRelease(t *testing.T) {
	steps := map[string]func(*tapFixture) error{
		"a key":           func(f *tapFixture) error { return f.driver.Key(ui.KeyTab, 0) },
		"typed text":      func(f *tapFixture) error { return f.driver.Click(20, 20) },
		"a pointer press": func(f *tapFixture) error { return f.driver.Click(20, 110) },
		"a scroll":        func(f *tapFixture) error { return f.driver.Scroll(ui.ScrollEvent{X: 20, Y: 20, DeltaY: 10}) },
		"focus loss":      func(f *tapFixture) error { return f.driver.LoseFocus() },
		"a second modifier": func(f *tapFixture) error {
			return f.driver.KeyDown(ui.KeyControlLeft, "")
		},
	}
	for name, step := range steps {
		f := newTapFixture(t)
		if err := f.driver.KeyDown(ui.KeyShiftLeft, ""); err != nil {
			t.Fatal(err)
		}
		if err := step(f); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if err := f.driver.KeyUp(ui.KeyShiftLeft, ""); err != nil {
			t.Fatal(err)
		}
		if len(f.taps) != 0 {
			t.Errorf("%s: still reported a tap: %+v", name, f.taps)
		}
	}
}

func TestModifierTapIsCancelledByTypingIntoAField(t *testing.T) {
	f := newTapFixture(t)
	if err := f.driver.Click(20, 20); err != nil {
		t.Fatal(err)
	}
	if err := f.driver.KeyDown(ui.KeyShiftLeft, ""); err != nil {
		t.Fatal(err)
	}
	if err := f.driver.Type("A"); err != nil { // Shift+a typed as text, which no key event reports
		t.Fatal(err)
	}
	if err := f.driver.KeyUp(ui.KeyShiftLeft, ""); err != nil {
		t.Fatal(err)
	}
	if len(f.taps) != 0 {
		t.Fatalf("typing a capital counted as a Shift tap: %+v", f.taps)
	}
}

func TestModifierTapNeedsTheCallback(t *testing.T) {
	driver, err := NewDriver(Config{Width: 100, Height: 100, Paint: func(*ui.Canvas, ui.Size) {}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.KeyDown(ui.KeyShiftLeft, ""); err != nil {
		t.Fatal(err)
	}
	if err := driver.KeyUp(ui.KeyShiftLeft, ""); err != nil {
		t.Fatal(err)
	}
}
