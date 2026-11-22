package datepicker

import (
	"time"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// RangePreset is a shortcut range the caller owns.
type RangePreset struct {
	Label      string
	Start, End time.Time
	// Disabled keeps the preset visible but inert.
	Disabled bool
}

// DateRangePicker picks a closed interval: both ends are inclusive, a second
// order is swapped, and OnChange runs only once both ends are chosen.
type DateRangePicker struct {
	Options
	Presets []RangePreset

	Start, End       time.Time
	StartSet, EndSet bool
	Open             bool
	// OnChange reports a complete range or a cleared one; a half-picked range
	// is a transition and is not reported.
	OnChange     func(start, end time.Time, hasValue bool)
	OnOpenChange func(open bool)

	handlers ui.Handlers
	drawFn   func(*ui.DrawContext)
	keyFn    func(gfx.KeyEvent) gfx.EventResult
	pointer  func(gfx.PointerEvent) gfx.EventResult

	view        time.Time
	today       time.Time
	layer       int
	anchorStart bool
	pending     time.Time
	hasPending  bool
	focus       int
	cells       [2 * 42]dayCell
	count       int
	columns     int
	rows        int
	titles      [2]string
	hoverValue  time.Time
	hoverValid  bool
	labels      Labels
	text        [2]string
	name        string

	cacheKey   rangeCacheKey
	cacheValid bool
	nameKey    rangeNameKey
	nameValid  bool
	revision   uint64
}

type rangeNameKey struct {
	label       string
	start, end  time.Time
	startSet    bool
	endSet      bool
	format      string
	labels      Labels
	granularity Granularity
}

type rangeCacheKey struct {
	granularity Granularity
	weekStart   time.Weekday
	view        time.Time
	start, end  time.Time
	today       time.Time
	min, max    time.Time
	startSet    bool
	endSet      bool
	open        bool
	layer       int
	small       bool
	format      string
	labels      Labels
	revision    uint64
}

type cellMarks struct {
	start, middle, end bool
	preview            bool
	selected           bool
}

// NewRange returns a day-granularity range picker unless another granularity is
// asked for.
func NewRange(granularity Granularity) *DateRangePicker {
	picker := &DateRangePicker{}
	picker.Reset()
	picker.Granularity = granularity
	return picker
}

func (picker *DateRangePicker) Reset() *DateRangePicker {
	picker.Options = Options{WeekStart: time.Monday, AllowClear: true}
	picker.Presets = nil
	picker.Start, picker.End = time.Time{}, time.Time{}
	picker.StartSet, picker.EndSet = false, false
	picker.Open = false
	picker.view, picker.today = time.Time{}, time.Time{}
	picker.layer = 0
	picker.anchorStart = true
	picker.pending = time.Time{}
	picker.hasPending = false
	picker.focus = 0
	picker.revision++
	picker.cacheValid = false
	picker.prepareRange()
	return picker
}

func (picker *DateRangePicker) Refresh() *DateRangePicker {
	picker.revision++
	return picker
}

func (picker *DateRangePicker) SetRange(start, end time.Time) *DateRangePicker {
	if start.IsZero() && end.IsZero() {
		return picker.Clear()
	}
	granularity := picker.granularity()
	start, end = normalize(start, granularity), normalize(end, granularity)
	if end.Before(start) {
		start, end = end, start
	}
	picker.Start, picker.End = start, end
	picker.StartSet, picker.EndSet = true, true
	picker.view = pageStart(start, granularity)
	picker.revision++
	return picker
}

func (picker *DateRangePicker) Clear() *DateRangePicker {
	picker.Start, picker.End = time.Time{}, time.Time{}
	picker.StartSet, picker.EndSet = false, false
	picker.pending, picker.hasPending = time.Time{}, false
	picker.revision++
	return picker
}

func (picker *DateRangePicker) Values() (start, end time.Time, hasValue bool) {
	return picker.Start, picker.End, picker.StartSet && picker.EndSet
}

// Pending reports the first end of a range being picked: it is shown in the
// panel as a preview and is not part of the value until the second end lands.
func (picker *DateRangePicker) Pending() (time.Time, bool) {
	return picker.pending, picker.hasPending
}

func (picker *DateRangePicker) View(c ui.C, label string) ui.Box {
	if picker.drawFn == nil {
		picker.prepareRange()
	}
	theme := c.Theme()
	width := float32(420)
	if picker.Small {
		width = 380
	}
	picker.name = picker.accessibleName(label)
	return ui.Control(c, picker.name, &picker.handlers).Size(width, picker.triggerHeight(theme.Metrics.ControlHeight)).Draw(picker.drawFn)
}

func (picker *DateRangePicker) prepareRange() {
	picker.handlers.Role = gfx.RoleGroup
	picker.keyFn = picker.key
	picker.pointer = picker.pointerDown
	picker.handlers.Key = picker.keyFn
	picker.handlers.Pointer = picker.pointer
	picker.drawFn = picker.draw
}

func (picker *DateRangePicker) triggerHeight(controlHeight float32) float32 {
	if picker.Small {
		return max(controlHeight-4, 16)
	}
	return controlHeight
}

func (picker *DateRangePicker) granularity() Granularity {
	switch picker.Granularity {
	case GranularityMonth, GranularityYear:
		return picker.Granularity
	}
	return GranularityDay
}

func (picker *DateRangePicker) renderGranularity() Granularity {
	switch picker.granularity() {
	case GranularityMonth:
		if picker.layer > 0 {
			return GranularityYear
		}
		return GranularityMonth
	case GranularityYear:
		return GranularityYear
	}
	switch picker.layer {
	case 1:
		return GranularityMonth
	case 2, 3:
		return GranularityYear
	}
	return GranularityDay
}

func (picker *DateRangePicker) maxLayer() int {
	switch picker.granularity() {
	case GranularityMonth:
		return 1
	case GranularityYear:
		return 0
	}
	return 2
}

func (picker *DateRangePicker) renderPage() time.Time {
	return pageStart(picker.view, picker.renderGranularity())
}

func (picker *DateRangePicker) now() time.Time {
	if picker.Now != nil {
		return picker.Now()
	}
	return time.Now()
}

func (picker *DateRangePicker) accessibleName(label string) string {
	key := rangeNameKey{
		label: label, start: picker.Start, end: picker.End,
		startSet: picker.StartSet, endSet: picker.EndSet,
		format: picker.Format, labels: picker.Labels, granularity: picker.granularity(),
	}
	if picker.nameValid && picker.nameKey == key {
		return picker.name
	}
	picker.nameKey, picker.nameValid = key, true
	labels := resolveLabels(picker.Labels)
	granularity := picker.granularity()
	layout := picker.Format
	if layout == "" {
		layout = labels.Layouts[int(granularity)]
	}
	start := labels.RangePlaceholder[0]
	if picker.StartSet && !picker.Start.IsZero() {
		start = picker.Start.Format(layout)
	}
	end := labels.RangePlaceholder[1]
	if picker.EndSet && !picker.End.IsZero() {
		end = picker.End.Format(layout)
	}
	picker.labels = labels
	picker.text[0], picker.text[1] = start, end
	picker.name = label + ": " + start + " " + labels.RangeSeparator + " " + end
	return picker.name
}

func (picker *DateRangePicker) ensureView() {
	granularity := picker.granularity()
	if picker.today.IsZero() {
		picker.today = normalize(picker.now(), GranularityDay)
	}
	if picker.StartSet && !picker.Start.IsZero() {
		picker.view = pageStart(picker.Start, granularity)
		return
	}
	if picker.view.IsZero() {
		picker.view = pageStart(picker.today, granularity)
	}
}

func (picker *DateRangePicker) openPanel(start bool) {
	if picker.Disabled {
		return
	}
	if picker.Open {
		return
	}
	picker.Open = true
	picker.layer = 0
	picker.anchorStart = start
	picker.ensureView()
	picker.rebuild()
	picker.focus = picker.initialFocus()
	if picker.OnOpenChange != nil {
		picker.OnOpenChange(true)
	}
}

func (picker *DateRangePicker) closePanel() {
	if !picker.Open {
		return
	}
	picker.Open = false
	picker.layer = 0
	picker.pending, picker.hasPending = time.Time{}, false
	if picker.OnOpenChange != nil {
		picker.OnOpenChange(false)
	}
}

func (picker *DateRangePicker) dismiss() {
	if picker.layer > 0 {
		picker.layer--
		picker.rebuild()
		picker.focus = picker.nearestSelectable(picker.initialFocus())
		return
	}
	picker.closePanel()
}

func (picker *DateRangePicker) initialTarget() time.Time {
	render := picker.renderGranularity()
	if picker.StartSet && !picker.Start.IsZero() {
		return normalize(picker.Start, render)
	}
	return normalize(picker.today, render)
}

func (picker *DateRangePicker) initialFocus() int {
	if index := picker.cellOf(picker.initialTarget()); index >= 0 && !picker.cells[index].disabled {
		return index
	}
	for index := 0; index < 2*picker.count; index++ {
		if !picker.cells[index].disabled {
			return index
		}
	}
	return 0
}

func (picker *DateRangePicker) cellOf(value time.Time) int {
	for index := 0; index < 2*picker.count; index++ {
		if picker.cells[index].value.Equal(value) {
			return index
		}
	}
	return -1
}

func (picker *DateRangePicker) shiftPage(pages int) {
	if picker.view.IsZero() {
		picker.ensureView()
	}
	picker.view = shiftPage(picker.view, picker.renderGranularity(), pages)
	picker.rebuild()
	if picker.focus >= 2*picker.count {
		picker.focus = picker.initialFocus()
	}
}

// rebuild lays out both panes: the second one is one page after the first, and
// both follow the same drill layer.
func (picker *DateRangePicker) rebuild() {
	render := picker.renderGranularity()
	page := picker.renderPage()
	labels := resolveLabels(picker.Labels)
	picker.labels = labels
	picker.columns, picker.rows = gridShape(render)
	if render == GranularityDay {
		picker.count = 7 * 6
	} else {
		picker.count = 12
	}
	today := picker.today
	if today.IsZero() {
		today = normalize(picker.now(), GranularityDay)
		picker.today = today
	}
	min, max := normalize(picker.Min, render), normalize(picker.Max, render)
	for pane := 0; pane < 2; pane++ {
		panePage := pageStart(shiftPage(page, render, pane), render)
		picker.titles[pane] = pageTitle(panePage, render, labels)
		for index := 0; index < picker.count; index++ {
			value := cellValue(panePage, render, index, picker.WeekStart)
			cell := dayCell{
				value:   value,
				iso:     cellISO(value, render),
				label:   cellLabel(value, render, labels),
				a11y:    value.Format(labels.A11yLayouts[int(render)]),
				outside: render == GranularityDay && !withinPage(value, panePage, render),
				today:   value.Equal(today),
			}
			if !min.IsZero() && value.Before(min) {
				cell.disabled = true
			}
			if !max.IsZero() && value.After(max) {
				cell.disabled = true
			}
			if !cell.disabled && picker.DisabledDate != nil && picker.DisabledDate(value) {
				cell.disabled = true
			}
			picker.cells[pane*picker.count+index] = cell
		}
	}
}

func (picker *DateRangePicker) nearestSelectable(index int) int {
	if index < 0 || index >= 2*picker.count {
		index = 0
	}
	for step := 0; step < 2*picker.count; step++ {
		next := (index + step) % (2 * picker.count)
		if !picker.cells[next].disabled {
			return next
		}
	}
	return index
}

// pickCell is the range state machine: the first press remembers an end, a
// later one at or after it commits, and one before it starts over.
func (picker *DateRangePicker) pickCell(index int) {
	if index < 0 || index >= 2*picker.count {
		return
	}
	value := picker.cells[index].value
	if picker.cells[index].disabled {
		return
	}
	if !picker.hasPending {
		picker.pending, picker.hasPending = value, true
		return
	}
	if value.Before(picker.pending) {
		picker.pending = value
		return
	}
	picker.commit(picker.pending, value)
}

func (picker *DateRangePicker) commit(start, end time.Time) {
	if end.Before(start) {
		start, end = end, start
	}
	picker.Start, picker.End = start, end
	picker.StartSet, picker.EndSet = true, true
	picker.pending, picker.hasPending = time.Time{}, false
	picker.revision++
	picker.closePanel()
	if picker.OnChange != nil {
		picker.OnChange(picker.Start, picker.End, true)
	}
}

func (picker *DateRangePicker) clearFromUI() {
	picker.Clear()
	if picker.OnChange != nil {
		picker.OnChange(time.Time{}, time.Time{}, false)
	}
}

func (picker *DateRangePicker) applyPreset(index int) {
	if index < 0 || index >= len(picker.Presets) || index >= partPresetWindow {
		return
	}
	preset := picker.Presets[index]
	if preset.Disabled {
		return
	}
	picker.commit(normalize(preset.Start, picker.granularity()), normalize(preset.End, picker.granularity()))
}

func (picker *DateRangePicker) enterLayer() {
	if picker.layer >= picker.maxLayer() {
		return
	}
	picker.layer++
	picker.rebuild()
	picker.focus = picker.initialFocus()
}

func (picker *DateRangePicker) jumpToToday() {
	picker.ensureView()
	picker.layer = 0
	picker.view = pageStart(picker.today, picker.granularity())
	picker.rebuild()
	picker.focus = picker.nearestSelectable(picker.cellOf(normalize(picker.today, picker.renderGranularity())))
}

func (picker *DateRangePicker) activate(index int) gfx.EventResult {
	switch {
	case index >= partCellBase && index < partCellBase+partCellWindow:
		cell := index - partCellBase
		if cell >= 2*picker.count || picker.renderGranularity() != picker.granularity() {
			if cell < 2*picker.count {
				picker.navigate(cell)
			}
		} else {
			picker.pickCell(cell)
		}
		return gfx.EventResult{Handled: true, Repaint: true}
	case index == partStartField:
		picker.toggle(startAnchor)
		return gfx.EventResult{Handled: true, Repaint: true}
	case index == partEndField:
		picker.toggle(false)
		return gfx.EventResult{Handled: true, Repaint: true}
	case index == partClear:
		picker.clearFromUI()
		return gfx.EventResult{Handled: true, Repaint: true}
	case index == partPrev:
		picker.shiftPage(-1)
		return gfx.EventResult{Handled: true, Repaint: true}
	case index == partNext:
		picker.shiftPage(1)
		return gfx.EventResult{Handled: true, Repaint: true}
	case index == partPrevYear:
		picker.shiftPage(-12)
		return gfx.EventResult{Handled: true, Repaint: true}
	case index == partNextYear:
		picker.shiftPage(12)
		return gfx.EventResult{Handled: true, Repaint: true}
	case index == partTitle:
		picker.enterLayer()
		return gfx.EventResult{Handled: true, Repaint: true}
	case index == partToday:
		picker.jumpToToday()
		return gfx.EventResult{Handled: true, Repaint: true}
	case index >= partPresetBase && index < partPresetBase+partPresetWindow:
		picker.applyPreset(index - partPresetBase)
		return gfx.EventResult{Handled: true, Repaint: true}
	}
	return gfx.EventResult{}
}

// startAnchor and endAnchor are the toggle targets for the two trigger fields.
const startAnchor = true
const endAnchor = false

func (picker *DateRangePicker) toggle(start bool) {
	if picker.Open && picker.anchorStart == start {
		picker.closePanel()
		return
	}
	if picker.Open {
		picker.anchorStart = start
		return
	}
	picker.openPanel(start)
}

func (picker *DateRangePicker) navigate(index int) {
	if index < 0 || index >= 2*picker.count {
		return
	}
	cell := picker.cells[index]
	if cell.disabled {
		return
	}
	switch picker.renderGranularity() {
	case GranularityMonth:
		picker.view = cell.value
		picker.layer = 0
	default:
		picker.view = cell.value
		if picker.granularity() == GranularityMonth {
			picker.layer = 0
		} else {
			picker.layer = 1
		}
	}
	picker.rebuild()
	picker.focus = picker.initialFocus()
}

func (picker *DateRangePicker) key(event gfx.KeyEvent) gfx.EventResult {
	if event.Release || picker.Disabled {
		return gfx.EventResult{}
	}
	index, isPart := event.Target.In(picker.handlers.Parts())
	if !picker.Open {
		if isPart {
			switch index {
			case partStartField, partEndField:
				if event.Key == gfx.KeyEnter || event.Key == gfx.KeySpace || event.Key == gfx.KeyDown {
					picker.toggle(index == partStartField)
					return gfx.EventResult{Handled: true, Repaint: true}
				}
			case partClear:
				if event.Key == gfx.KeyEnter || event.Key == gfx.KeySpace {
					picker.clearFromUI()
					return gfx.EventResult{Handled: true, Repaint: true}
				}
			}
			return gfx.EventResult{}
		}
		switch event.Key {
		case gfx.KeyEnter, gfx.KeySpace, gfx.KeyDown:
			picker.openPanel(picker.anchorStart)
			return gfx.EventResult{Handled: true, Repaint: true}
		}
		return gfx.EventResult{}
	}
	if !isPart {
		return gfx.EventResult{}
	}
	switch event.Key {
	case gfx.KeyEnter, gfx.KeySpace:
		return picker.activate(index)
	case gfx.KeyLeft, gfx.KeyRight, gfx.KeyUp, gfx.KeyDown, gfx.KeyHome, gfx.KeyEnd:
		return picker.moveCell(index, event.Key)
	case gfx.KeyPageUp:
		return picker.pageKey(index, -1, event.Modifiers)
	case gfx.KeyPageDown:
		return picker.pageKey(index, 1, event.Modifiers)
	}
	return gfx.EventResult{}
}

func (picker *DateRangePicker) pageKey(index int, direction int, modifiers gfx.Modifiers) gfx.EventResult {
	if index < partCellBase || index >= partCellBase+2*picker.count {
		return gfx.EventResult{}
	}
	pages := direction
	if modifiers&gfx.ModifierShift != 0 {
		switch picker.renderGranularity() {
		case GranularityDay:
			pages = direction * 12
		case GranularityMonth:
			pages = direction * 10
		}
	}
	picker.shiftPage(pages)
	return gfx.EventResult{
		Handled: true, Repaint: true,
		Focus: picker.handlers.Parts().At(partCellBase + picker.focus), FocusVisible: true,
	}
}

func (picker *DateRangePicker) moveCell(index int, key gfx.Key) gfx.EventResult {
	if index < partCellBase || index >= partCellBase+2*picker.count {
		return gfx.EventResult{}
	}
	current := index - partCellBase
	pane, local := current/picker.count, current%picker.count
	row, column := local/picker.columns, local%picker.columns
	switch key {
	case gfx.KeyLeft:
		column--
		if column < 0 {
			row, column = row-1, picker.columns-1
		}
	case gfx.KeyRight:
		column++
		if column >= picker.columns {
			row, column = row+1, 0
		}
	case gfx.KeyUp:
		row--
	case gfx.KeyDown:
		row++
	case gfx.KeyHome:
		column = 0
	case gfx.KeyEnd:
		column = picker.columns - 1
	}
	if row < 0 {
		if pane == 0 {
			row, column = 0, 0
		} else {
			pane, row, column = 0, picker.rows-1, picker.columns-1
		}
	}
	if row >= picker.rows {
		if pane == 1 {
			row, column = picker.rows-1, picker.columns-1
		} else {
			pane, row, column = 1, 0, 0
		}
	}
	next := pane*picker.count + row*picker.columns + column
	if picker.cells[next].disabled {
		next = picker.nearestSelectable(next)
	}
	picker.focus = next
	return gfx.EventResult{
		Handled: true, Repaint: true,
		Focus: picker.handlers.Parts().At(partCellBase + next), FocusVisible: true,
	}
}

func (picker *DateRangePicker) pointerDown(event gfx.PointerEvent) gfx.EventResult {
	if event.Kind != gfx.PointerDown || picker.Disabled {
		return gfx.EventResult{}
	}
	index, isPart := event.Target.In(picker.handlers.Parts())
	if !isPart {
		picker.toggle(picker.anchorStart)
		return gfx.EventResult{Handled: true, Repaint: true}
	}
	switch {
	case index == partDismiss:
		picker.dismiss()
		return gfx.EventResult{Handled: true, Repaint: true}
	case index >= partCellBase && index < partCellBase+2*picker.count:
		picker.focus = index - partCellBase
	}
	return picker.activate(index)
}
