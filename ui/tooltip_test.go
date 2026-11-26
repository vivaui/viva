package ui_test

import (
	"strings"
	"testing"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

func textTooltip(d *pageDriver) string {
	var lines []string
	for _, label := range d.driver.Snapshot().Labels {
		if label.Layer == ui.TooltipLayer {
			lines = append(lines, label.Value)
		}
	}
	return strings.Join(lines, "")
}

// openTipsPage opens a page with EllipsisTooltips on.
func openTipsPage(t *testing.T, View func(ui.C)) *pageDriver {
	t.Helper()
	return openPageWith(t, ui.PageConfig{View: View, EllipsisTooltips: true}, app.Config{})
}

// An ellipsized label keeps its complete value available without taking the
// hit, focus or click of the row that contains it. A memo must keep that value.
func TestEllipsisTooltip(t *testing.T) {
	for _, memo := range []bool{false, true} {
		t.Run(map[bool]string{false: "plain", true: "memo"}[memo], func(t *testing.T) {
			const full = "完整文件名称-with-a-long-extension.txt"
			width := float32(100)
			clicks := 0
			var label ui.Box
			d := openTipsPage(t, func(c ui.C) {
				ui.Column(c).Pad(40).Gap(20).Children(func() {
					row := func() {
						if ui.Row(c).Width(width).HitWith(ui.ID(c), "Open file", gfx.HitOptions{Role: gfx.RoleButton}).Children(func() {
							label = ui.Text(c, full).Ellipsis()
						}).Clicked() {
							clicks++
						}
					}
					if memo {
						ui.Memo(c, width, row)
					} else {
						row()
					}
					ui.Text(c, "Already complete…").Ellipsis()
				})
			})
			d.repaint() // allow the memo to replay while the pointer is elsewhere
			if err := d.driver.Move(center(d.hit("Open file").Bounds)); err != nil {
				t.Fatal(err)
			}
			if got := textTooltip(d); got != full {
				t.Fatalf("tooltip %q, want %q", got, full)
			}
			if got := d.driver.Snapshot().Hovered; got != d.hit("Open file").ID {
				t.Fatalf("text took the row's pointer: %v", got)
			}
			d.click("Open file")
			if clicks != 1 || textTooltip(d) != "" {
				t.Fatalf("clicks %d, tooltip after click %q", clicks, textTooltip(d))
			}
			if err := d.driver.Move(300, 300); err != nil {
				t.Fatal(err)
			}
			if textTooltip(d) != "" {
				t.Fatal("tooltip followed the pointer away")
			}
			width = 390
			d.repaint()
			if err := d.driver.Move(center(label.Rect())); err != nil {
				t.Fatal(err)
			}
			if textTooltip(d) != "" {
				t.Fatal("complete text still has an overflow tooltip")
			}
		})
	}
}

func TestEllipsisTooltipWithoutAHit(t *testing.T) {
	theme := ui.LightTheme()
	theme.TooltipDelay = 500 * time.Millisecond
	var first, second ui.Box
	d := openPageWith(t, ui.PageConfig{Theme: &theme, EllipsisTooltips: true, View: func(c ui.C) {
		ui.Column(c).Pad(40).Gap(20).Children(func() {
			first = ui.Text(c, "First complete filename.txt").Width(80).Ellipsis()
			second = ui.Text(c, "Second complete filename.txt").Width(80).Ellipsis()
		})
	}}, app.Config{})
	if err := d.driver.Move(center(first.Rect())); err != nil {
		t.Fatal(err)
	}
	if textTooltip(d) != "" {
		t.Fatal("tooltip appeared before its delay")
	}
	if err := d.driver.Advance(600 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if got := textTooltip(d); got != "First complete filename.txt" {
		t.Fatalf("first tooltip %q", got)
	}
	if err := d.driver.Move(center(second.Rect())); err != nil {
		t.Fatal(err)
	}
	if textTooltip(d) != "" {
		t.Fatal("moving to a different label did not restart the delay")
	}
	if err := d.driver.Advance(600 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if got := textTooltip(d); got != "Second complete filename.txt" {
		t.Fatalf("second tooltip %q", got)
	}
}

func TestEllipsisTooltipClippingAndExistingHint(t *testing.T) {
	const full = "Complete filename before the suffix.txt"
	var text ui.Box
	covered := false
	d := openTipsPage(t, func(c ui.C) {
		ui.Column(c).Pad(40).Children(func() {
			ui.Row(c).Width(60).Height(30).Clip().HitWith(ui.ID(c), "Row", gfx.HitOptions{Role: gfx.RoleButton}).Tooltip("Open file").Children(func() {
				text = ui.Text(c, full).Width(100).Shrink(0).Ellipsis()
			})
			ui.Text(c, "Already complete…").Ellipsis()
			if covered {
				ui.Row(c).Absolute().Left(0).Top(0).Size(200, 200).HitWith(ui.ID(c), "Cover", gfx.HitOptions{Role: gfx.RoleButton})
			}
		})
	})
	if err := d.driver.Move(45, text.Rect().Y+5); err != nil {
		t.Fatal(err)
	}
	if got := textTooltip(d); got != full {
		t.Fatalf("visible text tooltip %q", got)
	}
	if err := d.driver.Move(45, text.Rect().Y+27); err != nil {
		t.Fatal(err)
	}
	if got := textTooltip(d); got != "Open file" {
		t.Fatalf("row's original hint %q", got)
	}
	if err := d.driver.Move(120, text.Rect().Y+5); err != nil {
		t.Fatal(err)
	}
	if got := textTooltip(d); got != "" {
		t.Fatalf("clipped text tooltip %q", got)
	}
	if err := d.driver.Move(45, text.Rect().Y+5); err != nil {
		t.Fatal(err)
	}
	covered = true
	d.repaint()
	if err := d.driver.Move(46, text.Rect().Y+5); err != nil {
		t.Fatal(err)
	}
	if got := textTooltip(d); got != "" {
		t.Fatalf("covered text tooltip %q", got)
	}
}

func TestMaxLinesTooltipAndLiteralEllipsis(t *testing.T) {
	const full = "First line\nSecond line\nThird line"
	var paragraph, complete ui.Box
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(40).Gap(30).EllipsisTooltip(true).Children(func() {
			paragraph = ui.Text(c, full).Width(220).MaxLines(2)
			complete = ui.Text(c, "Already complete…").Ellipsis()
		})
	})
	if err := d.driver.Move(center(paragraph.Rect())); err != nil {
		t.Fatal(err)
	}
	if got := textTooltip(d); got != "First lineSecond lineThird line" {
		t.Fatalf("paragraph tooltip %q", got)
	}
	if err := d.driver.Move(center(complete.Rect())); err != nil {
		t.Fatal(err)
	}
	if textTooltip(d) != "" {
		t.Fatal("a literal ellipsis was mistaken for truncation")
	}
}

// A page that does not turn EllipsisTooltips on paints shortened text as it
// did: no tooltip, and a Select clips its value at its edge rather than ending
// it with an ellipsis. OverflowTooltip, given explicitly, still shows.
func TestEllipsisTooltipIsOffByDefault(t *testing.T) {
	const full = "Complete configuration name (3)"
	value := full
	var text ui.Box
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(40).Gap(20).Children(func() {
			text = ui.Text(c, "Complete filename before the suffix.txt").Width(80).Ellipsis()
			ui.Select(c, &value, []string{full}, nil).Width(112)
		})
	})
	if err := d.driver.Move(center(text.Rect())); err != nil {
		t.Fatal(err)
	}
	if err := d.driver.Advance(2 * time.Second); err != nil {
		t.Fatal(err)
	}
	if got := textTooltip(d); got != "" {
		t.Fatalf("text tooltip %q with the tooltips off", got)
	}
	if !tooltipShown(d, full) {
		t.Fatal("the Select no longer draws its whole value, clipped")
	}
	for _, label := range d.driver.Snapshot().Labels {
		if strings.HasPrefix(label.Value, "Complete configuration") && strings.HasSuffix(label.Value, "…") {
			t.Fatalf("the Select ended its value with an ellipsis: %q", label.Value)
		}
	}
	if err := d.driver.Move(center(selectTrigger(d).Bounds)); err != nil {
		t.Fatal(err)
	}
	if got := textTooltip(d); got != "" {
		t.Fatalf("Select tooltip %q with the tooltips off", got)
	}
}

func selectTrigger(d *pageDriver) gfx.Hit {
	d.t.Helper()
	for _, hit := range d.driver.Snapshot().Hits {
		if hit.Role == gfx.RoleCombobox {
			return hit
		}
	}
	d.t.Fatal("no Select trigger")
	return gfx.Hit{}
}

// EllipsisTooltip on a Select turns the tooltips on for its trigger and for the
// list that hangs off it. Assistive technology reads the whole value from the
// trigger whatever the drawing shortened.
func TestEllipsisTooltipOnASelectAndItsList(t *testing.T) {
	const full = "A complete configuration name far too long for the trigger, and for the list that opens below it"
	value := full
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(40).Children(func() {
			ui.Select(c, &value, []string{full, "Short"}, nil).Width(112).EllipsisTooltip(true)
		})
	})
	trigger := selectTrigger(d)
	if trigger.Value != full {
		t.Fatalf("trigger's accessible value %q, want the whole value", trigger.Value)
	}
	if tooltipShown(d, full) {
		t.Fatal("the trigger still draws its whole value")
	}
	if err := d.driver.Move(center(trigger.Bounds)); err != nil {
		t.Fatal(err)
	}
	if got := textTooltip(d); !sameWrapped(got, full) {
		t.Fatalf("trigger tooltip %q", got)
	}
	if err := d.driver.Click(center(trigger.Bounds)); err != nil {
		t.Fatal(err)
	}
	d.repaint()
	var row gfx.Hit
	for _, hit := range d.driver.Snapshot().Hits {
		if hit.Label == full && hit.Role != gfx.RoleCombobox {
			row = hit
		}
	}
	if row.Bounds.Width == 0 {
		t.Fatal("the list did not open")
	}
	if err := d.driver.Move(row.Bounds.X+30, row.Bounds.Y+row.Bounds.Height/2); err != nil {
		t.Fatal(err)
	}
	if got := textTooltip(d); !sameWrapped(got, full) {
		t.Fatalf("list row tooltip %q", got)
	}
}

// sameWrapped compares a tooltip's lines, joined, with the text it wrapped.
func sameWrapped(lines, text string) bool {
	return lines != "" && strings.ReplaceAll(lines, " ", "") == strings.ReplaceAll(text, " ", "")
}

// EllipsisTooltip changes the setting for an element and what it holds, over
// the page's and over one further out.
func TestEllipsisTooltipFollowsTheSubtree(t *testing.T) {
	var off, on ui.Box
	d := openTipsPage(t, func(c ui.C) {
		ui.Column(c).Pad(40).Gap(20).EllipsisTooltip(false).Children(func() {
			off = ui.Text(c, "Off: complete filename before the suffix.txt").Width(80).Ellipsis()
			ui.Row(c).EllipsisTooltip(true).Children(func() {
				on = ui.Text(c, "On: complete filename before the suffix.txt").Width(80).Ellipsis()
			})
		})
	})
	if err := d.driver.Move(center(off.Rect())); err != nil {
		t.Fatal(err)
	}
	if got := textTooltip(d); got != "" {
		t.Fatalf("tooltip %q inside EllipsisTooltip(false)", got)
	}
	if err := d.driver.Move(center(on.Rect())); err != nil {
		t.Fatal(err)
	}
	if got := textTooltip(d); got != "On: complete filename before the suffix.txt" {
		t.Fatalf("tooltip %q inside EllipsisTooltip(true)", got)
	}
}

// A Popup placed by a rectangle follows the element it was declared in, not
// the one its rectangle happens to be over.
func TestEllipsisTooltipInAPopupFollowsWhereItWasDeclared(t *testing.T) {
	var inside, outside ui.Box
	d := openPage(t, func(c ui.C) {
		ui.Column(c).EllipsisTooltip(true).Children(func() {
			ui.Overlay(c, ui.OverlaySpec{Bounds: gfx.Rect{X: 20, Y: 20, Width: 200, Height: 40}}).Children(func() {
				inside = ui.Text(c, "Inside: complete filename before the suffix.txt").Width(80).Ellipsis()
			})
		})
		ui.Overlay(c, ui.OverlaySpec{Bounds: gfx.Rect{X: 20, Y: 120, Width: 200, Height: 40}}).Children(func() {
			outside = ui.Text(c, "Outside: complete filename before the suffix.txt").Width(80).Ellipsis()
		})
	})
	if err := d.driver.Move(center(inside.Rect())); err != nil {
		t.Fatal(err)
	}
	if got := textTooltip(d); got != "Inside: complete filename before the suffix.txt" {
		t.Fatalf("tooltip %q in a popup declared inside EllipsisTooltip(true)", got)
	}
	if err := d.driver.Move(center(outside.Rect())); err != nil {
		t.Fatal(err)
	}
	if got := textTooltip(d); got != "" {
		t.Fatalf("tooltip %q in a popup declared outside it", got)
	}
}

// A Memo's copy is painted again when the setting around it changed, so it does
// not bring back the hints of the paint it copies.
func TestEllipsisTooltipSettingRedeclaresAMemo(t *testing.T) {
	const full = "Complete filename before the suffix.txt"
	on := true
	var text ui.Box
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(40).EllipsisTooltip(on).Children(func() {
			ui.Memo(c, 1, func() { text = ui.Text(c, full).Width(80).Ellipsis() })
		})
	})
	at := text.Rect() // a copied Memo declares no Box
	d.repaint()       // the Memo is copied from here on
	if err := d.driver.Move(center(at)); err != nil {
		t.Fatal(err)
	}
	if got := textTooltip(d); got != full {
		t.Fatalf("tooltip %q", got)
	}
	if err := d.driver.Move(300, 300); err != nil {
		t.Fatal(err)
	}
	on = false
	d.repaint()
	d.repaint()
	if err := d.driver.Move(center(at)); err != nil {
		t.Fatal(err)
	}
	if got := textTooltip(d); got != "" {
		t.Fatalf("a copied Memo kept tooltip %q after EllipsisTooltip(false)", got)
	}
}

// tipBounds is where the tooltip of shortened text shows: its one line.
func tipBounds(t *testing.T, d *pageDriver) gfx.Rect {
	t.Helper()
	for _, label := range d.driver.Snapshot().Labels {
		if label.Layer == ui.TooltipLayer {
			return label.Bounds
		}
	}
	t.Fatal("no tooltip")
	return gfx.Rect{}
}

// beside reports whether tip lies wholly on side of text.
func beside(tip, text gfx.Rect, side ui.TooltipSide) bool {
	switch side {
	case ui.TooltipTop:
		return tip.Y+tip.Height <= text.Y
	case ui.TooltipRight:
		return tip.X >= text.X+text.Width
	case ui.TooltipLeft:
		return tip.X+tip.Width <= text.X
	}
	return tip.Y >= text.Y+text.Height
}

// The page under the pointer and the bounds of the hint the pointer rests on.
func hoveredHint(t *testing.T, d *pageDriver) gfx.Rect {
	t.Helper()
	hint := d.page.HoveredText()
	if hint.Text == "" {
		t.Fatal("no shortened text under the pointer")
	}
	return hint.Bounds
}

// openSidesPage opens a page large enough for a tooltip on every side of text
// in its middle.
func openSidesPage(t *testing.T, tips bool, View func(ui.C)) *pageDriver {
	t.Helper()
	return openPageWith(t, ui.PageConfig{View: View, EllipsisTooltips: tips}, app.Config{Width: 1200, Height: 900})
}

// EllipsisTooltipOn opens the tooltip beside the side it names, below by
// default, and on the opposite side where its own has no room.
func TestEllipsisTooltipOn(t *testing.T) {
	const full = "Complete filename before the suffix.txt"
	for _, side := range []ui.TooltipSide{ui.TooltipTop, ui.TooltipRight, ui.TooltipBottom, ui.TooltipLeft} {
		var text ui.Box
		d := openSidesPage(t, true, func(c ui.C) {
			ui.Column(c).Pad(400).EllipsisTooltipOn(side).Children(func() {
				text = ui.Text(c, full).Width(80).Ellipsis()
			})
		})
		if err := d.driver.Move(center(text.Rect())); err != nil {
			t.Fatal(err)
		}
		if tip, at := tipBounds(t, d), hoveredHint(t, d); !beside(tip, at, side) {
			t.Fatalf("side %d: the tooltip %v is not beside its text %v", side, tip, at)
		}
	}
	var plain, top ui.Box
	d := openSidesPage(t, true, func(c ui.C) {
		ui.Column(c).Pad(4).EllipsisTooltipOn(ui.TooltipTop).Children(func() {
			top = ui.Text(c, full).Width(80).Ellipsis()
		})
		ui.Column(c).Pad(400).Children(func() { plain = ui.Text(c, full).Width(80).Ellipsis() })
	})
	if err := d.driver.Move(center(top.Rect())); err != nil {
		t.Fatal(err)
	}
	if tip, at := tipBounds(t, d), hoveredHint(t, d); !beside(tip, at, ui.TooltipBottom) {
		t.Fatalf("with no room above, the tooltip %v is not below its text %v", tip, at)
	}
	if err := d.driver.Move(center(plain.Rect())); err != nil {
		t.Fatal(err)
	}
	if tip, at := tipBounds(t, d), hoveredHint(t, d); !beside(tip, at, ui.TooltipBottom) {
		t.Fatalf("without EllipsisTooltipOn, the tooltip %v is not below its text %v", tip, at)
	}
}

// The side follows the subtree apart from the switch: EllipsisTooltip keeps the
// side from further out, the nearest EllipsisTooltipOn wins, either order on one
// element sets both, and the side alone turns nothing on.
func TestEllipsisTooltipOnFollowsTheSubtree(t *testing.T) {
	var alone, kept, nearer, reversed ui.Box
	d := openSidesPage(t, false, func(c ui.C) {
		ui.Column(c).Pad(400).Gap(40).EllipsisTooltipOn(ui.TooltipRight).Children(func() {
			alone = ui.Text(c, "Alone: complete filename before the suffix.txt").Width(80).Ellipsis()
			ui.Row(c).EllipsisTooltip(true).Children(func() {
				kept = ui.Text(c, "Kept: complete filename before the suffix.txt").Width(80).Ellipsis()
			})
			ui.Row(c).EllipsisTooltip(true).EllipsisTooltipOn(ui.TooltipTop).Children(func() {
				nearer = ui.Text(c, "Nearer: complete filename before the suffix.txt").Width(80).Ellipsis()
			})
			ui.Row(c).EllipsisTooltipOn(ui.TooltipLeft).EllipsisTooltip(true).Children(func() {
				reversed = ui.Text(c, "Reversed: complete filename before the suffix.txt").Width(80).Ellipsis()
			})
		})
	})
	wants := []struct {
		at   gfx.Rect
		side ui.TooltipSide
	}{{kept.Rect(), ui.TooltipRight}, {nearer.Rect(), ui.TooltipTop}, {reversed.Rect(), ui.TooltipLeft}}
	if err := d.driver.Move(center(alone.Rect())); err != nil {
		t.Fatal(err)
	}
	if got := textTooltip(d); got != "" {
		t.Fatalf("EllipsisTooltipOn alone turned the tooltip on: %q", got)
	}
	for _, want := range wants {
		if err := d.driver.Move(center(want.at)); err != nil {
			t.Fatal(err)
		}
		if tip, at := tipBounds(t, d), hoveredHint(t, d); !beside(tip, at, want.side) {
			t.Fatalf("side %d: the tooltip %v is not beside its text %v", want.side, tip, at)
		}
	}
}

// A Select's list opens its tooltips on the side of the Select, and a Popup
// placed by a rectangle on the side of the element it was declared in.
func TestEllipsisTooltipOnInAPopup(t *testing.T) {
	const full = "A complete configuration name far too long for the trigger, and for the list that opens below it, which the window cannot hold either, so the list shortens it with an ellipsis and the tooltip shows the rest"
	value := full
	var overlay ui.Box
	d := openSidesPage(t, true, func(c ui.C) {
		ui.Column(c).Pad(400).Children(func() {
			ui.Select(c, &value, []string{full, "Short"}, nil).Width(112).EllipsisTooltipOn(ui.TooltipTop)
		})
		ui.Column(c).EllipsisTooltipOn(ui.TooltipRight).Children(func() {
			ui.Overlay(c, ui.OverlaySpec{Bounds: gfx.Rect{X: 300, Y: 540, Width: 200, Height: 40}}).Children(func() {
				overlay = ui.Text(c, "Overlay: complete filename before the suffix.txt").Width(80).Ellipsis()
			})
		})
	})
	if err := d.driver.Move(center(overlay.Rect())); err != nil {
		t.Fatal(err)
	}
	if tip, at := tipBounds(t, d), hoveredHint(t, d); !beside(tip, at, ui.TooltipRight) {
		t.Fatalf("the overlay's tooltip %v is not right of its text %v", tip, at)
	}
	trigger := selectTrigger(d)
	if err := d.driver.Click(center(trigger.Bounds)); err != nil {
		t.Fatal(err)
	}
	d.repaint()
	var row gfx.Hit
	for _, hit := range d.driver.Snapshot().Hits {
		if hit.Label == full && hit.Role != gfx.RoleCombobox {
			row = hit
		}
	}
	if row.Bounds.Width == 0 {
		t.Fatal("the list did not open")
	}
	if err := d.driver.Move(row.Bounds.X+30, row.Bounds.Y+row.Bounds.Height/2); err != nil {
		t.Fatal(err)
	}
	if tip, at := tipBounds(t, d), hoveredHint(t, d); !beside(tip, at, ui.TooltipTop) {
		t.Fatalf("the list row's tooltip %v is not above its text %v", tip, at)
	}
}

// OverflowTooltip opens beside the side EllipsisTooltipOn gives, on a page that
// leaves EllipsisTooltips off.
func TestOverflowTooltipOpensOnTheEllipsisSide(t *testing.T) {
	const full = "Trust all projects in /workspace/long-directory/projects"
	checked := false
	d := openSidesPage(t, false, func(c ui.C) {
		ui.Column(c).Pad(400).EllipsisTooltipOn(ui.TooltipLeft).Children(func() {
			ui.Checkbox(c, &checked, "Trust /workspace/…/projects").OverflowTooltip(full)
		})
	})
	if err := d.driver.Move(center(d.hit("Trust /workspace/…/projects").Bounds)); err != nil {
		t.Fatal(err)
	}
	if tip, at := tipBounds(t, d), hoveredHint(t, d); !beside(tip, at, ui.TooltipLeft) {
		t.Fatalf("the tooltip %v is not left of its text %v", tip, at)
	}
}

// A Memo's copy is painted again when the side around it changed.
func TestEllipsisTooltipOnRedeclaresAMemo(t *testing.T) {
	const full = "Complete filename before the suffix.txt"
	side := ui.TooltipRight
	var text ui.Box
	d := openSidesPage(t, true, func(c ui.C) {
		ui.Column(c).Pad(400).EllipsisTooltipOn(side).Children(func() {
			ui.Memo(c, 1, func() { text = ui.Text(c, full).Width(80).Ellipsis() })
		})
	})
	at := text.Rect() // a copied Memo declares no Box
	d.repaint()       // the Memo is copied from here on
	if err := d.driver.Move(center(at)); err != nil {
		t.Fatal(err)
	}
	if tip := tipBounds(t, d); !beside(tip, at, ui.TooltipRight) {
		t.Fatalf("the tooltip %v is not right of its text %v", tip, at)
	}
	if err := d.driver.Move(10, 10); err != nil {
		t.Fatal(err)
	}
	side = ui.TooltipLeft
	d.repaint()
	d.repaint()
	if err := d.driver.Move(center(at)); err != nil {
		t.Fatal(err)
	}
	if tip := tipBounds(t, d); !beside(tip, at, ui.TooltipLeft) {
		t.Fatalf("a copied Memo kept the old side: the tooltip %v is not left of its text %v", tip, at)
	}
}

func TestOverflowTooltipKeepsCheckboxCaptionClickable(t *testing.T) {
	const full = "Trust all projects in /workspace/long-directory/projects"
	checked := false
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(40).Children(func() {
			ui.Checkbox(c, &checked, "Trust /workspace/…/projects").OverflowTooltip(full)
		})
	})
	if err := d.driver.Move(center(d.hit("Trust /workspace/…/projects").Bounds)); err != nil {
		t.Fatal(err)
	}
	if got := textTooltip(d); got != full {
		t.Fatalf("checkbox tooltip %q, want %q", got, full)
	}
	d.click("Trust /workspace/…/projects")
	if !checked || textTooltip(d) != "" {
		t.Fatalf("checked %v, tooltip after click %q", checked, textTooltip(d))
	}
}

// tooltipShown reports whether the frame shows text in a tooltip: a label with
// that text and no control of that name.
func tooltipShown(d *pageDriver, Text string) bool {
	for _, label := range d.driver.Snapshot().Labels {
		if label.Value == Text {
			return true
		}
	}
	return false
}

func TestIdleTooltipPaintAllocatesNothing(t *testing.T) {
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Children(func() {
			ui.Button(c, "Info").Tooltip("More information")
			ui.Button(c, "Plain")
		})
	})
	if err := d.driver.Move(center(d.hit("Plain").Bounds)); err != nil {
		t.Fatal(err)
	}
	if allocs := testing.AllocsPerRun(100, d.repaint); allocs != 0 {
		t.Fatalf("idle tooltip paint: %v allocations per repaint, want zero", allocs)
	}
}

// A Tooltip appears while the pointer rests on its control, is the control's
// description, hides on a press until the pointer leaves, and a Memo's copy keeps it.
func TestTooltip(t *testing.T) {
	for _, memo := range []bool{false, true} {
		d := openPage(t, func(c ui.C) {
			ui.Column(c).Pad(40).Gap(40).Children(func() {
				row := func() { ui.Button(c, "Delete").Tooltip("Delete the host") }
				if memo {
					ui.Memo(c, 1, row)
				} else {
					row()
				}
				ui.Button(c, "Other")
			})
		})
		if hit := d.hit("Delete"); hit.Hint != "Delete the host" {
			t.Fatalf("memo=%v: description %q", memo, hit.Hint)
		}
		if tooltipShown(d, "Delete the host") {
			t.Fatalf("memo=%v: the tip shows with the pointer elsewhere", memo)
		}
		if err := d.driver.Move(center(d.hit("Delete").Bounds)); err != nil {
			t.Fatal(err)
		}
		if !tooltipShown(d, "Delete the host") {
			t.Fatalf("memo=%v: the tip did not appear over its control", memo)
		}
		d.click("Delete")
		if tooltipShown(d, "Delete the host") {
			t.Fatalf("memo=%v: the tip stayed through a press", memo)
		}
		if err := d.driver.Move(center(d.hit("Other").Bounds)); err != nil {
			t.Fatal(err)
		}
		if tooltipShown(d, "Delete the host") {
			t.Fatalf("memo=%v: the tip follows the pointer away", memo)
		}
	}
}

// tooltipLabel is the label of the shown tooltip saying text.
func tooltipLabel(t *testing.T, d *pageDriver, Text string) gfx.Rect {
	t.Helper()
	for _, label := range d.driver.Snapshot().Labels {
		if label.Value == Text {
			return label.Bounds
		}
	}
	t.Fatalf("no tooltip says %q", Text)
	return gfx.Rect{}
}

// TooltipOn opens the tip beside the side it names, below the control by
// default, and a Memo's copy keeps the side.
func TestTooltipOn(t *testing.T) {
	for _, memo := range []bool{false, true} {
		d := openPage(t, func(c ui.C) {
			ui.Column(c).Pad(40).Gap(40).Children(func() {
				row := func() {
					ui.Button(c, "Info").Tooltip("Opens to the right").TooltipOn(ui.TooltipRight)
					ui.Button(c, "Plain").Tooltip("Opens below")
				}
				if memo {
					ui.Memo(c, 1, row)
				} else {
					row()
				}
			})
		})
		info := d.hit("Info").Bounds
		if err := d.driver.Move(center(info)); err != nil {
			t.Fatal(err)
		}
		if tip := tooltipLabel(t, d, "Opens to the right"); tip.X < info.X+info.Width || tip.Y >= info.Y+info.Height {
			t.Fatalf("memo=%v: the tip %v is not to the right of its control %v", memo, tip, info)
		}
		plain := d.hit("Plain").Bounds
		if err := d.driver.Move(center(plain)); err != nil {
			t.Fatal(err)
		}
		if tip := tooltipLabel(t, d, "Opens below"); tip.Y < plain.Y+plain.Height {
			t.Fatalf("memo=%v: the tip %v is not below its control %v", memo, tip, plain)
		}
	}
}

// The page's theme sets how long the pointer rests before a tip appears; zero, the
// default, still shows it with the pointer.
func TestTooltipDelayComesFromTheTheme(t *testing.T) {
	theme := ui.LightTheme()
	theme.TooltipDelay = 500 * time.Millisecond
	d := openPageWith(t, ui.PageConfig{Theme: &theme, View: func(c ui.C) {
		ui.Column(c).Pad(40).Children(func() {
			ui.Button(c, "Delete").Tooltip("Delete the host")
		})
	}}, app.Config{})
	if err := d.driver.Move(center(d.hit("Delete").Bounds)); err != nil {
		t.Fatal(err)
	}
	if tooltipShown(d, "Delete the host") {
		t.Fatal("the tip appeared before the theme's delay")
	}
	if err := d.driver.Advance(600 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if !tooltipShown(d, "Delete the host") {
		t.Fatal("the tip did not appear after the theme's delay")
	}
}
