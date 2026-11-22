package datepicker

import (
	"time"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/style"
	"github.com/vivaui/viva/ui"
)

const (
	// rangeBandAlpha tints a committed range between its ends.
	rangeBandAlpha = float32(0.14)
	// rangeBandPreviewAlpha tints a range being previewed.
	rangeBandPreviewAlpha = float32(0.08)
)

func (picker *DateRangePicker) draw(d *ui.DrawContext) {
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

func (picker *DateRangePicker) sync(theme gfx.Theme) {
	key := rangeCacheKey{
		granularity: picker.granularity(),
		weekStart:   picker.WeekStart,
		view:        picker.view,
		start:       picker.Start,
		end:         picker.End,
		today:       picker.today,
		min:         picker.Min,
		max:         picker.Max,
		startSet:    picker.StartSet,
		endSet:      picker.EndSet,
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
	picker.text[0], picker.text[1] = labels.RangePlaceholder[0], labels.RangePlaceholder[1]
	if picker.StartSet && !picker.Start.IsZero() {
		picker.text[0] = picker.Start.Format(layout)
	}
	if picker.EndSet && !picker.End.IsZero() {
		picker.text[1] = picker.End.Format(layout)
	}
	if !picker.Open {
		picker.count = 0
		return
	}
	picker.rebuild()
}

func (picker *DateRangePicker) paintTrigger(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, state gfx.State) {
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
	}, !picker.StartSet && !picker.EndSet)
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
	separator := iconSize + theme.Metrics.GapItem
	right := bounds.X + bounds.Width - inset
	clearWidth := float32(0)
	if picker.AllowClear && (picker.StartSet || picker.EndSet) && !picker.Disabled {
		clearWidth = iconSize + gap
	}
	iconBounds := gfx.Rect{X: bounds.X + inset, Y: bounds.Y + (bounds.Height-iconSize)/2, Width: iconSize, Height: iconSize}
	canvas.DrawIcon(calendarIcon, iconBounds, look.Chevron)
	fieldsX := iconBounds.X + iconSize + gap
	available := bounds.X + bounds.Width - inset - clearWidth - fieldsX - separator
	half := max(available/2, 0)
	startRect := gfx.Rect{X: fieldsX, Y: bounds.Y, Width: half, Height: bounds.Height}
	endRect := gfx.Rect{X: startRect.X + half + separator, Y: bounds.Y, Width: half, Height: bounds.Height}
	separatorRect := gfx.Rect{X: startRect.X + half, Y: bounds.Y, Width: separator, Height: bounds.Height}

	parts := picker.handlers.Parts()
	valueColor, placeholderColor := look.Foreground, theme.MutedForeground
	startColor, endColor := placeholderColor, placeholderColor
	if picker.StartSet {
		startColor = valueColor
	}
	if picker.EndSet {
		endColor = valueColor
	}
	picker.paintFieldText(canvas, theme, startRect, picker.text[0], startColor)
	picker.paintFieldText(canvas, theme, endRect, picker.text[1], endColor)
	canvas.LabelCenter(picker.labels.RangeSeparator, separatorRect, gfx.TextStyle{
		Size: theme.Metrics.FontSizeM, Color: theme.MutedForeground,
	})
	canvas.HitWithOptions(parts.At(partStartField), startRect, picker.labels.RangePlaceholder[0], gfx.HitOptions{
		Role: gfx.RoleCombobox, Value: picker.text[0], Expanded: picker.Open && picker.anchorStart,
		Disabled: picker.Disabled, Invalid: picker.Invalid,
	})
	canvas.HitWithOptions(parts.At(partEndField), endRect, picker.labels.RangePlaceholder[1], gfx.HitOptions{
		Role: gfx.RoleCombobox, Value: picker.text[1], Expanded: picker.Open && !picker.anchorStart,
		Disabled: picker.Disabled, Invalid: picker.Invalid,
	})
	if clearWidth > 0 {
		clearBounds := gfx.Rect{X: right - iconSize, Y: bounds.Y + (bounds.Height-iconSize)/2, Width: iconSize, Height: iconSize}
		paint := gfx.Rect{X: clearBounds.X - 2, Y: clearBounds.Y - 2, Width: clearBounds.Width + 4, Height: clearBounds.Height + 4}
		if canvas.Hovered() == parts.At(partClear) || canvas.Pressed() == parts.At(partClear) {
			canvas.Rounded(paint, theme.Metrics.RadiusItem(), theme.Muted)
		}
		canvas.DrawIcon(closeIcon, clearBounds, look.Chevron)
		canvas.HitWithOptions(parts.At(partClear), paint, picker.labels.Clear, gfx.HitOptions{Role: gfx.RoleButton})
	}
}

func (picker *DateRangePicker) paintFieldText(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, text string, color gfx.Color) {
	canvas.Label(text, bounds, gfx.TextStyle{Size: theme.Metrics.FontSizeM, Weight: gfx.WeightRegular, Color: color})
}

type rangeGeometry struct {
	panel       gfx.Rect
	presets     gfx.Rect
	panes       [2]gfx.Rect
	cellW       float32
	cellH       float32
	rowH        float32
	columns     int
	rows        int
	hasPresets  bool
	presetRects [partPresetWindow]gfx.Rect
}

func (picker *DateRangePicker) geometry(canvas *gfx.Canvas, theme gfx.Theme, trigger, viewport gfx.Rect) rangeGeometry {
	metrics := theme.Metrics
	pad := metrics.GapItem
	gap := metrics.GapItem
	rowGap := metrics.GapItem / 4
	weekdayHeight := metrics.FontSizeS * 2
	extraRow := metrics.GapItem
	triggerHeight := picker.triggerHeight(metrics.ControlHeight)
	chipGap := metrics.GapItem / 2
	cellH := max(triggerHeight-cellInset, metrics.FontSizeS)
	render := picker.renderGranularity()
	columns, rows := gridShape(render)
	navs := float32(4)
	if render != GranularityDay {
		navs = 2
	}
	titleWidth := max(
		canvas.TextWidthStyle(picker.titles[0], gfx.TextStyle{Size: metrics.FontSizeM, Weight: gfx.WeightMedium}),
		canvas.TextWidthStyle(picker.titles[1], gfx.TextStyle{Size: metrics.FontSizeM, Weight: gfx.WeightMedium}),
	)
	paneWidth := max(7*cellH+2*pad, navs*triggerHeight+titleWidth+2*pad+gap)
	rowH := cellH
	hasWeekdays := render == GranularityDay
	if !hasWeekdays {
		rowH = cellH + extraRow
	}
	gridHeight := float32(rows)*rowH + float32(rows-1)*rowGap
	paneHeight := triggerHeight + gap + gridHeight
	if hasWeekdays {
		paneHeight += weekdayHeight + gap
	}
	width := 2*pad + 2*paneWidth + gap
	// Preset chips wrap inside the panel's bottom row.
	var presetRects [partPresetWindow]gfx.Rect
	chipArea := width - 2*pad
	presetRows := 0
	chipX, chipY := float32(0), float32(0)
	for index := 0; index < len(picker.Presets) && index < partPresetWindow; index++ {
		label := picker.Presets[index].Label
		chipWidth := max(canvas.TextWidthStyle(label, gfx.TextStyle{Size: metrics.FontSizeS, Weight: gfx.WeightMedium})+2*metrics.PadControlX, 2*metrics.PadControlX+2*metrics.FontSizeS)
		if presetRows == 0 {
			presetRows = 1
		}
		if chipX > 0 && chipX+chipWidth > chipArea {
			chipX, chipY = 0, chipY+triggerHeight+chipGap
			presetRows++
		}
		presetRects[index] = gfx.Rect{X: chipX, Y: chipY, Width: chipWidth, Height: triggerHeight}
		chipX += chipWidth + chipGap
	}
	hasPresets := presetRows > 0
	presetHeight := triggerHeight
	if presetRows > 1 {
		presetHeight = float32(presetRows)*triggerHeight + float32(presetRows-1)*chipGap
	}
	height := 2*pad + paneHeight
	if hasPresets {
		height += gap + presetHeight
	}

	panelX := trigger.X
	if panelX+width > viewport.X+viewport.Width {
		panelX = viewport.X + viewport.Width - width
	}
	if panelX < viewport.X {
		panelX = viewport.X
	}
	panelY := trigger.Y + trigger.Height + panelOffset
	if panelY+height > viewport.Y+viewport.Height {
		if above := trigger.Y - panelOffset - height; above >= viewport.Y {
			panelY = above
		} else {
			panelY = max(viewport.Y, viewport.Y+viewport.Height-height)
		}
	}
	layout := rangeGeometry{
		panel:      gfx.Rect{X: panelX, Y: panelY, Width: width, Height: height},
		cellW:      paneWidth / float32(columns),
		cellH:      cellH,
		rowH:       rowH,
		columns:    columns,
		rows:       rows,
		hasPresets: hasPresets,
	}
	if hasPresets {
		layout.presets = gfx.Rect{
			X: panelX + pad, Y: panelY + height - pad - presetHeight,
			Width: width - 2*pad, Height: presetHeight,
		}
		for index := range presetRects {
			if presetRects[index].Width <= 0 {
				continue
			}
			presetRects[index].X += layout.presets.X
			presetRects[index].Y += layout.presets.Y
		}
		layout.presetRects = presetRects
	}
	left := panelX + pad
	layout.panes[0] = gfx.Rect{X: left, Y: panelY + pad, Width: paneWidth, Height: paneHeight}
	layout.panes[1] = gfx.Rect{X: left + paneWidth + gap, Y: panelY + pad, Width: paneWidth, Height: paneHeight}
	return layout
}

func (picker *DateRangePicker) paintPanel(d *ui.DrawContext, theme gfx.Theme, trigger gfx.Rect) {
	canvas := d.Canvas
	parts := picker.handlers.Parts()
	layout := picker.geometry(d.Canvas, theme, trigger, d.Frame.GetBounds())
	anchor := parts.At(partAnchor)
	picker.hoverValid = false
	if index, ok := canvas.Hovered().In(parts); ok && index >= partCellBase && index < partCellBase+2*picker.count {
		picker.hoverValue, picker.hoverValid = picker.cells[index-partCellBase].value, true
	}

	canvas.PushLayer(d.Frame.WidgetLayer() + 2)
	canvas.PushFocusScope(anchor)
	canvas.PushOverlayClip(d.Frame.GetBounds())
	canvas.HitWithOptions(parts.At(partDismiss), d.Frame.GetBounds(), picker.labels.Dismiss, gfx.HitOptions{
		Role: gfx.RoleButton, Backdrop: true, PassSecondary: true, SkipFocus: true,
	})
	canvas.Block(layout.panel)
	canvas.Elevate(layout.panel, theme.Metrics.RadiusPopup(), gfx.ShadowMD)
	canvas.RoundedBorder(layout.panel, theme.Metrics.RadiusPopup(), 1, theme.Border, theme.Popover)
	if layout.hasPresets {
		picker.paintPresets(canvas, theme, layout)
	}
	for pane := 0; pane < 2; pane++ {
		picker.paintPane(canvas, theme, layout, pane)
	}
	canvas.TrapFocus(anchor)
	canvas.SetInitialFocus(parts.At(partCellBase + picker.focus))
	canvas.SetEscapeTarget(parts.At(partDismiss))
	canvas.PopOverlayClip()
	canvas.PopFocusScope()
	canvas.PopLayer()
}

// paintPresets draws the shortcut chips along the panel's bottom edge, wrapped
// onto as many rows as they need.
func (picker *DateRangePicker) paintPresets(canvas *gfx.Canvas, theme gfx.Theme, layout rangeGeometry) {
	parts := picker.handlers.Parts()
	style := gfx.TextStyle{Size: theme.Metrics.FontSizeS, Weight: gfx.WeightMedium}
	for index, chip := range layout.presetRects {
		if chip.Width <= 0 {
			continue
		}
		label := picker.Presets[index].Label
		id := parts.At(partPresetBase + index)
		hovered := canvas.Hovered() == id || canvas.Pressed() == id
		disabled := picker.Presets[index].Disabled
		background := theme.Background
		background.Alpha = 0
		if hovered && !disabled {
			background = theme.Muted
		}
		canvas.Rounded(chip, theme.Metrics.RadiusItem(), background)
		color := theme.Foreground
		if disabled {
			color = theme.MutedForeground
			color.Alpha *= 0.5
		}
		canvas.LabelCenter(label, chip, gfx.TextStyle{Size: style.Size, Weight: style.Weight, Color: color})
		canvas.HitWithOptions(id, chip, label, gfx.HitOptions{Role: gfx.RoleButton, Disabled: disabled})
	}
}

func (picker *DateRangePicker) paintPane(canvas *gfx.Canvas, theme gfx.Theme, layout rangeGeometry, pane int) {
	parts := picker.handlers.Parts()
	area := layout.panes[pane]
	header := gfx.Rect{X: area.X, Y: area.Y, Width: area.Width, Height: picker.triggerHeight(theme.Metrics.ControlHeight)}
	size := header.Height
	title := gfx.Rect{X: header.X, Y: header.Y, Width: header.Width, Height: header.Height}
	day := picker.renderGranularity() == GranularityDay
	// A day panel turns months and has two arrows a side; a month or year panel
	// turns years or decades and has one.
	previousLabel, nextLabel := picker.labels.Previous, picker.labels.Next
	if !day {
		previousLabel, nextLabel = picker.labels.PreviousYear, picker.labels.NextYear
		if picker.renderGranularity() == GranularityYear {
			previousLabel, nextLabel = picker.labels.PreviousDecade, picker.labels.NextDecade
		}
	}
	if pane == 0 {
		if day {
			paintNavButton(canvas, theme, parts.At(partPrevYear), gfx.Rect{X: header.X, Y: header.Y, Width: size, Height: size}, picker.labels.PreviousYear, &chevronsLeftIcon, size)
			paintNavButton(canvas, theme, parts.At(partPrev), gfx.Rect{X: header.X + size, Y: header.Y, Width: size, Height: size}, previousLabel, &chevronLeftIcon, size)
			title.Width -= 2 * size
			title.X += 2 * size
		} else {
			paintNavButton(canvas, theme, parts.At(partPrev), gfx.Rect{X: header.X, Y: header.Y, Width: size, Height: size}, previousLabel, &chevronLeftIcon, size)
			title.Width -= size
			title.X += size
		}
	} else {
		if day {
			paintNavButton(canvas, theme, parts.At(partNext), gfx.Rect{X: header.X + header.Width - 2*size, Y: header.Y, Width: size, Height: size}, nextLabel, &chevronRightIcon, size)
			paintNavButton(canvas, theme, parts.At(partNextYear), gfx.Rect{X: header.X + header.Width - size, Y: header.Y, Width: size, Height: size}, picker.labels.NextYear, &chevronsRightIcon, size)
			title.Width -= 2 * size
		} else {
			paintNavButton(canvas, theme, parts.At(partNext), gfx.Rect{X: header.X + header.Width - size, Y: header.Y, Width: size, Height: size}, nextLabel, &chevronRightIcon, size)
			title.Width -= size
		}
	}
	drillable := pane == 0 && picker.layer < picker.maxLayer()
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
	canvas.LabelCenter(picker.titles[pane], title, gfx.TextStyle{
		Size: theme.Metrics.FontSizeM, Weight: gfx.WeightMedium, Color: theme.Foreground,
	})
	if pane == 0 {
		options := gfx.HitOptions{Role: gfx.RoleStatus, Live: gfx.LivePolite, SkipFocus: true}
		if drillable {
			hint := picker.labels.ShowYear
			if day {
				hint = picker.labels.ShowMonth
			}
			options = gfx.HitOptions{Role: gfx.RoleButton, Hint: hint}
		}
		canvas.HitWithOptions(parts.At(partTitle), title, picker.titles[0], options)
	} else {
		canvas.HitWithOptions(parts.At(partTitleRight), title, picker.titles[1], gfx.HitOptions{
			Role: gfx.RoleStatus, Live: gfx.LivePolite, SkipFocus: true,
		})
	}

	y := area.Y + size + 8
	if day {
		weekdays := gfx.Rect{X: area.X, Y: y, Width: area.Width, Height: 24}
		for column := 0; column < 7; column++ {
			weekday := time.Weekday((int(picker.WeekStart) + column) % 7)
			cell := gfx.Rect{X: weekdays.X + float32(column)*layout.cellW, Y: weekdays.Y, Width: layout.cellW, Height: weekdays.Height}
			canvas.LabelCenter(picker.labels.Weekdays[weekday], cell, gfx.TextStyle{
				Size: theme.Metrics.FontSizeS, Color: theme.MutedForeground,
			})
		}
		y += 24 + 8
	}
	gridHeight := float32(layout.rows)*layout.rowH + float32(layout.rows-1)*2
	grid := gfx.Rect{X: area.X, Y: y, Width: area.Width, Height: gridHeight}
	canvas.Container(grid, gfx.RoleGrid, picker.titles[pane])
	for index := 0; index < picker.count; index++ {
		column, row := index%layout.columns, index/layout.columns
		cell := gfx.Rect{
			X:     grid.X + float32(column)*layout.cellW,
			Y:     grid.Y + float32(row)*(layout.rowH+2),
			Width: layout.cellW, Height: layout.rowH,
		}
		picker.paintCell(canvas, theme, cell, pane*picker.count+index)
	}
}

func (picker *DateRangePicker) marks(value time.Time) cellMarks {
	if picker.hasPending {
		low, high := picker.pending, picker.pending
		if picker.hoverValid {
			if picker.hoverValue.Before(picker.pending) {
				low, high = picker.hoverValue, picker.pending
			} else {
				high = picker.hoverValue
			}
		}
		if value.Before(low) || value.After(high) {
			return cellMarks{}
		}
		return cellMarks{
			start: value.Equal(low), end: value.Equal(high),
			middle: value.After(low) && value.Before(high), preview: true,
		}
	}
	if !picker.StartSet || !picker.EndSet {
		return cellMarks{}
	}
	if value.Before(picker.Start) || value.After(picker.End) {
		return cellMarks{}
	}
	return cellMarks{
		start: value.Equal(picker.Start), end: value.Equal(picker.End),
		middle: value.After(picker.Start) && value.Before(picker.End), selected: true,
	}
}

func (picker *DateRangePicker) paintCell(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, index int) {
	cell := picker.cells[index]
	id := picker.handlers.Parts().At(partCellBase + index)
	marks := picker.marks(cell.value)
	hovered := canvas.Hovered() == id || canvas.Pressed() == id
	radius := min(theme.Metrics.RadiusControl, bounds.Height/2)
	background := theme.Background
	background.Alpha = 0
	foreground := theme.Foreground
	committed := !marks.preview && (marks.start || marks.middle || marks.end)
	preview := marks.preview && (marks.start || marks.middle || marks.end)
	switch {
	case committed && (marks.start || marks.end):
		background = theme.Primary
		foreground = theme.PrimaryForeground
	case committed && marks.middle:
		background = rangeBand(theme, false)
	case preview && marks.middle:
		background = rangeBand(theme, true)
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
	if marks.start && !marks.end {
		canvas.Fill(gfx.Rect{X: bounds.X + bounds.Width/2, Y: bounds.Y, Width: bounds.Width / 2, Height: bounds.Height}, picker.marksColor(theme, committed))
	}
	if marks.end && !marks.start {
		canvas.Fill(gfx.Rect{X: bounds.X, Y: bounds.Y, Width: bounds.Width / 2, Height: bounds.Height}, picker.marksColor(theme, committed))
	}
	if !cell.disabled && canvas.FocusVisible() == id {
		ring := theme.Ring
		ring.Alpha *= 0.5
		canvas.Rounded(gfx.Rect{X: bounds.X - focusRingOffset, Y: bounds.Y - focusRingOffset, Width: bounds.Width + 2*focusRingOffset, Height: bounds.Height + 2*focusRingOffset}, radius+focusRingOffset, ring)
	}
	canvas.Rounded(bounds, radius, background)
	if preview && (marks.start || marks.end) {
		canvas.RoundedOutline(bounds, radius, 1, theme.Primary)
	}
	canvas.LabelCenter(cell.label, bounds, gfx.TextStyle{
		Size: theme.Metrics.FontSizeM, Weight: gfx.WeightRegular, Color: foreground,
	})
	canvas.HitWithOptions(id, bounds, cell.a11y, gfx.HitOptions{
		Role: gfx.RoleGridCell, Value: cell.iso, Selected: committed || (preview && (marks.start || marks.end)),
		Disabled: cell.disabled, SkipTab: index != picker.focus,
	})
}

// rangeBand is the tint a range paints between its ends: the primary colour at
// a low alpha, so the band reads against both themes without becoming a second
// selected state.
func rangeBand(theme gfx.Theme, preview bool) gfx.Color {
	color := theme.Primary
	if preview {
		color.Alpha *= rangeBandPreviewAlpha
	} else {
		color.Alpha *= rangeBandAlpha
	}
	return color
}

// marksColor is the band a range endpoint paints beyond its own cell.
func (picker *DateRangePicker) marksColor(theme gfx.Theme, committed bool) gfx.Color {
	return rangeBand(theme, !committed)
}
