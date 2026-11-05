package components

import (
	"testing"
	"time"

	"github.com/vivaui/viva/ui"
)

type iconHoverFrames struct {
	canvas *ui.Canvas
	now    time.Duration
}

func newIconHoverFrames() *iconHoverFrames {
	frames := &iconHoverFrames{canvas: new(ui.Canvas)}
	frames.canvas.SetClock(func() time.Duration { return frames.now })
	return frames
}

func (frames *iconHoverFrames) at(at time.Duration, hovered ui.ID, state *IconHover, id ui.ID) (float32, bool) {
	frames.now = at
	frames.canvas.SetInteraction(hovered, ui.ID{}, ui.ID{}, false)
	frames.canvas.BeginFrame()
	progress, playing := state.Progress(frames.canvas, id, 0, 130*time.Millisecond, 160*time.Millisecond)
	frames.canvas.EndFrame()
	return progress, playing
}

func TestIconHoverWaitsForItsDwellThenPlaysOnce(t *testing.T) {
	id := ui.Item(1)
	frames := newIconHoverFrames()
	var hover IconHover
	if progress, playing := frames.at(0, id, &hover, id); progress != 1 || playing {
		t.Fatalf("first frame = %v playing=%v, want the still icon while it rests", progress, playing)
	}
	if _, ok := frames.canvas.NextFrame(); !ok {
		t.Fatal("the dwell did not ask for the frame that ends it")
	}
	if progress, playing := frames.at(129*time.Millisecond, id, &hover, id); progress != 1 || playing {
		t.Fatalf("before the dwell = %v playing=%v", progress, playing)
	}
	if progress, playing := frames.at(130*time.Millisecond, id, &hover, id); progress != 0 || !playing {
		t.Fatalf("at the dwell = %v playing=%v, want the gesture to start", progress, playing)
	}
	if progress, playing := frames.at(210*time.Millisecond, id, &hover, id); progress != 0.5 || !playing {
		t.Fatalf("half way = %v playing=%v", progress, playing)
	}
	if progress, playing := frames.at(290*time.Millisecond, id, &hover, id); progress != 1 || playing {
		t.Fatalf("end = %v playing=%v", progress, playing)
	}
	if progress, playing := frames.at(500*time.Millisecond, id, &hover, id); progress != 1 || playing {
		t.Fatalf("still hovered after the gesture = %v playing=%v, want it not to loop", progress, playing)
	}
}

func TestIconHoverFinishesWhenThePointerLeaves(t *testing.T) {
	id := ui.Item(2)
	frames := newIconHoverFrames()
	var hover IconHover
	frames.at(0, id, &hover, id)
	frames.at(130*time.Millisecond, id, &hover, id)
	progress, playing := frames.at(170*time.Millisecond, ui.ID{}, &hover, id)
	if progress != 0.25 || !playing {
		t.Fatalf("leaving mid-gesture = %v playing=%v, want it to finish", progress, playing)
	}
	if progress, playing := frames.at(290*time.Millisecond, ui.ID{}, &hover, id); progress != 1 || playing {
		t.Fatalf("after leaving = %v playing=%v", progress, playing)
	}
}

func TestIconHoverReArmsOnlyAfterLeaving(t *testing.T) {
	id := ui.Item(3)
	frames := newIconHoverFrames()
	var hover IconHover
	frames.at(0, id, &hover, id)
	frames.at(130*time.Millisecond, id, &hover, id)
	frames.at(290*time.Millisecond, id, &hover, id)
	if progress, playing := frames.at(300*time.Millisecond, id, &hover, id); progress != 1 || playing {
		t.Fatalf("a held pointer replayed = %v playing=%v", progress, playing)
	}
	frames.at(310*time.Millisecond, ui.ID{}, &hover, id)
	frames.at(400*time.Millisecond, id, &hover, id)
	if progress, playing := frames.at(530*time.Millisecond, id, &hover, id); progress != 0 || !playing {
		t.Fatalf("the second hover = %v playing=%v", progress, playing)
	}
}

func TestIconHoverIsStillUnderReducedMotion(t *testing.T) {
	id := ui.Item(4)
	frames := newIconHoverFrames()
	frames.canvas.SetReduceMotion(true)
	var hover IconHover
	frames.at(0, id, &hover, id)
	if progress, playing := frames.at(130*time.Millisecond, id, &hover, id); progress != 1 || playing || frames.canvas.Animating() {
		t.Fatalf("reduced motion = %v playing=%v animating=%v", progress, playing, frames.canvas.Animating())
	}
}
