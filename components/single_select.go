package components

import (
	"math"

	"github.com/vivaui/viva/base"

	"github.com/vivaui/viva/gfx"
)

// singleSelectIcon reports whether an icon has anything to paint.
func singleSelectIcon(icon gfx.Icon) bool {
	return len(icon.Commands) > 0 || len(icon.Layers) > 0
}

// singleSelectDetailGap is the space between an option's label and its detail.
const singleSelectDetailGap = 8

// popupWidth is the width that shows every label of choices untruncated.
func popupWidth(s *SingleSelectState, canvas *gfx.Canvas, theme gfx.Theme, choices []SingleSelectChoice, o SingleSelectOptions) float32 {
	size := theme.Metrics.FontSizeM
	if s.WidestRebuilt != s.Rebuilt || s.WidestSize != size {
		s.WidestValue = 0
		for _, choice := range choices {
			if choice.Create {
				continue
			}
			width := canvas.TextWidthWeight(choice.Label, size, gfx.WeightRegular)
			if choice.Content.Paint != nil {
				width = choice.Content.Width
			}
			if choice.Leading.Paint != nil {
				width += choice.Leading.Width + singleSelectDetailGap
			}
			if choice.Trailing.Paint != nil {
				width += choice.Trailing.Width + singleSelectDetailGap
			}
			if choice.Detail != "" && choice.Content.Paint == nil {
				width += singleSelectDetailGap + canvas.TextWidthWeight(choice.Detail, theme.Metrics.FontSizeS, gfx.WeightRegular)
			}
			if singleSelectIcon(choice.Icon) {
				width += menuIconSize + menuIconGap
			}
			s.WidestValue = max(s.WidestValue, width)
		}
		s.WidestRebuilt, s.WidestSize = s.Rebuilt, size
	}
	widest := s.WidestValue
	if len(choices) > 0 && choices[0].Create {
		widest = max(widest, canvas.TextWidthWeight(o.CreatePrefix+choices[0].Label, size, gfx.WeightRegular))
	}
	// A row spends 8 on the left and 30 on the check before the menu padding.
	return float32(math.Ceil(float64(widest))) + 46 + 1
}

// PaintSingleSelect shares text-entry and menu primitives with Combobox and
// MultiSelect. The query stays a native editor; no chip or multi-selection is
// introduced when the selected value changes.
func PaintSingleSelect(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, s *SingleSelectState, o SingleSelectOptions) SingleSelectLayout {
	if bounds.Height <= 0 {
		bounds.Height = theme.Metrics.ControlHeight
	}
	layout := SingleSelectLayout{Bounds: bounds}
	if canvas == nil || bounds.Width <= 0 || bounds.Height <= 0 {
		return layout
	}
	choices := s.Choices()
	open := s.Open && !o.Disabled
	floating := o.Viewport.Width > 0 && o.Viewport.Height > 0
	if s.OpenPainted && !open && canvas.Focused() == o.InputID {
		canvas.RequestFocus(o.InputID)
	}
	s.OpenPainted = open
	if open {
		canvas.PushLayer(o.Layer)
		defer canvas.PopLayer()
		canvas.PushFocusScope(o.Scope)
		defer canvas.PopFocusScope()
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
	paintTextEntrySurface(canvas, &theme, bounds, gfx.State{Focused: canvas.Focused() == o.InputID || open, Disabled: o.Disabled, Invalid: o.Invalid})
	canvas.HitWithOptions(o.TriggerID, bounds, o.Label, gfx.HitOptions{Role: gfx.RoleButton, Expanded: open, Disabled: o.Disabled, SkipFocus: true, Invalid: o.Invalid})
	trigger := gfx.Rect{X: bounds.X + bounds.Width - 28, Y: bounds.Y + (bounds.Height-24)/2, Width: 24, Height: 24}
	right := trigger.X - 4
	query := s.Query.String()
	iconColor := theme.MutedForeground
	if o.Disabled {
		iconColor.Alpha *= .5
	}
	if !o.ClearID.IsZero() && (s.ValueText != "" || query != "") {
		clear := gfx.Rect{X: right - 24, Y: trigger.Y, Width: 24, Height: 24}
		if !o.Disabled && canvas.Hovered() == o.ClearID {
			canvas.Rounded(clear, theme.Metrics.RadiusItem(), theme.Muted)
		}
		paintCross(canvas, clear, 12, iconColor)
		label := o.ClearLabel
		if label == "" {
			label = "Clear selection"
		}
		canvas.HitWithOptions(o.ClearID, clear, label, gfx.HitOptions{Role: gfx.RoleButton, Disabled: o.Disabled})
		right = clear.X - 4
	}
	inset := theme.Metrics.TextInsetX()
	layout.Input = gfx.Rect{X: bounds.X + inset, Y: bounds.Y + 1, Width: max(1, right-bounds.X-inset), Height: bounds.Height - 2}
	display, color := query, theme.Foreground
	if display == "" {
		display = s.Label(s.ValueText)
		if display == "" {
			display, color = o.Placeholder, theme.MutedForeground
		} else if open {
			color = theme.MutedForeground
		}
	}
	if o.Disabled {
		color.Alpha *= .5
	}
	if icon := s.SelectedIcon(); singleSelectIcon(icon) {
		side := min(menuIconSize, layout.Input.Height)
		canvas.DrawIcon(icon, gfx.Rect{X: layout.Input.X, Y: bounds.Y + (bounds.Height-side)/2, Width: side, Height: side}, color)
		layout.Input.X += menuIconSize + menuIconGap
		layout.Input.Width = max(1, layout.Input.Width-(menuIconSize+menuIconGap))
	}
	if canvas.Focused() != o.InputID || query == "" {
		detail := ""
		if query == "" && !open {
			detail = s.Detail(s.ValueText)
		}
		paintLabelAndDetail(canvas, theme, layout.Input.X, layout.Input.Y, layout.Input.Width, layout.Input.Height, display, detail, color, o.Disabled)
	}
	accessibleValue := query
	if accessibleValue == "" {
		accessibleValue = s.Label(s.ValueText)
	}
	canvas.HitWithOptions(o.InputID, layout.Input, o.Label, gfx.HitOptions{Role: gfx.RoleCombobox, Value: accessibleValue, Expanded: open, Disabled: o.Disabled, Invalid: o.Invalid, Cursor: gfx.PointerCursorText, TextBounds: layout.Input, TextSize: theme.Metrics.FontSizeM, TextColor: theme.Foreground, NavigationKeys: true})
	paintChevron(canvas, &chevronDownIcon, trigger, iconColor)
	if s.FocusInput {
		canvas.RequestFocus(o.InputID)
		s.FocusInput = false
	}
	if !open {
		return layout
	}
	if s.TreeActive() {
		layout.Popup = PaintHierarchyMenu(canvas, theme, bounds, &s.Tree, HierarchyMenuOptions{
			OptionIDs: o.OptionIDs, ScrollID: o.ScrollID, Scope: o.Scope, Layer: o.Layer, Label: o.Label,
			EmptyLabel: o.EmptyLabel, Viewport: o.Viewport, MaxRows: o.MaxRows,
		})
		return layout
	}
	rows := o.MaxRows
	if rows <= 0 {
		rows = 7
	}
	height := float32(max(1, min(len(choices), rows))*32 + 8)
	layout.Popup = gfx.Rect{X: bounds.X, Y: bounds.Y + bounds.Height + 6, Width: bounds.Width, Height: height}
	// Like Radix's popper content, the list is at least as wide as the field and
	// grows to its longest label so options that share a long prefix stay
	// distinguishable, until the viewport stops it.
	if want := popupWidth(s, canvas, theme, choices, o); want > layout.Popup.Width {
		layout.Popup.Width = want
		if o.Viewport.Width > 0 {
			layout.Popup.Width = min(want, max(o.Viewport.Width-2, bounds.Width))
			layout.Popup.X = max(o.Viewport.X+1, min(layout.Popup.X, o.Viewport.X+o.Viewport.Width-1-layout.Popup.Width))
		}
	}
	placePopupBelow(&layout.Popup, bounds, height, o.Viewport)
	if floating {
		canvas.PushOverlayClip(o.Viewport)
		defer canvas.PopOverlayClip()
	}
	canvas.Block(layout.Popup)
	canvas.Container(layout.Popup, gfx.RoleListBox, o.Label)
	paintPopupSurface(canvas, &theme, layout.Popup, gfx.ShadowMD)
	s.Scroll.BarID = o.ScrollID
	menu := menuSurface{canvas: canvas, theme: theme, bounds: layout.Popup, options: menuSurfaceOptions{layer: o.Layer, scope: o.Scope, scroll: &s.Scroll}, top: layout.Popup.Y + 4, bottom: layout.Popup.Y + layout.Popup.Height - 4, viewHeight: layout.Popup.Height - 8}
	menu.beginScroll()
	// Move the cursor before any row is painted: syncing it while the rows are
	// drawn would tint the previous row and the new one in the same frame.
	hovered := canvas.Hovered()
	if s.Hover.Entered(hovered) && !hovered.IsZero() {
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
				if s.CreatePrefix != o.CreatePrefix || s.CreateValue != label {
					s.CreatePrefix, s.CreateValue = o.CreatePrefix, label
					s.CreateText = o.CreatePrefix + label
				}
				label = s.CreateText
			}
			left := row.X + 8
			if singleSelectIcon(choice.Icon) {
				side := min(menuIconSize, row.Height)
				canvas.DrawIcon(choice.Icon, gfx.Rect{X: left, Y: row.Y + (row.Height-side)/2, Width: side, Height: side}, color)
				left += menuIconSize + menuIconGap
			}
			right := row.X + row.Width - 30
			if slot := choice.Trailing; slot.Paint != nil {
				right -= slot.Width
				slot.Paint(canvas, gfx.Rect{X: right, Y: row.Y, Width: slot.Width, Height: row.Height}, theme)
				right -= singleSelectDetailGap
			}
			if slot := choice.Leading; slot.Paint != nil {
				slot.Paint(canvas, gfx.Rect{X: left, Y: row.Y, Width: slot.Width, Height: row.Height}, theme)
				left += slot.Width + singleSelectDetailGap
			}
			labelWidth := max(right-left-8, 0)
			if content := choice.Content; content.Paint != nil {
				content.Paint(canvas, gfx.Rect{X: left, Y: row.Y, Width: labelWidth, Height: row.Height}, theme)
			} else {
				paintLabelAndDetail(canvas, theme, left, row.Y, labelWidth, row.Height, label, choice.Detail, color, choice.Disabled)
			}
			if choice.Selected {
				paintCheck(canvas, gfx.Rect{X: row.X + row.Width - 25, Y: row.Y, Width: 16, Height: row.Height}, 14, color)
			}
			canvas.HitWithOptions(id, row, label, gfx.HitOptions{Role: gfx.RoleOption, Value: choice.Value, Selected: choice.Selected, Disabled: choice.Disabled, SkipFocus: true})
		})
	}
	if len(choices) == 0 {
		canvas.LabelCenter(o.EmptyLabel, layout.Popup, gfx.TextStyle{Size: theme.Metrics.FontSizeM, Color: theme.MutedForeground})
	}
	menu.end()
	if s.Reveal && s.Scroll.Reveal(o.OptionIDs.At(s.Active)) {
		canvas.RequestFrame()
	}
	s.Reveal = false
	return layout
}

// paintLabelAndDetail paints a label in the width it has and, after it, the muted detail in
// what is left, as Combobox rows did: the label keeps what it needs up to all but 40 pixels.
func paintLabelAndDetail(canvas *gfx.Canvas, theme gfx.Theme, x, y, width, height float32, label, detail string, color gfx.Color, disabled bool) {
	sizeM, sizeS := theme.Metrics.FontSizeM, theme.Metrics.FontSizeS
	if detail == "" {
		canvas.Label(canvas.FitLine(label, sizeM, gfx.WeightRegular, width), gfx.Rect{X: x, Y: y, Width: width, Height: height}, gfx.TextStyle{Size: sizeM, Color: color})
		return
	}
	labelWidth := min(canvas.TextWidthWeight(label, sizeM, gfx.WeightRegular), max(width-40, 0))
	canvas.Label(canvas.FitLine(label, sizeM, gfx.WeightRegular, labelWidth), gfx.Rect{X: x, Y: y, Width: labelWidth, Height: height}, gfx.TextStyle{Size: sizeM, Color: color})
	detailX := x + labelWidth + singleSelectDetailGap
	if detailWidth := max(x+width-detailX, 0); detailWidth > 0 {
		muted := theme.MutedForeground
		if disabled {
			muted.Alpha *= .5
		}
		canvas.Label(canvas.FitLine(detail, sizeS, gfx.WeightRegular, detailWidth), gfx.Rect{X: detailX, Y: y, Width: detailWidth, Height: height}, gfx.TextStyle{Size: sizeS, Color: muted})
	}
}

// Aliases into the style vocabulary.
type SingleSelectOption = base.SingleSelectOption
type SingleSelectPaint = base.SingleSelectPaint
type SingleSelectChoice = base.SingleSelectChoice
type SingleSelectState = base.SingleSelectState
type SingleSelectOptions = base.SingleSelectOptions
type SingleSelectLayout = base.SingleSelectLayout
