package base

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/style"
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
func (f *Frame) RecipeTheme(n *Node, scratch *Theme) *Theme {
	if n.HasOwnTheme() {
		*scratch = f.OwnTheme(n)
		return scratch
	}
	return &f.Palette
}

// applyRecipes gives every themed element the measurements its recipe has, before the page
// is laid out: the corners, the border and the padding of its text. Colors depend on the
// state of the frame being painted, so they are filled in by themedDefaults at paint time.
func (f *Frame) applyRecipes() {
	for i := range f.Nodes {
		n := &f.Nodes[i]
		if n.Flags&flagThemed == 0 {
			continue
		}
		switch n.Kind {
		case kindInput, kindTextarea:
			theme := f.RecipeTheme(n, &f.scratch.theme)
			look := f.TextEntryLook(theme, gfx.State{})
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
			if n.Kind == kindTextarea && n.MinS[AxisY] == 0 {
				n.MinS[AxisY] = 64 // the official minimum height
			}
		case kindButton:
			f.applyButtonRecipe(n)
		case kindBadge:
			theme := f.RecipeTheme(n, &f.scratch.theme)
			height, padding, _, fontSize := style.BadgeGeometry(theme.Metrics)
			if d := n.Size[AxisY]; d.Kind == dimPx {
				height = d.Value
			}
			if n.FontSize > 0 {
				fontSize = n.FontSize
			}
			if n.Flags&flagHasRadius == 0 {
				n.Radius = f.badgeLook(n, theme, gfx.State{}).Radius
			}
			if n.PadSet&padRight == 0 {
				n.Pad[1] = padding
			}
			if n.PadSet&padLeft == 0 {
				n.Pad[3] = padding
			}
			if n.Flags&flagChildren != 0 {
				weight := gfx.WeightMedium
				if n.WeightSet || n.Weight != 0 {
					weight = n.Weight
				}
				f.buttonTextDefaults(n, fontSize, weight, n.LineH)
			}
			if n.Size[AxisY].Kind == dimAuto && n.MinS[AxisY] == 0 {
				n.MinS[AxisY] = height
			}
		case kindToggle:
			theme := f.RecipeTheme(n, &f.scratch.theme)
			height, padding, fontSize := style.ToggleGeometry(theme.Metrics, n.ButtonSize)
			if n.FontSize > 0 {
				fontSize = n.FontSize
			}
			if n.Flags&flagHasRadius == 0 {
				n.Radius = f.toggleLook(n, theme, gfx.State{}).Radius
			}
			if n.PadSet&padRight == 0 {
				n.Pad[1] = padding
			}
			if n.PadSet&padLeft == 0 {
				n.Pad[3] = padding
			}
			if n.Flags&flagChildren != 0 {
				weight := gfx.WeightMedium
				if n.WeightSet || n.Weight != 0 {
					weight = n.Weight
				}
				f.buttonTextDefaults(n, fontSize, weight, n.LineH)
			}
			// shadcn's min-w: a toggle is at least as wide as it is tall.
			if n.Size[AxisX].Kind == dimAuto && n.MinS[AxisX] == 0 {
				n.MinS[AxisX] = height
			}
			if n.Size[AxisY].Kind == dimAuto && n.MinS[AxisY] == 0 {
				n.MinS[AxisY] = height
			}
		case kindTabs:
			theme := f.RecipeTheme(n, &f.scratch.theme)
			_, listPadding, triggerHeight, _, lineGap, _, _ := style.TabsGeometry(theme.Metrics)
			t := &WidgetsOf(f).tabs[n.Aux]
			gap := float32(0)
			if t.line {
				gap = lineGap
			}
			if n.Gap != 0 {
				gap = n.Gap
			}
			n.Gap = gap
			if n.PadSet&padTop == 0 {
				n.Pad[0] = listPadding
			}
			if n.PadSet&padRight == 0 {
				n.Pad[1] = listPadding
			}
			if n.PadSet&padBottom == 0 {
				n.Pad[2] = listPadding
			}
			if n.PadSet&padLeft == 0 {
				n.Pad[3] = listPadding
			}
			if n.Flags&flagHasRadius == 0 {
				n.Radius = f.tabsLook(n, theme).Radius
			}
			if n.Size[AxisY].Kind == dimAuto && n.MinS[AxisY] == 0 {
				n.MinS[AxisY] = triggerHeight + n.Pad[0] + n.Pad[2]
			}
		case kindSelect:
			theme := f.RecipeTheme(n, &f.scratch.theme)
			d := &WidgetsOf(f).selects[n.Aux]
			placeholder := d.selected < 0 || d.selected >= len(d.labels)
			surface := style.TextEntryLook{}
			if SelectLookFn != nil {
				surface, _, _ = SelectLookFn(f, n, theme, gfx.State{}, placeholder)
			}
			if n.Flags&flagHasRadius == 0 {
				n.Radius = surface.Radius
			}
			if n.Flags&flagSetBorder == 0 {
				n.BorderWidth = surface.BorderWidth
			}
			height, _ := style.SelectGeometry(theme.Metrics, n.ButtonSize)
			if d := n.Size[AxisY]; d.Kind == dimPx {
				height = d.Value
			}
			if n.Size[AxisY].Kind == dimAuto && n.MinS[AxisY] == 0 {
				n.MinS[AxisY] = height
			}
			if n.Flags&flagChildren != 0 {
				_, textSize := style.SelectGeometry(theme.Metrics, n.ButtonSize)
				if n.FontSize > 0 {
					textSize = n.FontSize
				}
				weight := gfx.WeightRegular
				if n.WeightSet || n.Weight != 0 {
					weight = n.Weight
				}
				f.buttonTextDefaults(n, textSize, weight, n.LineH)
			}
		case kindCheckbox:
			theme := f.RecipeTheme(n, &f.scratch.theme)
			look := style.CheckboxLook{}
			if CheckboxLookFn != nil {
				look = CheckboxLookFn(f, n, theme, gfx.State{})
			}
			if n.Flags&flagHasRadius == 0 {
				n.Radius = look.Radius
			}
			if n.Flags&flagSetBorder == 0 {
				n.BorderWidth = 1
			}
		case kindSwitch:
			theme := f.RecipeTheme(n, &f.scratch.theme)
			_, height := SwitchExtent(*theme, n.Flags&flagSmall != 0)
			if d := n.Size[AxisY]; d.Kind == dimPx {
				height = d.Value
			}
			if n.Flags&flagHasRadius == 0 {
				n.Radius = height / 2
			}
			if n.Flags&flagSetBorder == 0 {
				bordered := n.Flags&FlagInvalid != 0
				if c := f.Surface; c != nil && !n.ID.IsZero() {
					bordered = bordered || c.FocusVisible() == n.ID
				}
				if bordered {
					n.BorderWidth = 1
				}
			}
		}
	}
}

// applyButtonRecipe supplies layout defaults while the control keeps its button
// kind. Caller-owned content uses the same typography as the default label.
func (f *Frame) applyButtonRecipe(n *Node) {
	theme := f.RecipeTheme(n, &f.scratch.theme)
	if n.Flags&flagHasRadius == 0 {
		n.Radius = f.buttonLook(n, theme, gfx.State{}).Radius
	}
	height, padding, fontSize, iconOnly := style.ButtonGeometry(theme.Metrics, n.ButtonSize)
	if n.FontSize > 0 {
		fontSize = n.FontSize
	}
	if iconOnly {
		padding = 0
	}
	if n.PadSet&padRight == 0 {
		n.Pad[1] = padding
	}
	if n.PadSet&padLeft == 0 {
		n.Pad[3] = padding
	}
	if n.Flags&flagChildren == 0 {
		return
	}
	weight := gfx.WeightMedium
	if n.WeightSet || n.Weight != 0 {
		weight = n.Weight
	}
	f.buttonTextDefaults(n, fontSize, weight, n.LineH)
	if n.Size[AxisY].Kind == dimAuto && n.MinS[AxisY] == 0 {
		n.MinS[AxisY] = height
	}
}

// Explicit child typography wins over the button's inherited defaults. A
// nested control owns its own defaults rather than borrowing the outer label's.
func (f *Frame) buttonTextDefaults(parent *Node, fontSize, weight, lineHeight float32) {
	for i := parent.First; i >= 0; i = f.Nodes[i].Next {
		n := &f.Nodes[i]
		if n.Kind == kindButton {
			continue
		}
		font, stroke, leading := fontSize, weight, lineHeight
		if n.FontSet {
			font = n.FontSize
		} else if n.Kind == kindText {
			n.FontSize = font
		}
		if n.WeightSet {
			stroke = n.Weight
		} else if n.Kind == kindText {
			n.Weight = stroke
		}
		if n.LineH > 0 {
			leading = n.LineH
		} else if n.Kind == kindText {
			n.LineH = leading
		}
		if n.First >= 0 {
			f.buttonTextDefaults(n, font, stroke, leading)
		}
	}
}

// themedDefaults fills in, for the state the element is in now, the colors of the resolved
// look the element did not set itself.
func (f *Frame) themedDefaults(n *Node, Extra *Extra, s *resolved) {
	var state gfx.State
	switch n.Kind {
	case kindButton:
		f.themedButtonDefaults(n, Extra, s)
		return
	case kindBadge, kindToggle:
		// A badge and a toggle are dressed by one ButtonLook the same way; the
		// toggle's state also carries its pressed and invalid marks.
		theme := f.RecipeTheme(n, &f.scratch.theme)
		if c := f.Surface; c != nil && !n.ID.IsZero() {
			state.Hovered = c.Hovered() == n.ID
			state.Pressed = c.Pressed() == n.ID
			state.FocusVisible = c.FocusVisible() == n.ID
		}
		if n.Kind == kindToggle {
			state.Invalid = n.Flags&FlagInvalid != 0
			state.Checked = n.Flags&flagChecked != 0
		}
		state.Disabled = f.Disabled(n)
		var look style.ButtonLook
		if n.Kind == kindBadge {
			look = f.badgeLook(n, theme, state)
		} else {
			look = f.toggleLook(n, theme, state)
		}
		if n.Flags&flagSetBackground == 0 {
			s.Background = gfx.Color{}
			if look.Filled {
				s.Background = look.Background
			}
		}
		bordered := look.Bordered && look.Filled
		if bordered && theme.Background.Alpha >= 1 {
			if n.Flags&flagSetBackground == 0 {
				s.Background = s.Background.Over(theme.Background)
			}
			look.Border = look.Border.Over(theme.Background)
		}
		if n.Flags&flagSetBorder == 0 {
			s.border = look.Border
			if bordered {
				s.borderWidth = 1
				s.splitBorder = theme.Background.Alpha < 1
			}
		}
		if n.Flags&flagSetShadow == 0 {
			s.shadow = look.Shadow
		}
		if Extra.ringWidth <= 0 && look.RingWidth > 0 {
			s.ring, s.ringWidth = look.Ring, look.RingWidth
			s.ringFilled, s.ringLine = !look.RingOutline, 0
			if look.RingOutline {
				s.ringLine = 2
			}
		}
		if n.Flags&flagHasColor == 0 {
			s.Color = look.Foreground
		}
		return
	case kindTabs:
		theme := f.RecipeTheme(n, &f.scratch.theme)
		look := f.tabsLook(n, theme)
		if n.Flags&flagSetBackground == 0 {
			s.Background = gfx.Color{}
			if look.Filled {
				s.Background = look.Background
			}
		}
		if n.Flags&flagSetBorder == 0 && look.Bordered {
			s.border = look.Border
			s.borderWidth = 1
			s.splitBorder = theme.Background.Alpha < 1
		}
		if n.Flags&flagSetShadow == 0 {
			s.shadow = look.Shadow
		}
		return
	case kindSelect:
		theme := f.RecipeTheme(n, &f.scratch.theme)
		d := &WidgetsOf(f).selects[n.Aux]
		placeholder := d.selected < 0 || d.selected >= len(d.labels)
		if c := f.Surface; c != nil {
			id := d.TriggerID(len(d.labels))
			state.Hovered = !id.IsZero() && c.Hovered() == id
			state.Pressed = !id.IsZero() && c.Pressed() == id
			state.Focused = !id.IsZero() && c.FocusVisible() == id
		}
		state.Invalid = n.Flags&FlagInvalid != 0
		state.Disabled = f.Disabled(n)
		surface, foreground, chevron := style.TextEntryLook{}, gfx.Color{}, gfx.Color{}
		if SelectLookFn != nil {
			surface, foreground, chevron = SelectLookFn(f, n, theme, state, placeholder)
		}
		f.selectForeground, f.selectChevron = foreground, chevron
		if n.Flags&flagSetBackground == 0 {
			s.Background = surface.Fill
		}
		if n.Flags&flagSetBorder == 0 {
			s.border = surface.Border
			s.borderWidth = surface.BorderWidth
			s.splitBorder = theme.Background.Alpha < 1
		}
		if n.Flags&flagSetShadow == 0 {
			s.shadow = surface.Shadow
		}
		if n.Flags&flagHasColor == 0 {
			s.Color = foreground
		}
		return
	case kindCheckbox:
		theme := f.RecipeTheme(n, &f.scratch.theme)
		state := gfx.State{Checked: n.Flags&flagChecked != 0, Mixed: n.Flags&flagMixed != 0, Invalid: n.Flags&FlagInvalid != 0}
		if c := f.Surface; c != nil && !n.ID.IsZero() {
			state.Hovered = c.Hovered() == n.ID
			state.Pressed = c.Pressed() == n.ID
			state.FocusVisible = c.FocusVisible() == n.ID
		}
		state.Disabled = f.Disabled(n)
		look := style.CheckboxLook{}
		if CheckboxLookFn != nil {
			look = CheckboxLookFn(f, n, theme, state)
		}
		f.checkboxPainted = look
		if n.Flags&flagSetBackground == 0 {
			s.Background = look.Fill
		}
		if n.Flags&flagSetBorder == 0 {
			s.border = look.Border
			s.borderWidth = 1
		}
		if n.Flags&flagSetShadow == 0 {
			s.shadow = look.Shadow
		}
		if Extra.ringWidth <= 0 && look.RingWidth > 0 {
			s.ring, s.ringWidth = look.Ring, look.RingWidth
			s.ringFilled = true
		}
		return
	case kindSwitch:
		theme := f.RecipeTheme(n, &f.scratch.theme)
		state := gfx.State{Checked: n.Flags&flagChecked != 0, Invalid: n.Flags&FlagInvalid != 0}
		if c := f.Surface; c != nil && !n.ID.IsZero() {
			state.Hovered = c.Hovered() == n.ID
			state.Pressed = c.Pressed() == n.ID
			state.FocusVisible = c.FocusVisible() == n.ID
		}
		state.Disabled = f.Disabled(n)
		progress := float32(0)
		if state.Checked {
			progress = 1
		}
		if c := f.Surface; c != nil && !n.ID.IsZero() {
			progress = c.Animate(n.ID, SwitchCheckedSlot, progress, theme.Transition)
		}
		bordered := state.FocusVisible || state.Invalid
		look := style.SwitchLook{}
		if own := f.Exts[n.Ext].GetSwitchRecipe(); own != nil {
			look = own(*theme, state, progress)
		} else if SwitchLookFn != nil {
			look = SwitchLookFn(f, n, theme, state, progress)
		}
		f.switchPainted = look
		if n.Flags&flagSetBackground == 0 {
			s.Background = look.Track
		}
		if n.Flags&flagSetBorder == 0 && bordered {
			s.border = look.Border
			s.borderWidth = 1
		}
		if n.Flags&flagSetShadow == 0 {
			s.shadow = look.Shadow
		}
		if Extra.ringWidth <= 0 && look.RingWidth > 0 {
			s.ring, s.ringWidth = look.Ring, look.RingWidth
			s.ringFilled = true
		}
		return
	case kindInput:
		snapshot := &WidgetsOf(f).inputs[n.Aux]
		state = gfx.State{Focused: snapshot.Focused || f.Focused(n), Invalid: snapshot.Invalid, Disabled: snapshot.Disabled}
	case kindTextarea:
		snapshot := &WidgetsOf(f).textareas[n.Aux]
		state = gfx.State{Focused: snapshot.Focused || f.Focused(n), Invalid: snapshot.Invalid, Disabled: snapshot.Disabled}
	default:
		return
	}
	look := f.TextEntryLook(f.RecipeTheme(n, &f.scratch.theme), state)
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
func (f *Frame) Focused(n *Node) bool {
	return f.Surface != nil && !n.ID.IsZero() && f.Surface.Focused() == n.ID
}

// contentRect is where the text of an element sits: inside its Pad and Border. A themed
// control whose Pad is the recipe's keeps the text area the shadcn control has, which is
// inset from the control's edge by the recipe's own amounts: the border is drawn over the
// control's edge, not taken out of the area the text is centered in.
func (f *Frame) ContentRect(n *Node) Rect {
	r := n.Rect
	if n.Flags&flagThemed != 0 && n.Flags&flagSetPad == 0 {
		switch n.Kind {
		case kindInput, kindTextarea:
			x, y := textEntryInsets(f.RecipeTheme(n, &f.scratch.theme), n.Kind)
			return Rect{X: r.X + x, Y: r.Y + y, Width: max(r.Width-2*x, 0), Height: max(r.Height-2*y, 0)}
		case kindSelect:
			left, right := style.SelectContentInsets(f.RecipeTheme(n, &f.scratch.theme).Metrics)
			return Rect{X: r.X + left, Y: r.Y, Width: max(r.Width-left-right, 0), Height: r.Height}
		}
	}
	left, right := n.Inset(AxisX)
	top, bottom := n.Inset(AxisY)
	return Rect{X: r.X + left, Y: r.Y + top, Width: max(r.Width-left-right, 0), Height: max(r.Height-top-bottom, 0)}
}

// inputLine is the height of one line of the text of an Input or InputBase: the element's
// LineHeight when it has one, and otherwise the size of the text plus the seven pixels a line of
// text is given everywhere else. The size is the element's FontSize, or the snapshot's own text
// size, or the theme's.
func (f *Frame) InputLine(n *Node) float32 {
	if n.LineH > 0 {
		return n.LineH
	}
	size := n.FontSize
	if size <= 0 {
		size = WidgetsOf(f).inputs[n.Aux].Text.Size
	}
	if size <= 0 {
		size = f.Palette.Metrics.FontSizeM
	}
	return size + 7
}

// textEntryInsets is the room the shadcn text-entry controls leave between their edge and
// their text: across, the metrics' text inset; down, nothing for a line (which is centered
// in the control's whole height) and a fixed margin for rows that start at the top.
func textEntryInsets(Palette *Theme, k Kind) (x, y float32) {
	x = Palette.Metrics.TextInsetX()
	if k == kindTextarea {
		y = TextareaPadY
	}
	return x, y
}

// themedButtonDefaults fills in, for the state the button is in now, what the recipe gives
// the resolved look and the element did not set itself: the fill, the border and its width,
// the shadow, the ring and the color of its content. A Hover, Pressed or Focus look applies
// over them as it does over any element.
func (f *Frame) themedButtonDefaults(n *Node, Extra *Extra, s *resolved) {
	theme := f.RecipeTheme(n, &f.scratch.theme)
	var state gfx.State
	if c := f.Surface; c != nil && !n.ID.IsZero() {
		state = gfx.State{Hovered: c.Hovered() == n.ID, Pressed: c.Pressed() == n.ID, FocusVisible: c.FocusVisible() == n.ID}
	}
	state.Disabled = f.Disabled(n)
	look := f.buttonLook(n, theme, state)
	f.buttonPainted = look
	if n.Flags&flagSetBackground == 0 {
		s.Background = gfx.Color{}
		if look.Filled {
			s.Background = look.Background
		}
	}
	// Over an opaque page a filled border is flattened onto it and painted as one shape
	// (RoundedBorder); over a translucent one the fill goes down whole and the line over it.
	bordered := look.Bordered && look.Filled
	if bordered && theme.Background.Alpha >= 1 {
		if n.Flags&flagSetBackground == 0 {
			s.Background = s.Background.Over(theme.Background)
		}
		look.Border = look.Border.Over(theme.Background)
	}
	if n.Flags&flagSetBorder == 0 {
		s.border = look.Border
		if bordered {
			s.borderWidth = 1
			s.splitBorder = theme.Background.Alpha < 1
		}
	}
	if n.Flags&flagSetShadow == 0 {
		s.shadow = look.Shadow
	}
	if Extra.ringWidth <= 0 && look.RingWidth > 0 {
		s.ring, s.ringWidth = look.Ring, look.RingWidth
		s.ringFilled, s.ringLine = !look.RingOutline, 0
		if look.RingOutline {
			s.ringLine = 2
		}
	}
	if n.Flags&flagHasColor == 0 {
		s.Color = look.Foreground
	}
}

// Disabled includes disabled ancestors, so rebuilding a descendant cannot
// reactivate a click or focus stop inside an unavailable control.
func (f *Frame) Disabled(n *Node) bool {
	for {
		if n.Flags&flagDisabled != 0 {
			return true
		}
		if n.Parent < 0 {
			return false
		}
		n = &f.Nodes[n.Parent]
	}
}
