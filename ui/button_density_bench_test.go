package ui_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var denseIcon = gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{
	gfx.MoveTo(4, 4), gfx.LineTo(20, 20),
}}

var denseLabels = [...]string{"Save", "Cancel", "Deploy", "Connect", "Open", "Rename", "Delete", "Refresh"}

// denseButtons is a page made of buttons, as a toolbar-heavy workbench is: every
// variant and size, with and without an icon, in a few rows of a table. It has no
// other control, so a change in what a button costs is the change in the page.
func denseButtons(c ui.C) {
	ui.Column(c).Fill().Pad(12).Gap(8).Children(func() {
		ui.Row(c).Gap(6).Children(func() {
			for i := 0; i < 8; i++ {
				ui.Button(c, denseLabels[i]).Small().Ghost().Icon(denseIcon)
			}
		})
		ui.Row(c).Gap(6).Children(func() {
			ui.Button(c, "Default")
			ui.Button(c, "Secondary").Secondary()
			ui.Button(c, "Outline").Outline()
			ui.Button(c, "Ghost").Ghost()
			ui.Button(c, "Link").Link()
			ui.Button(c, "Destructive").Destructive()
			ui.Button(c, "XSmall").XSmall()
			ui.Button(c, "Large").Large()
			ui.Button(c, "").IconOnly().Icon(denseIcon)
		})
		for r := 0; r < 40; r++ {
			ui.Row(c).FillWidth().Gap(6).Children(func() {
				ui.Text(c, "row").Grow(1)
				ui.Button(c, denseLabels[r%len(denseLabels)]).Small().Outline()
				ui.Button(c, "More").Small().Ghost().Icon(denseIcon)
			})
		}
	})
}

// BenchmarkPageButtons is the page-level cost of themed buttons: 8 + 9 + 80 of them.
func BenchmarkPageButtons(b *testing.B) {
	page, canvas := ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas()
	b.ReportAllocs()
	for b.Loop() {
		drawPage(page, canvas, denseButtons)
	}
}

// A steady page of buttons allocates nothing, whatever a button is made of.
func TestPageOfButtonsAllocatesNothing(t *testing.T) {
	page, canvas := ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas()
	for range 3 {
		drawPage(page, canvas, denseButtons)
	}
	if allocs := testing.AllocsPerRun(50, func() { drawPage(page, canvas, denseButtons) }); allocs != 0 {
		t.Fatalf("a steady page of buttons allocates %v times", allocs)
	}
}
