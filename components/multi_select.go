package components

import (
	"strconv"

	"github.com/vivaui/viva/base"

	"github.com/vivaui/viva/gfx"
)

// MultiSelectOption, MultiSelectChoice and MultiSelectState are the engine's:
// selection, filtering and the native query editor live in package base, so a
// page can own them without importing the style library.
type (
	MultiSelectOption = base.MultiSelectOption
	MultiSelectChoice = base.MultiSelectChoice
	MultiSelectState  = base.MultiSelectState
)

// MultiSelectOptions supplies stable action IDs and localized presentation.
// OptionIDs and RemoveIDs are indexed by Choices and Values respectively.
type MultiSelectOptions struct {
	InputID, TriggerID, DismissID, Scope         gfx.ID
	ScrollID                                     gfx.ID
	OptionIDs, RemoveIDs                         gfx.Scope
	Label, Placeholder, EmptyLabel, CreatePrefix string
	// Viewport bounds the popup and backdrop independently of the input's
	// ancestor scroll clips. Set it to the window or a narrower overlay region.
	Viewport          gfx.Rect
	Layer             uint16
	Disabled, Invalid bool
	MaxRows           int
}

type MultiSelectLayout struct {
	Bounds, Input, Popup gfx.Rect
	Hidden               int
}

// PaintMultiSelect paints a compact editable multi-select. Chips that cannot fit
// beside the native query input collapse to +N; every value remains available
// in the popup for deselection. The popup traps focus around the input as well
// as the list, so typing and IME stay native while it is open.
func PaintMultiSelect(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, s *MultiSelectState, o MultiSelectOptions) MultiSelectLayout {
	if bounds.Height <= 0 {
		bounds.Height = theme.Metrics.ControlHeight
	}
	layout := MultiSelectLayout{Bounds: bounds}
	if canvas == nil || bounds.Width <= 0 || bounds.Height <= 0 {
		return layout
	}
	choices := s.Choices()
	open := s.Open && !o.Disabled
	floating := o.Viewport.Width > 0 && o.Viewport.Height > 0
	// The popup takes focus into its own scope and the runtime restores the
	// target from before that scope opened, which is not the query input when
	// the list was opened by the trigger instead of a press on the input. Hand
	// focus back when the input is the one losing it, and only then: a press on
	// another control has already moved focus and keeps it.
	if s.OpenPainted() && !open && canvas.Focused() == o.InputID {
		canvas.RequestFocus(o.InputID)
	}
	s.SetOpenPainted(open)
	if open {
		canvas.PushLayer(o.Layer)
		canvas.PushFocusScope(o.Scope)
		defer canvas.PopFocusScope()
		defer canvas.PopLayer()
		canvas.TrapFocus(o.Scope)
		canvas.SetInitialFocus(o.InputID)
		canvas.SetEscapeTarget(o.DismissID)
		if floating {
			canvas.PushOverlayClip(o.Viewport)
		}
		canvas.HitWithOptions(o.DismissID, o.Viewport, o.Label, gfx.HitOptions{Role: gfx.RoleButton, SkipFocus: true, Backdrop: true, PassSecondary: true})
		if floating {
			canvas.PopOverlayClip()
		}
	}
	paintTextEntrySurface(canvas, &theme, bounds, gfx.State{Focused: canvas.Focused() == o.InputID || open, Invalid: o.Invalid, Disabled: o.Disabled})
	canvas.HitWithOptions(o.TriggerID, bounds, o.Label, gfx.HitOptions{Role: gfx.RoleButton, Expanded: open, Disabled: o.Disabled, SkipFocus: true, Invalid: o.Invalid})
	trigger := gfx.Rect{X: bounds.X + bounds.Width - 28, Y: bounds.Y + (bounds.Height-24)/2, Width: 24, Height: 24}
	x, right := bounds.X+7, trigger.X-6
	for i, value := range s.Values() {
		label := s.ValueLabel(value)
		width := min(canvas.TextWidthWeight(label, theme.Metrics.FontSizeS, gfx.WeightMedium)+34, max(bounds.Width/2, 32))
		reserve := float32(72)
		if i < len(s.Values())-1 {
			reserve += 34
		}
		if x+width+reserve > right {
			layout.Hidden = len(s.Values()) - i
			break
		}
		chip := gfx.Rect{X: x, Y: bounds.Y + (bounds.Height-22)/2, Width: width, Height: 22}
		paintComboboxChip(canvas, theme, chip, ComboboxChip{RemoveID: o.RemoveIDs.At(i), Label: canvas.FitLine(label, theme.Metrics.FontSizeS, gfx.WeightMedium, width-30), Value: value, Disabled: o.Disabled})
		x += width + 5
	}
	if layout.Hidden > 0 {
		badge := gfx.Rect{X: x, Y: bounds.Y + (bounds.Height-22)/2, Width: 30, Height: 22}
		canvas.Rounded(badge, theme.Metrics.RadiusItem(), theme.Muted)
		if _, hidden := s.Overflow(); hidden != layout.Hidden {
			s.SetOverflow("+"+strconv.Itoa(layout.Hidden), layout.Hidden)
		}
		overflowText, _ := s.Overflow()
		canvas.LabelCenter(overflowText, badge, gfx.TextStyle{Size: theme.Metrics.FontSizeS, Color: theme.MutedForeground})
		x += 35
	}
	layout.Input = gfx.Rect{X: x, Y: bounds.Y + 1, Width: max(right-x, 1), Height: bounds.Height - 2}
	query := s.Query.String()
	focused := canvas.Focused() == o.InputID
	if !focused || query == "" {
		placeholder := o.Placeholder
		if len(s.Values()) > 0 {
			placeholder = ""
		}
		inputPaintText(canvas, theme, layout.Input, InputSnapshot{Value: query, Placeholder: placeholder, Disabled: o.Disabled}, false)
	}
	canvas.HitWithOptions(o.InputID, layout.Input, o.Label, gfx.HitOptions{Role: gfx.RoleCombobox, Value: query, Expanded: open, Disabled: o.Disabled, Invalid: o.Invalid, Cursor: gfx.PointerCursorText, TextBounds: layout.Input, TextSize: theme.Metrics.FontSizeM, TextColor: theme.Foreground, NavigationKeys: true})
	paintChevron(canvas, &chevronDownIcon, trigger, theme.MutedForeground)
	if s.FocusInput() {
		canvas.RequestFocus(o.InputID)
		s.SetFocusInput(false)
	}
	if !open {
		return layout
	}
	rows := o.MaxRows
	if rows <= 0 {
		rows = 7
	}
	height := float32(max(1, min(len(choices), rows))*32 + 8)
	layout.Popup = gfx.Rect{X: bounds.X, Y: bounds.Y + bounds.Height + 6, Width: bounds.Width, Height: height}
	if o.Viewport.Height > 0 {
		top, bottom := o.Viewport.Y+1, o.Viewport.Y+o.Viewport.Height-1
		if layout.Popup.Y+layout.Popup.Height > bottom {
			// Prefer room above for at least one usable row. Otherwise scroll
			// inside the space left below, even when that is less than a row.
			if above := bounds.Y - 6 - top; above >= min(height, 40) {
				layout.Popup.Height = min(height, above)
				layout.Popup.Y = bounds.Y - layout.Popup.Height - 6
			}
		}
		layout.Popup.Y = max(top, min(layout.Popup.Y, bottom-1))
		layout.Popup.Height = max(1, min(layout.Popup.Height, bottom-layout.Popup.Y))
	}
	if floating {
		canvas.PushOverlayClip(o.Viewport)
		defer canvas.PopOverlayClip()
	}
	canvas.Block(layout.Popup)
	canvas.Container(layout.Popup, gfx.RoleListBox, o.Label)
	paintPopupSurface(canvas, &theme, layout.Popup, gfx.ShadowMD)
	s.ScrollArea().BarID = o.ScrollID
	menu := menuSurface{canvas: canvas, theme: theme, bounds: layout.Popup, options: menuSurfaceOptions{layer: o.Layer, scope: o.Scope, scroll: s.ScrollArea()}, top: layout.Popup.Y + 4, bottom: layout.Popup.Y + layout.Popup.Height - 4, viewHeight: layout.Popup.Height - 8}
	menu.beginScroll()
	// Move the cursor before any row is painted: syncing it while the rows are
	// drawn would tint the previous row and the new one in the same frame.
	hovered := canvas.Hovered()
	if s.AdoptHover(hovered) && !hovered.IsZero() {
		for i := range choices {
			if o.OptionIDs.At(i) == hovered && !choices[i].Disabled {
				s.Active = i
				break
			}
		}
	}
	for i, choice := range choices {
		id := o.OptionIDs.At(i)
		row := menu.rowBounds(32)
		if !menu.takeItem(row, id, choice.Label, choice.Disabled) {
			continue
		}
		menu.paintRow(row, func() {
			look := menuItemRecipe(&theme, gfx.State{Hovered: i == s.Active, Disabled: choice.Disabled})
			if look.Fill.Alpha > 0 {
				canvas.Rounded(row, look.Radius, look.Fill)
			}
			color := look.Foreground
			label := choice.Label
			if choice.Create {
				text, prefix, value := s.CreateText()
				if prefix != o.CreatePrefix || value != label {
					text = o.CreatePrefix + label
					s.SetCreateText(text, o.CreatePrefix, label)
				}
				label = text
			}
			label = canvas.FitLine(label, theme.Metrics.FontSizeM, gfx.WeightRegular, max(row.Width-38, 0))
			canvas.Label(label, gfx.Rect{X: row.X + 8, Y: row.Y, Width: max(row.Width-38, 0), Height: row.Height}, gfx.TextStyle{Size: theme.Metrics.FontSizeM, Color: color})
			if choice.Selected {
				paintCheck(canvas, gfx.Rect{X: row.X + row.Width - 25, Y: row.Y, Width: 16, Height: row.Height}, 14, color)
			}
			canvas.HitWithOptions(id, row, choice.Label, gfx.HitOptions{Role: gfx.RoleOption, Value: choice.Value, Selected: choice.Selected, Disabled: choice.Disabled, SkipFocus: true})
		})
	}
	if len(choices) == 0 {
		canvas.LabelCenter(o.EmptyLabel, layout.Popup, gfx.TextStyle{Size: theme.Metrics.FontSizeM, Color: theme.MutedForeground})
	}
	menu.end()
	if s.RevealActive() && s.ScrollArea().Reveal(o.OptionIDs.At(s.Active)) {
		canvas.RequestFrame()
	}
	s.SetRevealActive(false)
	return layout
}
