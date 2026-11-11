package interaction

import "github.com/vivaui/viva/gfx"

// TextInputHit returns the painted hit that owns the platform text editor when
// one is focused, and its index in canvas.Hits. A text field and a terminal
// always take it. A combobox takes it only when it names the text geometry: one
// that paints its own query keeps what it draws.
func TextInputHit(canvas *gfx.Canvas, focused gfx.ID) (gfx.Hit, int, bool) {
	if focused.IsZero() {
		return gfx.Hit{}, -1, false
	}
	hits := canvas.Hits()
	for index := range hits {
		hit := hits[index]
		if hit.ID != focused || hit.Disabled {
			continue
		}
		if hit.Role == gfx.RoleTextField || hit.Role == gfx.RoleTerminal || hit.TypeAhead {
			return hit, index, true
		}
		if hit.Role == gfx.RoleCombobox {
			if text, ok := canvas.HitText(&hit); ok && text.Bounds != (gfx.Rect{}) {
				return hit, index, true
			}
		}
	}
	return gfx.Hit{}, -1, false
}
