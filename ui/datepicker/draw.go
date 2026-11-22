package datepicker

import (
	"time"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/style"
	"github.com/vivaui/viva/ui"
)

// Part identities are fixed so a key aimed at a part can be turned back into a
// cell without keeping a map. The window for presets and the one for cells are
// reserved whether or not they are painted this frame.
const (
	partAnchor = iota // never hit: the panel's focus scope
	partClear
	partDismiss
	partPrev
	partNext
	partPrevYear
	partNextYear
	partTitle
	partToday
	partTitleRight
	partStartField
	partEndField
)

const (
	partPresetBase   = 16
	partPresetWindow = 16
	partCellBase     = 64
	partCellWindow   = 128
)

const (
	// cellInset is how much shorter a cell is than the control, as Calendar's
	// cells are.
	cellInset = float32(4)
	// focusRingOffset is how far the focus ring reaches beyond a cell, as
	// Calendar's ring does.
	focusRingOffset = float32(3)
	// panelOffset is the distance between the trigger and the panel.
	panelOffset = float32(4)
	// clearHitInset grows the clear button's hit beyond its glyph, as an icon
	// button's padding does.
	clearHitInset = float32(2)
)

func (picker *DatePicker) draw(d *ui.DrawContext) {
	canvas := d.Canvas
	theme := *d.Theme
	picker.sync(theme)
	parts := picker.handlers.Parts()
	if picker.Open && canvas.Clicked(parts.At(partDismiss)) {
		picker.dismiss()
	}
	picker.paintTrigger(canvas, theme, d.Rect, d.State)
	if !picker.Open {
		return
	}
	picker.paintPanel(d, theme, d.Rect)
}

// sync rebuilds the derived text and cells only when an input changed; a warm
// repaint reads the previous frame's snapshot and allocates nothing.
func (picker *DatePicker) sync(theme gfx.Theme) {
	key := cacheKey{
		granularity: picker.granularity(),
		weekStart:   picker.WeekStart,
		view:        picker.view,
		value:       picker.Value,
		today:       picker.today,
		min:         picker.Min,
		max:         picker.Max,
		hasValue:    picker.HasValue,
		open:        picker.Open,
		layer:       picker.layer,
		small:       picker.Small,
		format:      picker.Format,
		labels:      picker.Labels,
		revision:    picker.revision,
	}
	if picker.cacheValid && picker.cacheKey == key {
		return
	}
	picker.cacheKey, picker.cacheValid = key, true
	labels := resolveLabels(picker.Labels)
	picker.labels = labels
	layout := picker.Format
	if layout == "" {
		layout = labels.Layouts[int(picker.granularity())]
	}
	picker.text = labels.Placeholder
	if picker.HasValue && !picker.Value.IsZero() {
		picker.text = picker.Value.Format(layout)
	}
	if !picker.Open {
		picker.count = 0
		return
	}
	picker.rebuild()
}

func (picker *DatePicker) paintTrigger(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, state gfx.State) {
	if bounds.Width <= 0 || bounds.Height <= 0 {
		return
	}
	enabled := !picker.Disabled
	look := components.SelectRecipe(theme, gfx.State{
		Focused:      state.Focused,
		FocusVisible: state.FocusVisible,
		Hovered:      enabled && (state.Hovered || state.Pressed),
		Disabled:     picker.Disabled,
		Invalid:      picker.Invalid,
	}, !picker.HasValue)
	surface := look.Surface
	canvas.Elevate(bounds, surface.Radius, surface.Shadow)
	if theme.Background.Alpha < 1 {
		canvas.Rounded(bounds, surface.Radius, surface.Fill)
		canvas.RoundedOutline(bounds, surface.Radius, surface.BorderWidth, surface.Border)
	} else {
		canvas.RoundedBorder(bounds, surface.Radius, surface.BorderWidth, surface.Border, surface.Fill)
	}

	inset := theme.Metrics.TextInsetX()
	iconSize := float32(style.ButtonIconSize)
	gap := theme.Metrics.GapItem / 2
	right := bounds.X + bounds.Width - inset
	iconBounds := gfx.Rect{X: bounds.X + inset, Y: bounds.Y + (bounds.Height-iconSize)/2, Width: iconSize, Height: iconSize}
	canvas.DrawIcon(calendarIcon, iconBounds, look.Chevron)
	textX := iconBounds.X + iconSize + gap
	trailing := float32(0)
	clearVisible := picker.AllowClear && picker.HasValue && !picker.Disabled
	if clearVisible {
		trailing = iconSize + gap
	}
	textWidth := max(bounds.Width-(textX-bounds.X)-inset-trailing, 0)
	canvas.Label(picker.text, gfx.Rect{X: textX, Y: bounds.Y, Width: textWidth, Height: bounds.Height}, gfx.TextStyle{
		Size: theme.Metrics.FontSizeM, Weight: gfx.WeightRegular, Color: look.Foreground,
	})
	if clearVisible {
		clearBounds := gfx.Rect{X: right - iconSize, Y: iconBounds.Y, Width: iconSize, Height: iconSize}
		parts := picker.handlers.Parts()
		if canvas.Hovered() == parts.At(partClear) || canvas.Pressed() == parts.At(partClear) {
			canvas.Rounded(gfx.Rect{X: clearBounds.X - clearHitInset, Y: clearBounds.Y - clearHitInset, Width: clearBounds.Width + 2*clearHitInset, Height: clearBounds.Height + 2*clearHitInset},
				theme.Metrics.RadiusItem(), theme.Muted)
		}
		canvas.DrawIcon(closeIcon, clearBounds, look.Chevron)
		canvas.HitWithOptions(parts.At(partClear), gfx.Rect{X: clearBounds.X - clearHitInset, Y: clearBounds.Y - clearHitInset, Width: clearBounds.Width + 2*clearHitInset, Height: clearBounds.Height + 2*clearHitInset},
			picker.labels.Clear, gfx.HitOptions{Role: gfx.RoleButton})
	}
}

// geometry is the panel's one layout pass: the same numbers place the header,
// the weekday row, the grid and the footer, so their sizes cannot drift apart.
type geometry struct {
	panel       gfx.Rect
	header      gfx.Rect
	weekdays    gfx.Rect
	grid        gfx.Rect
	footer      gfx.Rect
	cellW       float32
	cellH       float32
	rowH        float32
	columns     int
	rows        int
	hasWeekdays bool
	hasFooter   bool
	presetRects [partPresetWindow]gfx.Rect
}

func (picker *DatePicker) geometry(canvas *gfx.Canvas, theme gfx.Theme, trigger, viewport gfx.Rect) geometry {
	metrics := theme.Metrics
	pad := metrics.GapItem
	gap := metrics.GapItem
	rowGap := metrics.GapItem / 4
	weekdayHeight := metrics.FontSizeS * 2
	monthCellExtra := metrics.GapItem
	triggerHeight := picker.triggerHeight(metrics.ControlHeight)
	cellH := max(triggerHeight-cellInset, metrics.FontSizeS)
	render := picker.renderGranularity()
	columns, rows := gridShape(render)
	// The title needs room between the navigation buttons; a month or year
	// panel has one button a side, the day panel two.
	navs := float32(4)
	if render != GranularityDay {
		navs = 2
	}
	titleWidth := canvas.TextWidthStyle(picker.title, gfx.TextStyle{Size: metrics.FontSizeM, Weight: gfx.WeightMedium})
	width := max(7*cellH+2*pad, navs*triggerHeight+titleWidth+2*pad+gap)
	rowH := cellH
	if render != GranularityDay {
		rowH = cellH + monthCellExtra
	}
	hasWeekdays := render == GranularityDay
	todayShown := render == GranularityDay && picker.ShowToday
	chipH := triggerHeight
	chipGap := metrics.GapItem / 2
	// Preset chips wrap inside the footer, reserving the Today link's room on
	// every row so the link always sits at the bottom right.
	todayWidth := float32(0)
	if todayShown {
		todayWidth = max(canvas.TextWidthStyle(picker.labels.Today, gfx.TextStyle{Size: metrics.FontSizeS, Weight: gfx.WeightMedium})+2*metrics.PadControlX, 2*metrics.PadControlX+2*metrics.FontSizeS)
	}
	chipArea := width - 2*pad - todayWidth
	if todayShown {
		chipArea -= chipGap
	}
	var presetRects [partPresetWindow]gfx.Rect
	presetRows := 0
	x, y := float32(0), float32(0)
	for index := 0; index < len(picker.Presets) && index < partPresetWindow; index++ {
		label := picker.Presets[index].Label
		chipWidth := max(canvas.TextWidthStyle(label, gfx.TextStyle{Size: metrics.FontSizeS, Weight: gfx.WeightMedium})+2*metrics.PadControlX, 2*metrics.PadControlX+2*metrics.FontSizeS)
		if presetRows == 0 {
			presetRows = 1
		}
		if x > 0 && x+chipWidth > chipArea {
			x, y = 0, y+chipH+chipGap
			presetRows++
		}
		presetRects[index] = gfx.Rect{X: x, Y: y, Width: chipWidth, Height: chipH}
		x += chipWidth + chipGap
	}
	hasFooter := todayShown || presetRows > 0
	footerHeight := chipH
	if presetRows > 1 {
		footerHeight = float32(presetRows)*chipH + float32(presetRows-1)*chipGap
	}
	height := 2*pad + triggerHeight + gap + float32(rows)*rowH + float32(rows-1)*rowGap
	if hasWeekdays {
		height += weekdayHeight + gap
	}
	if hasFooter {
		height += gap + footerHeight
	}

	x = trigger.X
	if x+width > viewport.X+viewport.Width {
		x = viewport.X + viewport.Width - width
	}
	if x < viewport.X {
		x = viewport.X
	}
	y = trigger.Y + trigger.Height + panelOffset
	if y+height > viewport.Y+viewport.Height {
		if above := trigger.Y - panelOffset - height; above >= viewport.Y {
			y = above
		} else {
			y = max(viewport.Y, viewport.Y+viewport.Height-height)
		}
	}
	panel := gfx.Rect{X: x, Y: y, Width: width, Height: height}
	layout := geometry{
		panel: panel, cellW: (width - 2*pad) / float32(columns), cellH: cellH, rowH: rowH,
		columns: columns, rows: rows, hasWeekdays: hasWeekdays, hasFooter: hasFooter,
	}
	y = panel.Y + pad
	layout.header = gfx.Rect{X: panel.X + pad, Y: y, Width: width - 2*pad, Height: triggerHeight}
	y += triggerHeight + gap
	if hasWeekdays {
		layout.weekdays = gfx.Rect{X: panel.X + pad, Y: y, Width: width - 2*pad, Height: weekdayHeight}
		y += weekdayHeight + gap
	}
	layout.grid = gfx.Rect{X: panel.X + pad, Y: y, Width: width - 2*pad, Height: float32(rows)*rowH + float32(rows-1)*rowGap}
	if hasFooter {
		layout.footer = gfx.Rect{X: panel.X + pad, Y: panel.Y + height - pad - footerHeight, Width: width - 2*pad, Height: footerHeight}
		for index := range presetRects {
			if presetRects[index].Width <= 0 {
				continue
			}
			presetRects[index].X += layout.footer.X
			presetRects[index].Y += layout.footer.Y
		}
		layout.presetRects = presetRects
	}
	return layout
}

func (picker *DatePicker) paintPanel(d *ui.DrawContext, theme gfx.Theme, trigger gfx.Rect) {
	canvas := d.Canvas
	parts := picker.handlers.Parts()
	layout := picker.geometry(d.Canvas, theme, trigger, d.Frame.GetBounds())
	anchor := parts.At(partAnchor)
	canvas.PushLayer(d.Frame.WidgetLayer() + 2)
	canvas.PushFocusScope(anchor)
	canvas.PushOverlayClip(d.Frame.GetBounds())
	canvas.HitWithOptions(parts.At(partDismiss), d.Frame.GetBounds(), picker.labels.Dismiss, gfx.HitOptions{
		Role: gfx.RoleButton, Backdrop: true, PassSecondary: true, SkipFocus: true,
	})
	canvas.Block(layout.panel)
	canvas.Elevate(layout.panel, theme.Metrics.RadiusPopup(), gfx.ShadowMD)
	canvas.RoundedBorder(layout.panel, theme.Metrics.RadiusPopup(), 1, theme.Border, theme.Popover)

	picker.paintHeader(canvas, theme, layout)
	if layout.hasWeekdays {
		picker.paintWeekdays(canvas, theme, layout)
	}
	picker.paintGrid(canvas, theme, layout)
	if layout.hasFooter {
		picker.paintFooter(canvas, theme, layout)
	}

	canvas.TrapFocus(anchor)
	canvas.SetInitialFocus(parts.At(partCellBase + picker.focus))
	canvas.SetEscapeTarget(parts.At(partDismiss))
	canvas.PopOverlayClip()
	canvas.PopFocusScope()
	canvas.PopLayer()
}

func (picker *DatePicker) paintHeader(canvas *gfx.Canvas, theme gfx.Theme, layout geometry) {
	parts := picker.handlers.Parts()
	header := layout.header
	size := header.Height
	left := header.X
	right := header.X + header.Width
	day := picker.renderGranularity() == GranularityDay
	titleLeft, titleRight := left, right
	if day {
		paintNavButton(canvas, theme, parts.At(partPrevYear), gfx.Rect{X: left, Y: header.Y, Width: size, Height: size}, picker.labels.PreviousYear, &chevronsLeftIcon, size)
		paintNavButton(canvas, theme, parts.At(partPrev), gfx.Rect{X: left + size, Y: header.Y, Width: size, Height: size}, picker.labels.Previous, &chevronLeftIcon, size)
		paintNavButton(canvas, theme, parts.At(partNext), gfx.Rect{X: right - 2*size, Y: header.Y, Width: size, Height: size}, picker.labels.Next, &chevronRightIcon, size)
		paintNavButton(canvas, theme, parts.At(partNextYear), gfx.Rect{X: right - size, Y: header.Y, Width: size, Height: size}, picker.labels.NextYear, &chevronsRightIcon, size)
		titleLeft, titleRight = left+2*size, right-2*size
	} else {
		previous, next := picker.labels.PreviousYear, picker.labels.NextYear
		if picker.renderGranularity() == GranularityYear {
			previous, next = picker.labels.PreviousDecade, picker.labels.NextDecade
		}
		paintNavButton(canvas, theme, parts.At(partPrev), gfx.Rect{X: left, Y: header.Y, Width: size, Height: size}, previous, &chevronLeftIcon, size)
		paintNavButton(canvas, theme, parts.At(partNext), gfx.Rect{X: right - size, Y: header.Y, Width: size, Height: size}, next, &chevronRightIcon, size)
		titleLeft, titleRight = left+size, right-size
	}
	title := gfx.Rect{X: titleLeft, Y: header.Y, Width: max(titleRight-titleLeft, 0), Height: size}
	drillable := picker.layer < picker.maxLayer()
	if drillable {
		id := parts.At(partTitle)
		background := theme.Background
		background.Alpha = 0
		if canvas.Hovered() == id || canvas.Pressed() == id {
			background = theme.Muted
		}
		canvas.Rounded(title, theme.Metrics.RadiusItem(), background)
		if canvas.FocusVisible() == id {
			ring := theme.Ring
			ring.Alpha *= 0.5
			canvas.Rounded(gfx.Rect{X: title.X - focusRingOffset, Y: title.Y - focusRingOffset, Width: title.Width + 2*focusRingOffset, Height: title.Height + 2*focusRingOffset}, theme.Metrics.RadiusItem()+focusRingOffset, ring)
		}
	}
	canvas.LabelCenter(picker.title, title, gfx.TextStyle{
		Size: theme.Metrics.FontSizeM, Weight: gfx.WeightMedium, Color: theme.Foreground,
	})
	options := gfx.HitOptions{Role: gfx.RoleStatus, Live: gfx.LivePolite, SkipFocus: true}
	if drillable {
		hint := picker.labels.ShowYear
		if picker.renderGranularity() == GranularityDay {
			hint = picker.labels.ShowMonth
		}
		options = gfx.HitOptions{Role: gfx.RoleButton, Hint: hint}
	}
	canvas.HitWithOptions(parts.At(partTitle), title, picker.title, options)
}

func paintNavButton(canvas *gfx.Canvas, theme gfx.Theme, id gfx.ID, bounds gfx.Rect, label string, icon *gfx.Icon, iconSize float32) {
	hovered := canvas.Hovered() == id || canvas.Pressed() == id
	background := theme.Background
	background.Alpha = 0
	if hovered {
		background = theme.Muted
	}
	canvas.Rounded(bounds, theme.Metrics.RadiusItem(), background)
	if canvas.FocusVisible() == id {
		ring := theme.Ring
		ring.Alpha *= 0.5
		canvas.Rounded(gfx.Rect{X: bounds.X - focusRingOffset, Y: bounds.Y - focusRingOffset, Width: bounds.Width + 2*focusRingOffset, Height: bounds.Height + 2*focusRingOffset}, theme.Metrics.RadiusItem()+focusRingOffset, ring)
	}
	canvas.DrawIcon(*icon, gfx.Rect{
		X: bounds.X + (bounds.Width-iconSize/2)/2, Y: bounds.Y + (bounds.Height-iconSize/2)/2,
		Width: iconSize / 2, Height: iconSize / 2,
	}, theme.Foreground)
	canvas.HitWithOptions(id, bounds, label, gfx.HitOptions{Role: gfx.RoleButton})
}

func (picker *DatePicker) paintWeekdays(canvas *gfx.Canvas, theme gfx.Theme, layout geometry) {
	for column := 0; column < 7; column++ {
		weekday := time.Weekday((int(picker.WeekStart) + column) % 7)
		cell := gfx.Rect{
			X: layout.grid.X + float32(column)*layout.cellW, Y: layout.weekdays.Y,
			Width: layout.cellW, Height: layout.weekdays.Height,
		}
		canvas.LabelCenter(picker.labels.Weekdays[weekday], cell, gfx.TextStyle{
			Size: theme.Metrics.FontSizeS, Weight: gfx.WeightRegular, Color: theme.MutedForeground,
		})
	}
}

func (picker *DatePicker) paintGrid(canvas *gfx.Canvas, theme gfx.Theme, layout geometry) {
	gridHeight := float32(layout.rows)*layout.rowH + float32(layout.rows-1)*2
	canvas.Container(gfx.Rect{X: layout.grid.X, Y: layout.grid.Y, Width: layout.grid.Width, Height: gridHeight}, gfx.RoleGrid, picker.title)
	for index := 0; index < picker.count; index++ {
		column, row := index%layout.columns, index/layout.columns
		cell := gfx.Rect{
			X:     layout.grid.X + float32(column)*layout.cellW,
			Y:     layout.grid.Y + float32(row)*(layout.rowH+2),
			Width: layout.cellW, Height: layout.rowH,
		}
		picker.paintCell(canvas, theme, cell, index)
	}
}

func (picker *DatePicker) paintCell(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, index int) {
	cell := picker.cells[index]
	id := picker.handlers.Parts().At(partCellBase + index)
	hovered := canvas.Hovered() == id || canvas.Pressed() == id
	radius := min(theme.Metrics.RadiusControl, bounds.Height/2)
	background := theme.Background
	background.Alpha = 0
	foreground := theme.Foreground
	switch {
	case cell.selected:
		background = theme.Primary
		foreground = theme.PrimaryForeground
	case cell.today:
		background = theme.Muted
	case hovered:
		background = theme.Muted
	}
	if cell.outside {
		foreground = theme.MutedForeground
	}
	if cell.disabled {
		foreground = theme.MutedForeground
		foreground.Alpha *= 0.5
		background.Alpha *= 0.5
	}
	if !cell.disabled && canvas.FocusVisible() == id {
		ring := theme.Ring
		ring.Alpha *= 0.5
		canvas.Rounded(gfx.Rect{X: bounds.X - focusRingOffset, Y: bounds.Y - focusRingOffset, Width: bounds.Width + 2*focusRingOffset, Height: bounds.Height + 2*focusRingOffset}, radius+focusRingOffset, ring)
	}
	canvas.Rounded(bounds, radius, background)
	canvas.LabelCenter(cell.label, bounds, gfx.TextStyle{
		Size: theme.Metrics.FontSizeM, Weight: gfx.WeightRegular, Color: foreground,
	})
	canvas.HitWithOptions(id, bounds, cell.a11y, gfx.HitOptions{
		Role: gfx.RoleGridCell, Value: cell.iso, Selected: cell.selected,
		Disabled: cell.disabled, SkipTab: index != picker.focus,
	})
}

// paintFooter draws the bottom row: the caller's shortcut chips, wrapped onto
// as many rows as they need, and the Today link at the right of the last row.
func (picker *DatePicker) paintFooter(canvas *gfx.Canvas, theme gfx.Theme, layout geometry) {
	parts := picker.handlers.Parts()
	style := gfx.TextStyle{Size: theme.Metrics.FontSizeS, Weight: gfx.WeightMedium}
	chipPad := theme.Metrics.PadControlX
	chipH := picker.triggerHeight(theme.Metrics.ControlHeight)
	right := layout.footer.X + layout.footer.Width
	if picker.granularity() == GranularityDay && picker.ShowToday {
		width := max(canvas.TextWidthStyle(picker.labels.Today, style)+2*chipPad, 2*chipPad+2*theme.Metrics.FontSizeS)
		button := gfx.Rect{X: right - width, Y: layout.footer.Y + layout.footer.Height - chipH, Width: width, Height: chipH}
		hovered := canvas.Hovered() == parts.At(partToday) || canvas.Pressed() == parts.At(partToday)
		background := theme.Background
		background.Alpha = 0
		if hovered {
			background = theme.Muted
		}
		canvas.Rounded(button, theme.Metrics.RadiusItem(), background)
		if canvas.FocusVisible() == parts.At(partToday) {
			ring := theme.Ring
			ring.Alpha *= 0.5
			canvas.Rounded(gfx.Rect{X: button.X - focusRingOffset, Y: button.Y - focusRingOffset, Width: button.Width + 2*focusRingOffset, Height: button.Height + 2*focusRingOffset}, theme.Metrics.RadiusItem()+focusRingOffset, ring)
		}
		canvas.LabelCenter(picker.labels.Today, button, gfx.TextStyle{Size: style.Size, Weight: style.Weight, Color: theme.Foreground})
		canvas.HitWithOptions(parts.At(partToday), button, picker.labels.Today, gfx.HitOptions{Role: gfx.RoleButton})
	}
	for index, chip := range layout.presetRects {
		if chip.Width <= 0 {
			continue
		}
		id := parts.At(partPresetBase + index)
		hovered := canvas.Hovered() == id || canvas.Pressed() == id
		background := theme.Background
		background.Alpha = 0
		if hovered {
			background = theme.Muted
		}
		canvas.Rounded(chip, theme.Metrics.RadiusItem(), background)
		canvas.LabelCenter(picker.Presets[index].Label, chip, gfx.TextStyle{Size: style.Size, Weight: style.Weight, Color: theme.Foreground})
		canvas.HitWithOptions(id, chip, picker.Presets[index].Label, gfx.HitOptions{Role: gfx.RoleButton})
	}
}

// applyPreset writes a shortcut value and reports it, as choosing a cell does.
func (picker *DatePicker) applyPreset(index int) {
	if index < 0 || index >= len(picker.Presets) {
		return
	}
	value := normalize(picker.Presets[index].Value, picker.granularity())
	if value.IsZero() {
		return
	}
	picker.Value, picker.HasValue = value, true
	picker.view = pageStart(value, picker.granularity())
	picker.revision++
	picker.closePanel()
	if picker.OnChange != nil {
		picker.OnChange(picker.Value, true)
	}
}
