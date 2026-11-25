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

// An ellipsized label keeps its complete value available without taking the
// hit, focus or click of the row that contains it. A memo must keep that value.
func TestEllipsisTooltip(t *testing.T) {
	for _, memo := range []bool{false, true} {
		t.Run(map[bool]string{false: "plain", true: "memo"}[memo], func(t *testing.T) {
			const full = "完整文件名称-with-a-long-extension.txt"
			width := float32(100)
			clicks := 0
			var label ui.Box
			d := openPage(t, func(c ui.C) {
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
	d := openPageWith(t, ui.PageConfig{Theme: &theme, View: func(c ui.C) {
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
	d := openPage(t, func(c ui.C) {
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
		ui.Column(c).Pad(40).Gap(30).Children(func() {
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
