package components

import "github.com/vivaui/viva/gfx"

// ComboboxOptions describes one application-owned editable selection snapshot.
type ComboboxOptions struct {
	InputID gfx.ID
	ClearID gfx.ID
	// TriggerID, when set, makes the chevron a button of its own (Vega's
	// icon-xs ghost trigger) with a hover background; leave it zero when the
	// whole input already toggles the popup.
	TriggerID gfx.ID
	Label     string
	Value     string
	// Detail is optional secondary text shown alongside Value when Query is empty,
	// rendered in a smaller, muted font (such as an address or subtitle).
	Detail      string
	Query       string
	Placeholder string
	Hint        string
	Open        bool
	Disabled    bool
	Invalid     bool
	ShowTrigger bool
	ShowClear   bool
	// MatchInputWidth makes the popup match the input's width exactly, rather
	// than extending slightly wider.
	MatchInputWidth bool
	Content         gfx.Rect
	Layer           uint16
	Scope           gfx.ID
	DismissID       gfx.ID
	// Viewport, with DismissID, makes the open popup declare a hit over the
	// whole window behind its surface, so pressing anywhere outside it reports
	// DismissID (Escape reports it too). See SelectOptions.Viewport.
	Viewport gfx.Rect
	// Scroll is the scroll position of a popup whose options do not all fit its
	// Content (see MenuScroll); without one only the options at the top show.
	Scroll *MenuScroll
}

// ComboboxItem describes one caller-filtered option.
type ComboboxItem struct {
	ID    gfx.ID
	Label string
	Value string
	Hint  string
	// Detail is optional secondary text shown alongside the label in a muted,
	// smaller font (such as an address or subtitle), and included in the accessible name.
	Detail   string
	Selected bool
	// Highlighted is the row the pointer or the keyboard is on: Radix's
	// data-highlighted, the row Enter chooses. Selected is the chosen value and
	// is told apart by its check, so the two never tint two rows at once.
	Highlighted bool
	Disabled    bool
}

// ComboboxChip describes one caller-owned multi-selection token.
type ComboboxChip struct {
	RemoveID gfx.ID
	Label    string
	Value    string
	Disabled bool
}

// Combobox paints an editable trigger and its optional popup snapshot.
type Combobox struct {
	canvas        *gfx.Canvas
	theme         gfx.Theme
	inputBounds   gfx.Rect
	contentBounds gfx.Rect
	options       ComboboxOptions
	rows          menuSurface
	open          bool
}

// BeginCombobox paints the input group and optional popup surface.
func BeginCombobox(canvas *gfx.Canvas, theme gfx.Theme, inputBounds gfx.Rect, options ComboboxOptions) Combobox {
	if inputBounds.Height <= 0 {
		inputBounds.Height = theme.Metrics.ControlHeight
	}
	combobox := Combobox{canvas: canvas, theme: theme, inputBounds: inputBounds, options: options, open: canvas != nil && options.Open}
	if canvas == nil || inputBounds.Width <= 0 || inputBounds.Height <= 0 {
		combobox.open = false
		return combobox
	}
	if !combobox.open || options.Content.Height <= 0 {
		combobox.paintInput()
		combobox.open = false
		return combobox
	}
	content := options.Content
	minimumWidth := inputBounds.Width + 28
	if options.MatchInputWidth {
		content.Width = inputBounds.Width
	} else if content.Width < minimumWidth {
		content.Width = minimumWidth
	}
	combobox.contentBounds = content
	combobox.rows = menuSurface{
		canvas: canvas, theme: theme, bounds: content,
		options: menuSurfaceOptions{layer: options.Layer, scope: options.Scope, scroll: options.Scroll, tabCloses: true},
		top:     content.Y + 4, bottom: content.Y + content.Height - 4,
		viewHeight: max(content.Height-8, 0),
	}
	combobox.rows.beginScroll()
	if options.Scroll != nil {
		options.Scroll.TabCloses = true
	}
	combobox.pushOverlay()
	// Claim after pushing the popup's scope, so a combobox inside a dialog owns
	// the trap and Escape while it is open.
	if !options.Scope.IsZero() {
		canvas.TrapFocus(options.Scope)
	}
	if !options.InputID.IsZero() {
		canvas.SetInitialFocus(options.InputID)
	}
	if !options.DismissID.IsZero() {
		canvas.SetEscapeTarget(options.DismissID)
	}
	if !options.DismissID.IsZero() && options.Viewport.Width > 0 && options.Viewport.Height > 0 {
		canvas.HitWithOptions(options.DismissID, options.Viewport, "Dismiss combobox", gfx.HitOptions{Role: gfx.RoleButton, SkipFocus: true, Backdrop: true, PassSecondary: true})
	}
	// Paint the input inside the overlay layer so it sits above the dismiss backdrop
	// and receives input focus properly while the popup is open.
	combobox.paintInput()
	// The popup is inside the combobox: a press on its padding or a disabled
	// option must not reach the dismiss backdrop behind it.
	canvas.Block(content)
	canvas.Container(content, gfx.RoleListBox, options.Label)
	side := popupSideOf(content, inputBounds)
	canvas.BeginPresence(overlayKey(options.InputID, options.DismissID, options.Scope), overlaySurfacePart,
		popupPresence(theme, overlayDuration, side), popupPivot(content, side, inputBounds))
	paintPopupSurface(canvas, &theme, content, gfx.ShadowMD) // shadow-md
	combobox.popOverlay()
	return combobox
}

// End ends the popup's entrance and exit: call it once the last item is painted,
// or the popup appears and disappears at once. A closed combobox needs no End,
// and is not harmed by one.
func (combobox Combobox) End() {
	if combobox.open {
		combobox.rows.end()
		combobox.canvas.EndPresence()
	}
}

// InputBounds returns the normalized input geometry.
func (combobox Combobox) InputBounds() gfx.Rect { return combobox.inputBounds }

// ContentBounds returns the normalized open popup geometry.
func (combobox Combobox) ContentBounds() gfx.Rect { return combobox.contentBounds }

// NextItemBounds returns the next option row without mutating state.
func (combobox Combobox) NextItemBounds() gfx.Rect { return combobox.rows.rowBounds(32) }

// Navigate moves the keyboard through the open options and scrolls the one it
// reaches into view; see MenuScroll.Navigate.
func (combobox Combobox) Navigate(key gfx.Key, from gfx.ID, loop bool) (gfx.ID, bool) {
	if combobox.options.Scroll == nil {
		return gfx.ID{}, false
	}
	return combobox.options.Scroll.Navigate(key, from, loop)
}

// Match is MenuScroll.Match for the open options.
func (combobox Combobox) Match(prefix string, from gfx.ID) (gfx.ID, bool) {
	if combobox.options.Scroll == nil {
		return gfx.ID{}, false
	}
	return combobox.options.Scroll.Match(prefix, from)
}

// AddLabel paints one compact group label.
func (combobox *Combobox) AddLabel(label string) gfx.Rect {
	bounds := combobox.rows.rowBounds(28)
	if !combobox.rows.take(bounds) {
		return gfx.Rect{}
	}
	combobox.rows.paintRow(bounds, func() {
		combobox.canvas.Label(label, gfx.Rect{X: bounds.X + 8, Y: bounds.Y, Width: comboboxMax(0, bounds.Width-16), Height: bounds.Height}, gfx.TextStyle{
			Size: combobox.theme.Metrics.FontSizeS, Weight: gfx.WeightMedium, Color: combobox.theme.MutedForeground,
		})
	})
	return bounds
}

// AddItem paints one option and its selected indicator.
func (combobox *Combobox) AddItem(item ComboboxItem) gfx.Rect {
	bounds := combobox.rows.rowBounds(32)
	if !combobox.rows.takeItem(bounds, item.ID, item.Label, item.Disabled) {
		return gfx.Rect{}
	}
	combobox.rows.paintRow(bounds, func() {
		// Only the highlighted row is tinted (Vega's data-highlighted); a selected
		// one is told apart by its check. Highlighted is the caller's row, so the
		// tint, the pointer and Enter never name different rows.
		look := menuItemRecipe(&combobox.theme, gfx.State{Hovered: !item.ID.IsZero() && item.Highlighted, Disabled: item.Disabled})
		if look.Fill.Alpha > 0 {
			combobox.canvas.Rounded(bounds, look.Radius, look.Fill)
		}
		foreground := look.Foreground
		if item.Detail == "" {
			combobox.canvas.Label(item.Label, gfx.Rect{X: bounds.X + 8, Y: bounds.Y, Width: comboboxMax(0, bounds.Width-40), Height: bounds.Height}, gfx.TextStyle{
				Size: combobox.theme.Metrics.FontSizeM, Weight: gfx.WeightRegular, Color: foreground,
			})
		} else {
			sizeM := combobox.theme.Metrics.FontSizeM
			sizeS := combobox.theme.Metrics.FontSizeS
			contentWidth := comboboxMax(0, bounds.Width-40)

			labelWidth := combobox.canvas.TextWidthWeight(item.Label, sizeM, gfx.WeightRegular)
			maxLabelWidth := comboboxMax(0, contentWidth-40)
			if labelWidth > maxLabelWidth {
				labelWidth = maxLabelWidth
			}

			combobox.canvas.Label(combobox.canvas.FitLine(item.Label, sizeM, gfx.WeightRegular, labelWidth), gfx.Rect{X: bounds.X + 8, Y: bounds.Y, Width: labelWidth, Height: bounds.Height}, gfx.TextStyle{
				Size: sizeM, Weight: gfx.WeightRegular, Color: foreground,
			})

			detailX := bounds.X + 8 + labelWidth + 8
			detailWidth := comboboxMax(0, bounds.X+8+contentWidth-detailX)
			if detailWidth > 0 {
				detailColor := combobox.theme.MutedForeground
				if item.Disabled {
					detailColor.Alpha *= .5
				}
				combobox.canvas.Label(combobox.canvas.FitLine(item.Detail, sizeS, gfx.WeightRegular, detailWidth), gfx.Rect{X: detailX, Y: bounds.Y, Width: detailWidth, Height: bounds.Height}, gfx.TextStyle{
					Size: sizeS, Weight: gfx.WeightRegular, Color: detailColor,
				})
			}
		}
		if item.Selected {
			paintCheck(combobox.canvas, gfx.Rect{X: bounds.X + bounds.Width - 26, Y: bounds.Y, Width: 16, Height: bounds.Height}, 14, foreground)
		}
		if !item.ID.IsZero() {
			hit := bounds
			hit.Width -= combobox.rows.hitInset
			combobox.canvas.HitWithOptions(item.ID, hit, item.Label, gfx.HitOptions{
				Role: gfx.RoleOption, Value: item.Value, Hint: item.Hint,
				Description: item.Detail,
				Disabled:    item.Disabled, Selected: item.Selected,
			})
		}
	})
	return bounds
}

// AddSeparator paints the official one-pixel divider.
func (combobox *Combobox) AddSeparator() gfx.Rect {
	bounds := combobox.rows.rowBounds(9)
	if !combobox.rows.take(bounds) {
		return gfx.Rect{}
	}
	combobox.rows.paintRow(bounds, func() {
		combobox.canvas.Fill(gfx.Rect{X: combobox.contentBounds.X, Y: bounds.Y + 4, Width: combobox.contentBounds.Width, Height: 1}, combobox.theme.Border)
	})
	return bounds
}

// AddEmpty paints the centered empty-result state.
func (combobox *Combobox) AddEmpty(label string) gfx.Rect {
	// Vega: py-2 around one 20px line.
	bounds := combobox.rows.rowBounds(36)
	if !combobox.rows.take(bounds) {
		return gfx.Rect{}
	}
	combobox.rows.paintRow(bounds, func() {
		combobox.canvas.LabelCenter(label, bounds, gfx.TextStyle{Size: combobox.theme.Metrics.FontSizeM, Weight: gfx.WeightRegular, Color: combobox.theme.MutedForeground})
	})
	return bounds
}

// comboboxChipHeight is Vega's h-5.5 chip, and comboboxChipRemove the width of
// its remove button.
const (
	comboboxChipHeight = float32(22)
	comboboxChipRemove = float32(24)
	comboboxChipPad    = float32(6)
	comboboxChipGap    = float32(6)
)

// PaintChip paints one source-owned multi-selection token. The caller provides
// both geometry and a stable remove action ID; PaintChips lays chips out and
// paints them in a container instead.
func (combobox Combobox) PaintChip(bounds gfx.Rect, chip ComboboxChip) gfx.Rect {
	if combobox.canvas == nil || bounds.Width <= 0 || bounds.Height <= 0 {
		return gfx.Rect{}
	}
	combobox.canvas.PushFocusScope(combobox.options.Scope)
	paintComboboxChip(combobox.canvas, combobox.theme, bounds, chip)
	combobox.canvas.PopFocusScope()
	return bounds
}

// paintComboboxChip paints a chip: bg-muted, text-xs medium, the label 6px in
// and a ghost remove button at the end that is half-faded until hovered.
func paintComboboxChip(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, chip ComboboxChip) {
	canvas.Rounded(bounds, theme.Metrics.RadiusItem(), theme.Muted)
	foreground := theme.Foreground
	if chip.Disabled {
		foreground.Alpha *= 0.50
	}
	canvas.Label(chip.Label, gfx.Rect{X: bounds.X + comboboxChipPad, Y: bounds.Y, Width: comboboxMax(0, bounds.Width-comboboxChipPad-comboboxChipRemove), Height: bounds.Height}, gfx.TextStyle{
		Size: theme.Metrics.FontSizeS, Weight: gfx.WeightMedium, Color: foreground,
	})
	remove := gfx.Rect{X: bounds.X + bounds.Width - comboboxChipRemove, Y: bounds.Y, Width: comboboxChipRemove, Height: bounds.Height}
	icon := foreground
	if chip.Disabled || chip.RemoveID.IsZero() || canvas.Hovered() != chip.RemoveID {
		icon.Alpha *= 0.50
	}
	paintCross(canvas, remove, 12, icon)
	if !chip.RemoveID.IsZero() {
		canvas.HitWithOptions(chip.RemoveID, remove, "Remove selection", gfx.HitOptions{Role: gfx.RoleButton, Value: chip.Value, Hint: chip.Label, Disabled: chip.Disabled})
	}
}

// ComboboxChipsOptions describes the multi-select container: the chips, then
// the query input on the same wrapping line. State is caller-owned like the
// rest of Combobox.
type ComboboxChipsOptions struct {
	// InputID is the hit of the whole container, the editable combobox; Label,
	// Value-less semantics follow ComboboxOptions.
	InputID     gfx.ID
	Label       string
	Hint        string
	Query       string
	Placeholder string
	Open        bool
	Focused     bool
	Disabled    bool
	Invalid     bool
	// Scope is the focus scope the chips' remove buttons belong to, as for
	// ComboboxOptions.Scope.
	Scope gfx.ID
}

// ComboboxChipsLayout reports where PaintComboboxChips put everything. Chips
// holds one rectangle per chip, in order; Input is the space left for the
// query after the last chip (at least 64 wide, wrapped onto a new line when
// less room remains).
type ComboboxChipsLayout struct {
	Bounds gfx.Rect
	Chips  []gfx.Rect
	Input  gfx.Rect
}

// PaintComboboxChips paints Vega's ComboboxChips: an input-like surface whose
// chips and query input wrap onto as many lines as they need (gap-1.5, min
// height of one control). The height of bounds is ignored: the result's
// Bounds has the height the content needs, so the container grows with its
// chips. chips is a parameter of its own, not a field of options, so a caller's
// array of them can stay on its stack. dst receives the chip rectangles (pass dst[:0] of a reused slice to
// stay allocation free).
func PaintComboboxChips(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, chips []ComboboxChip, options ComboboxChipsOptions, dst []gfx.Rect) ComboboxChipsLayout {
	padX := theme.Metrics.TextInsetX()
	if len(chips) > 0 {
		padX = 1 + comboboxChipPad
	}
	padY := comboboxMax(0, (theme.Metrics.ControlHeight-2-comboboxChipHeight)/2)
	left, right := bounds.X+padX, bounds.X+bounds.Width-padX
	x, y := left, bounds.Y+1+padY
	layout := ComboboxChipsLayout{Chips: dst}
	for _, chip := range chips {
		width := comboboxChipPad + comboboxChipRemove + 4 // a little slack: labels clip to their rectangle
		if canvas != nil {
			width += canvas.TextWidthWeight(chip.Label, theme.Metrics.FontSizeS, gfx.WeightMedium)
		}
		width = min(width, right-left)
		if x > left && x+width > right {
			x, y = left, y+comboboxChipHeight+comboboxChipGap
		}
		layout.Chips = append(layout.Chips, gfx.Rect{X: x, Y: y, Width: width, Height: comboboxChipHeight})
		x += width + comboboxChipGap
	}
	inputWidth := right - x
	if inputWidth < 64 && len(chips) > 0 {
		x, y = left, y+comboboxChipHeight+comboboxChipGap
		inputWidth = right - x
	}
	layout.Input = gfx.Rect{X: x, Y: y, Width: comboboxMax(0, inputWidth), Height: comboboxChipHeight}
	layout.Bounds = gfx.Rect{X: bounds.X, Y: bounds.Y, Width: bounds.Width, Height: y + comboboxChipHeight + padY + 1 - bounds.Y}
	if canvas == nil || bounds.Width <= 0 {
		return layout
	}
	paintTextEntrySurface(canvas, &theme, layout.Bounds, gfx.State{Focused: options.Focused, Invalid: options.Invalid, Disabled: options.Disabled})
	if !options.InputID.IsZero() {
		canvas.HitWithOptions(options.InputID, layout.Bounds, options.Label, gfx.HitOptions{
			Role: gfx.RoleCombobox, Hint: options.Hint, Disabled: options.Disabled, Expanded: options.Open, NavigationKeys: true,
		})
	}
	canvas.PushFocusScope(options.Scope)
	for index, chip := range chips {
		chip.Disabled = chip.Disabled || options.Disabled
		paintComboboxChip(canvas, theme, layout.Chips[index], chip)
	}
	canvas.PopFocusScope()
	text, color := options.Query, theme.Foreground
	if text == "" {
		text, color = options.Placeholder, theme.MutedForeground
	}
	if options.Disabled {
		color.Alpha *= 0.50
	}
	if len(chips) > 0 && options.Query == "" {
		text = ""
	}
	canvas.Label(text, layout.Input, gfx.TextStyle{Size: theme.Metrics.FontSizeM, Weight: gfx.WeightRegular, Color: color})
	return layout
}

func (combobox Combobox) paintInput() {
	canvas := combobox.canvas
	bounds := combobox.inputBounds
	options := combobox.options
	paintTextEntrySurface(canvas, &combobox.theme, bounds, gfx.State{
		Focused:  !options.InputID.IsZero() && canvas.Focused() == options.InputID,
		Invalid:  options.Invalid,
		Disabled: options.Disabled,
	})
	value := options.Query
	if value == "" {
		value = options.Value
	}
	foreground := combobox.theme.Foreground
	if value == "" {
		value = options.Placeholder
		foreground = combobox.theme.MutedForeground
	}
	if options.Disabled {
		foreground.Alpha *= 0.50
	}
	// Vega: an InputGroup whose end addon holds an icon-xs ghost button (24
	// square, 4px in from the border) for the clear action or the trigger.
	showClear := options.ShowClear && !options.ClearID.IsZero() && (options.Query != "" || options.Value != "")
	button := gfx.Rect{X: bounds.X + bounds.Width - 5 - comboboxChipRemove, Y: bounds.Y + (bounds.Height-comboboxChipRemove)/2, Width: comboboxChipRemove, Height: comboboxChipRemove}
	inset := combobox.theme.Metrics.TextInsetX()
	right := inset
	if showClear || options.ShowTrigger {
		right = bounds.X + bounds.Width - button.X + 6
	}
	availableWidth := comboboxMax(0, bounds.Width-inset-right)
	textRect := gfx.Rect{X: bounds.X + inset, Y: bounds.Y, Width: availableWidth, Height: bounds.Height}
	if options.Query == "" && options.Detail != "" && options.Value != "" {
		sizeM := combobox.theme.Metrics.FontSizeM
		sizeS := combobox.theme.Metrics.FontSizeS
		labelWidth := canvas.TextWidthWeight(options.Value, sizeM, gfx.WeightRegular)
		maxLabelWidth := comboboxMax(0, availableWidth-40)
		if labelWidth > maxLabelWidth {
			labelWidth = maxLabelWidth
		}
		canvas.Label(canvas.FitLine(options.Value, sizeM, gfx.WeightRegular, labelWidth), gfx.Rect{X: bounds.X + inset, Y: bounds.Y, Width: labelWidth, Height: bounds.Height}, gfx.TextStyle{
			Size: sizeM, Weight: gfx.WeightRegular, Color: foreground,
		})
		detailX := bounds.X + inset + labelWidth + 8
		detailWidth := comboboxMax(0, bounds.X+inset+availableWidth-detailX)
		if detailWidth > 0 {
			detailColor := combobox.theme.MutedForeground
			if options.Disabled {
				detailColor.Alpha *= 0.50
			}
			canvas.Label(canvas.FitLine(options.Detail, sizeS, gfx.WeightRegular, detailWidth), gfx.Rect{X: detailX, Y: bounds.Y, Width: detailWidth, Height: bounds.Height}, gfx.TextStyle{
				Size: sizeS, Weight: gfx.WeightRegular, Color: detailColor,
			})
		}
	} else if options.Query == "" || options.InputID.IsZero() || canvas.Focused() != options.InputID || options.Disabled {
		canvas.Label(canvas.FitLine(value, combobox.theme.Metrics.FontSizeM, gfx.WeightRegular, availableWidth), textRect, gfx.TextStyle{Size: combobox.theme.Metrics.FontSizeM, Weight: gfx.WeightRegular, Color: foreground})
	}
	if !options.InputID.IsZero() {
		canvas.HitWithOptions(options.InputID, bounds, options.Label, gfx.HitOptions{
			Role:           gfx.RoleCombobox,
			Value:          options.Value,
			Hint:           options.Hint,
			Description:    options.Detail,
			Disabled:       options.Disabled,
			Expanded:       options.Open,
			Cursor:         gfx.PointerCursorText,
			TextBounds:     textRect,
			TextSize:       combobox.theme.Metrics.FontSizeM,
			TextColor:      combobox.theme.Foreground,
			NavigationKeys: true,
		})
	}
	iconColor := combobox.theme.MutedForeground
	if options.Disabled {
		iconColor.Alpha *= 0.50
	}
	switch {
	case showClear:
		combobox.paintIconButton(button, options.ClearID, "Clear "+options.Label, iconColor, true)
	case options.ShowTrigger:
		combobox.paintIconButton(button, options.TriggerID, "Show options", iconColor, false)
	}
}

// paintIconButton paints the 24px ghost icon button at the end of the input: a
// muted background under the pointer, the cross of the clear action or the
// chevron of the trigger, and a hit when it has an ID.
func (combobox Combobox) paintIconButton(bounds gfx.Rect, id gfx.ID, label string, color gfx.Color, cross bool) {
	canvas := combobox.canvas
	enabled := !id.IsZero() && !combobox.options.Disabled
	if enabled && (canvas.Hovered() == id || canvas.Pressed() == id) {
		canvas.Rounded(bounds, combobox.theme.Metrics.RadiusItem(), combobox.theme.Muted)
	}
	if cross {
		paintCross(canvas, bounds, 14, color)
	} else {
		paintChevron(canvas, &chevronDownIcon, bounds, color)
	}
	if !id.IsZero() {
		canvas.HitWithOptions(id, bounds, label, gfx.HitOptions{Role: gfx.RoleButton, Disabled: combobox.options.Disabled})
	}
}

func (combobox Combobox) pushOverlay() {
	combobox.canvas.PushLayer(combobox.options.Layer)
	if !combobox.options.Scope.IsZero() {
		combobox.canvas.PushFocusScope(combobox.options.Scope)
	}
}
func (combobox Combobox) popOverlay() {
	if !combobox.options.Scope.IsZero() {
		combobox.canvas.PopFocusScope()
	}
	combobox.canvas.PopLayer()
}
func comboboxMax(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}
