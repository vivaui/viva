package gfx

// HoverHint is a noninteractive description of text hidden by truncation. It
// has no pointer target, tab stop or accessibility control. Owner is the page
// that paints its tooltip; bounds are clipped and move with the painted text.
// Side is what SetHoverHintSide set when the hint was declared: where its page
// opens the tooltip, in the page's own numbering.
type HoverHint struct {
	Owner  Scope
	Bounds Rect
	Text   string
	Side   uint8
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
		Owner: owner, Bounds: bounds, Text: text, Side: canvas.hoverHintSide, layer: canvas.currentLayer(),
		scope: canvas.currentFocusScope(), hitAt: uint32(len(canvas.hits)),
	}
	if len(target) > 0 {
		hint.target = target[0]
	}
	canvas.hoverHints = append(canvas.hoverHints, hint)
}

// LabelEllipsis draws value fitted to bounds.Width with an ellipsis and keeps
// the complete value as a hover hint when shortened. The hint is dropped where
// the page leaves its ellipsis tooltips off.
func (canvas *Canvas) LabelEllipsis(value string, bounds Rect, style TextStyle, target ...ID) {
	shown := canvas.FitLine(value, style.Size, style.Weight, bounds.Width)
	canvas.Label(shown, bounds, style)
	if shown != "" && shown != value {
		canvas.HoverHint(0, bounds, value, target...)
	}
}

// LabelOrEllipsis draws a value a control has always clipped at the edge of its
// bounds, as Label does. Where the page shows ellipsis tooltips it ends the
// value with an ellipsis instead, as LabelEllipsis does, since the hint shows
// the rest.
func (canvas *Canvas) LabelOrEllipsis(value string, bounds Rect, style TextStyle, target ...ID) {
	if canvas.hoverHintOwner.IsZero() {
		canvas.Label(value, bounds, style)
		return
	}
	canvas.LabelEllipsis(value, bounds, style, target...)
}

// SetHoverHintOwner sets the page whose text components are painting and returns
// the previous owner, to restore after a nested page. A zero owner disables
// automatic hints: for components painted outside a Page, and where the page
// leaves its EllipsisTooltips off.
func (canvas *Canvas) SetHoverHintOwner(owner Scope) Scope {
	previous := canvas.hoverHintOwner
	canvas.hoverHintOwner = owner
	return previous
}

// HoverHintOwner is the page SetHoverHintOwner set, zero when automatic hints
// are off. A component that shortens text only when the hint can show the rest
// reads it.
func (canvas *Canvas) HoverHintOwner() Scope { return canvas.hoverHintOwner }

// SetHoverHintSide sets the Side of the hints declared next, every hint of
// whatever owner, and returns the previous one to restore. The page that owns a
// hint gives it the edge its tooltip opens beside; zero after Reset.
func (canvas *Canvas) SetHoverHintSide(side uint8) uint8 {
	previous := canvas.hoverHintSide
	canvas.hoverHintSide = side
	return previous
}

// SameText reports whether two hints describe the same visible text. Changes
// to unrelated pointer targets do not restart the tooltip's delay, nor does a
// new Side, which only moves the tooltip.
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
