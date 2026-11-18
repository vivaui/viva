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
func (f *frame) recipeTheme(n *node, scratch *Theme) *Theme {
	if n.hasOwnTheme() {
		*scratch = f.ownTheme(n)
		return scratch
	}
	return &f.theme
}

// textEntryInsets is the room the shadcn text-entry controls leave between their edge and
// their text: across, the metrics' text inset; down, nothing for a line (which is centered
// in the control's whole height) and a fixed margin for rows that start at the top.
func textEntryInsets(theme *Theme, k Kind) (x, y float32) {
	x = theme.Metrics.TextInsetX()
	if k == kindTextarea {
		y = components.TextareaPadY
	}
	return x, y
}

// textEntryLook is the recipe a themed Input or Textarea is dressed with: the
// page's Styles.TextEntry when it set one, else the theme's.
func (f *frame) textEntryLook(theme *Theme, state gfx.State) components.TextEntryLook {
	if f.styles != nil && f.styles.TextEntry != nil {
		return f.styles.TextEntry(*theme, state)
	}
	return components.TextEntryRecipe(*theme, state)
}

// applyRecipes gives every themed element the measurements its recipe has, before the page
// is laid out: the corners, the border and the padding of its text. Colors depend on the
// state of the frame being painted, so they are filled in by themedDefaults at paint time.
func (f *frame) applyRecipes() {
	for i := range f.Nodes {
		n := &f.Nodes[i]
		if n.Flags&flagThemed == 0 {
			continue
		}
		switch n.Kind {
		case kindInput, kindTextarea:
			var scratch Theme
			theme := f.recipeTheme(n, &scratch)
			look := f.textEntryLook(theme, gfx.State{})
			if n.Flags&flagSetBorder == 0 {
				n.BorderWidth = look.BorderWidth
			}
			if n.Flags&flagHasRadius == 0 {
				n.Radius = look.Radius
			}
			if n.Flags&flagSetPad == 0 {
				x, y := textEntryInsets(theme, n.Kind)
				n.Pad = [4]float32{max(y-n.BorderWidth, 0), max(x-n.BorderWidth, 0), max(y-n.BorderWidth, 0), max(x-n.BorderWidth, 0)}
			}
			if n.Kind == kindTextarea && n.MinS[axisY] == 0 {
				n.MinS[axisY] = 64 // the official minimum height
			}
		}
	}
}

// themedDefaults fills in, for the state the element is in now, the colors of the resolved
// look the element did not set itself.
func (f *frame) themedDefaults(n *node, extra *extra, s *resolved) {
	var state gfx.State
	switch n.Kind {
	case kindInput:
		snapshot := &widgetsOf(f).inputs[n.Aux]
		state = gfx.State{Focused: snapshot.Focused || f.focused(n), Invalid: snapshot.Invalid, Disabled: snapshot.Disabled}
	case kindTextarea:
		snapshot := &widgetsOf(f).textareas[n.Aux]
		state = gfx.State{Focused: snapshot.Focused || f.focused(n), Invalid: snapshot.Invalid, Disabled: snapshot.Disabled}
	default:
		return
	}
	var scratch Theme
	look := f.textEntryLook(f.recipeTheme(n, &scratch), state)
	if n.Flags&flagSetBackground == 0 {
		s.Background = look.Fill
	}
	if n.Flags&flagSetBorder == 0 {
		s.border = look.Border
	}
	if n.Flags&flagSetShadow == 0 {
		s.shadow = look.Shadow
	}
}

// focused reports whether the keyboard is on the element.
func (f *frame) focused(n *node) bool {
	return f.Surface != nil && !n.ID.IsZero() && f.Surface.Focused() == n.ID
}

// contentRect is where the text of an element sits: inside its Pad and Border. A themed
// control whose Pad is the recipe's keeps the text area the shadcn control has, which is
// inset from the control's edge by the recipe's own amounts: the border is drawn over the
// control's edge, not taken out of the area the text is centered in.
func (f *frame) contentRect(n *node) Rect {
	r := n.rect
	if n.Flags&flagThemed != 0 && n.Flags&flagSetPad == 0 {
		var scratch Theme
		x, y := textEntryInsets(f.recipeTheme(n, &scratch), n.Kind)
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
func (f *frame) inputLine(n *node) float32 {
	if n.LineH > 0 {
		return n.LineH
	}
	size := n.FontSize
	if size <= 0 {
		size = widgetsOf(f).inputs[n.Aux].Text.Size
	}
	if size <= 0 {
		size = f.theme.Metrics.FontSizeM
	}
	return size + 7
}
