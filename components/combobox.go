package components

import "github.com/vivaui/viva/ui"

// ComboboxOptions describes one application-owned editable selection snapshot.
type ComboboxOptions struct {
	InputID ui.ID
	ClearID ui.ID
	// TriggerID, when set, makes the chevron a button of its own (Vega's
	// icon-xs ghost trigger) with a hover background; leave it zero when the
	// whole input already toggles the popup.
	TriggerID   ui.ID
	Label       string
	Value       string
	Query       string
	Placeholder string
	Hint        string
	Open        bool
	Disabled    bool
	Invalid     bool
	ShowTrigger bool
	ShowClear   bool
	Content     ui.Rect
	Layer       uint16
	Scope       ui.ID
	DismissID   ui.ID
	// Viewport, with DismissID, makes the open popup declare a hit over the
	// whole window behind its surface, so pressing anywhere outside it reports
	// DismissID (Escape reports it too). See SelectOptions.Viewport.
	Viewport ui.Rect
	// Scroll is the scroll position of a popup whose options do not all fit its
	// Content (see MenuScroll); without one only the options at the top show.
	Scroll *MenuScroll
}

// ComboboxItem describes one caller-filtered option.
type ComboboxItem struct {
	ID       ui.ID
	Label    string
	Value    string
	Hint     string
	Selected bool
	Disabled bool
}

// ComboboxChip describes one caller-owned multi-selection token.
type ComboboxChip struct {
	RemoveID ui.ID
	Label    string
	Value    string
	Disabled bool
}

// Combobox paints an editable trigger and its optional popup snapshot.
type Combobox struct {
	canvas        *ui.Canvas
	theme         ui.Theme
	inputBounds   ui.Rect
	contentBounds ui.Rect
	options       ComboboxOptions
	rows          menuSurface
	open          bool
}

// BeginCombobox paints the input group and optional popup surface.
func BeginCombobox(canvas *ui.Canvas, theme ui.Theme, inputBounds ui.Rect, options ComboboxOptions) Combobox {
	if inputBounds.Height <= 0 {
		inputBounds.Height = theme.Metrics.ControlHeight
	}
	combobox := Combobox{canvas: canvas, theme: theme, inputBounds: inputBounds, options: options, open: canvas != nil && options.Open}
	if canvas == nil || inputBounds.Width <= 0 || inputBounds.Height <= 0 {
		combobox.open = false
		return combobox
	}
	combobox.paintInput()
	if !combobox.open || options.Content.Height <= 0 {
		combobox.open = false
		return combobox
	}
	content := options.Content
	minimumWidth := inputBounds.Width + 28
	if content.Width < minimumWidth {
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
	if !options.DismissID.IsZero() {
		canvas.SetEscapeTarget(options.DismissID)
	}
	if !options.DismissID.IsZero() && options.Viewport.Width > 0 && options.Viewport.Height > 0 {
		canvas.HitWithOptions(options.DismissID, options.Viewport, "Dismiss combobox", ui.HitOptions{Role: ui.RoleButton, SkipFocus: true, Backdrop: true, PassSecondary: true})
	}
	// The popup is inside the combobox: a press on its padding or a disabled
	// option must not reach the dismiss backdrop behind it.
	canvas.Block(content)
	canvas.Container(content, ui.RoleListBox, options.Label)
	side := popupSideOf(content, inputBounds)
	canvas.BeginPresence(overlayKey(options.InputID, options.DismissID, options.Scope), overlaySurfacePart,
		popupPresence(theme, overlayDuration, side), popupPivot(content, side, inputBounds))
	canvas.Elevate(content, theme.Metrics.RadiusPopup(), ui.ShadowMD) // shadow-md
	canvas.RoundedBorder(content, theme.Metrics.RadiusPopup(), 1, theme.Border, theme.Popover)
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
func (combobox Combobox) InputBounds() ui.Rect { return combobox.inputBounds }

// ContentBounds returns the normalized open popup geometry.
func (combobox Combobox) ContentBounds() ui.Rect { return combobox.contentBounds }

// NextItemBounds returns the next option row without mutating state.
func (combobox Combobox) NextItemBounds() ui.Rect { return combobox.rows.rowBounds(32) }

// Navigate moves the keyboard through the open options and scrolls the one it
// reaches into view; see MenuScroll.Navigate.
func (combobox Combobox) Navigate(key ui.Key, from ui.ID, loop bool) (ui.ID, bool) {
	if combobox.options.Scroll == nil {
		return ui.ID{}, false
	}
	return combobox.options.Scroll.Navigate(key, from, loop)
}

// Match is MenuScroll.Match for the open options.
func (combobox Combobox) Match(prefix string, from ui.ID) (ui.ID, bool) {
	if combobox.options.Scroll == nil {
		return ui.ID{}, false
	}
	return combobox.options.Scroll.Match(prefix, from)
}

// AddLabel paints one compact group label.
func (combobox *Combobox) AddLabel(label string) ui.Rect {
	bounds := combobox.rows.rowBounds(28)
	if !combobox.rows.take(bounds) {
		return ui.Rect{}
	}
	combobox.rows.paintRow(bounds, func() {
		combobox.canvas.Label(label, ui.Rect{X: bounds.X + 8, Y: bounds.Y, Width: comboboxMax(0, bounds.Width-16), Height: bounds.Height}, ui.TextStyle{
			Size: combobox.theme.Metrics.FontSizeS, Weight: ui.WeightMedium, Color: combobox.theme.MutedForeground,
		})
	})
	return bounds
}

// AddItem paints one option and its selected indicator.
func (combobox *Combobox) AddItem(item ComboboxItem) ui.Rect {
	bounds := combobox.rows.rowBounds(32)
	if !combobox.rows.takeItem(bounds, item.ID, item.Label, item.Disabled) {
		return ui.Rect{}
	}
	combobox.rows.paintRow(bounds, func() {
		// Only the highlighted row is tinted (Vega's data-highlighted); a selected
		// one is told apart by its check.
		active := !item.Disabled && !item.ID.IsZero() && (combobox.canvas.Hovered() == item.ID || combobox.canvas.Pressed() == item.ID || combobox.canvas.FocusVisible() == item.ID)
		if active {
			combobox.canvas.Rounded(bounds, combobox.theme.Metrics.RadiusItem(), combobox.theme.Accent)
		}
		foreground := combobox.theme.PopoverForeground
		if item.Disabled {
			foreground.Alpha *= 0.50
		}
		combobox.canvas.Label(item.Label, ui.Rect{X: bounds.X + 8, Y: bounds.Y, Width: comboboxMax(0, bounds.Width-40), Height: bounds.Height}, ui.TextStyle{
			Size: combobox.theme.Metrics.FontSizeM, Weight: ui.WeightRegular, Color: foreground,
		})
		if item.Selected {
			paintCheck(combobox.canvas, ui.Rect{X: bounds.X + bounds.Width - 26, Y: bounds.Y, Width: 16, Height: bounds.Height}, 14, foreground)
		}
		if !item.ID.IsZero() {
			hit := bounds
			hit.Width -= combobox.rows.hitInset
			combobox.canvas.HitWithOptions(item.ID, hit, item.Label, ui.HitOptions{
				Role: ui.RoleOption, Value: item.Value, Hint: item.Hint,
				Disabled: item.Disabled, Selected: item.Selected,
			})
		}
	})
	return bounds
}

// AddSeparator paints the official one-pixel divider.
func (combobox *Combobox) AddSeparator() ui.Rect {
	bounds := combobox.rows.rowBounds(9)
	if !combobox.rows.take(bounds) {
		return ui.Rect{}
	}
	combobox.rows.paintRow(bounds, func() {
		combobox.canvas.Fill(ui.Rect{X: combobox.contentBounds.X, Y: bounds.Y + 4, Width: combobox.contentBounds.Width, Height: 1}, combobox.theme.Border)
	})
	return bounds
}

// AddEmpty paints the centered empty-result state.
func (combobox *Combobox) AddEmpty(label string) ui.Rect {
	// Vega: py-2 around one 20px line.
	bounds := combobox.rows.rowBounds(36)
	if !combobox.rows.take(bounds) {
		return ui.Rect{}
	}
	combobox.rows.paintRow(bounds, func() {
		combobox.canvas.LabelCenter(label, bounds, ui.TextStyle{Size: combobox.theme.Metrics.FontSizeM, Weight: ui.WeightRegular, Color: combobox.theme.MutedForeground})
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
func (combobox Combobox) PaintChip(bounds ui.Rect, chip ComboboxChip) ui.Rect {
	if combobox.canvas == nil || bounds.Width <= 0 || bounds.Height <= 0 {
		return ui.Rect{}
	}
	combobox.canvas.PushFocusScope(combobox.options.Scope)
	paintComboboxChip(combobox.canvas, combobox.theme, bounds, chip)
	combobox.canvas.PopFocusScope()
	return bounds
}

// paintComboboxChip paints a chip: bg-muted, text-xs medium, the label 6px in
// and a ghost remove button at the end that is half-faded until hovered.
func paintComboboxChip(canvas *ui.Canvas, theme ui.Theme, bounds ui.Rect, chip ComboboxChip) {
	canvas.Rounded(bounds, theme.Metrics.RadiusItem(), theme.Muted)
	foreground := theme.Foreground
	if chip.Disabled {
		foreground.Alpha *= 0.50
	}
	canvas.Label(chip.Label, ui.Rect{X: bounds.X + comboboxChipPad, Y: bounds.Y, Width: comboboxMax(0, bounds.Width-comboboxChipPad-comboboxChipRemove), Height: bounds.Height}, ui.TextStyle{
		Size: theme.Metrics.FontSizeS, Weight: ui.WeightMedium, Color: foreground,
	})
	remove := ui.Rect{X: bounds.X + bounds.Width - comboboxChipRemove, Y: bounds.Y, Width: comboboxChipRemove, Height: bounds.Height}
	icon := foreground
	if chip.Disabled || chip.RemoveID.IsZero() || canvas.Hovered() != chip.RemoveID {
		icon.Alpha *= 0.50
	}
	paintCross(canvas, remove, 12, icon)
	if !chip.RemoveID.IsZero() {
		canvas.HitWithOptions(chip.RemoveID, remove, "Remove selection", ui.HitOptions{Role: ui.RoleButton, Value: chip.Value, Hint: chip.Label, Disabled: chip.Disabled})
	}
}

// ComboboxChipsOptions describes the multi-select container: the chips, then
// the query input on the same wrapping line. State is caller-owned like the
// rest of Combobox.
type ComboboxChipsOptions struct {
	// InputID is the hit of the whole container, the editable combobox; Label,
	// Value-less semantics follow ComboboxOptions.
	InputID     ui.ID
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
	Scope ui.ID
}

// ComboboxChipsLayout reports where PaintComboboxChips put everything. Chips
// holds one rectangle per chip, in order; Input is the space left for the
// query after the last chip (at least 64 wide, wrapped onto a new line when
// less room remains).
type ComboboxChipsLayout struct {
	Bounds ui.Rect
	Chips  []ui.Rect
	Input  ui.Rect
}

// PaintComboboxChips paints Vega's ComboboxChips: an input-like surface whose
// chips and query input wrap onto as many lines as they need (gap-1.5, min
// height of one control). The height of bounds is ignored: the result's
// Bounds has the height the content needs, so the container grows with its
// chips. chips is a parameter of its own, not a field of options, so a caller's
// array of them can stay on its stack. dst receives the chip rectangles (pass dst[:0] of a reused slice to
// stay allocation free).
func PaintComboboxChips(canvas *ui.Canvas, theme ui.Theme, bounds ui.Rect, chips []ComboboxChip, options ComboboxChipsOptions, dst []ui.Rect) ComboboxChipsLayout {
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
			width += canvas.TextWidthWeight(chip.Label, theme.Metrics.FontSizeS, ui.WeightMedium)
		}
		width = min(width, right-left)
		if x > left && x+width > right {
			x, y = left, y+comboboxChipHeight+comboboxChipGap
		}
		layout.Chips = append(layout.Chips, ui.Rect{X: x, Y: y, Width: width, Height: comboboxChipHeight})
		x += width + comboboxChipGap
	}
	inputWidth := right - x
	if inputWidth < 64 && len(chips) > 0 {
		x, y = left, y+comboboxChipHeight+comboboxChipGap
		inputWidth = right - x
	}
	layout.Input = ui.Rect{X: x, Y: y, Width: comboboxMax(0, inputWidth), Height: comboboxChipHeight}
	layout.Bounds = ui.Rect{X: bounds.X, Y: bounds.Y, Width: bounds.Width, Height: y + comboboxChipHeight + padY + 1 - bounds.Y}
	if canvas == nil || bounds.Width <= 0 {
		return layout
	}
	paintTextEntrySurface(canvas, theme, layout.Bounds, textEntryState{Focused: options.Focused, Invalid: options.Invalid, Disabled: options.Disabled})
	if !options.InputID.IsZero() {
		canvas.HitWithOptions(options.InputID, layout.Bounds, options.Label, ui.HitOptions{
			Role: ui.RoleCombobox, Hint: options.Hint, Disabled: options.Disabled, Expanded: options.Open,
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
	canvas.Label(text, layout.Input, ui.TextStyle{Size: theme.Metrics.FontSizeM, Weight: ui.WeightRegular, Color: color})
	return layout
}

func (combobox Combobox) paintInput() {
	canvas := combobox.canvas
	bounds := combobox.inputBounds
	options := combobox.options
	paintTextEntrySurface(canvas, combobox.theme, bounds, textEntryState{
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
	button := ui.Rect{X: bounds.X + bounds.Width - 5 - comboboxChipRemove, Y: bounds.Y + (bounds.Height-comboboxChipRemove)/2, Width: comboboxChipRemove, Height: comboboxChipRemove}
	inset := combobox.theme.Metrics.TextInsetX()
	right := inset
	if showClear || options.ShowTrigger {
		right = bounds.X + bounds.Width - button.X + 6
	}
	canvas.Label(value, ui.Rect{X: bounds.X + inset, Y: bounds.Y, Width: comboboxMax(0, bounds.Width-inset-right), Height: bounds.Height}, ui.TextStyle{Size: combobox.theme.Metrics.FontSizeM, Weight: ui.WeightRegular, Color: foreground})
	if !options.InputID.IsZero() {
		canvas.HitWithOptions(options.InputID, bounds, options.Label, ui.HitOptions{
			Role: ui.RoleCombobox, Value: options.Value, Hint: options.Hint,
			Disabled: options.Disabled, Expanded: options.Open,
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
func (combobox Combobox) paintIconButton(bounds ui.Rect, id ui.ID, label string, color ui.Color, cross bool) {
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
		canvas.HitWithOptions(id, bounds, label, ui.HitOptions{Role: ui.RoleButton, Disabled: combobox.options.Disabled})
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
