package interaction

import "github.com/vivaui/viva/ui"

// CursorForTarget is the pointer policy shared by paint and tests: the press
// target wins over hover, and the front-most hit with that ID declares the
// pointer. A label is not a control; it keeps the one its control has. An
// unknown cursor, or a picture that has gone away, falls back to the arrow.
func CursorForTarget(hits []ui.Hit, hovered, pressTarget ui.ID) (ui.PointerCursor, *ui.CursorImage) {
	target := hovered
	if !pressTarget.IsZero() {
		target = pressTarget
	}
	if target.IsZero() {
		return ui.PointerCursorDefault, nil
	}
	for index := len(hits) - 1; index >= 0; index-- {
		hit := hits[index]
		if hit.ID != target || hit.Disabled || hit.IsLabel {
			continue
		}
		if hit.Cursor < ui.PointerCursorDefault || hit.Cursor > ui.PointerCursorImage {
			return ui.PointerCursorDefault, nil
		}
		if hit.Cursor == ui.PointerCursorImage && hit.CursorImage == nil {
			return ui.PointerCursorDefault, nil
		}
		return hit.Cursor, hit.CursorImage
	}
	return ui.PointerCursorDefault, nil
}
