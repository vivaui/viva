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
	return frames.atSlot(at, hovered, state, id, 0)
}

func (frames *iconHoverFrames) atSlot(at time.Duration, hovered ui.ID, state *IconHover, id ui.ID, slot uint8) (float32, bool) {
	frames.now = at
	frames.canvas.SetInteraction(hovered, ui.ID{}, ui.ID{}, false)
	frames.canvas.BeginFrame()
	progress, playing := state.Progress(frames.canvas, id, slot, 130*time.Millisecond, 160*time.Millisecond)
	frames.canvas.EndFrame()
	return progress, playing
}

// slotProgress is one gesture's result in a frame that advanced several.
type slotProgress struct {
	progress float32
	playing  bool
}

// atSlots advances several gestures of one identity in a single frame, which is
// what a control painting more than one gesture on its glyph does.
func (frames *iconHoverFrames) atSlots(at time.Duration, hovered ui.ID, state *IconHover, id ui.ID, slots ...uint8) []slotProgress {
	frames.now = at
	frames.canvas.SetInteraction(hovered, ui.ID{}, ui.ID{}, false)
	frames.canvas.BeginFrame()
	out := make([]slotProgress, len(slots))
	for index, slot := range slots {
		progress, playing := state.Progress(frames.canvas, id, slot, 130*time.Millisecond, 160*time.Millisecond)
		out[index] = slotProgress{progress: progress, playing: playing}
	}
	frames.canvas.EndFrame()
	return out
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

func TestIconHoverTracksEachSlotOnItsOwn(t *testing.T) {
	id := ui.Item(5)
	frames := newIconHoverFrames()
	var hover IconHover
	// Two gestures on one identity must not starve each other: before the dwell
	// ends neither plays, and once it does both start in the same frame.
	for _, got := range frames.atSlots(0, id, &hover, id, 0, 1) {
		if got.progress != 1 || got.playing {
			t.Fatalf("before the dwell = %+v, want both still", got)
		}
	}
	started := frames.atSlots(130*time.Millisecond, id, &hover, id, 0, 1)
	if started[0].progress != 0 || !started[0].playing {
		t.Fatalf("the first slot = %+v, want its own pass", started[0])
	}
	if started[1].progress != 0 || !started[1].playing {
		t.Fatalf("the second slot on one identity = %+v, want its own pass", started[1])
	}
	// A held pointer replays neither.
	if got := frames.atSlots(140*time.Millisecond, id, &hover, id, 0, 1); got[0].progress == 0 || got[1].progress == 0 {
		t.Fatalf("a held pointer restarted a slot: %+v", got)
	}
	// Leaving re-arms every slot on the identity.
	frames.atSlots(400*time.Millisecond, ui.ID{}, &hover, id, 0, 1)
	frames.atSlots(410*time.Millisecond, id, &hover, id, 0, 1)
	if got := frames.atSlots(540*time.Millisecond, id, &hover, id, 0, 1); got[0].progress != 0 || !got[0].playing || got[1].progress != 0 || !got[1].playing {
		t.Fatalf("the re-armed slots = %+v, want fresh passes on both", got)
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
