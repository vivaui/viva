package main

import (
	"strings"
	"testing"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

func TestBackgroundResultReachesTheWindow(t *testing.T) {
	slowWork = func() string { time.Sleep(10 * time.Millisecond); return "done" }
	var invalidator app.Invalidator
	var canvas gfx.Canvas
	driver, err := app.NewDriver(newConfig(&model{status: "idle"}, &invalidator), &canvas)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	for _, hit := range driver.Snapshot().Hits {
		if hit.Label == "Refresh" {
			b := hit.Bounds
			if err := driver.Click(b.X+b.Width/2, b.Y+b.Height/2); err != nil {
				t.Fatal(err)
			}
		}
	}
	select {
	case <-driver.Wakes():
	case <-time.After(2 * time.Second):
		t.Fatal("the worker never woke the window")
	}
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, label := range driver.Snapshot().Labels {
		found = found || strings.Contains(label.Value, "done")
	}
	if !found {
		t.Fatal("the result did not reach the page")
	}
}
