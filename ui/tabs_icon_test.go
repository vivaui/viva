package ui_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// A page Tabs with TabIcons is the same tab strip with an icon before each label: a click
// still writes the chosen tab, and both tabs are named for assistive technology.
func TestPageTabsTabIconsSelectsAndNamesTabs(t *testing.T) {
	tabs := []string{"Overview", "Activity"}
	choice := tabs[0]
	icon := gfx.Icon{ViewBox: 24, Stroke: 2, Commands: []gfx.PathCommand{gfx.MoveTo(4, 4), gfx.LineTo(20, 20)}}
	d := openPage(t, func(c ui.C) {
		ui.Tabs(c, &choice, tabs, nil).TabIcons(func(int) gfx.Icon { return icon })
	})
	if d.hit("Overview").Role != gfx.RoleTab || d.hit("Activity").Role != gfx.RoleTab {
		t.Fatal("the tabs are not declared as tabs")
	}
	d.click("Activity")
	if choice != "Activity" {
		t.Fatalf("choice = %q, want Activity", choice)
	}
}
