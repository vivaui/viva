package devreload

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	"github.com/vivaui/viva/internal/reloadwire"
	"github.com/vivaui/viva/ui"
)

func TestSupervisorForwardsInputEventsAndResults(t *testing.T) {
	wantPointer := ui.PointerEvent{Kind: ui.PointerMove, Button: ui.PointerButtonPrimary, X: 8, Y: 12, Target: ui.Item(3), Captured: true}
	wantKey := ui.KeyEvent{Key: ui.KeyEnter, Modifiers: ui.ModifierMeta, Target: ui.Item(5), Repeat: true}
	wantShortcut := ui.KeyEvent{Modifiers: ui.ModifierMeta, Target: ui.Item(5), Text: "k"}
	wantScroll := ui.ScrollEvent{
		Modifiers: ui.ModifierAlt, X: 17, Y: 23, DeltaX: 4, DeltaY: -9,
		Viewport: ui.Size{Width: 200, Height: 100}, Target: ui.Item(7), Precise: true,
	}
	pointerResult := ui.EventResult{Focus: ui.Item(3), Handled: true, Repaint: true}
	keyResult := ui.EventResult{Focus: ui.Item(5), Handled: true, FocusVisible: true}
	shortcutResult := ui.EventResult{Handled: true, Repaint: true}
	scrollResult := ui.EventResult{Blur: true, Repaint: true}
	wantTap := ui.ModifierTapEvent{Key: ui.KeyControlLeft, Target: ui.Item(5)}
	tapResult := ui.EventResult{Handled: true}
	var gotTap ui.ModifierTapEvent
	var gotPointer ui.PointerEvent
	var gotKey ui.KeyEvent
	var gotShortcut ui.KeyEvent
	var gotScroll ui.ScrollEvent

	worker, shutdown := startInputWorker(t, reloadwire.WorkerConfig{
		Window:      reloadwire.WindowConfig{Title: "input", Width: 200, Height: 100, HasPointer: true, HasKey: true, HasShortcut: true, HasScroll: true, HasModifierTap: true},
		Pointer:     func(event ui.PointerEvent) ui.EventResult { gotPointer = event; return pointerResult },
		Key:         func(event ui.KeyEvent) ui.EventResult { gotKey = event; return keyResult },
		Shortcut:    func(event ui.KeyEvent) ui.EventResult { gotShortcut = event; return shortcutResult },
		Scroll:      func(event ui.ScrollEvent) ui.EventResult { gotScroll = event; return scrollResult },
		ModifierTap: func(event ui.ModifierTapEvent) ui.EventResult { gotTap = event; return tapResult },
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
	save := ui.Item(3)
	clicks := 0
	worker, shutdown := startInputWorker(t, reloadwire.WorkerConfig{
		Window: reloadwire.WindowConfig{Title: "click", Width: 200, Height: 100, HasClick: true},
		Paint: func(canvas *ui.Canvas, size ui.Size) {
			canvas.Hit(save, ui.Rect{Width: 50, Height: 20})
			if canvas.Clicked(save) {
				clicks++
			}
		},
	})
	defer shutdown()
	supervisor := newSupervisor(worker, reloadwire.PaintRequest{}, reloadwire.CanvasSnapshot{}, io.Discard)
	config := supervisor.hostConfig()

	var canvas ui.Canvas
	config.Paint(&canvas, ui.Size{Width: 200, Height: 100})
	if !canvas.ListensForClicks() {
		t.Fatal("the host does not listen for clicks the worker's paint reads")
	}
	canvas.BeginEventPass(save)
	config.Paint(&canvas, ui.Size{Width: 200, Height: 100})
	canvas.EndEventPass()
	if clicks != 1 {
		t.Fatalf("the worker read the click %d times, want once", clicks)
	}
	config.Paint(&canvas, ui.Size{Width: 200, Height: 100})
	if clicks != 1 {
		t.Fatalf("an ordinary paint delivered the click again: %d", clicks)
	}
}

func TestSupervisorKeepsInputCallbacksAvailableAcrossCapabilityChanges(t *testing.T) {
	calls := 0
	worker, shutdown := startInputWorker(t, reloadwire.WorkerConfig{
		Window:  reloadwire.WindowConfig{Title: "input", Width: 200, Height: 100},
		Pointer: func(ui.PointerEvent) ui.EventResult { calls++; return ui.EventResult{Handled: true} },
		Key:     func(ui.KeyEvent) ui.EventResult { calls++; return ui.EventResult{Handled: true} },
		Scroll:  func(ui.ScrollEvent) ui.EventResult { calls++; return ui.EventResult{Handled: true} },
	})
	defer shutdown()
	supervisor := newSupervisor(worker, reloadwire.PaintRequest{}, reloadwire.CanvasSnapshot{}, io.Discard)
	config := supervisor.hostConfig()

	if config.OnPointer == nil || config.OnKey == nil || config.OnShortcut == nil || config.OnScroll == nil {
		t.Fatal("host input callbacks must remain installed for later worker swaps")
	}
	if got := config.OnPointer(ui.PointerEvent{Kind: ui.PointerDown}); got != (ui.EventResult{}) {
		t.Fatalf("unsupported pointer result = %+v", got)
	}
	if got := config.OnKey(ui.KeyEvent{Key: ui.KeyEnter}); got != (ui.EventResult{}) {
		t.Fatalf("unsupported key result = %+v", got)
	}
	if got := config.OnShortcut(ui.KeyEvent{Modifiers: ui.ModifierControl, Text: "x"}); got != (ui.EventResult{}) {
		t.Fatalf("unsupported shortcut result = %+v", got)
	}
	if got := config.OnScroll(ui.ScrollEvent{DeltaY: 1}); got != (ui.EventResult{}) {
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
