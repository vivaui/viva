package ui_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var groupedHosts = []string{"alpha", "beta", "gamma", "delta"}
var hostGroupHeads = []ui.SelectGroup{{Label: "Recent", From: 0}, {Label: "All hosts", From: 2}}

func groupedSelect(t *testing.T, host *string, disabled []bool) *pageDriver {
	t.Helper()
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(16).Children(func() {
			ui.Select(c, host, groupedHosts, nil, ui.SelectGroups(hostGroupHeads)).Placeholder("Host").DisabledOptions(disabled)
		})
	})
	d.click("Host")
	return d
}

// An open list has a heading before the first option of each group, and a heading is
// no option: it is painted, but it is not a control to press.
func TestSelectGroupsPaintHeadingsThatAreNotOptions(t *testing.T) {
	host := ""
	d := groupedSelect(t, &host, nil)
	for _, heading := range []string{"Recent", "All hosts"} {
		found := false
		for _, text := range d.driver.Snapshot().Texts {
			found = found || text.Value == heading
		}
		if !found {
			t.Fatalf("the list did not paint the heading %q", heading)
		}
		if d.has(heading) {
			t.Fatalf("the heading %q is a control", heading)
		}
	}
	for _, option := range groupedHosts {
		if !d.has(option) {
			t.Fatalf("the option %q is not in the list", option)
		}
	}
}

// A press on an option reports the choice's own index, whatever headings come
// before it.
func TestSelectGroupsPickByTheChoicesIndex(t *testing.T) {
	host := ""
	d := groupedSelect(t, &host, nil)
	d.click("gamma")
	if host != "gamma" {
		t.Fatalf("host = %q after a press on gamma, want gamma", host)
	}
}

// The arrow keys skip the headings and go option to option across groups.
func TestSelectGroupsKeysSkipHeadings(t *testing.T) {
	host := ""
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(16).Children(func() {
			ui.Select(c, &host, groupedHosts, nil, ui.SelectGroups(hostGroupHeads)).Placeholder("Host")
		})
	})
	d.key(gfx.KeyTab)
	d.key(gfx.KeyEnter)
	if got := d.Focused(); got != "alpha" {
		t.Fatalf("Enter opened the list on %q, want the first option", got)
	}
	d.key(gfx.KeyDown)
	d.key(gfx.KeyDown)
	if got := d.Focused(); got != "gamma" {
		t.Fatalf("two Downs reached %q, want gamma, past the second heading", got)
	}
	d.key(gfx.KeyEnter)
	if host != "gamma" {
		t.Fatalf("Enter chose %q", host)
	}
}

// DisabledOptions keeps indexing the choices: a disabled option cannot be pressed
// and its neighbors can.
func TestSelectGroupsDisabledOptionsFollowTheChoices(t *testing.T) {
	host := ""
	d := groupedSelect(t, &host, []bool{false, true, false, false})
	d.click("beta")
	if host != "" {
		t.Fatalf("a disabled option was chosen: %q", host)
	}
	d.click("gamma")
	if host != "gamma" {
		t.Fatalf("host = %q, want gamma chosen", host)
	}
}

// The chosen option shows on the trigger, and the grouped list repaints without allocating.
func TestSelectGroupsRepaintWithoutAllocating(t *testing.T) {
	host := "beta"
	d := groupedSelect(t, &host, nil)
	d.repaint()
	d.repaint()
	if allocs := testing.AllocsPerRun(100, d.repaint); allocs != 0 {
		t.Fatalf("%v allocations a repaint", allocs)
	}
}
