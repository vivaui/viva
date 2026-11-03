//go:build darwin || windows || linux

package app

import (
	"errors"
	"sync"
	"testing"

	"github.com/vivaui/viva/ui"
)

type fullscreenTestControl struct {
	fakeWindow
	mu       sync.Mutex
	requests []bool
}

func (control *fullscreenTestControl) SetFullscreen(fullscreen bool) error {
	control.mu.Lock()
	control.requests = append(control.requests, fullscreen)
	control.mu.Unlock()
	return nil
}

func (control *fullscreenTestControl) Fullscreen() bool { return false }

func TestFullscreenRequestsAndObservedStateAreDistinct(t *testing.T) {
	control := &fullscreenTestControl{}
	window := &Window{control: control}
	if err := window.SetFullscreen(true); err != nil {
		t.Fatal(err)
	}
	if len(control.requests) != 1 || !control.requests[0] {
		t.Fatal("request was not delegated to the host")
	}
	if window.Fullscreen() {
		t.Fatal("an accepted request must not claim a completed host transition")
	}
	window.fullscreen.Store(true)
	if !window.Fullscreen() {
		t.Fatal("observed state was not published")
	}
	window.closed.Store(true)
	if err := window.SetFullscreen(false); err == nil || window.Fullscreen() {
		t.Fatal("closed window accepted fullscreen or reported a live state")
	}
	var missing *Window
	if err := missing.SetFullscreen(true); err == nil || missing.Fullscreen() {
		t.Fatal("nil window accepted fullscreen or reported a live state")
	}
	unsupported := &Window{control: &fakeWindow{}}
	if err := unsupported.SetFullscreen(true); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("unsupported host: %v", err)
	}
}

func TestFullscreenHostCallbackPublishesBeforeApplicationCallback(t *testing.T) {
	host := &fakeHost{}
	application := &Application{host: host}
	var window *Window
	var states []bool
	window, err := application.OpenWindow(Config{Title: "fullscreen", Width: 320, Height: 200,
		OnFullscreen: func(fullscreen bool) {
			if window.Fullscreen() != fullscreen {
				t.Fatal("Window.Fullscreen is stale in OnFullscreen")
			}
			states = append(states, fullscreen)
		},
	}, &ui.Canvas{})
	if err != nil {
		t.Fatal(err)
	}
	defer window.Close()
	host.windows[0].config.OnFullscreen(true)
	host.windows[0].config.OnFullscreen(false)
	if len(states) != 2 || !states[0] || states[1] {
		t.Fatalf("host transitions = %v", states)
	}
}

func BenchmarkWindowFullscreen(b *testing.B) {
	window := &Window{}
	window.fullscreen.Store(true)
	b.ReportAllocs()
	for b.Loop() {
		if !window.Fullscreen() {
			b.Fatal("lost fullscreen")
		}
	}
}
