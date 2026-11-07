//go:build darwin || windows || linux

package app

import (
	"errors"
	"testing"

	"github.com/vivaui/viva/internal/platform"
)

type topmostTestControl struct {
	fakeWindow
	requests []bool
	err      error
}

func (control *topmostTestControl) SetAlwaysOnTop(enabled bool) error {
	control.requests = append(control.requests, enabled)
	return control.err
}

func TestWindowAlwaysOnTopChangesOnlyAfterSuccessfulRequest(t *testing.T) {
	control := &topmostTestControl{}
	window := &Window{control: control}
	if window.AlwaysOnTop() {
		t.Fatal("new window starts topmost")
	}
	denied := errors.New("window manager denied request")
	control.err = denied
	if err := window.SetAlwaysOnTop(true); !errors.Is(err, denied) || window.AlwaysOnTop() {
		t.Fatalf("rejected topmost request = %v, state=%v", err, window.AlwaysOnTop())
	}
	control.err = nil
	if err := window.SetAlwaysOnTop(true); err != nil || !window.AlwaysOnTop() {
		t.Fatalf("accepted topmost request = %v, state=%v", err, window.AlwaysOnTop())
	}
	if err := window.SetAlwaysOnTop(true); err != nil || len(control.requests) != 3 {
		t.Fatalf("a repeated request must still reach the host, got %v, requests=%v", err, control.requests)
	}
	if err := window.SetAlwaysOnTop(false); err != nil || window.AlwaysOnTop() {
		t.Fatalf("clear topmost = %v, state=%v", err, window.AlwaysOnTop())
	}
	control.err = platform.ErrAlwaysOnTopUnsupported
	if err := window.SetAlwaysOnTop(true); !errors.Is(err, ErrUnsupported) || window.AlwaysOnTop() {
		t.Fatalf("unsupported native window = %v, state=%v", err, window.AlwaysOnTop())
	}
	window.closed.Store(true)
	if err := window.SetAlwaysOnTop(true); err == nil || window.AlwaysOnTop() {
		t.Fatalf("closed window accepted request = %v", err)
	}
	if err := (*Window)(nil).SetAlwaysOnTop(true); err == nil {
		t.Fatal("nil window accepted request")
	}
	unsupported := &Window{control: &fakeWindow{}}
	if err := unsupported.SetAlwaysOnTop(true); !errors.Is(err, ErrUnsupported) || unsupported.AlwaysOnTop() {
		t.Fatalf("unsupported control = %v, state=%v", err, unsupported.AlwaysOnTop())
	}
}
