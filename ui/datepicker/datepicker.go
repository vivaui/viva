package datepicker

import (
	"strconv"
	"time"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// Granularity is the one unit a picker edits. It is fixed when the picker is
// made: drilling into the month or year panel navigates, it does not change it.
type Granularity uint8

const (
	// GranularityDay picks one civil day.
	GranularityDay Granularity = iota
	// GranularityMonth picks one month; its value is the first day at 00:00 local.
	GranularityMonth
	// GranularityYear picks one year; its value is January 1st at 00:00 local.
	GranularityYear
)

// Preset is a shortcut value the caller owns; the picker draws its Label and
// writes Value, and has no business meaning of its own.
type Preset struct {
	Label string
	Value time.Time
}

// Options is the configuration of a picker. Fields are values: changing one and
// painting again shows the change.
type Options struct {
	Granularity Granularity
	// WeekStart is the first column of the day grid. New sets Monday.
	WeekStart time.Weekday
	Labels    Labels
	// Format is the Go layout the trigger shows. Empty uses Labels.Layouts.
	Format string

	// Min and Max bound every selectable value, inclusively. A zero value is
	// unbounded.
	Min, Max time.Time
	// DisabledDate, when set, disables a normalized value.
	DisabledDate func(time.Time) bool

	// Now is read when the panel opens and when Today is pressed; nil reads the
	// system clock. Paint never reads it.
	Now func() time.Time

	// AllowClear shows the clear button while a value is set. New sets true.
	AllowClear bool
	// ShowToday shows the Today link in the day panel. New sets true.
	ShowToday bool
	Small     bool
	Invalid   bool
	Disabled  bool
}

// DatePicker is one date field with its popup: the application owns the
// exported fields and reads the callbacks; the picker draws them and turns
// input into changes.
type DatePicker struct {
	Options
	Presets []Preset

	Value    time.Time
	HasValue bool
	// Open is the panel's state. Opening and closing an open panel is the
	// application's decision too: set it and paint.
	Open bool

	OnChange     func(value time.Time, hasValue bool)
	OnOpenChange func(open bool)

	handlers ui.Handlers
	drawFn   func(*ui.DrawContext)
	pointer  func(gfx.PointerEvent) gfx.EventResult
	keyFn    func(gfx.KeyEvent) gfx.EventResult

	view    time.Time
	today   time.Time
	layer   int
	focus   int
	cells   [42]dayCell
	count   int
	columns int
	rows    int
	title   string
	labels  Labels
	text    string
	name    string

	cacheKey   cacheKey
	cacheValid bool
	nameKey    nameKey
	nameValid  bool
	revision   uint64
}

type dayCell struct {
	value    time.Time
	iso      string
	label    string
	a11y     string
	outside  bool
	disabled bool
	today    bool
	selected bool
}

type cacheKey struct {
	granularity Granularity
	weekStart   time.Weekday
	view        time.Time
	value       time.Time
	today       time.Time
	min, max    time.Time
	hasValue    bool
	open        bool
	layer       int
	small       bool
	format      string
	labels      Labels
	revision    uint64
}

type nameKey struct {
	label       string
	value       time.Time
	has         bool
	small       bool
	format      string
	labels      Labels
	granularity Granularity
}

// New returns a day picker unless another granularity is asked for.
func New(granularity Granularity) *DatePicker {
	picker := &DatePicker{}
	picker.Reset()
	picker.Granularity = granularity
	return picker
}

// Reset returns every option and the value to their defaults, keeping the
// handler wiring: Monday weeks, a clear button, and Today.
func (picker *DatePicker) Reset() *DatePicker {
	picker.Options = Options{
		WeekStart:  time.Monday,
		AllowClear: true,
		ShowToday:  true,
	}
	picker.Presets = nil
	picker.Value = time.Time{}
	picker.HasValue = false
	picker.Open = false
	picker.view = time.Time{}
	picker.today = time.Time{}
	picker.layer = 0
	picker.focus = 0
	picker.revision++
	picker.cacheValid = false
	picker.nameValid = false
	picker.prepare()
	return picker
}

// Refresh makes the next paint rebuild its derived labels and cells. Call it
// after changing Labels, presets, the disabled predicate or another field the
// picker cannot compare cheaply.
func (picker *DatePicker) Refresh() *DatePicker {
	picker.revision++
	return picker
}

// SetValue normalizes t to the picker's granularity and sets it. A zero t
// clears the value. It does not call OnChange: that is for the user's own
// choices.
func (picker *DatePicker) SetValue(t time.Time) *DatePicker {
	if t.IsZero() {
		return picker.Clear()
	}
	picker.Value = normalize(t, picker.Granularity)
	picker.HasValue = true
	picker.view = pageStart(picker.Value, picker.Granularity)
	picker.revision++
	return picker
}

// Clear drops the value without calling OnChange.
func (picker *DatePicker) Clear() *DatePicker {
	picker.Value = time.Time{}
	picker.HasValue = false
	picker.revision++
	return picker
}

// Values exposes the current value for callers that bind a row of pickers.
func (picker *DatePicker) Values() (time.Time, bool) { return picker.Value, picker.HasValue }

// View declares the field. The caller owns its width; the height follows the
// size step.
func (picker *DatePicker) View(c ui.C, label string) ui.Box {
	if picker.drawFn == nil {
		picker.prepare()
	}
	theme := c.Theme()
	width := float32(240)
	if picker.Small {
		width = 220
	}
	picker.name = picker.accessibleName(label)
	return ui.Control(c, picker.name, &picker.handlers).Size(width, picker.triggerHeight(theme.Metrics.ControlHeight)).Draw(picker.drawFn)
}

func (picker *DatePicker) prepare() {
	picker.handlers.Role = gfx.RoleCombobox
	picker.keyFn = picker.key
	picker.pointer = picker.pointerDown
	picker.handlers.Key = picker.keyFn
	picker.handlers.Pointer = picker.pointer
	picker.drawFn = picker.draw
}

func (picker *DatePicker) triggerHeight(controlHeight float32) float32 {
	if picker.Small {
		return max(controlHeight-4, 16)
	}
	return controlHeight
}

func (picker *DatePicker) granularity() Granularity {
	switch picker.Granularity {
	case GranularityMonth, GranularityYear:
		return picker.Granularity
	}
	return GranularityDay
}

// renderGranularity is the panel that is showing: the picker's own, or the
// month and year panels a title drill opened. Selecting in a drilled panel
// navigates and does not choose.
func (picker *DatePicker) renderGranularity() Granularity {
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

// maxLayer is how deep the title drills: day to months to years, month to
// years, year nowhere.
func (picker *DatePicker) maxLayer() int {
	switch picker.granularity() {
	case GranularityMonth:
		return 1
	case GranularityYear:
		return 0
	}
	return 2
}

// renderPage is the first value of the page the panel shows: a month for the
// day grid, a year for the month grid and a decade for the year grid.
func (picker *DatePicker) renderPage() time.Time {
	return pageStart(picker.view, picker.renderGranularity())
}

func (picker *DatePicker) now() time.Time {
	if picker.Now != nil {
		return picker.Now()
	}
	return time.Now()
}

// accessibleName is what the field is called by assistive technology: its label,
// and the value when one is set.
func (picker *DatePicker) accessibleName(label string) string {
	key := nameKey{
		label:       label,
		value:       picker.Value,
		has:         picker.HasValue,
		small:       picker.Small,
		format:      picker.Format,
		labels:      picker.Labels,
		granularity: picker.granularity(),
	}
	if picker.nameValid && picker.nameKey == key {
		return picker.name
	}
	picker.nameKey = key
	picker.nameValid = true
	labels := resolveLabels(picker.Labels)
	layout := picker.Format
	if layout == "" {
		layout = labels.Layouts[int(picker.granularity())]
	}
	picker.text = labels.Placeholder
	if picker.HasValue && !picker.Value.IsZero() {
		picker.text = picker.Value.Format(layout)
		picker.name = label + ", " + picker.text
	} else {
		picker.name = label
	}
	return picker.name
}

// ensureView places the panel on the page that holds the value, or today.
func (picker *DatePicker) ensureView() {
	granularity := picker.granularity()
	if picker.today.IsZero() {
		picker.today = normalize(picker.now(), GranularityDay)
	}
	if picker.HasValue && !picker.Value.IsZero() {
		picker.view = pageStart(picker.Value, granularity)
		return
	}
	if picker.view.IsZero() {
		picker.view = pageStart(picker.today, granularity)
	}
}

func (picker *DatePicker) openPanel() {
	if picker.Disabled || picker.Open {
		return
	}
	picker.Open = true
	picker.layer = 0
	picker.ensureView()
	picker.rebuild()
	picker.focus = picker.initialFocus()
	if picker.OnOpenChange != nil {
		picker.OnOpenChange(true)
	}
}

// closePanel closes the panel and forgets a drilled panel.
func (picker *DatePicker) closePanel() {
	if !picker.Open {
		return
	}
	picker.Open = false
	picker.layer = 0
	if picker.OnOpenChange != nil {
		picker.OnOpenChange(false)
	}
}

// dismiss is Escape or a press outside: a drilled panel steps back to the one
// that opened it before the panel itself closes.
func (picker *DatePicker) dismiss() {
	if picker.layer > 0 {
		picker.layer--
		picker.rebuild()
		picker.focus = picker.nearestSelectable(picker.cellOf(picker.initialTarget()))
		return
	}
	picker.closePanel()
}

// initialTarget is the value when there is one, today otherwise, normalized to
// the panel that is showing.
func (picker *DatePicker) initialTarget() time.Time {
	if picker.HasValue && !picker.Value.IsZero() {
		return normalize(picker.Value, picker.renderGranularity())
	}
	return normalize(picker.today, picker.renderGranularity())
}

// initialFocus is the target's cell, else the first selectable one.
func (picker *DatePicker) initialFocus() int {
	if index := picker.cellOf(picker.initialTarget()); index >= 0 && !picker.cells[index].disabled {
		return index
	}
	for index := 0; index < picker.count; index++ {
		if !picker.cells[index].disabled {
			return index
		}
	}
	return 0
}

func (picker *DatePicker) cellOf(value time.Time) int {
	for index := 0; index < picker.count; index++ {
		if picker.cells[index].value.Equal(value) {
			return index
		}
	}
	return -1
}

// enterLayer opens the parent panel of what is showing.
func (picker *DatePicker) enterLayer() {
	if picker.layer >= picker.maxLayer() {
		return
	}
	picker.layer++
	picker.rebuild()
	picker.focus = picker.initialFocus()
}

// rebuild derives the page's cells, title and labels. It runs when a value
// changes, not on every paint, and never reads the clock beyond the today it
// was given when the panel opened.
func (picker *DatePicker) rebuild() {
	render := picker.renderGranularity()
	page := picker.renderPage()
	labels := resolveLabels(picker.Labels)
	picker.labels = labels
	picker.columns, picker.rows = gridShape(render)
	picker.count = 12
	if render == GranularityDay {
		picker.count = 7 * 6
	}
	today := picker.today
	if today.IsZero() {
		today = normalize(picker.now(), GranularityDay)
		picker.today = today
	}
	picker.title = pageTitle(page, render, labels)
	min, max := normalize(picker.Min, render), normalize(picker.Max, render)
	selected := picker.HasValue && !picker.Value.IsZero()
	target := normalize(picker.Value, render)
	for index := 0; index < picker.count; index++ {
		value := cellValue(page, render, index, picker.WeekStart)
		cell := dayCell{
			value:    value,
			iso:      cellISO(value, render),
			label:    cellLabel(value, render, labels),
			a11y:     value.Format(labels.A11yLayouts[int(render)]),
			outside:  render == GranularityDay && !withinPage(value, page, render),
			today:    value.Equal(today),
			selected: selected && value.Equal(target),
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
		picker.cells[index] = cell
	}
}

func gridShape(granularity Granularity) (columns, rows int) {
	if granularity == GranularityDay {
		return 7, 6
	}
	return 3, 4
}

// cellValue is the value of cell index on the page: days run from the week that
// starts the month, months fill the year, and years fill the decade.
func cellValue(page time.Time, granularity Granularity, index int, weekStart time.Weekday) time.Time {
	switch granularity {
	case GranularityMonth:
		return time.Date(page.Year(), time.Month(index+1), 1, 0, 0, 0, 0, time.Local)
	case GranularityYear:
		return time.Date(page.Year()+index, time.January, 1, 0, 0, 0, 0, time.Local)
	}
	first := time.Date(page.Year(), page.Month(), 1, 0, 0, 0, 0, time.Local)
	nth := (int(first.Weekday()) - int(weekStart) + 7) % 7
	return first.AddDate(0, 0, index-nth)
}

func cellLabel(value time.Time, granularity Granularity, labels Labels) string {
	switch granularity {
	case GranularityMonth:
		return labels.Months[int(value.Month())-1]
	case GranularityYear:
		return strconv.Itoa(value.Year())
	}
	return strconv.Itoa(value.Day())
}

// cellISO is the cell's value in the machine-readable form the accessibility
// tree carries.
func cellISO(value time.Time, granularity Granularity) string {
	switch granularity {
	case GranularityMonth:
		return value.Format(monthLayout())
	case GranularityYear:
		return value.Format(yearLayout())
	}
	return value.Format(dayLayout())
}

func pageTitle(page time.Time, granularity Granularity, labels Labels) string {
	switch granularity {
	case GranularityMonth:
		return page.Format(labels.TitleLayouts[1])
	case GranularityYear:
		return strconv.Itoa(page.Year()) + "-" + strconv.Itoa(page.Year()+11)
	}
	return page.Format(labels.TitleLayouts[0])
}

func (picker *DatePicker) choose(index int) {
	if index < 0 || index >= picker.count {
		return
	}
	cell := picker.cells[index]
	if cell.disabled {
		return
	}
	picker.Value = cell.value
	picker.HasValue = true
	picker.view = pageStart(cell.value, picker.granularity())
	picker.revision++
	picker.closePanel()
	if picker.OnChange != nil {
		picker.OnChange(picker.Value, true)
	}
}

// navigate chooses a month or a year in a drilled panel: it moves the view and
// steps back one layer, and never writes the value.
func (picker *DatePicker) navigate(index int) {
	if index < 0 || index >= picker.count {
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
		if picker.granularity() == GranularityMonth {
			picker.view = cell.value
			picker.layer = 0
		} else {
			picker.view = cell.value
			picker.layer = 1
		}
	}
	picker.rebuild()
	picker.focus = picker.initialFocus()
}

func (picker *DatePicker) clearFromUI() {
	picker.Value = time.Time{}
	picker.HasValue = false
	picker.revision++
	if picker.OnChange != nil {
		picker.OnChange(time.Time{}, false)
	}
}

// shiftPage moves the visible page: days by months, months by years and years
// by decades, at whatever panel is showing.
func (picker *DatePicker) shiftPage(pages int) {
	if picker.view.IsZero() {
		picker.ensureView()
	}
	picker.view = shiftPage(picker.view, picker.renderGranularity(), pages)
	picker.rebuild()
	if picker.focus >= picker.count || picker.cells[picker.focus].disabled {
		picker.focus = picker.nearestSelectable(picker.focus)
	}
}

func (picker *DatePicker) nearestSelectable(index int) int {
	if index < 0 || index >= picker.count {
		index = 0
	}
	for step := 0; step < picker.count; step++ {
		next := (index + step) % picker.count
		if !picker.cells[next].disabled {
			return next
		}
	}
	return index
}

func (picker *DatePicker) activate(index int) gfx.EventResult {
	switch {
	case index >= partCellBase && index < partCellBase+partCellWindow:
		cell := index - partCellBase
		if picker.renderGranularity() != picker.granularity() {
			picker.navigate(cell)
		} else {
			picker.choose(cell)
		}
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

// jumpToToday chooses today when it is selectable, as antd's footer link does,
// and otherwise only shows today's page and puts the keyboard there.
func (picker *DatePicker) jumpToToday() {
	picker.ensureView()
	picker.layer = 0
	today := normalize(picker.today, picker.granularity())
	picker.view = pageStart(today, picker.granularity())
	picker.rebuild()
	index := picker.cellOf(today)
	if index >= 0 && !picker.cells[index].disabled {
		picker.Value, picker.HasValue = today, true
		picker.revision++
		picker.closePanel()
		if picker.OnChange != nil {
			picker.OnChange(picker.Value, true)
		}
		return
	}
	picker.focus = picker.nearestSelectable(index)
}

func (picker *DatePicker) key(event gfx.KeyEvent) gfx.EventResult {
	if event.Release || picker.Disabled {
		return gfx.EventResult{}
	}
	index, isPart := event.Target.In(picker.handlers.Parts())
	if !picker.Open {
		if isPart && index == partClear {
			if event.Key == gfx.KeyEnter || event.Key == gfx.KeySpace {
				picker.clearFromUI()
				return gfx.EventResult{Handled: true, Repaint: true}
			}
			return gfx.EventResult{}
		}
		switch event.Key {
		case gfx.KeyEnter, gfx.KeySpace, gfx.KeyDown:
			picker.openPanel()
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
	case gfx.KeyLeft, gfx.KeyRight, gfx.KeyUp, gfx.KeyDown:
		return picker.moveCell(index, event.Key)
	case gfx.KeyHome, gfx.KeyEnd:
		return picker.moveCell(index, event.Key)
	case gfx.KeyPageUp:
		return picker.pageKey(index, -1, event.Modifiers)
	case gfx.KeyPageDown:
		return picker.pageKey(index, 1, event.Modifiers)
	}
	return gfx.EventResult{}
}

// pageKey turns a page: PageUp and PageDown move one page, and with Shift the
// larger step of the panel that is showing.
func (picker *DatePicker) pageKey(index int, direction int, modifiers gfx.Modifiers) gfx.EventResult {
	if index < partCellBase || index >= partCellBase+partCellWindow {
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

func (picker *DatePicker) moveCell(index int, key gfx.Key) gfx.EventResult {
	if index < partCellBase || index >= partCellBase+partCellWindow {
		return gfx.EventResult{}
	}
	current := index - partCellBase
	next := current
	switch key {
	case gfx.KeyLeft:
		next = picker.stepInRow(current, -1)
	case gfx.KeyRight:
		next = picker.stepInRow(current, 1)
	case gfx.KeyUp:
		next = picker.stepInColumn(current, -1)
	case gfx.KeyDown:
		next = picker.stepInColumn(current, 1)
	case gfx.KeyHome:
		next = picker.rowEdge(current, false)
	case gfx.KeyEnd:
		next = picker.rowEdge(current, true)
	}
	picker.focus = next
	return gfx.EventResult{
		Handled: true, Repaint: true,
		Focus: picker.handlers.Parts().At(partCellBase + next), FocusVisible: true,
	}
}

func (picker *DatePicker) stepInRow(index, direction int) int {
	row, column := index/picker.columns, index%picker.columns
	for step := 0; step < picker.columns-1; step++ {
		column += direction
		if column < 0 || column >= picker.columns {
			return index
		}
		next := row*picker.columns + column
		if !picker.cells[next].disabled {
			return next
		}
	}
	return index
}

func (picker *DatePicker) stepInColumn(index, direction int) int {
	row, column := index/picker.columns, index%picker.columns
	for step := 0; step < picker.rows-1; step++ {
		row += direction
		if row < 0 || row >= picker.rows {
			return index
		}
		next := row*picker.columns + column
		if !picker.cells[next].disabled {
			return next
		}
	}
	return index
}

func (picker *DatePicker) rowEdge(index int, end bool) int {
	row := index / picker.columns
	start, stop, step := 0, picker.columns, 1
	if end {
		start, stop, step = picker.columns-1, -1, -1
	}
	for column := start; column != stop; column += step {
		next := row*picker.columns + column
		if !picker.cells[next].disabled {
			return next
		}
	}
	return index
}

func (picker *DatePicker) pointerDown(event gfx.PointerEvent) gfx.EventResult {
	if event.Kind != gfx.PointerDown || picker.Disabled {
		return gfx.EventResult{}
	}
	index, isPart := event.Target.In(picker.handlers.Parts())
	if !isPart {
		if picker.Open {
			picker.dismiss()
		} else {
			picker.openPanel()
		}
		return gfx.EventResult{Handled: true, Repaint: true}
	}
	switch {
	case index == partDismiss:
		picker.dismiss()
		return gfx.EventResult{Handled: true, Repaint: true}
	case index >= partCellBase && index < partCellBase+partCellWindow:
		picker.focus = index - partCellBase
	}
	return picker.activate(index)
}
