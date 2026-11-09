//go:build darwin || windows || linux

package headless

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

func TestScrollControlCarriesPointModifiersAndDerivedFields(t *testing.T) {
	var received gfx.ScrollEvent
	config := app.Config{Width: 320, Height: 200}
	config.Paint = func(canvas *gfx.Canvas, _ gfx.Size) {
		canvas.Hit(gfx.Item(7), gfx.Rect{X: 40, Y: 50, Width: 100, Height: 80})
	}
	config.OnScroll = func(event gfx.ScrollEvent) gfx.EventResult {
		received = event
		return gfx.EventResult{Handled: true}
	}
	session, err := Open(config, nil, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	reply, _ := execute(session, "scroll 60 70 4 -48 shift+ctrl")
	if reply != "ok\n" {
		t.Fatalf("reply = %q", reply)
	}
	want := gfx.ScrollEvent{
		Modifiers: gfx.ModifierShift | gfx.ModifierControl,
		X:         60, Y: 70, DeltaX: 4, DeltaY: -48,
		Viewport: gfx.Size{Width: 320, Height: 200}, Target: gfx.Item(7), Precise: true,
	}
	if received != want {
		t.Fatalf("scroll = %+v, want %+v", received, want)
	}
	for _, command := range []string{"scroll 1 2", "scroll 1 2 3 4 shift extra", "scroll 1 2 3 4 hyper"} {
		reply, _ = execute(session, command)
		if !strings.HasPrefix(reply, "error ") {
			t.Fatalf("%q reply = %q", command, reply)
		}
	}
}
