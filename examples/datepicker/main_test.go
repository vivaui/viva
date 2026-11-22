package main

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

// TestPageShowsThePickers paints the whole demo and opens one panel, so a
// broken example fails here rather than on someone's screen.
func TestPageShowsThePickers(t *testing.T) {
	m := newModel()
	page := ui.NewPage(ui.PageConfig{View: m.page})
	invalidator := new(app.Invalidator)
	driver, err := app.NewDriver(ui.Configure(page, app.Config{Width: 1000, Height: 720, Invalidator: invalidator}), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	has := func(prefix string) bool {
		for _, hit := range driver.Snapshot().Hits {
			if strings.HasPrefix(hit.Label, prefix) {
				return true
			}
		}
		return false
	}
	for _, prefix := range []string{"日粒度 · 空值", "日粒度 · 已选", "月粒度, "} {
		if !has(prefix) {
			t.Fatalf("the demo does not show a field labeled %q", prefix)
		}
	}
	for _, hit := range driver.Snapshot().Hits {
		if strings.HasPrefix(hit.Label, "日粒度 · 已选") {
			continue
		}
		if err := driver.Click(hit.Bounds.X+80, hit.Bounds.Y+hit.Bounds.Height/2); err != nil {
			t.Fatal(err)
		}
		invalidator.Invalidate()
		if err := driver.Pump(); err != nil {
			t.Fatal(err)
		}
		break
	}
	if !has("2026年11月") {
		t.Fatal("the day panel did not show its month")
	}
}
