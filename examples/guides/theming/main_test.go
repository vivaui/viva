package main

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

func TestToggleChangesTheBackground(t *testing.T) {
	m := &model{}
	var canvas gfx.Canvas
	driver, err := app.NewDriver(newConfig(m), &canvas)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	before := driver.Snapshot().Primitives[0].Color
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label == "Toggle theme" {
			b := hit.Bounds
			if err := driver.Click(b.X+b.Width/2, b.Y+b.Height/2); err != nil {
				t.Fatal(err)
			}
		}
	}
	after := driver.Snapshot().Primitives[0].Color
	if !m.dark || before == after {
		t.Fatalf("dark=%v before=%+v after=%+v", m.dark, before, after)
	}
}
