//go:build darwin || windows || linux

package app

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// TestDriverActivateWindowIsNotEditorFocus checks that OnActivate follows the
// top-level window only: focus moving between text fields inside the window
// must not report an activation, and a closed window reports none.
func TestDriverActivateWindowIsNotEditorFocus(t *testing.T) {
	var count int
	config := Config{Title: "activation", Width: 320, Height: 200, OnActivate: func() { count++ }}
	config.Paint = func(canvas *gfx.Canvas, _ gfx.Size) {
		canvas.HitWithOptions(gfx.Item(1), gfx.Rect{Width: 32, Height: 32}, "first field", gfx.HitOptions{Role: gfx.RoleTextField})
		canvas.HitWithOptions(gfx.Item(2), gfx.Rect{Y: 32, Width: 32, Height: 32}, "second field", gfx.HitOptions{Role: gfx.RoleTextField})
	}
	driver, err := NewDriver(config, &gfx.Canvas{})
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.ActivateWindow(); err != nil {
		t.Fatal(err)
	}
	if err := driver.ActivateWindow(); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("OnActivate ran %d times after two activations, want 2", count)
	}
	if err := driver.Click(16, 16); err != nil {
		t.Fatal(err)
	}
	if err := driver.Click(16, 48); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("OnActivate ran %d times after focus moved between fields, want 2", count)
	}
	driver.Close()
	if err := driver.ActivateWindow(); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("OnActivate ran %d times after close, want 2", count)
	}
}
