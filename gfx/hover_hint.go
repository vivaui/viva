package gfx

// HoverHint is a noninteractive description of text hidden by truncation. It
// has no pointer target, tab stop or accessibility control. Owner is the page
// that paints its tooltip; bounds are clipped and move with the painted text.
type HoverHint struct {
	Owner  Scope
	Bounds Rect
	Text   string
	target ID
	layer  uint16
	scope  ID
	hitAt  uint32
}

// HoverHint declares full text for an ellipsized label or group of labels. It
// does not change pointer routing. A Page paints hints belonging to its owner.
func (canvas *Canvas) HoverHint(owner Scope, bounds Rect, text string, target ...ID) {
	if owner.IsZero() {
		owner = canvas.hoverHintOwner
	}
	if owner.IsZero() || text == "" {
		return
	}
	if clip, clipped := canvas.clipSnapshot(); clipped {
		bounds = intersectCanvasRect(bounds, clip)
	}
	if bounds.Width <= 0 || bounds.Height <= 0 {
		return
	}
	hint := HoverHint{
		Owner: owner, Bounds: bounds, Text: text, layer: canvas.currentLayer(),
		scope: canvas.currentFocusScope(), hitAt: uint32(len(canvas.hits)),
	}
	if len(target) > 0 {
		hint.target = target[0]
	}
	canvas.hoverHints = append(canvas.hoverHints, hint)
}

// SetHoverHintOwner sets the page whose text components are painting and returns
// the previous owner, to restore after a nested page. A zero owner disables
// automatic hints for components painted outside a Page.
func (canvas *Canvas) SetHoverHintOwner(owner Scope) Scope {
	previous := canvas.hoverHintOwner
	canvas.hoverHintOwner = owner
	return previous
}

// SameText reports whether two hints describe the same visible text. Changes
// to unrelated pointer targets do not restart the tooltip's delay.
func (hint HoverHint) SameText(other HoverHint) bool {
	return hint.Owner == other.Owner && hint.Bounds == other.Bounds && hint.Text == other.Text &&
		hint.layer == other.layer && hint.scope == other.scope && hint.target == other.target
}

// HoverHintAt returns the frontmost hint owned by owner under the pointer.
// Later controls and opaque overlays cover it just as they cover its text.
func (canvas *Canvas) HoverHintAt(owner Scope, x, y float32) HoverHint {
	var best HoverHint
	for i := len(canvas.hoverHints) - 1; i >= 0; i-- {
		hint := canvas.hoverHints[i]
		if hint.Owner != owner || !hintContains(hint.Bounds, x, y) ||
			!canvas.trappedScope.IsZero() && hint.scope != canvas.trappedScope ||
			best.Text != "" && hint.layer <= best.layer {
			continue
		}
		covered := false
		for k := len(canvas.hits) - 1; k >= 0; k-- {
			hit := &canvas.hits[k]
			if hit.Container || hit.IsLabel || !hint.target.IsZero() && hit.ID == hint.target || hit.Layer < hint.layer ||
				hit.Layer == hint.layer && uint32(k) < hint.hitAt || !hintContains(hit.Bounds, x, y) {
				continue
			}
			covered = true
			break
		}
		if !covered {
			best = hint
		}
	}
	return best
}

func hintContains(r Rect, x, y float32) bool {
	return x >= r.X && y >= r.Y && x < r.X+r.Width && y < r.Y+r.Height
}
