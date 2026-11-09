package components

import (
	"testing"
	"time"

	"github.com/vivaui/viva/gfx"
)

// caretFrames drives one CaretBlink on a fake clock, one frame per call.
type caretFrames struct {
	canvas *gfx.Canvas
	blink  CaretBlink
	now    time.Duration
}

func newCaretFrames() *caretFrames {
	frames := &caretFrames{canvas: new(gfx.Canvas)}
	frames.canvas.SetClock(func() time.Duration { return frames.now })
	return frames
}

// at runs one frame at now with the given caret anchor and reports whether the
// caret was lit and when the next frame was asked for.
func (frames *caretFrames) at(now time.Duration, anchor int) (lit bool, next time.Duration, scheduled bool) {
	frames.now = now
	frames.canvas.BeginFrame()
	lit = frames.blink.Lit(frames.canvas, anchor)
	frames.canvas.EndFrame()
	next, scheduled = frames.canvas.NextFrame()
	return lit, next, scheduled
}

// A caret starts lit so typing looks immediate, then turns over once per half
// period, and it asks for exactly the frame that changes what is on screen.
func TestCaretBlinkAlternatesAndAsksForTheTurnover(t *testing.T) {
	frames := newCaretFrames()
	start := time.Duration(0)

	if lit, next, ok := frames.at(start, 7); !lit || !ok || next != CaretBlinkHalf {
		t.Fatalf("first frame: lit=%v next=%v scheduled=%v, want lit with the turnover due in %v",
			lit, next, ok, CaretBlinkHalf)
	}
	if lit, _, _ := frames.at(start+CaretBlinkHalf+time.Millisecond, 7); lit {
		t.Fatal("the caret is still lit a half period after it was drawn")
	}
	if lit, _, _ := frames.at(start+2*CaretBlinkHalf+time.Millisecond, 7); !lit {
		t.Fatal("the caret did not come back a full period after it was drawn")
	}

	// A caret that moved restarts: it is lit at once, not inherited from the old
	// position's phase.
	if lit, _, _ := frames.at(start+CaretBlinkHalf+10*time.Millisecond, 8); !lit {
		t.Fatal("a caret that moved did not restart lit")
	}
}

// A window left alone must not keep asking for frames: after the span the caret
// stays lit and the canvas comes to rest.
func TestCaretBlinkRestsAndStopsAsking(t *testing.T) {
	frames := newCaretFrames()
	start := time.Duration(0)
	frames.at(start, 3)

	lit, next, scheduled := frames.at(start+CaretBlinkSpan+time.Hour, 3)
	if !lit {
		t.Fatal("the idle caret is dark; a window left alone must not blink")
	}
	if scheduled {
		t.Fatalf("the idle caret still asks for a frame in %v", next)
	}
	// It stays at rest on the following frames too.
	if _, _, scheduled := frames.at(start+CaretBlinkSpan+2*time.Hour, 3); scheduled {
		t.Fatal("the idle caret asked for another frame")
	}
}

// Reduced motion keeps the caret lit and never asks for frames, so an
// accessibility setting that suppresses animation suppresses this too.
func TestCaretBlinkHonoursReducedMotion(t *testing.T) {
	frames := newCaretFrames()
	frames.canvas.SetReduceMotion(true)

	for offset := time.Duration(0); offset < 3*CaretBlinkHalf; offset += CaretBlinkHalf / 2 {
		lit, _, scheduled := frames.at(offset, 5)
		if !lit {
			t.Fatalf("reduced motion darkened the caret at %v", offset)
		}
		if scheduled {
			t.Fatalf("reduced motion asked for a frame at %v", offset)
		}
	}
}

// Rest starts the phase over, which is what regaining the keyboard looks like.
func TestCaretBlinkRestRestartsLit(t *testing.T) {
	frames := newCaretFrames()
	frames.at(0, 4)
	if lit, _, _ := frames.at(CaretBlinkHalf+time.Millisecond, 4); lit {
		t.Fatal("the caret did not turn over before the rest")
	}
	frames.blink.Rest()
	if lit, _, _ := frames.at(2*CaretBlinkHalf+time.Millisecond, 4); !lit {
		t.Fatal("the caret did not restart lit after the rest")
	}
}

// Outside a running frame there is no clock to blink on, so the caret simply
// stays lit: tests, static previews and a runtime without a frame loop all get
// a caret without a rhythm.
func TestCaretBlinkWithoutAFrameClockStaysLit(t *testing.T) {
	var canvas gfx.Canvas
	var blink CaretBlink
	if !blink.Lit(&canvas, 0) {
		t.Fatal("a caret outside a frame went dark")
	}
	canvas.BeginFrame()
	if !blink.Lit(&canvas, 0) || blink.seen {
		t.Fatal("a frame without a clock started a phase")
	}
	canvas.EndFrame()
	if _, scheduled := canvas.NextFrame(); scheduled {
		t.Fatal("a frame without a clock asked for another one")
	}
}

// Painting the caret every frame must not allocate: it runs for the whole
// session, and a caret that allocates asks the garbage collector to work while
// someone is typing.
func TestCaretBlinkDoesNotAllocateWhenWarm(t *testing.T) {
	frames := newCaretFrames()
	var lit bool
	frames.now = 0
	frames.canvas.BeginFrame()
	frames.blink.Lit(frames.canvas, 1)
	frames.canvas.EndFrame()

	if allocations := testing.AllocsPerRun(100, func() {
		frames.canvas.BeginFrame()
		lit = frames.blink.Lit(frames.canvas, 1)
		frames.canvas.EndFrame()
	}); allocations != 0 {
		t.Fatalf("one caret frame allocated %v times", allocations)
	}
	if !lit {
		t.Fatal("the caret stopped being lit")
	}
}

func BenchmarkCaretBlinkLit(b *testing.B) {
	frames := newCaretFrames()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		frames.canvas.BeginFrame()
		frames.blink.Lit(frames.canvas, 1)
		frames.canvas.EndFrame()
	}
}
