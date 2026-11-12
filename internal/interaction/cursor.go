package interaction

import "github.com/vivaui/viva/gfx"

// CursorForTarget is the pointer policy shared by paint and tests: the press
// target wins over hover, and the front-most hit with that ID declares the
// pointer. A label is not a control; it keeps the one its control has. An
// unknown cursor, or a picture that has gone away, falls back to the arrow.
func CursorForTarget(hits []gfx.Hit, hovered, pressTarget gfx.ID) (gfx.PointerCursor, *gfx.CursorImage) {
	target := hovered
	if !pressTarget.IsZero() {
		target = pressTarget
	}
	if target.IsZero() {
		return gfx.PointerCursorDefault, nil
	}
	for index := len(hits) - 1; index >= 0; index-- {
		hit := hits[index]
		if hit.ID != target || hit.Disabled || hit.IsLabel {
			continue
		}
		if hit.Cursor < gfx.PointerCursorDefault || hit.Cursor > gfx.PointerCursorImage {
			return gfx.PointerCursorDefault, nil
		}
		if hit.Cursor == gfx.PointerCursorImage && hit.CursorImage == nil {
			return gfx.PointerCursorDefault, nil
		}
		return hit.Cursor, hit.CursorImage
	}
	return gfx.PointerCursorDefault, nil
}
