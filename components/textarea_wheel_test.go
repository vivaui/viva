package components

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
)

// A wheel over a TextareaLayout scrolls it while it can move in that direction, and reports
// nothing once it is at the end, so the page can scroll instead.
func TestTextareaWheelScrollsUntilItsEnd(t *testing.T) {
	var lines []string
	for index := range 40 {
		lines = append(lines, "row "+string(rune('a'+index%26)))
	}
	text := strings.Join(lines, "\n")
	box := newWrapBox(t, 100)
	box.paint(TextareaSnapshot{Value: text, Caret: 0})
	center := gfx.Point{X: box.layout.content.X + 4, Y: box.layout.content.Y + 4}

	outside := gfx.ScrollEvent{X: box.layout.content.X - 10, Y: center.Y, DeltaY: 30}
	if box.layout.Wheel(outside) {
		t.Fatal("a wheel outside the text scrolled the box")
	}
	up := gfx.ScrollEvent{X: center.X, Y: center.Y, DeltaY: -30}
	if box.layout.Wheel(up) {
		t.Fatal("a wheel up at the top scrolled the box")
	}

	down := gfx.ScrollEvent{X: center.X, Y: center.Y, DeltaY: 30}
	if !box.layout.Wheel(down) {
		t.Fatal("a wheel down over the text did not scroll the box")
	}
	for range 200 {
		box.paint(TextareaSnapshot{Value: text, Caret: 0})
		if !box.layout.Wheel(down) {
			break
		}
	}
	box.paint(TextareaSnapshot{Value: text, Caret: 0})
	if box.layout.Wheel(down) {
		t.Fatal("a wheel down at the end of the text still scrolled the box")
	}
}
