package main

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

func cardOrigin(t *testing.T, driver *app.Driver, text string) (x, y float32) {
	t.Helper()
	for _, label := range driver.Snapshot().Labels {
		if label.Value == text {
			return label.Bounds.X, label.Bounds.Y
		}
	}
	t.Fatalf("no label %q", text)
	return
}

func TestGridFollowsTheWindow(t *testing.T) {
	var canvas gfx.Canvas
	driver, err := app.NewDriver(app.Config{Title: "Layout", Width: 720, Height: 480, Paint: paint}, &canvas)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	_, y1 := cardOrigin(t, driver, "Card 1")
	_, y2 := cardOrigin(t, driver, "Card 2")
	if y1 != y2 {
		t.Fatalf("at 720 wide the first two cards share a row: %v %v", y1, y2)
	}
	if err := driver.Resize(260, 900); err != nil {
		t.Fatal(err)
	}
	_, y1 = cardOrigin(t, driver, "Card 1")
	_, y2 = cardOrigin(t, driver, "Card 2")
	if y2 <= y1 {
		t.Fatalf("at 260 wide the cards stack: %v %v", y1, y2)
	}
}
