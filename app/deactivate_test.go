//go:build darwin || windows || linux

package app

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// Deactivate is the window's, not the keyboard's: the keys that were down are
// released before the application hears about it, so what it saves or hides sees
// no key still held.
func TestDeactivateReleasesKeysBeforeItsCallback(t *testing.T) {
	var order []string
	probe := &keyProbe{releases: true}
	config := probe.config()
	key := config.OnKey
	config.OnKey = func(event gfx.KeyEvent) gfx.EventResult {
		if event.Release {
			order = append(order, "release")
		}
		return key(event)
	}
	calls := 0
	config.OnDeactivate = func() gfx.EventResult {
		calls++
		order = append(order, "deactivate")
		return gfx.EventResult{Repaint: true}
	}
	driver, err := NewDriver(config, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	if err := driver.Click(100, 100); err != nil {
		t.Fatal(err)
	}
	probe.keys = nil
	if err := driver.KeyDown(gfx.KeyUnknown, "a"); err != nil {
		t.Fatal(err)
	}
	order = nil
	if err := driver.DeactivateWindow(); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("OnDeactivate ran %d times, want 1", calls)
	}
	if len(order) != 2 || order[0] != "release" || order[1] != "deactivate" {
		t.Fatalf("the order was %v, want the held key released, then the callback", order)
	}
}

// Losing the keyboard inside the window is not a deactivation: a page or a view
// taking the keys releases them without telling the application the window is gone.
func TestLosingTheKeyboardInsideTheWindowDoesNotDeactivate(t *testing.T) {
	probe := &keyProbe{releases: true}
	config := probe.config()
	calls := 0
	config.OnDeactivate = func() gfx.EventResult {
		calls++
		return gfx.EventResult{}
	}
	driver, err := NewDriver(config, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	if err := driver.Click(100, 100); err != nil {
		t.Fatal(err)
	}
	if err := driver.KeyDown(gfx.KeyUnknown, "a"); err != nil {
		t.Fatal(err)
	}
	if err := driver.LoseFocus(); err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatalf("losing the keyboard called OnDeactivate %d times, want none", calls)
	}
}

// A window with no key handling still reports its deactivation.
func TestDeactivateRunsWithoutKeys(t *testing.T) {
	calls := 0
	driver, err := NewDriver(Config{
		Width: 200, Height: 120,
		Paint: func(canvas *gfx.Canvas, size gfx.Size) {},
		OnDeactivate: func() gfx.EventResult {
			calls++
			return gfx.EventResult{}
		},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	if err := driver.DeactivateWindow(); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("OnDeactivate ran %d times, want 1", calls)
	}
}
