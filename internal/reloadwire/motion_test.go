package reloadwire

import (
	"reflect"
	"testing"
	"time"

	"github.com/vivaui/viva/gfx"
)

func TestPaintRequestCodecCarriesFrameClockAndReducedMotion(t *testing.T) {
	want := PaintRequest{
		Width: 640, Height: 480, Focused: gfx.Item(7), FocusVisible: true,
		Now: uint64(3*time.Second + 250*time.Millisecond), ReduceMotion: true,
	}
	got, err := decodePaintRequest(encodePaintRequest(want))
	if err != nil || got != want {
		t.Fatalf("paint request = %+v, want %+v, err = %v", got, want, err)
	}
}

func TestCanvasCodecCarriesAnimatingFlagToHostCanvas(t *testing.T) {
	data, err := encodeCanvas(CanvasSnapshot{Animating: true})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := decodeCanvas(data)
	if err != nil || !snapshot.Animating {
		t.Fatalf("decoded snapshot = %+v, err = %v", snapshot, err)
	}
	var host gfx.Canvas
	host.BeginFrame()
	ApplyCanvas(&host, snapshot)
	host.EndFrame()
	if !host.Animating() {
		t.Fatal("host canvas must keep requesting frames while the worker animates")
	}

	host.BeginFrame()
	ApplyCanvas(&host, CanvasSnapshot{})
	host.EndFrame()
	if host.Animating() {
		t.Fatal("a settled worker must let the host go idle")
	}
}

// A worker repaints on the host's clock, so a transition progresses with the
// Now values it is sent and the Animating flag falls once it settles.
func TestWorkerPaintAdvancesTransitionsOnHostClock(t *testing.T) {
	var canvas gfx.Canvas
	checked := false
	config := WorkerConfig{Paint: func(canvas *gfx.Canvas, size gfx.Size) {
		target := float32(0)
		if checked {
			target = 1
		}
		position := canvas.Animate(gfx.Item(1), 0, target, gfx.Motion{Duration: 100 * time.Millisecond, Easing: gfx.EaseLinear})
		canvas.Fill(gfx.Rect{X: position * 100, Width: 10, Height: 10}, gfx.Color{Alpha: 1})
	}}
	paint := func(now time.Duration, reduce bool) CanvasSnapshot {
		t.Helper()
		request := frame{kind: messagePaint, payload: encodePaintRequest(PaintRequest{Width: 200, Height: 100, Now: uint64(now), ReduceMotion: reduce})}
		payload, err, _ := handleRequest(&config, &canvas, CanvasSnapshot{}, request, nil)
		if err != nil {
			t.Fatal(err)
		}
		snapshot, err := decodeCanvas(payload)
		if err != nil {
			t.Fatal(err)
		}
		return snapshot
	}

	paint(time.Second, false)
	checked = true
	started := paint(2*time.Second, false)
	half := paint(2*time.Second+50*time.Millisecond, false)
	done := paint(2*time.Second+100*time.Millisecond, false)

	if !started.Animating || started.Rectangles[0].Bounds.X != 0 {
		t.Fatalf("first frame = animating %v at x=%v", started.Animating, started.Rectangles[0].Bounds.X)
	}
	if !half.Animating || half.Rectangles[0].Bounds.X != 50 {
		t.Fatalf("half frame = animating %v at x=%v, want 50", half.Animating, half.Rectangles[0].Bounds.X)
	}
	if done.Animating || done.Rectangles[0].Bounds.X != 100 {
		t.Fatalf("final frame = animating %v at x=%v, want settled at 100", done.Animating, done.Rectangles[0].Bounds.X)
	}

	checked = false
	reduced := paint(3*time.Second, true)
	if reduced.Animating || reduced.Rectangles[0].Bounds.X != 0 {
		t.Fatalf("reduced motion frame = animating %v at x=%v", reduced.Animating, reduced.Rectangles[0].Bounds.X)
	}
	if !reflect.DeepEqual(paint(4*time.Second, false), paint(4*time.Second, false)) {
		t.Fatal("repeat paints at rest must be identical")
	}
}

func TestCanvasCodecCarriesADelayedFrameRequestToHostCanvas(t *testing.T) {
	var worker gfx.Canvas
	worker.BeginFrame()
	worker.RequestFrameIn(125 * time.Millisecond)
	worker.EndFrame()
	snapshot := CaptureCanvas(&worker)
	if snapshot.Animating || snapshot.FrameDelayMs != 125 {
		t.Fatalf("captured %+v, want a 125ms request and no transition", snapshot)
	}
	data, err := encodeCanvas(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeCanvas(data)
	if err != nil || decoded.FrameDelayMs != 125 {
		t.Fatalf("decoded %+v, err = %v", decoded, err)
	}
	var host gfx.Canvas
	host.BeginFrame()
	ApplyCanvas(&host, decoded)
	host.EndFrame()
	if delay, ok := host.NextFrame(); !ok || delay != 125*time.Millisecond || host.Animating() {
		t.Fatalf("host canvas next frame = %v, %v", delay, ok)
	}

	// A sub-millisecond request is not rounded down to "now".
	worker.BeginFrame()
	worker.RequestFrameIn(time.Microsecond)
	worker.EndFrame()
	if snapshot := CaptureCanvas(&worker); snapshot.FrameDelayMs != 1 {
		t.Fatalf("1us request captured as %dms", snapshot.FrameDelayMs)
	}
}

// A click reaches a worker whose paint asks Clicked in the worker's own event
// pass, where its state lives, and the host's next paint does not deliver it again.
func TestWorkerClickIsDeliveredInTheEventPass(t *testing.T) {
	var canvas gfx.Canvas
	var tree gfx.Tree
	save := tree.ID(tree.Feature("page"), "save")
	saved := 0
	config := WorkerConfig{Paint: func(canvas *gfx.Canvas, size gfx.Size) {
		canvas.Hit(save, gfx.Rect{Width: 10, Height: 10})
		if canvas.Clicked(save) {
			saved++
		}
	}}
	paint := func() {
		t.Helper()
		request := frame{kind: messagePaint, payload: encodePaintRequest(PaintRequest{Width: 200, Height: 100})}
		if _, err, _ := handleRequest(&config, &canvas, CanvasSnapshot{}, request, nil); err != nil {
			t.Fatal(err)
		}
	}
	paint()
	var payload encoder
	payload.id(save)
	payload.u32(0) // no modifier
	payload.u8(1)  // a single click
	if _, err, _ := handleRequest(&config, &canvas, CanvasSnapshot{}, frame{kind: messageClick, payload: payload.data}, nil); err != nil {
		t.Fatal(err)
	}
	if saved != 1 {
		t.Fatalf("the worker handled the click %d times, want 1", saved)
	}
	paint()
	if saved != 1 {
		t.Fatalf("a later paint delivered the click again: %d", saved)
	}
}

// A pointer event reaches a worker whose paint asks PointerOn in the worker's own
// event pass, and what a control consumes comes back as handled.
func TestWorkerPointerIsDeliveredInTheEventPass(t *testing.T) {
	var canvas gfx.Canvas
	var tree gfx.Tree
	area := tree.ID(tree.Feature("page"), "area")
	opened := 0
	config := WorkerConfig{Paint: func(canvas *gfx.Canvas, size gfx.Size) {
		canvas.Hit(area, gfx.Rect{Width: 100, Height: 100})
		if event, ok := canvas.PointerOn(area); ok && event.Button == gfx.PointerButtonSecondary {
			opened++
			canvas.Consume()
		}
	}}
	request := frame{kind: messagePaint, payload: encodePaintRequest(PaintRequest{Width: 200, Height: 100})}
	if _, err, _ := handleRequest(&config, &canvas, CanvasSnapshot{}, request, nil); err != nil {
		t.Fatal(err)
	}
	event := gfx.PointerEvent{Kind: gfx.PointerDown, Button: gfx.PointerButtonSecondary, Target: area}
	payload, err, _ := handleRequest(&config, &canvas, CanvasSnapshot{}, frame{kind: messagePointer, payload: encodePointerEvent(event)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	result, err := decodeEventResult(payload)
	if err != nil {
		t.Fatal(err)
	}
	if opened != 1 || !result.Handled {
		t.Fatalf("opened=%d handled=%v, want 1 and true", opened, result.Handled)
	}
}
