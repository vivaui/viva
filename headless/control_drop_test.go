//go:build darwin || windows || linux

package headless

import (
	"reflect"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

func TestDropControlsDeliverHoverLeaveAndDropToTheHitUnderThePoint(t *testing.T) {
	var events []gfx.FileDropEvent
	config := app.Config{Width: 320, Height: 200}
	config.Paint = func(canvas *gfx.Canvas, _ gfx.Size) {
		canvas.Hit(gfx.Item(7), gfx.Rect{X: 40, Y: 50, Width: 100, Height: 80})
	}
	config.OnFileDrop = func(event gfx.FileDropEvent) gfx.EventResult {
		events = append(events, event)
		return gfx.EventResult{Handled: event.Target == gfx.Item(7)}
	}
	session, err := Open(config, nil, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	// In order: the events below are checked by position.
	for _, step := range [][2]string{
		{"dragover 60 70", "ok accepted\n"},
		{"dragover 300 190", "ok refused\n"},
		{"dragleave", "ok refused\n"},
		{`drop 60 70 /tmp/a "/tmp/b c"`, "ok accepted\n"},
		{"drop 300 190 /tmp/a", "ok refused\n"},
	} {
		if reply, _ := execute(session, step[0]); reply != step[1] {
			t.Errorf("%q replied %q, want %q", step[0], reply, step[1])
		}
	}
	if len(events) != 5 {
		t.Fatalf("events = %+v", events)
	}
	if events[0].Phase != gfx.FileDropOver || events[0].Target != gfx.Item(7) || !events[1].Target.IsZero() || events[2].Phase != gfx.FileDropLeave {
		t.Fatalf("events = %+v", events[:3])
	}
	if events[3].Phase != gfx.FileDropDrop || events[3].Target != gfx.Item(7) || !reflect.DeepEqual(events[3].Paths, []string{"/tmp/a", "/tmp/b c"}) {
		t.Fatalf("drop = %+v", events[3])
	}
	for _, command := range []string{"drop 1 2", "drop 1", "dragover 1", "drop a b /x"} {
		if reply, _ := execute(session, command); !strings.HasPrefix(reply, "error ") {
			t.Errorf("%q replied %q", command, reply)
		}
	}
}
