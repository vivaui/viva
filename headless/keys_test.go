//go:build darwin || windows || linux

package headless

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

func TestControlKeydownKeyupHoldKeysAndModifiers(t *testing.T) {
	var keys []gfx.KeyEvent
	var pointers []gfx.PointerEvent
	config := app.Config{Title: "hold", Width: 300, Height: 120}
	config.Paint = func(canvas *gfx.Canvas, size gfx.Size) {
		canvas.HitWithOptions(gfx.Item(1), gfx.Rect{Width: size.Width, Height: size.Height}, "Terminal", gfx.HitOptions{Role: gfx.RoleTerminal})
	}
	config.OnKey = func(event gfx.KeyEvent) gfx.EventResult {
		keys = append(keys, event)
		return gfx.EventResult{Handled: true}
	}
	config.OnPointer = func(event gfx.PointerEvent) gfx.EventResult {
		pointers = append(pointers, event)
		return gfx.EventResult{}
	}
	config.TextInput = func(app.TextInputContext) *app.TextInput {
		return &app.TextInput{Bounds: gfx.Rect{Width: 8, Height: 16}, FontSize: 13, KeyReleases: true}
	}
	session, err := Open(config, nil, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if err := session.ClickLabel("Terminal"); err != nil {
		t.Fatal(err)
	}
	keys, pointers = nil, nil
	reply := func(line string) string { r, _ := execute(session, line); return r }
	for _, line := range []string{"keydown shift", "click 10 10", "keydown left", "keydown alt", "blur", "keyup left"} {
		if got := reply(line); !strings.HasPrefix(got, "ok") {
			t.Fatalf("%s: %s", line, got)
		}
	}
	var shiftClicks int
	for _, event := range pointers {
		if event.Kind == gfx.PointerDown && event.Modifiers == gfx.ModifierShift {
			shiftClicks++
		}
	}
	if shiftClicks != 1 {
		t.Fatalf("shift-click not seen: %+v", pointers)
	}
	var down, up int
	for _, event := range keys {
		if event.Release {
			up++
		} else {
			down++
		}
	}
	if down != 3 || up != 3 {
		t.Fatalf("down=%d up=%d (blur must release all, keyup of released key must not repeat): %+v", down, up, keys)
	}
	if got := reply("keydown bogus"); !strings.HasPrefix(got, "error") {
		t.Fatalf("unknown key accepted: %s", got)
	}
}
