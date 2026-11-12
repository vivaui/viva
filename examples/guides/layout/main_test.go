package main

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

func TestSaveSitsAtTheBottomRightAndCountsClicks(t *testing.T) {
	m := &model{}
	var canvas gfx.Canvas
	driver, err := app.NewDriver(newConfig(m), &canvas)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	// Save is the last button of the last row: against the right and bottom
	// edges less the 24 pixel padding, wherever the window is.
	if err := driver.Click(420-24-20, 220-24-15); err != nil {
		t.Fatal(err)
	}
	if !m.saved {
		t.Fatal("a click in the bottom right corner did not reach Save")
	}
}
