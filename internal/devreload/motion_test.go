package devreload

import (
	"io"
	"net"
	"testing"
	"time"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/reloadwire"
)

// startMotionWorker serves an in-process worker whose paint callback runs one
// linear 100ms transition toward the returned target pointer.
func startMotionWorker(t *testing.T, target *float32) *workerProcess {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	t.Setenv(reloadwire.AddressEnvironment, listener.Addr().String())
	t.Setenv(reloadwire.TokenEnvironment, "motion")
	go reloadwire.Serve(reloadwire.WorkerConfig{
		Window: reloadwire.WindowConfig{Title: "motion", Width: 200, Height: 100},
		Paint: func(canvas *gfx.Canvas, size gfx.Size) {
			x := canvas.Animate(gfx.Item(1), 0, *target, gfx.Motion{Duration: 100 * time.Millisecond, Easing: gfx.EaseLinear})
			canvas.Fill(gfx.Rect{X: x * 100, Width: 10, Height: 10}, gfx.Color{Alpha: 1})
		},
	})
	connection, err := listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	hello, client, err := reloadwire.Accept(connection, "motion")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close() })
	return &workerProcess{client: client, config: hello.Config}
}

// The host owns the clock and the frame loop; the worker owns transition
// state. The host canvas must mirror the worker's progress and keep asking for
// frames exactly while the worker reports an unfinished transition.
func TestHostCanvasFollowsWorkerTransitionsOnItsOwnClock(t *testing.T) {
	target := float32(0)
	worker := startMotionWorker(t, &target)
	supervisor := newSupervisor(worker, reloadwire.PaintRequest{}, reloadwire.CanvasSnapshot{}, io.Discard)

	var host gfx.Canvas
	var hostNow time.Duration
	host.SetClock(func() time.Duration { return hostNow })
	frame := func(at time.Duration) float32 {
		hostNow = at
		host.BeginFrame()
		supervisor.paint(&host, gfx.Size{Width: 200, Height: 100})
		host.EndFrame()
		return host.Rectangles()[0].Bounds.X
	}

	if x := frame(time.Second); x != 0 || host.Animating() {
		t.Fatalf("rest: x=%v animating=%v", x, host.Animating())
	}
	target = 1
	if x := frame(2 * time.Second); x != 0 || !host.Animating() {
		t.Fatalf("start: x=%v animating=%v", x, host.Animating())
	}
	if x := frame(2*time.Second + 50*time.Millisecond); x != 50 || !host.Animating() {
		t.Fatalf("midpoint: x=%v animating=%v", x, host.Animating())
	}
	if x := frame(2*time.Second + 100*time.Millisecond); x != 100 || host.Animating() {
		t.Fatalf("settled: x=%v animating=%v", x, host.Animating())
	}
}

func TestHostFrameLoopStopsWhenWorkerSnapshotIsStale(t *testing.T) {
	target := float32(0)
	worker := startMotionWorker(t, &target)
	supervisor := newSupervisor(worker, reloadwire.PaintRequest{}, reloadwire.CanvasSnapshot{}, io.Discard)
	var host gfx.Canvas
	var hostNow time.Duration
	host.SetClock(func() time.Duration { return hostNow })
	paintHost := func(at time.Duration) {
		hostNow = at
		host.BeginFrame()
		supervisor.paint(&host, gfx.Size{Width: 200, Height: 100})
		host.EndFrame()
	}
	paintHost(time.Second)
	target = 1
	paintHost(2 * time.Second)
	if !host.Animating() {
		t.Fatal("worker transition should be in flight")
	}

	// Lose the worker: repainting the last snapshot must not spin the loop.
	supervisor.mu.Lock()
	supervisor.active = nil
	supervisor.mu.Unlock()
	paintHost(2*time.Second + 10*time.Millisecond)
	if host.Animating() {
		t.Fatal("a stale snapshot kept the host frame loop alive")
	}
}
