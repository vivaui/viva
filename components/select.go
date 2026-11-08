package components

import "github.com/vivaui/viva/ui"

// SelectSize selects the height of the trigger and, for the extra-small one, the
// type step of its value.
type SelectSize uint8

const (
	SelectSizeDefault SelectSize = iota
	SelectSizeSmall
	// SelectSizeXSmall is the chip step: eight pixels shorter than a control and
	// one type step below the body, like ButtonSizeXSmall. The options in its
	// popup stay at body size, as a compact control's menu does on the desktop.
	SelectSizeXSmall
)

// SelectOptions describes application-owned trigger and popup state.
type SelectOptions struct {
	TriggerID   ui.ID
	Label       string
	Value       string
	Placeholder string
	Hint        string
	Open        bool
	Disabled    bool
	Invalid     bool
	Size        SelectSize
	// Icon, when it has any geometry, is painted before the trigger's value. A
	// single-color icon takes the value's color; a multi-color one keeps its own.
	Icon ui.Icon
	// IconImage is a full-color image in the icon slot when Icon is empty.
	IconImage ui.Bitmap
	// IconText is a short text glyph when Icon and IconImage are empty or invalid.
	IconText  string
	Content   ui.Rect
	Layer     uint16
	Scope     ui.ID
	DismissID ui.ID
	// Viewport, with DismissID, makes the open popup declare a hit over the
	// whole window behind its surface, so pressing anywhere outside it,
	// including the trigger, reports DismissID (Escape reports it too). Leave
	// it empty to dismiss only on Escape and handle outside presses yourself.
	Viewport ui.Rect
	// Scroll is the scroll position of a popup whose items do not all fit its
	// Content (see MenuScroll); without one such a popup shows only the items at
	// its top. Keep it for as long as the popup is open and reset it when it opens.
	Scroll *MenuScroll
	// Unpainted makes the select declare everything it is to the pointer, the keyboard and
	// assistive technology (the trigger's hit, the popup's focus scope, escape and dismiss
	// targets, its entrance, the rows' hits and scrolling) and paint none of it: the caller
	// paints the trigger, the panel and the rows in the rectangles TriggerBounds,
	// ContentBounds and the AddItem results give, from SelectRecipe, PopupRecipe and
	// MenuItemRecipe or from its own look. It is the same code path as the painted select, so
	// a select of the application's own cannot behave differently.
	Unpainted bool
}

// SelectItem describes one stable value in the open popup.
type SelectItem struct {
	ID       ui.ID
	Label    string
	Value    string
	Hint     string
	Selected bool
	Disabled bool
	// Icon, when it has any geometry, is painted before the label.
	Icon ui.Icon
	// IconImage is a full-color image in the icon slot when Icon is empty.
	IconImage ui.Bitmap
	// IconText is a short text glyph when Icon and IconImage are empty or invalid.
	IconText string
}

// selectMinWidth is the popup's narrowest width: Vega's min-w-36.
const selectMinWidth = 144

// selectIconSize and selectIconGap place an option icon: the label moves right
// by their sum.
const (
	selectIconSize = 16
	selectIconGap  = 8
)

// selectTriggerSize is the height of a trigger and the type step of its value.
// Only the extra-small step changes the type, as in Button: a compact control
// keeps the body size and just gets shorter.
func selectTriggerSize(metrics ui.Metrics, size SelectSize) (height, textSize float32) {
	switch size {
	case SelectSizeSmall:
		return metrics.ControlHeight - 4, metrics.FontSizeM
	case SelectSizeXSmall:
		return metrics.ControlHeight - 8, metrics.FontSizeS
	}
	return metrics.ControlHeight, metrics.FontSizeM
}

// Select paints a typed native select trigger and, when open, its source-owned
// popup. Value ownership and popup placement remain application state.
type Select struct {
	canvas        *ui.Canvas
	theme         ui.Theme
	triggerBounds ui.Rect
	contentBounds ui.Rect
	options       SelectOptions
	// textSize is the type step of the value in the trigger; the popup's rows
	// stay at body size.
	textSize float32
	// rows lays the popup's rows out, scrolled when the options have a Scroll.
	rows menuSurface
	open bool
}

// BeginSelect paints the trigger and optional open popup.
func BeginSelect(canvas *ui.Canvas, theme ui.Theme, triggerBounds ui.Rect, options SelectOptions) Select {
	height, textSize := selectTriggerSize(theme.Metrics, options.Size)
	if triggerBounds.Height <= 0 {
		triggerBounds.Height = height
	}
	selectControl := Select{
		canvas: canvas, theme: theme, triggerBounds: triggerBounds, textSize: textSize,
		options: options, open: canvas != nil && options.Open,
	}
	if canvas == nil || triggerBounds.Width <= 0 || triggerBounds.Height <= 0 {
		selectControl.open = false
		return selectControl
	}
	selectControl.paintTrigger()
	if !selectControl.open {
		return selectControl
	}
	content := options.Content
	if content.Width < selectMinWidth {
		content.Width = selectMinWidth
	}
	if content.Width < triggerBounds.Width {
		content.Width = triggerBounds.Width
	}
	if content.Height <= 0 {
		selectControl.open = false
		return selectControl
	}
	selectControl.contentBounds = content
	selectControl.rows = menuSurface{
		canvas: canvas, theme: theme, bounds: content,
		options: menuSurfaceOptions{layer: options.Layer, scope: options.Scope, scroll: options.Scroll},
		top:     content.Y + menuPadding, bottom: content.Y + content.Height - menuPadding,
		viewHeight: max(content.Height-2*menuPadding, 0),
	}
	selectControl.rows.beginScroll()
	if options.Scroll != nil {
		options.Scroll.TabCloses = true
	}
	selectControl.pushOverlay()
	// Claim after pushing the popup's scope, so a select inside a dialog owns
	// the trap and Escape while it is open.
	if !options.Scope.IsZero() {
		canvas.TrapFocus(options.Scope)
	}
	if !options.DismissID.IsZero() {
		canvas.SetEscapeTarget(options.DismissID)
	}
	if !options.DismissID.IsZero() && options.Viewport.Width > 0 && options.Viewport.Height > 0 {
		canvas.HitWithOptions(options.DismissID, options.Viewport, "Dismiss select", ui.HitOptions{Role: ui.RoleButton, SkipFocus: true, Backdrop: true, PassSecondary: true})
	}
	// The popup is inside the select: a press on its padding or a disabled option
	// must not reach the dismiss backdrop behind it.
	canvas.Block(content)
	canvas.Container(content, ui.RoleListBox, options.Label)
	side := popupSideOf(content, triggerBounds)
	canvas.BeginPresence(overlayKey(options.TriggerID, options.DismissID, options.Scope), overlaySurfacePart,
		popupPresence(theme, overlayDuration, side), popupPivot(content, side, triggerBounds))
	if !options.Unpainted {
		paintPopupSurface(canvas, &theme, content, ui.ShadowMD) // shadow-md
	}
	selectControl.popOverlay()
	return selectControl
}

// End paints the scrollbar of a popup whose items did not all fit, and ends the
// popup's entrance and exit: call it once the last item is painted, or the popup
// appears and disappears at once. A closed select needs no End, and is not harmed
// by one.
func (selectControl Select) End() {
	if selectControl.open {
		selectControl.rows.end()
		selectControl.canvas.EndPresence()
	}
}

// TriggerBounds returns the normalized trigger geometry.
func (selectControl Select) TriggerBounds() ui.Rect {
	return selectControl.triggerBounds
}

// ContentBounds returns the normalized popup geometry, or zero when closed.
func (selectControl Select) ContentBounds() ui.Rect {
	return selectControl.contentBounds
}

// NextItemBounds reports the next keyboard-navigation row.
func (selectControl Select) NextItemBounds() ui.Rect {
	return selectControl.rowBounds(32)
}

// AddLabel paints one muted group label in the open popup.
func (selectControl *Select) AddLabel(label string) ui.Rect {
	bounds := selectControl.rowBounds(28)
	if !selectControl.reserve(bounds) {
		return ui.Rect{}
	}
	selectControl.rows.paintRow(bounds, func() {
		if selectControl.options.Unpainted {
			return
		}
		selectControl.canvas.Label(label, ui.Rect{X: bounds.X + 8, Y: bounds.Y, Width: selectMax(0, bounds.Width-16), Height: bounds.Height}, ui.TextStyle{
			Size: selectControl.theme.Metrics.FontSizeS, Weight: ui.WeightRegular, Color: selectControl.theme.MutedForeground,
		})
	})
	return bounds
}

// AddItem paints one option with selected and disabled semantics. The rectangle
// is empty when the row is scrolled out of the popup.
func (selectControl *Select) AddItem(item SelectItem) ui.Rect {
	bounds := selectControl.rowBounds(32)
	if !selectControl.open || !selectControl.rows.takeItem(bounds, item.ID, item.Label, item.Disabled) {
		return ui.Rect{}
	}
	selectControl.rows.paintRow(bounds, func() { selectControl.paintItem(bounds, item) })
	return bounds
}

// AddSeparator paints a group divider.
func (selectControl *Select) AddSeparator() ui.Rect {
	bounds := selectControl.rowBounds(9)
	bounds.X, bounds.Width = selectControl.contentBounds.X, selectControl.contentBounds.Width
	if !selectControl.reserve(bounds) {
		return ui.Rect{}
	}
	selectControl.rows.paintRow(bounds, func() {
		if selectControl.options.Unpainted {
			return
		}
		selectControl.canvas.Fill(ui.Rect{X: bounds.X, Y: bounds.Y + 4, Width: bounds.Width, Height: 1}, selectControl.theme.Border)
	})
	return bounds
}

// AddScrollUp paints the explicit scroll-up action used by bounded popups.
func (selectControl *Select) AddScrollUp(id ui.ID, disabled bool) ui.Rect {
	return selectControl.addScrollAction(id, &chevronUpIcon, "Scroll up", disabled)
}

// AddScrollDown paints the explicit scroll-down action used by bounded popups.
func (selectControl *Select) AddScrollDown(id ui.ID, disabled bool) ui.Rect {
	return selectControl.addScrollAction(id, &chevronDownIcon, "Scroll down", disabled)
}

func (selectControl Select) paintTrigger() {
	canvas := selectControl.canvas
	bounds := selectControl.triggerBounds
	options := selectControl.options
	enabled := !options.TriggerID.IsZero() && !options.Disabled
	if !options.Unpainted {
		state := ui.State{
			Focused:  enabled && canvas.FocusVisible() == options.TriggerID,
			Invalid:  options.Invalid,
			Disabled: options.Disabled,
			Hovered:  enabled && (canvas.Hovered() == options.TriggerID || canvas.Pressed() == options.TriggerID),
		}
		look := selectRecipe(&selectControl.theme, state, options.Value == "")
		paintTextEntryLook(canvas, selectControl.theme.Background.Alpha, bounds, look.Surface)
		foreground := look.Foreground
		value := options.Value
		if value == "" {
			value = options.Placeholder
		}
		// Vega: pl-2.5 and pr-2 inside the border, gap-1.5 before the 16px chevron.
		const chevronSpan = 1 + 8 + selectIconSize
		inset := selectControl.theme.Metrics.TextInsetX()
		textX, textWidth := bounds.X+inset, bounds.Width-inset-chevronSpan-6
		if paintSelectIcon(canvas, options.Icon, options.IconImage, options.IconText, ui.Rect{X: textX, Y: bounds.Y, Width: selectIconSize, Height: bounds.Height}, foreground, selectControl.textSize) {
			textX += selectIconSize + selectIconGap
			textWidth -= selectIconSize + selectIconGap
		}
		canvas.Label(value, ui.Rect{X: textX, Y: bounds.Y, Width: selectMax(0, textWidth), Height: bounds.Height}, ui.TextStyle{
			Size: selectControl.textSize, Weight: ui.WeightRegular, Color: foreground,
		})
		paintChevron(canvas, &chevronDownIcon, ui.Rect{X: bounds.X + bounds.Width - chevronSpan, Y: bounds.Y, Width: selectIconSize, Height: bounds.Height}, look.Chevron)
	}
	if !options.TriggerID.IsZero() {
		canvas.HitWithOptions(options.TriggerID, bounds, options.Label, ui.HitOptions{
			Role: ui.RoleCombobox, Value: options.Value, Hint: options.Hint,
			Disabled: options.Disabled, Expanded: options.Open,
		})
	}
}

func (selectControl *Select) paintItem(bounds ui.Rect, item SelectItem) {
	if !selectControl.options.Unpainted {
		state := ui.State{Disabled: item.Disabled}
		if !item.ID.IsZero() {
			state = selectControl.canvas.StateOf(item.ID)
			state.Disabled = item.Disabled
		}
		look := menuItemRecipe(&selectControl.theme, state)
		if look.Fill.Alpha > 0 {
			selectControl.canvas.Rounded(bounds, look.Radius, look.Fill)
		}
		foreground := look.Foreground
		textX, textWidth := bounds.X+8, bounds.Width-40
		if paintSelectIcon(selectControl.canvas, item.Icon, item.IconImage, item.IconText, ui.Rect{X: textX, Y: bounds.Y, Width: selectIconSize, Height: bounds.Height}, foreground, selectControl.theme.Metrics.FontSizeM) {
			textX += selectIconSize + selectIconGap
			textWidth -= selectIconSize + selectIconGap
		}
		selectControl.canvas.Label(item.Label, ui.Rect{X: textX, Y: bounds.Y, Width: selectMax(0, textWidth), Height: bounds.Height}, ui.TextStyle{
			Size: selectControl.theme.Metrics.FontSizeM, Weight: ui.WeightRegular, Color: foreground,
		})
		if item.Selected {
			paintCheck(selectControl.canvas, ui.Rect{X: bounds.X + bounds.Width - 24, Y: bounds.Y, Width: 14, Height: bounds.Height}, 14, foreground)
		}
	}
	if !item.ID.IsZero() {
		hit := bounds
		hit.Width -= selectControl.rows.hitInset
		selectControl.canvas.HitWithOptions(item.ID, hit, item.Label, ui.HitOptions{
			Role: ui.RoleOption, Value: item.Value, Hint: item.Hint,
			Disabled: item.Disabled, Selected: item.Selected,
		})
	}
}

func (selectControl *Select) addScrollAction(id ui.ID, icon *ui.Icon, label string, disabled bool) ui.Rect {
	bounds := selectControl.rowBounds(24)
	if !selectControl.reserve(bounds) {
		return ui.Rect{}
	}
	selectControl.rows.paintRow(bounds, func() {
		if !selectControl.options.Unpainted {
			color := selectControl.theme.MutedForeground
			if disabled {
				color.Alpha *= 0.50
			}
			paintChevron(selectControl.canvas, icon, bounds, color)
		}
		if !id.IsZero() {
			selectControl.canvas.HitWithOptions(id, bounds, label, ui.HitOptions{
				Role: ui.RoleMenuItem, Disabled: disabled,
			})
		}
	})
	return bounds
}

// reserve takes the row's room and reports whether any of it shows.
func (selectControl *Select) reserve(row ui.Rect) bool {
	return selectControl.open && selectControl.rows.take(row)
}

func (selectControl Select) rowBounds(height float32) ui.Rect {
	return selectControl.rows.rowBounds(height)
}

func (selectControl Select) pushOverlay() {
	selectControl.canvas.PushLayer(selectControl.options.Layer)
	if !selectControl.options.Scope.IsZero() {
		selectControl.canvas.PushFocusScope(selectControl.options.Scope)
	}
}

func (selectControl Select) popOverlay() {
	if !selectControl.options.Scope.IsZero() {
		selectControl.canvas.PopFocusScope()
	}
	selectControl.canvas.PopLayer()
}

// paintSelectIcon centers a vector icon, image or text glyph in the slot, in
// that order of preference, and reports whether it had anything to draw.
func paintSelectIcon(canvas *ui.Canvas, icon ui.Icon, image ui.Bitmap, text string, slot ui.Rect, tint ui.Color, fontSize float32) bool {
	if icon.ViewBox <= 0 || len(icon.Commands) == 0 && len(icon.Layers) == 0 {
		if image.Width > 0 && image.Height > 0 && len(image.Pix) > 0 {
			before := len(canvas.Images())
			scale := min(slot.Width/float32(image.Width), slot.Height/float32(image.Height))
			width, height := float32(image.Width)*scale, float32(image.Height)*scale
			canvas.DrawBitmap(image, ui.Rect{X: slot.X + (slot.Width-width)/2, Y: slot.Y + (slot.Height-height)/2, Width: width, Height: height})
			if len(canvas.Images()) > before {
				return true
			}
		}
		if text == "" {
			return false
		}
		canvas.LabelCenter(text, slot, ui.TextStyle{Size: fontSize, Weight: ui.WeightMedium, Color: tint})
		return true
	}
	side := min(slot.Width, slot.Height)
	canvas.DrawIcon(icon, ui.Rect{X: slot.X, Y: slot.Y + (slot.Height-side)/2, Width: side, Height: side}, tint)
	return true
}

func selectMax(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

// SelectLook is the look of a select's trigger in one state.
type SelectLook struct {
	// Surface is the trigger's box: the text-entry recipe, which the select shares with the
	// input, in the state the trigger is in (a hovered or pressed trigger takes the dark
	// theme's hover fill).
	Surface TextEntryLook
	// Foreground is the color of the value, or of the placeholder while there is no value.
	Foreground ui.Color
	// Chevron is the color of the arrow at the trigger's end.
	Chevron ui.Color
}

// SelectRecipe resolves the look of a select trigger. Hovered, Focused, Invalid and Disabled of
// state are read; placeholder says the trigger shows its placeholder, which is muted. A pure
// function, the one definition the trigger is painted from.
func SelectRecipe(theme ui.Theme, state ui.State, placeholder bool) SelectLook {
	return selectRecipe(&theme, state, placeholder)
}

// selectRecipe is the recipe for a theme that is not to be copied: the controls paint every frame,
// and a Theme is large.
func selectRecipe(theme *ui.Theme, state ui.State, placeholder bool) SelectLook {
	look := SelectLook{Surface: textEntryRecipe(theme, state), Foreground: theme.Foreground, Chevron: theme.MutedForeground}
	if placeholder {
		look.Foreground = theme.MutedForeground
	}
	if state.Disabled {
		look.Foreground.Alpha *= 0.50
		look.Chevron.Alpha *= 0.50
	}
	return look
}
