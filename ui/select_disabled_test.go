package ui_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

func openSelect(t *testing.T, size *string, disabled []bool, options ...func(ui.Box) ui.Box) *pageDriver {
	t.Helper()
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(16).Gap(8).Children(func() {
			b := ui.Select(c, size, []string{"S", "M", "L"}, nil).Placeholder("Size").DisabledOptions(disabled)
			for _, option := range options {
				b = option(b)
			}
		})
	})
	for _, hit := range d.driver.Snapshot().Hits {
		if hit.Role == gfx.RoleCombobox {
			if err := d.driver.Click(center(hit.Bounds)); err != nil {
				t.Fatal(err)
			}
			return d
		}
	}
	t.Fatal("no select trigger")
	return nil
}

// A disabled option is listed but cannot be chosen: a click on it picks nothing
// and leaves the list open, while its neighbours still pick.
func TestSelectDisabledOptionTakesNoClick(t *testing.T) {
	size := ""
	d := openSelect(t, &size, []bool{false, true, false})
	if err := d.driver.Click(center(d.hit("M").Bounds)); err != nil {
		t.Fatal(err)
	}
	if size != "" {
		t.Fatalf("a disabled option was chosen: %q", size)
	}
	if !d.has("L") {
		t.Fatal("the list closed on a click on a disabled option")
	}
	d.click("L")
	if size != "L" {
		t.Fatalf("an enabled option beside a disabled one was not chosen: %q", size)
	}
}

// The accessibility tree says which options are unavailable.
func TestSelectDisabledOptionIsReportedDisabled(t *testing.T) {
	size := ""
	d := openSelect(t, &size, []bool{false, true, false})
	if hit := d.hit("M"); !hit.Disabled {
		t.Fatal("the disabled option is not reported disabled")
	}
	if hit := d.hit("S"); hit.Disabled {
		t.Fatal("an enabled option is reported disabled")
	}
}

// XSmall is a shorter trigger than the default, as a button's is.
func TestSelectXSmallIsShorterThanDefault(t *testing.T) {
	height := func(xsmall bool) float32 {
		size := ""
		d := openPage(t, func(c ui.C) {
			b := ui.Select(c, &size, []string{"S", "M"}, nil).Placeholder("Size")
			if xsmall {
				b.XSmall()
			}
		})
		for _, hit := range d.driver.Snapshot().Hits {
			if hit.Role == gfx.RoleCombobox {
				return hit.Bounds.Height
			}
		}
		t.Fatal("no select trigger")
		return 0
	}
	if normal, small := height(false), height(true); small >= normal {
		t.Fatalf("XSmall trigger is %v tall, the default %v", small, normal)
	}
}

// Declaring a Select with DisabledOptions allocates nothing once the page is warm.
func TestSelectDisabledOptionsDeclareWithoutAllocating(t *testing.T) {
	size := "M"
	disabled := []bool{false, true, false}
	choices := []string{"S", "M", "L"}
	page, canvas := ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas()
	view := func(c ui.C) {
		ui.Column(c).Pad(16).Children(func() {
			ui.Select(c, &size, choices, nil).DisabledOptions(disabled).XSmall()
		})
	}
	drawPage(page, canvas, view)
	if got := testing.AllocsPerRun(50, func() { drawPage(page, canvas, view) }); got != 0 {
		t.Fatalf("a warm paint of a Select with DisabledOptions allocates %v times", got)
	}
}
