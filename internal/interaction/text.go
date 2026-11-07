package interaction

import "github.com/vivaui/viva/ui"

// TextInputHit returns the painted hit that owns the platform text editor when
// one is focused. A text field and a terminal always take it. A combobox takes
// it only when it names the text geometry: one that paints its own query keeps
// what it draws.
func TextInputHit(hits []ui.Hit, focused ui.ID) (ui.Hit, bool) {
	if focused.IsZero() {
		return ui.Hit{}, false
	}
	for _, hit := range hits {
		if hit.ID != focused || hit.Disabled {
			continue
		}
		if hit.Role == ui.RoleTextField || hit.Role == ui.RoleTerminal || hit.TypeAhead ||
			hit.Role == ui.RoleCombobox && hit.TextBounds != (ui.Rect{}) {
			return hit, true
		}
	}
	return ui.Hit{}, false
}
