package datepicker_test

import (
	"testing"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/datepicker"
)

type rangeHarness struct {
	t      *testing.T
	picker *datepicker.DateRangePicker
	box    ui.Box
	page   *ui.Page
	driver *app.Driver
	inv    *app.Invalidator
}

func openRange(t *testing.T, picker *datepicker.DateRangePicker) *rangeHarness {
	t.Helper()
	h := &rangeHarness{t: t, picker: picker}
	h.page = ui.NewPage(ui.PageConfig{View: func(c ui.C) {
		ui.Column(c).Pad(16).Gap(8).Children(func() {
			h.box = picker.View(c, "Period").Width(460)
			ui.Button(c, "after")
		})
	}})
	h.inv = new(app.Invalidator)
	driver, err := app.NewDriver(ui.Configure(h.page, app.Config{Width: 720, Height: 560, Invalidator: h.inv}), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	h.driver = driver
	h.repaint()
	return h
}

func (h *rangeHarness) repaint() {
	h.t.Helper()
	h.inv.Invalidate()
	if err := h.driver.Pump(); err != nil {
		h.t.Fatal(err)
	}
}

func (h *rangeHarness) hit(label string) gfx.Hit {
	h.t.Helper()
	hits := h.driver.Snapshot().Hits
	for index := len(hits) - 1; index >= 0; index-- {
		hit := hits[index]
		if hit.Label == label && !hit.Backdrop && !hit.ID.IsZero() && !hit.IsLabel {
			return hit
		}
	}
	h.t.Fatalf("no hit labeled %q", label)
	return gfx.Hit{}
}

func (h *rangeHarness) has(label string) bool {
	for _, hit := range h.driver.Snapshot().Hits {
		if hit.Label == label && !hit.Backdrop {
			return true
		}
	}
	return false
}

func (h *rangeHarness) click(label string) {
	h.t.Helper()
	hit := h.hit(label)
	if err := h.driver.Click(hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2); err != nil {
		h.t.Fatal(err)
	}
	h.repaint()
}

func (h *rangeHarness) clickTrigger() {
	h.t.Helper()
	r := h.box.Rect()
	if err := h.driver.Click(r.X+40, r.Y+r.Height/2); err != nil {
		h.t.Fatal(err)
	}
	h.repaint()
}

func (h *rangeHarness) key(key gfx.Key) {
	h.t.Helper()
	if err := h.driver.Key(key, 0); err != nil {
		h.t.Fatal(err)
	}
	h.repaint()
}

func (h *rangeHarness) focused() string {
	snapshot := h.driver.Snapshot()
	for _, hit := range snapshot.Hits {
		if hit.ID == snapshot.Focused && !hit.IsLabel && !hit.Backdrop {
			return hit.Label
		}
	}
	return ""
}

func day(year int, month time.Month, date int) time.Time {
	return time.Date(year, month, date, 0, 0, 0, 0, time.Local)
}

// A range commits once both ends are chosen, and only then.
func TestRangeCommitsBothEnds(t *testing.T) {
	picker := datepicker.NewRange(datepicker.GranularityDay)
	picker.Now = fixedNow
	calls := 0
	picker.OnChange = func(start, end time.Time, has bool) {
		calls++
		if !has {
			t.Fatal("a range commit reported no value")
		}
	}
	h := openRange(t, picker)
	h.clickTrigger()
	if !picker.Open {
		t.Fatal("the field did not open the panel")
	}
	if !h.has("2026-11-10") {
		t.Fatal("the range panel did not show today's month")
	}
	h.click("2026-11-10")
	if !picker.Open || calls != 0 {
		t.Fatalf("the first end closed the panel or committed: open %v calls %d", picker.Open, calls)
	}
	if picker.StartSet || picker.EndSet {
		t.Fatalf("a half-picked range reached the value: startSet %v endSet %v", picker.StartSet, picker.EndSet)
	}
	if pending, ok := picker.Pending(); !ok || !pending.Equal(day(2026, 11, 10)) {
		t.Fatalf("pending %v (set %v)", pending, ok)
	}
	h.click("2026-11-15")
	if picker.Open || calls != 1 {
		t.Fatalf("the second end left open %v and calls %d", picker.Open, calls)
	}
	if !picker.Start.Equal(day(2026, 11, 10)) || !picker.End.Equal(day(2026, 11, 15)) {
		t.Fatalf("range %v..%v", picker.Start, picker.End)
	}
}

// A programmatic range with its ends the wrong way round is swapped.
func TestRangeOrdersEnds(t *testing.T) {
	picker := datepicker.NewRange(datepicker.GranularityDay)
	picker.SetRange(day(2026, 11, 20), day(2026, 11, 10))
	if !picker.Start.Equal(day(2026, 11, 10)) || !picker.End.Equal(day(2026, 11, 20)) {
		t.Fatalf("swapped range %v..%v", picker.Start, picker.End)
	}
}

// A press before the pending start restarts the range rather than committing.
func TestRangeRestartsBeforeThePendingStart(t *testing.T) {
	picker := datepicker.NewRange(datepicker.GranularityDay)
	picker.Now = fixedNow
	calls := 0
	picker.OnChange = func(start, end time.Time, has bool) { calls++ }
	h := openRange(t, picker)
	h.clickTrigger()
	h.click("2026-11-15")
	h.click("2026-11-10")
	if !picker.Open || calls != 0 {
		t.Fatalf("an earlier press committed or closed: open %v calls %d", picker.Open, calls)
	}
	h.click("2026-11-12")
	if !picker.Start.Equal(day(2026, 11, 10)) || !picker.End.Equal(day(2026, 11, 12)) {
		t.Fatalf("range after restart %v..%v", picker.Start, picker.End)
	}
}

// Escape drops a half-picked range and keeps the value the range had.
func TestEscapeDropsThePendingEnd(t *testing.T) {
	picker := datepicker.NewRange(datepicker.GranularityDay)
	picker.Now = fixedNow
	picker.SetRange(day(2026, 11, 1), day(2026, 11, 5))
	h := openRange(t, picker)
	h.clickTrigger()
	h.click("2026-11-10")
	h.key(gfx.KeyEscape)
	if picker.Open {
		t.Fatal("Escape did not close the panel")
	}
	if !picker.Start.Equal(day(2026, 11, 1)) || !picker.End.Equal(day(2026, 11, 5)) {
		t.Fatalf("Escape changed the range to %v..%v", picker.Start, picker.End)
	}
}

func TestRangeClearAndPresets(t *testing.T) {
	picker := datepicker.NewRange(datepicker.GranularityDay)
	picker.Now = fixedNow
	picker.Presets = []datepicker.RangePreset{
		{Label: "This month", Start: day(2026, 11, 1), End: day(2026, 11, 30)},
		{Label: "All time", Disabled: true, Start: day(2026, 1, 1), End: day(2026, 12, 31)},
	}
	var last [2]time.Time
	picker.OnChange = func(start, end time.Time, has bool) { last = [2]time.Time{start, end} }
	h := openRange(t, picker)
	h.clickTrigger()
	h.click("This month")
	if picker.Open || !picker.Start.Equal(day(2026, 11, 1)) || !picker.End.Equal(day(2026, 11, 30)) {
		t.Fatalf("the preset left open %v and range %v..%v", picker.Open, picker.Start, picker.End)
	}
	if !last[0].Equal(day(2026, 11, 1)) {
		t.Fatalf("the preset reported %v", last)
	}
	h.click("Clear")
	if picker.StartSet || picker.EndSet || !picker.Start.IsZero() {
		t.Fatalf("clear left %v..%v", picker.Start, picker.End)
	}
}

// Month ranges show two years side by side.
func TestMonthRangeShowsTwoYears(t *testing.T) {
	picker := datepicker.NewRange(datepicker.GranularityMonth)
	picker.Now = fixedNow
	h := openRange(t, picker)
	h.clickTrigger()
	if !h.has("2026") || !h.has("2027") {
		t.Fatal("the month range did not show two years")
	}
	h.click("2026-03")
	h.click("2026-07")
	if !picker.Start.Equal(day(2026, 3, 1)) || !picker.End.Equal(day(2026, 7, 1)) {
		t.Fatalf("month range %v..%v", picker.Start, picker.End)
	}
}

// The range trigger's two fields each open the panel, and the clear button owns
// its own region.
func TestRangeTriggerFields(t *testing.T) {
	picker := datepicker.NewRange(datepicker.GranularityDay)
	picker.Now = fixedNow
	h := openRange(t, picker)
	h.click("End date")
	if !picker.Open {
		t.Fatal("pressing the end field did not open the panel")
	}
	r := h.hit("End date").Bounds
	if r.Width <= 0 {
		t.Fatal("the end field has no width")
	}
	h.key(gfx.KeyEscape)
	if picker.Open {
		t.Fatal("Escape did not close the panel")
	}
}

// Hovering after the first end previews the second without committing.
func TestRangeHoverPreviewsTheSecondEnd(t *testing.T) {
	picker := datepicker.NewRange(datepicker.GranularityDay)
	picker.Now = fixedNow
	h := openRange(t, picker)
	h.clickTrigger()
	h.click("2026-11-10")
	target := h.hit("2026-11-15").Bounds
	if err := h.driver.Move(target.X+target.Width/2, target.Y+target.Height/2); err != nil {
		t.Fatal(err)
	}
	h.repaint()
	if !h.hit("2026-11-15").Selected {
		t.Fatal("the hovered end is not marked as the preview end")
	}
	if picker.EndSet {
		t.Fatal("hovering committed the range")
	}
}

// The keyboard picks both ends with Enter and moves with the arrows.
func TestRangeKeyboard(t *testing.T) {
	picker := datepicker.NewRange(datepicker.GranularityDay)
	picker.Now = fixedNow
	h := openRange(t, picker)
	h.clickTrigger()
	if focus := h.focused(); focus != "2026-11-22" {
		t.Fatalf("opening focused %q", focus)
	}
	h.key(gfx.KeyRight)
	h.key(gfx.KeyEnter)
	if pending, ok := picker.Pending(); !ok || !pending.Equal(day(2026, 11, 23)) {
		t.Fatalf("pending %v (set %v)", pending, ok)
	}
	h.key(gfx.KeyRight)
	h.key(gfx.KeyEnter)
	if !picker.Start.Equal(day(2026, 11, 23)) || !picker.End.Equal(day(2026, 11, 24)) {
		t.Fatalf("keyboard range %v..%v", picker.Start, picker.End)
	}
}

// A warm repaint of an open range panel allocates nothing.
func TestRangeRepaintAllocates(t *testing.T) {
	picker := datepicker.NewRange(datepicker.GranularityDay)
	picker.Now = fixedNow
	h := openRange(t, picker)
	h.clickTrigger()
	if !picker.Open {
		t.Fatal("the panel did not open")
	}
	tmp := h.hit("2026-11-15").Bounds
	if err := h.driver.Move(tmp.X+tmp.Width/2, tmp.Y+tmp.Height/2); err != nil {
		t.Fatal(err)
	}
	h.repaint()
	if allocs := testing.AllocsPerRun(100, h.repaint); allocs != 0 {
		t.Fatalf("%v allocations a repaint", allocs)
	}
}

// Drilling a range's title navigates without committing, and Escape steps back.
func TestRangeDrill(t *testing.T) {
	picker := datepicker.NewRange(datepicker.GranularityDay)
	picker.Now = fixedNow
	h := openRange(t, picker)
	h.clickTrigger()
	h.click("November 2026")
	if !h.has("2026-03") {
		t.Fatal("the range title did not open the month panel")
	}
	if picker.StartSet || picker.EndSet {
		t.Fatal("drilling committed a range")
	}
	h.key(gfx.KeyEscape)
	if !picker.Open || !h.has("November 2026") {
		t.Fatal("Escape did not step back to the day panel")
	}
}

// Range presets sit below both panes, not beside them.
func TestRangePresetsAreAtTheBottom(t *testing.T) {
	picker := datepicker.NewRange(datepicker.GranularityDay)
	picker.Now = fixedNow
	picker.Presets = []datepicker.RangePreset{
		{Label: "This month", Start: day(2026, 11, 1), End: day(2026, 11, 30)},
	}
	h := openRange(t, picker)
	h.clickTrigger()
	chip := h.hit("This month")
	cell := h.hit("2026-11-22")
	if chip.Bounds.Y <= cell.Bounds.Y {
		t.Fatalf("the preset chip at y %v is not below the cell at y %v", chip.Bounds.Y, cell.Bounds.Y)
	}
	if chip.Bounds.X <= 0 {
		t.Fatal("the preset chip is missing")
	}
}

// Range preset chips wrap too, and a wrapped chip still commits.
func TestRangePresetsWrap(t *testing.T) {
	picker := datepicker.NewRange(datepicker.GranularityDay)
	picker.Now = fixedNow
	presets := []string{"Last seven days", "Last thirty days", "Quarter to date", "Year to date", "Month to date", "Previous quarter", "Previous year", "Next thirty days"}
	picker.Presets = make([]datepicker.RangePreset, 0, len(presets))
	for _, label := range presets {
		picker.Presets = append(picker.Presets, datepicker.RangePreset{Label: label, Start: day(2026, 1, 1), End: day(2026, 11, 22)})
	}
	h := openRange(t, picker)
	h.clickTrigger()
	rows := map[float32]bool{}
	for _, label := range presets {
		rows[h.hit(label).Bounds.Y] = true
	}
	if len(rows) < 2 {
		t.Fatalf("the chips share %d row(s), want them to wrap", len(rows))
	}
	h.click("Year to date")
	if !picker.Start.Equal(day(2026, 1, 1)) || !picker.End.Equal(day(2026, 11, 22)) {
		t.Fatalf("a wrapped chip chose %v..%v", picker.Start, picker.End)
	}
}

// A month or year range turns its page with a single arrow a side.
func TestRangePageArrowsAreSingleAtMonthAndYear(t *testing.T) {
	month := datepicker.NewRange(datepicker.GranularityMonth)
	month.Now = fixedNow
	h := openRange(t, month)
	h.clickTrigger()
	if !h.has("Previous year") || !h.has("Next year") {
		t.Fatal("the month range has no single year arrows")
	}
	h.click("Next year")
	if !h.has("2027") || !h.has("2028") {
		t.Fatalf("Next year left the month range on %v", h.has("2027"))
	}

	year := datepicker.NewRange(datepicker.GranularityYear)
	year.Now = fixedNow
	h = openRange(t, year)
	h.clickTrigger()
	if !h.has("Previous decade") || !h.has("Next decade") {
		t.Fatal("the year range has no single decade arrows")
	}
	h.click("Next decade")
	if !h.has("2030-2041") || !h.has("2040-2051") {
		t.Fatal("Next decade did not turn both decades")
	}
}
