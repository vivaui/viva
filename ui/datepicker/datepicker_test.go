package datepicker_test

import (
	"testing"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/datepicker"
)

func fixedNow() time.Time {
	return time.Date(2026, time.November, 22, 15, 30, 0, 0, time.Local)
}

type harness struct {
	t      *testing.T
	picker *datepicker.DatePicker
	box    ui.Box
	page   *ui.Page
	driver *app.Driver
	inv    *app.Invalidator
}

func open(t *testing.T, picker *datepicker.DatePicker) *harness {
	t.Helper()
	h := &harness{t: t, picker: picker}
	h.page = ui.NewPage(ui.PageConfig{View: func(c ui.C) {
		ui.Column(c).Pad(16).Gap(8).Children(func() {
			h.box = picker.View(c, "Start date").Width(240)
			ui.Button(c, "after")
		})
	}})
	h.inv = new(app.Invalidator)
	driver, err := app.NewDriver(ui.Configure(h.page, app.Config{Width: 480, Height: 480, Invalidator: h.inv}), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	h.driver = driver
	h.repaint()
	return h
}

func (h *harness) repaint() {
	h.t.Helper()
	h.inv.Invalidate()
	if err := h.driver.Pump(); err != nil {
		h.t.Fatal(err)
	}
}

func (h *harness) hit(label string) gfx.Hit {
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

func (h *harness) has(label string) bool {
	for _, hit := range h.driver.Snapshot().Hits {
		if hit.Label == label && !hit.Backdrop {
			return true
		}
	}
	return false
}

func (h *harness) click(label string) {
	h.t.Helper()
	hit := h.hit(label)
	if err := h.driver.Click(hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2); err != nil {
		h.t.Fatal(err)
	}
	h.repaint()
}

func (h *harness) clickTrigger() {
	h.t.Helper()
	r := h.box.Rect()
	if err := h.driver.Click(r.X+r.Width/2, r.Y+r.Height/2); err != nil {
		h.t.Fatal(err)
	}
	h.repaint()
}

func (h *harness) key(key gfx.Key) {
	h.t.Helper()
	if err := h.driver.Key(key, 0); err != nil {
		h.t.Fatal(err)
	}
	h.repaint()
}

func (h *harness) focused() string {
	snapshot := h.driver.Snapshot()
	for _, hit := range snapshot.Hits {
		if hit.ID == snapshot.Focused && !hit.IsLabel && !hit.Backdrop {
			return hit.Label
		}
	}
	return ""
}

// The field opens on today's page, chooses a day, closes and returns the keys
// to the trigger.
func TestDayPickerChoosesADay(t *testing.T) {
	picker := datepicker.New(datepicker.GranularityDay)
	picker.Now = fixedNow
	var changes []time.Time
	picker.OnChange = func(value time.Time, has bool) {
		if !has {
			t.Fatal("OnChange reported no value")
		}
		changes = append(changes, value)
	}
	h := open(t, picker)
	h.clickTrigger()
	if !picker.Open {
		t.Fatal("the trigger did not open the panel")
	}
	if !h.has("2026-11-22") {
		t.Fatal("the panel did not show today's month")
	}
	if focus := h.focused(); focus != "2026-11-22" {
		t.Fatalf("opening focused %q, want today", focus)
	}
	h.click("2026-11-15")
	if picker.Open {
		t.Fatal("choosing a day left the panel open")
	}
	want := time.Date(2026, time.November, 15, 0, 0, 0, 0, time.Local)
	if !picker.HasValue || !picker.Value.Equal(want) {
		t.Fatalf("value %v (set %v), want %v", picker.Value, picker.HasValue, want)
	}
	if len(changes) != 1 || !changes[0].Equal(want) {
		t.Fatalf("changes %v, want one %v", changes, want)
	}
	if focus := h.focused(); focus != "Start date, 2026-11-15" {
		t.Fatalf("closing left the focus on %q, want the trigger with its value", focus)
	}
}

// Normalization is the wall clock's fields at 00:00 local, whatever the
// granularity and the input's location.
func TestValuesNormalizeToCivilDates(t *testing.T) {
	location := time.FixedZone("UTC+5", 5*60*60)
	input := time.Date(2026, time.November, 22, 23, 40, 0, 0, location)
	cases := []struct {
		granularity datepicker.Granularity
		want        time.Time
	}{
		{datepicker.GranularityDay, time.Date(2026, 11, 22, 0, 0, 0, 0, time.Local)},
		{datepicker.GranularityMonth, time.Date(2026, 11, 1, 0, 0, 0, 0, time.Local)},
		{datepicker.GranularityYear, time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local)},
	}
	for _, test := range cases {
		picker := datepicker.New(test.granularity)
		picker.SetValue(input)
		if !picker.Value.Equal(test.want) || picker.Value.Location() != time.Local {
			t.Fatalf("granularity %d: value %v, want %v local", test.granularity, picker.Value, test.want)
		}
	}
}

func TestMonthAndYearPickersChooseAtTheirGranularity(t *testing.T) {
	monthPicker := datepicker.New(datepicker.GranularityMonth)
	monthPicker.Now = fixedNow
	h := open(t, monthPicker)
	h.clickTrigger()
	if !h.has("2026") {
		t.Fatal("the month panel did not show the year")
	}
	h.click("2026-03")
	if monthPicker.Open || !monthPicker.Value.Equal(time.Date(2026, 3, 1, 0, 0, 0, 0, time.Local)) {
		t.Fatalf("month value %v open %v", monthPicker.Value, monthPicker.Open)
	}

	yearPicker := datepicker.New(datepicker.GranularityYear)
	yearPicker.Now = fixedNow
	h = open(t, yearPicker)
	h.clickTrigger()
	if !h.has("2020-2031") {
		t.Fatal("the year panel did not show the decade")
	}
	h.click("2024")
	if yearPicker.Open || !yearPicker.Value.Equal(time.Date(2024, 1, 1, 0, 0, 0, 0, time.Local)) {
		t.Fatalf("year value %v open %v", yearPicker.Value, yearPicker.Open)
	}
}

func TestClearButtonDropsTheValue(t *testing.T) {
	picker := datepicker.New(datepicker.GranularityDay)
	picker.Now = fixedNow
	var cleared bool
	picker.OnChange = func(_ time.Time, has bool) {
		if has {
			t.Fatal("a clear reported a value")
		}
		cleared = true
	}
	picker.SetValue(time.Date(2026, 11, 3, 0, 0, 0, 0, time.Local))
	h := open(t, picker)
	h.click("Clear")
	if picker.HasValue || !picker.Value.IsZero() {
		t.Fatalf("clear left %v (has %v)", picker.Value, picker.HasValue)
	}
	if !cleared {
		t.Fatal("clear did not call OnChange")
	}
}

// Escape closes and hands the keyboard back; page keys do not leak through.
func TestEscapeClosesAndKeysTrap(t *testing.T) {
	picker := datepicker.New(datepicker.GranularityDay)
	picker.Now = fixedNow
	h := open(t, picker)
	h.clickTrigger()
	if !h.has("2026-11-15") {
		t.Fatal("the panel did not open")
	}
	h.key(gfx.KeyTab)
	for range 6 {
		h.key(gfx.KeyTab)
		if focus := h.focused(); focus == "after" {
			t.Fatal("Tab left the trapped panel")
		}
	}
	h.key(gfx.KeyEscape)
	if picker.Open {
		t.Fatal("Escape did not close the panel")
	}
	if focus := h.focused(); focus != "Start date" {
		t.Fatalf("Escape left the focus on %q", focus)
	}
}

// Arrow keys move by cell, PageUp and PageDown by page, Enter chooses.
func TestKeyboardChoosesADay(t *testing.T) {
	picker := datepicker.New(datepicker.GranularityDay)
	picker.Now = fixedNow
	h := open(t, picker)
	h.clickTrigger()
	h.key(gfx.KeyLeft)
	if focus := h.focused(); focus != "2026-11-21" {
		t.Fatalf("Left reached %q", focus)
	}
	h.key(gfx.KeyDown)
	if focus := h.focused(); focus != "2026-11-28" {
		t.Fatalf("Down reached %q", focus)
	}
	h.key(gfx.KeyUp)
	h.key(gfx.KeyUp)
	if focus := h.focused(); focus != "2026-11-14" {
		t.Fatalf("Up twice reached %q", focus)
	}
	h.key(gfx.KeyPageDown)
	if !h.has("December 2026") {
		t.Fatal("PageDown did not show the next month")
	}
	h.key(gfx.KeyEnter)
	if picker.Open {
		t.Fatal("Enter did not choose")
	}
	if picker.Value.Month() != time.December {
		t.Fatalf("Enter chose %v", picker.Value)
	}
}

// A warm repaint of the field and an open panel allocates nothing.
func TestDatePickerRepaintAllocates(t *testing.T) {
	picker := datepicker.New(datepicker.GranularityDay)
	picker.Now = fixedNow
	h := open(t, picker)
	h.clickTrigger()
	if !picker.Open {
		t.Fatal("the panel did not open")
	}
	if allocs := testing.AllocsPerRun(100, h.repaint); allocs != 0 {
		t.Fatalf("%v allocations a repaint", allocs)
	}
}

// Today's footer link chooses today when it is selectable, like antd's.
func TestTodayLinkChoosesToday(t *testing.T) {
	picker := datepicker.New(datepicker.GranularityDay)
	picker.Now = fixedNow
	picker.SetValue(time.Date(2020, 1, 5, 0, 0, 0, 0, time.Local))
	var chosen time.Time
	picker.OnChange = func(value time.Time, has bool) { chosen = value }
	h := open(t, picker)
	h.clickTrigger()
	if !h.has("January 2020") {
		t.Fatal("the panel did not open on the value's page")
	}
	h.click("Today")
	today := time.Date(2026, 11, 22, 0, 0, 0, 0, time.Local)
	if picker.Open || !picker.Value.Equal(today) || !chosen.Equal(today) {
		t.Fatalf("Today left open %v, value %v, reported %v", picker.Open, picker.Value, chosen)
	}
}

// When today itself is not selectable the link only shows its page.
func TestTodayLinkOnlyNavigatesWhenTodayIsDisabled(t *testing.T) {
	picker := datepicker.New(datepicker.GranularityDay)
	picker.Now = fixedNow
	picker.SetValue(time.Date(2020, 1, 5, 0, 0, 0, 0, time.Local))
	picker.DisabledDate = func(time.Time) bool { return true }
	h := open(t, picker)
	h.clickTrigger()
	h.click("Today")
	if !picker.Open || !h.has("November 2026") {
		t.Fatalf("Today closed the panel or skipped its page: open %v", picker.Open)
	}
	if !picker.Value.Equal(time.Date(2020, 1, 5, 0, 0, 0, 0, time.Local)) {
		t.Fatalf("Today changed the value to %v", picker.Value)
	}
}

// The title drills from days to months to years without touching the value,
// and a drilled panel steps back before the panel closes.
func TestTitleDrillsAndEscapeStepsBack(t *testing.T) {
	picker := datepicker.New(datepicker.GranularityDay)
	picker.Now = fixedNow
	h := open(t, picker)
	h.clickTrigger()
	h.click("November 2026")
	if !h.has("2026-03") {
		t.Fatal("the title did not open the month panel")
	}
	if picker.HasValue || !picker.Open {
		t.Fatalf("drilling changed the value or closed the panel: has %v open %v", picker.HasValue, picker.Open)
	}
	h.key(gfx.KeyEscape)
	if !picker.Open {
		t.Fatal("Escape closed the drilled panel instead of stepping back")
	}
	if !h.has("November 2026") {
		t.Fatal("Escape did not return to the day panel")
	}
	h.click("November 2026")
	h.click("2026")
	if !h.has("2020-2031") {
		t.Fatal("the year title did not open the decade panel")
	}
	h.click("2024")
	if !h.has("2024") || h.has("June 2024") {
		t.Fatal("choosing a year did not show that year's months")
	}
	h.click("2024-06")
	if !h.has("June 2024") || picker.HasValue {
		t.Fatalf("choosing a month closed the panel or changed the value: has %v", picker.HasValue)
	}
	h.click("2024-06-15")
	if !picker.Value.Equal(time.Date(2024, 6, 15, 0, 0, 0, 0, time.Local)) {
		t.Fatalf("chose %v", picker.Value)
	}
}

// Bounds and a predicate disable cells: they cannot be chosen, and the
// keyboard skips them.
func TestBoundsAndDisabledDates(t *testing.T) {
	picker := datepicker.New(datepicker.GranularityDay)
	picker.Now = fixedNow
	picker.Min = time.Date(2026, 11, 10, 0, 0, 0, 0, time.Local)
	picker.Max = time.Date(2026, 11, 20, 0, 0, 0, 0, time.Local)
	picker.DisabledDate = func(value time.Time) bool { return value.Weekday() == time.Sunday }
	h := open(t, picker)
	h.clickTrigger()
	if !h.hit("2026-11-09").Disabled {
		t.Fatal("a day before Min is not disabled")
	}
	if !h.hit("2026-11-22").Disabled {
		t.Fatal("a Sunday is not disabled")
	}
	h.click("2026-11-16")
	if !picker.Value.Equal(time.Date(2026, 11, 16, 0, 0, 0, 0, time.Local)) {
		t.Fatalf("chose %v", picker.Value)
	}
}

// The small step shrinks the field and still opens a working panel.
func TestSmallStep(t *testing.T) {
	picker := datepicker.New(datepicker.GranularityDay)
	picker.Now = fixedNow
	picker.Small = true
	h := open(t, picker)
	if height := h.box.Rect().Height; height >= 32 {
		t.Fatalf("small field is %v high", height)
	}
	h.clickTrigger()
	h.click("2026-11-10")
	if !picker.Value.Equal(time.Date(2026, 11, 10, 0, 0, 0, 0, time.Local)) {
		t.Fatalf("small picker chose %v", picker.Value)
	}
}

// ChineseLabels localizes the title, the cells and their accessible names.
func TestChineseLabels(t *testing.T) {
	picker := datepicker.New(datepicker.GranularityDay)
	picker.Now = fixedNow
	picker.Labels = datepicker.ChineseLabels()
	h := open(t, picker)
	h.clickTrigger()
	if !h.has("2026年11月") {
		t.Fatal("the Chinese title is missing")
	}
	if !h.has("2026年11月15日") {
		t.Fatal("the Chinese accessibility name is missing")
	}
}

// Shortcut chips sit on the bottom row and choose like a cell.
func TestDayPickerPresets(t *testing.T) {
	picker := datepicker.New(datepicker.GranularityDay)
	picker.Now = fixedNow
	picker.Presets = []datepicker.Preset{
		{Label: "Christmas", Value: time.Date(2026, 12, 25, 0, 0, 0, 0, time.Local)},
	}
	var chosen time.Time
	picker.OnChange = func(value time.Time, has bool) { chosen = value }
	h := open(t, picker)
	h.clickTrigger()
	chip := h.hit("Christmas")
	cell := h.hit("2026-11-22")
	if chip.Bounds.Y <= cell.Bounds.Y {
		t.Fatalf("the preset chip at y %v is not below the grid cell at y %v", chip.Bounds.Y, cell.Bounds.Y)
	}
	h.click("Christmas")
	if !picker.HasValue || !picker.Value.Equal(time.Date(2026, 12, 25, 0, 0, 0, 0, time.Local)) {
		t.Fatalf("the preset chose %v", picker.Value)
	}
	if picker.Open || !chosen.Equal(picker.Value) {
		t.Fatalf("the preset left open %v and reported %v", picker.Open, chosen)
	}
}

// Preset chips that do not fit wrap onto another row instead of being cut.
func TestPresetsWrapOntoMoreRows(t *testing.T) {
	picker := datepicker.New(datepicker.GranularityDay)
	picker.Now = fixedNow
	picker.Presets = []datepicker.Preset{
		{Label: "Last seven days", Value: day(2026, 11, 16)},
		{Label: "Last thirty days", Value: day(2026, 10, 24)},
		{Label: "Quarter to date", Value: day(2026, 10, 1)},
		{Label: "Year to date", Value: day(2026, 1, 1)},
	}
	h := open(t, picker)
	h.clickTrigger()
	rows := map[float32]bool{}
	for _, label := range []string{"Last seven days", "Last thirty days", "Quarter to date", "Year to date"} {
		rows[h.hit(label).Bounds.Y] = true
	}
	if len(rows) < 2 {
		t.Fatalf("the chips share %d row(s), want them to wrap", len(rows))
	}
	var chosen time.Time
	picker.OnChange = func(value time.Time, has bool) { chosen = value }
	h.click("Year to date")
	if !picker.HasValue || !chosen.Equal(day(2026, 1, 1)) {
		t.Fatalf("a wrapped chip chose %v (reported %v)", picker.Value, chosen)
	}
}

// A drilled title is a button; the top-level title is only a status.
func TestDrillTitleIsAButton(t *testing.T) {
	picker := datepicker.New(datepicker.GranularityDay)
	picker.Now = fixedNow
	h := open(t, picker)
	h.clickTrigger()
	if role := h.hit("November 2026").Role; role != gfx.RoleButton {
		t.Fatalf("the drill title has role %v, want a button", role)
	}
	h.click("November 2026")
	if role := h.hit("2026").Role; role != gfx.RoleButton {
		t.Fatalf("the month panel's year has role %v, want a button", role)
	}
	h.click("2026")
	if role := h.hit("2020-2031").Role; role != gfx.RoleStatus {
		t.Fatalf("the decade title has role %v, want a status", role)
	}
}
