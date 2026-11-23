//go:build darwin || windows || linux

package app

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// The page reads how much of its titlebar strip the window buttons cover from
// the window it paints: the platform's buttons in a custom titlebar, nothing in
// full screen, where they leave the strip, and nothing under a system titlebar.
func TestTitlebarInsetsFollowTheWindow(t *testing.T) {
	read := func(mode WindowMode, fullscreen bool) (float32, float32) {
		var canvas gfx.Canvas
		var leading, trailing float32
		config := Config{Title: "insets", Width: 400, Height: 300, WindowMode: mode, TitlebarHeight: 36}
		config.Paint = func(canvas *gfx.Canvas, size gfx.Size) { leading, trailing = canvas.TitlebarInsets() }
		painter := newTestPainter(t, &canvas, config)
		painter.fullscreenHandler(nil)(fullscreen)
		if _, err := painter.repaint(); err != nil {
			t.Fatal(err)
		}
		return leading, trailing
	}
	wantLeading, wantTrailing := platformTitlebarInsets()
	if leading, trailing := read(WindowCustomTitlebar, false); leading != wantLeading || trailing != wantTrailing {
		t.Fatalf("custom titlebar: insets %v, %v, want the platform's %v, %v", leading, trailing, wantLeading, wantTrailing)
	}
	if leading, trailing := read(WindowCustomTitlebar, true); leading != 0 || trailing != 0 {
		t.Fatalf("full screen: insets %v, %v, want none", leading, trailing)
	}
	if leading, trailing := read(WindowTitled, false); leading != 0 || trailing != 0 {
		t.Fatalf("system titlebar: insets %v, %v, want none", leading, trailing)
	}
}
