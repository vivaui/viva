package components

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// A list goes to the side of its trigger with more room and is shortened to that
// room, never covering the trigger: a trigger low in the window with a short
// space below and a larger one above opens up; with more room below it stays
// below even when the room above would hold a few rows.
func TestAListOpensOnTheSideWithMoreRoom(t *testing.T) {
	viewport := gfx.Rect{Width: 400, Height: 400}
	cases := []struct {
		name    string
		trigger gfx.Rect
		above   bool
	}{
		{"room above", gfx.Rect{X: 10, Y: 330, Width: 120, Height: 32}, true},
		{"more room below", gfx.Rect{X: 10, Y: 60, Width: 120, Height: 32}, false},
	}
	for _, c := range cases {
		popup := gfx.Rect{X: c.trigger.X, Width: c.trigger.Width}
		placeList(&popup, c.trigger, 600, viewport)
		if popup.Y < viewport.Y || popup.Y+popup.Height > viewport.Y+viewport.Height {
			t.Fatalf("%s: list %v leaves the window", c.name, popup)
		}
		overlaps := popup.Y < c.trigger.Y+c.trigger.Height && popup.Y+popup.Height > c.trigger.Y
		if overlaps || (popup.Y < c.trigger.Y) != c.above {
			t.Fatalf("%s: list %v against trigger %v, want above=%v without covering it", c.name, popup, c.trigger, c.above)
		}
	}
}
