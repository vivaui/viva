package interaction

import "github.com/vivaui/viva/gfx"

// Control finds the control a request names. Labels share their control's ID
// and are not controls, and a region declared by Container has no ID.
func Control(hits []gfx.Hit, id gfx.ID) (gfx.Hit, bool) {
	if id.IsZero() {
		return gfx.Hit{}, false
	}
	for index := len(hits) - 1; index >= 0; index-- {
		hit := hits[index]
		if hit.ID == id && !hit.IsLabel && !hit.Block && !hit.Backdrop && !hit.Container {
			return hit, true
		}
	}
	return gfx.Hit{}, false
}
