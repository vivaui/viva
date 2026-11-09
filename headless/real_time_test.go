//go:build darwin || windows || linux

package headless

import (
	"testing"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

// A page that hands over after a delay asks for a frame at that delay. While a
// script waits in real time for the next page, that time has to count for the
// application's timers too, or a flow that works in a window never completes here.
func TestWaitingInRealTimeLetsAFrameTheApplicationAskedForComeDue(t *testing.T) {
	start := time.Now()
	handedOver := false
	config := app.Config{
		Title: "hand over", Width: 200, Height: 80,
		ClearColor: app.Color{Alpha: 1},
		Paint: func(canvas *gfx.Canvas, size gfx.Size) {
			if time.Since(start) < 150*time.Millisecond {
				canvas.Label("arriving", gfx.Rect{X: 8, Y: 8, Width: 180, Height: 24}, gfx.TextStyle{Size: 14})
				canvas.RequestFrameIn(150 * time.Millisecond)
				return
			}
			handedOver = true
			canvas.Label("arrived", gfx.Rect{X: 8, Y: 8, Width: 180, Height: 24}, gfx.TextStyle{Size: 14})
		},
	}
	session, err := Open(config, nil, Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	err = session.WaitFor(func(snapshot app.Snapshot) bool { return snapshotHasText(snapshot, "arrived") }, 3*time.Second)
	if err != nil || !handedOver {
		t.Fatalf("the frame requested 150 ms ahead never came due during a 3 s wait: %v", err)
	}
	if elapsed := time.Since(start); elapsed < 140*time.Millisecond {
		t.Fatalf("the hand-over happened after %v, before its time", elapsed)
	}
}
