package interaction

import "github.com/vivaui/viva/gfx"

// CursorForTarget is the pointer policy shared by paint and tests: the press
// target wins over hover, and the front-most hit with that ID declares the
// pointer. A label is not a control; it keeps the one its control has. An
// unknown cursor, or a picture that has gone away, falls back to the arrow.
// A hit whose picture depends on the pointer's place is asked for it at at.
func CursorForTarget(canvas *gfx.Canvas, hovered, pressTarget gfx.ID, at gfx.Point) (gfx.PointerCursor, *gfx.CursorImage) {
	hits := canvas.Hits()
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
		picture := hit.CursorImage
		if hit.Cursor == gfx.PointerCursorImage && hit.CursorImageAt != 0 {
			if imageAt := canvas.CursorImageAt(&hit); imageAt != nil {
				picture = imageAt(at)
			}
		}
		if hit.Cursor == gfx.PointerCursorImage && picture == nil {
			return gfx.PointerCursorDefault, nil
		}
		return hit.Cursor, picture
	}
	return gfx.PointerCursorDefault, nil
}
