//go:build darwin || windows || linux

package headless

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

// TestControlWindowMoveMinimizeAndTitle drives the window state the way a
// window manager would, without opening a window.
func TestControlWindowMoveMinimizeAndTitle(t *testing.T) {
	var frames []gfx.Rect
	var minimizes []bool
	var maximizes []bool
	origin := gfx.Point{X: 10, Y: 20}
	config := app.Config{
		Title: "main", Width: 300, Height: 200, Origin: &origin,
		OnFrameChange: func(frame gfx.Rect) { frames = append(frames, frame) },
		OnMinimize:    func(minimized bool) { minimizes = append(minimizes, minimized) },
		OnMaximize:    func(maximized bool) { maximizes = append(maximizes, maximized) },
	}
	session, err := Open(config, nil, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	reply := func(line string) string { r, _ := execute(session, line); return r }

	if got := reply("windowmove 60 70"); !strings.HasPrefix(got, "ok") {
		t.Fatalf("windowmove: %s", got)
	}
	if len(frames) != 1 || frames[0] != (gfx.Rect{X: 60, Y: 70, Width: 300, Height: 200}) {
		t.Fatalf("frames after move = %+v", frames)
	}
	if got := reply("minimize on"); !strings.HasPrefix(got, "ok") {
		t.Fatalf("minimize on: %s", got)
	}
	if got := reply("minimize off"); !strings.HasPrefix(got, "ok") {
		t.Fatalf("minimize off: %s", got)
	}
	if len(minimizes) != 2 || !minimizes[0] || minimizes[1] {
		t.Fatalf("minimize changes = %v", minimizes)
	}
	if got := reply("minimize sideways"); !strings.HasPrefix(got, "error ") {
		t.Fatalf("minimize sideways = %s", got)
	}
	if got := reply("maximize on"); !strings.HasPrefix(got, "ok") {
		t.Fatalf("maximize on: %s", got)
	}
	if got := reply("maximize off"); !strings.HasPrefix(got, "ok") {
		t.Fatalf("maximize off: %s", got)
	}
	if len(maximizes) != 2 || !maximizes[0] || maximizes[1] {
		t.Fatalf("maximize changes = %v", maximizes)
	}
	if got := reply("maximize sideways"); !strings.HasPrefix(got, "error ") {
		t.Fatalf("maximize sideways = %s", got)
	}
}
