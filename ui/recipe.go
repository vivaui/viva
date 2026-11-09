package ui

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

// A control declared with the theme's look (Input, and the controls that follow it) is a
// base element dressed by a recipe: a pure function of the theme and the control's state
// that gives the fill, border, corners, shadow and padding the shadcn control has. The
// recipe fills in only what the element did not set. A Background, Border, Radius,
// Shadow or Pad called on the element is explicit and replaces the recipe's value, so a
// control is restyled with the calls any element takes, and never needs a variant or a
// copy of the control's behavior to look different.
//
// The state-dependent colors (the focus border, the invalid border, the disabled dimming)
// come from the recipe too, and apply to the fields the element did not set; a Hover,
// Pressed or Focus look applies over them as it does over any element.

// recipeTheme is the theme a themed control's recipe reads: the frame's, or one with the
// element's own font size when it asked for one. It points into the frame, or at scratch the
// caller owns, so reading it does not copy the theme.
func (f *Frame) recipeTheme(n *node, scratch *Theme) *Theme {
	if n.hasOwnTheme() {
		*scratch = f.ownTheme(n)
		return scratch
	}
	return &f.theme
}

// textEntryInsets is the room the shadcn text-entry controls leave between their edge and
// their text: across, the metrics' text inset; down, nothing for a line (which is centered
// in the control's whole height) and a fixed margin for rows that start at the top.
func textEntryInsets(theme *Theme, k kind) (x, y float32) {
	x = theme.Metrics.TextInsetX()
	if k == kindTextarea {
		y = components.TextareaPadY
	}
	return x, y
}

// applyRecipes gives every themed element the measurements its recipe has, before the page
// is laid out: the corners, the border and the padding of its text. Colors depend on the
// state of the frame being painted, so they are filled in by themedDefaults at paint time.
func (f *Frame) applyRecipes() {
	for i := range f.nodes {
		n := &f.nodes[i]
		if n.flags&flagThemed == 0 {
			continue
		}
		switch n.kind {
		case kindInput, kindTextarea:
			var scratch Theme
			theme := f.recipeTheme(n, &scratch)
			if n.flags&flagSetBorder == 0 {
				n.borderWidth = components.TextEntryBorderWidth
			}
			if n.flags&flagHasRadius == 0 {
				n.radius = theme.Metrics.RadiusControl
			}
			if n.flags&flagSetPad == 0 {
				x, y := textEntryInsets(theme, n.kind)
				n.pad = [4]float32{max(y-n.borderWidth, 0), max(x-n.borderWidth, 0), max(y-n.borderWidth, 0), max(x-n.borderWidth, 0)}
			}
			if n.kind == kindTextarea && n.minS[axisY] == 0 {
				n.minS[axisY] = 64 // the official minimum height
			}
		}
	}
}

// themedDefaults fills in, for the state the element is in now, the colors of the resolved
// look the element did not set itself.
func (f *Frame) themedDefaults(n *node, extra *extra, s *resolved) {
	var state gfx.State
	switch n.kind {
	case kindInput:
		snapshot := &f.inputs[n.aux]
		state = gfx.State{Focused: snapshot.Focused || f.focused(n), Invalid: snapshot.Invalid, Disabled: snapshot.Disabled}
	case kindTextarea:
		snapshot := &f.textareas[n.aux]
		state = gfx.State{Focused: snapshot.Focused || f.focused(n), Invalid: snapshot.Invalid, Disabled: snapshot.Disabled}
	default:
		return
	}
	var scratch Theme
	look := components.TextEntryRecipe(*f.recipeTheme(n, &scratch), state)
	if n.flags&flagSetBackground == 0 {
		s.background = look.Fill
	}
	if n.flags&flagSetBorder == 0 {
		s.border = look.Border
	}
	if n.flags&flagSetShadow == 0 {
		s.shadow = look.Shadow
	}
}

// focused reports whether the keyboard is on the element.
func (f *Frame) focused(n *node) bool {
	return f.canvas != nil && !n.id.IsZero() && f.canvas.Focused() == n.id
}

// contentRect is where the text of an element sits: inside its Pad and Border. A themed
// control whose Pad is the recipe's keeps the text area the shadcn control has, which is
// inset from the control's edge by the recipe's own amounts: the border is drawn over the
// control's edge, not taken out of the area the text is centered in.
func (f *Frame) contentRect(n *node) Rect {
	r := n.rect
	if n.flags&flagThemed != 0 && n.flags&flagSetPad == 0 {
		var scratch Theme
		x, y := textEntryInsets(f.recipeTheme(n, &scratch), n.kind)
		return Rect{X: r.X + x, Y: r.Y + y, Width: max(r.Width-2*x, 0), Height: max(r.Height-2*y, 0)}
	}
	left, right := n.inset(axisX)
	top, bottom := n.inset(axisY)
	return Rect{X: r.X + left, Y: r.Y + top, Width: max(r.Width-left-right, 0), Height: max(r.Height-top-bottom, 0)}
}

// inputLine is the height of one line of the text of an Input or InputBase: the element's
// LineHeight when it has one, and otherwise the size of the text plus the seven pixels a line of
// text is given everywhere else. The size is the element's FontSize, or the snapshot's own text
// size, or the theme's.
func (f *Frame) inputLine(n *node) float32 {
	if n.lineH > 0 {
		return n.lineH
	}
	size := n.fontSize
	if size <= 0 {
		size = f.inputs[n.aux].Text.Size
	}
	if size <= 0 {
		size = f.theme.Metrics.FontSizeM
	}
	return size + 7
}
