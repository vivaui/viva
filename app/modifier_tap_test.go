//go:build darwin || windows || linux

package app

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

type tapFixture struct {
	driver *Driver
	taps   []gfx.ModifierTapEvent
	keys   []gfx.KeyEvent
}

func newTapFixture(t *testing.T) *tapFixture {
	t.Helper()
	f := &tapFixture{}
	editor := text.NewEditor("")
	driver, err := NewDriver(Config{
		Width: 320, Height: 180,
		Paint: func(canvas *gfx.Canvas, _ gfx.Size) {
			canvas.HitWithOptions(gfx.Item(1), gfx.Rect{X: 10, Y: 10, Width: 200, Height: 32}, "Field", gfx.HitOptions{Role: gfx.RoleTextField})
			canvas.Hit(gfx.Item(2), gfx.Rect{X: 10, Y: 100, Width: 100, Height: 32})
		},
		TextInput: func(TextInputContext) *TextInput { return &TextInput{Editor: editor} },
		OnModifierTap: func(event gfx.ModifierTapEvent) gfx.EventResult {
			f.taps = append(f.taps, event)
			return gfx.EventResult{}
		},
		OnKey: func(event gfx.KeyEvent) gfx.EventResult { f.keys = append(f.keys, event); return gfx.EventResult{} },
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	f.driver = driver
	return f
}

func (f *tapFixture) tap(t *testing.T, key gfx.Key) {
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
	f.tap(t, gfx.KeyShiftLeft)
	f.tap(t, gfx.KeyShiftLeft)
	if len(f.taps) != 2 || f.taps[0].Key != gfx.KeyShiftLeft {
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
		"a key":           func(f *tapFixture) error { return f.driver.Key(gfx.KeyTab, 0) },
		"typed text":      func(f *tapFixture) error { return f.driver.Click(20, 20) },
		"a pointer press": func(f *tapFixture) error { return f.driver.Click(20, 110) },
		"a scroll":        func(f *tapFixture) error { return f.driver.Scroll(gfx.ScrollEvent{X: 20, Y: 20, DeltaY: 10}) },
		"focus loss":      func(f *tapFixture) error { return f.driver.LoseFocus() },
		"a second modifier": func(f *tapFixture) error {
			return f.driver.KeyDown(gfx.KeyControlLeft, "")
		},
	}
	for name, step := range steps {
		f := newTapFixture(t)
		if err := f.driver.KeyDown(gfx.KeyShiftLeft, ""); err != nil {
			t.Fatal(err)
		}
		if err := step(f); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if err := f.driver.KeyUp(gfx.KeyShiftLeft, ""); err != nil {
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
	if err := f.driver.KeyDown(gfx.KeyShiftLeft, ""); err != nil {
		t.Fatal(err)
	}
	if err := f.driver.Type("A"); err != nil { // Shift+a typed as text, which no key event reports
		t.Fatal(err)
	}
	if err := f.driver.KeyUp(gfx.KeyShiftLeft, ""); err != nil {
		t.Fatal(err)
	}
	if len(f.taps) != 0 {
		t.Fatalf("typing a capital counted as a Shift tap: %+v", f.taps)
	}
}

func TestModifierTapNeedsTheCallback(t *testing.T) {
	driver, err := NewDriver(Config{Width: 100, Height: 100, Paint: func(*gfx.Canvas, gfx.Size) {}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.KeyDown(gfx.KeyShiftLeft, ""); err != nil {
		t.Fatal(err)
	}
	if err := driver.KeyUp(gfx.KeyShiftLeft, ""); err != nil {
		t.Fatal(err)
	}
}
