package devreload

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/reloadwire"
)

func TestSupervisorForwardsInputEventsAndResults(t *testing.T) {
	wantPointer := gfx.PointerEvent{Kind: gfx.PointerMove, Button: gfx.PointerButtonPrimary, X: 8, Y: 12, Target: gfx.Item(3), Captured: true}
	wantKey := gfx.KeyEvent{Key: gfx.KeyEnter, Modifiers: gfx.ModifierMeta, Target: gfx.Item(5), Repeat: true}
	wantShortcut := gfx.KeyEvent{Modifiers: gfx.ModifierMeta, Target: gfx.Item(5), Text: "k"}
	wantScroll := gfx.ScrollEvent{
		Modifiers: gfx.ModifierAlt, X: 17, Y: 23, DeltaX: 4, DeltaY: -9,
		Viewport: gfx.Size{Width: 200, Height: 100}, Target: gfx.Item(7), Precise: true,
	}
	pointerResult := gfx.EventResult{Focus: gfx.Item(3), Handled: true, Repaint: true}
	keyResult := gfx.EventResult{Focus: gfx.Item(5), Handled: true, FocusVisible: true}
	shortcutResult := gfx.EventResult{Handled: true, Repaint: true}
	scrollResult := gfx.EventResult{Blur: true, Repaint: true}
	wantTap := gfx.ModifierTapEvent{Key: gfx.KeyControlLeft, Target: gfx.Item(5)}
	tapResult := gfx.EventResult{Handled: true}
	var gotTap gfx.ModifierTapEvent
	var gotPointer gfx.PointerEvent
	var gotKey gfx.KeyEvent
	var gotShortcut gfx.KeyEvent
	var gotScroll gfx.ScrollEvent

	worker, shutdown := startInputWorker(t, reloadwire.WorkerConfig{
		Window:      reloadwire.WindowConfig{Title: "input", Width: 200, Height: 100, HasPointer: true, HasKey: true, HasShortcut: true, HasScroll: true, HasModifierTap: true},
		Pointer:     func(event gfx.PointerEvent) gfx.EventResult { gotPointer = event; return pointerResult },
		Key:         func(event gfx.KeyEvent) gfx.EventResult { gotKey = event; return keyResult },
		Shortcut:    func(event gfx.KeyEvent) gfx.EventResult { gotShortcut = event; return shortcutResult },
		Scroll:      func(event gfx.ScrollEvent) gfx.EventResult { gotScroll = event; return scrollResult },
		ModifierTap: func(event gfx.ModifierTapEvent) gfx.EventResult { gotTap = event; return tapResult },
	})
	defer shutdown()
	supervisor := newSupervisor(worker, reloadwire.PaintRequest{}, reloadwire.CanvasSnapshot{}, io.Discard)
	config := supervisor.hostConfig()

	if got := config.OnPointer(wantPointer); got != pointerResult {
		t.Fatalf("pointer result = %+v, want %+v", got, pointerResult)
	}
	if got := config.OnKey(wantKey); got != keyResult {
		t.Fatalf("key result = %+v, want %+v", got, keyResult)
	}
	if got := config.OnShortcut(wantShortcut); got != shortcutResult {
		t.Fatalf("shortcut result = %+v, want %+v", got, shortcutResult)
	}
	if got := config.OnScroll(wantScroll); got != scrollResult {
		t.Fatalf("scroll result = %+v, want %+v", got, scrollResult)
	}
	if got := config.OnModifierTap(wantTap); got != tapResult || gotTap != wantTap {
		t.Fatalf("modifier tap = %+v / %+v, want %+v / %+v", got, gotTap, tapResult, wantTap)
	}
	if gotPointer != wantPointer || gotKey != wantKey || gotShortcut != wantShortcut || gotScroll != wantScroll {
		t.Fatalf("worker events = (%+v, %+v, %+v, %+v)", gotPointer, gotKey, gotShortcut, gotScroll)
	}
}

// The host cannot see which controls ask Clicked: it declares that it listens,
// and carries the event pass of a click to the worker, where the state lives.
func TestSupervisorCarriesTheClickEventPassToTheWorker(t *testing.T) {
	save := gfx.Item(3)
	clicks := 0
	worker, shutdown := startInputWorker(t, reloadwire.WorkerConfig{
		Window: reloadwire.WindowConfig{Title: "click", Width: 200, Height: 100, HasClick: true},
		Paint: func(canvas *gfx.Canvas, size gfx.Size) {
			canvas.Hit(save, gfx.Rect{Width: 50, Height: 20})
			if canvas.Clicked(save) {
				clicks++
			}
		},
	})
	defer shutdown()
	supervisor := newSupervisor(worker, reloadwire.PaintRequest{}, reloadwire.CanvasSnapshot{}, io.Discard)
	config := supervisor.hostConfig()

	var canvas gfx.Canvas
	config.Paint(&canvas, gfx.Size{Width: 200, Height: 100})
	if !canvas.ListensForClicks() {
		t.Fatal("the host does not listen for clicks the worker's paint reads")
	}
	canvas.BeginEventPass(save, gfx.ClickInfo{})
	config.Paint(&canvas, gfx.Size{Width: 200, Height: 100})
	canvas.EndEventPass()
	if clicks != 1 {
		t.Fatalf("the worker read the click %d times, want once", clicks)
	}
	config.Paint(&canvas, gfx.Size{Width: 200, Height: 100})
	if clicks != 1 {
		t.Fatalf("an ordinary paint delivered the click again: %d", clicks)
	}
}

func TestSupervisorKeepsInputCallbacksAvailableAcrossCapabilityChanges(t *testing.T) {
	calls := 0
	worker, shutdown := startInputWorker(t, reloadwire.WorkerConfig{
		Window:  reloadwire.WindowConfig{Title: "input", Width: 200, Height: 100},
		Pointer: func(gfx.PointerEvent) gfx.EventResult { calls++; return gfx.EventResult{Handled: true} },
		Key:     func(gfx.KeyEvent) gfx.EventResult { calls++; return gfx.EventResult{Handled: true} },
		Scroll:  func(gfx.ScrollEvent) gfx.EventResult { calls++; return gfx.EventResult{Handled: true} },
	})
	defer shutdown()
	supervisor := newSupervisor(worker, reloadwire.PaintRequest{}, reloadwire.CanvasSnapshot{}, io.Discard)
	config := supervisor.hostConfig()

	if config.OnPointer == nil || config.OnKey == nil || config.OnShortcut == nil || config.OnScroll == nil {
		t.Fatal("host input callbacks must remain installed for later worker swaps")
	}
	if got := config.OnPointer(gfx.PointerEvent{Kind: gfx.PointerDown}); got != (gfx.EventResult{}) {
		t.Fatalf("unsupported pointer result = %+v", got)
	}
	if got := config.OnKey(gfx.KeyEvent{Key: gfx.KeyEnter}); got != (gfx.EventResult{}) {
		t.Fatalf("unsupported key result = %+v", got)
	}
	if got := config.OnShortcut(gfx.KeyEvent{Modifiers: gfx.ModifierControl, Text: "x"}); got != (gfx.EventResult{}) {
		t.Fatalf("unsupported shortcut result = %+v", got)
	}
	if got := config.OnScroll(gfx.ScrollEvent{DeltaY: 1}); got != (gfx.EventResult{}) {
		t.Fatalf("unsupported scroll result = %+v", got)
	}
	if calls != 0 {
		t.Fatalf("unadvertised worker handlers called %d times", calls)
	}
}

func startInputWorker(t *testing.T, config reloadwire.WorkerConfig) (*workerProcess, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(reloadwire.AddressEnvironment, listener.Addr().String())
	t.Setenv(reloadwire.TokenEnvironment, "supervisor-input")
	serveDone := make(chan error, 1)
	go func() { serveDone <- reloadwire.Serve(config) }()
	connection, err := listener.Accept()
	if err != nil {
		listener.Close()
		t.Fatal(err)
	}
	hello, client, err := reloadwire.Accept(connection, "supervisor-input")
	if err != nil {
		listener.Close()
		t.Fatal(err)
	}
	worker := &workerProcess{client: client, config: hello.Config}
	return worker, func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		err := client.Shutdown(ctx)
		cancel()
		if err != nil {
			t.Errorf("shutdown input worker: %v", err)
		}
		if err := <-serveDone; err != nil {
			t.Errorf("serve input worker: %v", err)
		}
		_ = client.Close()
		_ = listener.Close()
	}
}
