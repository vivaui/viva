package interaction

import "github.com/vivaui/viva/ui"

// Control finds the control a request names. Labels share their control's ID
// and are not controls, and a region declared by Container has no ID.
func Control(hits []ui.Hit, id ui.ID) (ui.Hit, bool) {
	if id.IsZero() {
		return ui.Hit{}, false
	}
	for index := len(hits) - 1; index >= 0; index-- {
		hit := hits[index]
		if hit.ID == id && !hit.IsLabel && !hit.Block && !hit.Backdrop && !hit.Container {
			return hit, true
		}
	}
	return ui.Hit{}, false
}
