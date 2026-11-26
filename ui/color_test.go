package ui_test

import (
	"testing"

	"github.com/vivaui/viva/base"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

func TestColorSwatchChoosesByClick(t *testing.T) {
	red, blue := gfx.RGB(220, 38, 38), gfx.RGB(37, 99, 235)
	chosen := red
	d := openPage(t, func(c ui.C) {
		ui.Row(c).Gap(4).Children(func() {
			for _, s := range []struct {
				Name string
				col  gfx.Color
			}{{"Red", red}, {"Blue", blue}} {
				if ui.ColorSwatch(c.Key(s.Name), s.col, chosen == s.col, s.Name).Clicked() {
					chosen = s.col
				}
			}
		})
	})
	d.click("Blue")
	if chosen != blue {
		t.Fatalf("a click chose %v", chosen)
	}
	n := 0
	for _, h := range d.driver.Snapshot().Hits {
		if h.Label == "Blue" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("swatch declares %d hits, want 1", n)
	}
}

func pickerPage(t *testing.T, state *ui.ColorPickerState, Color *gfx.Color) *pageDriver {
	return openPage(t, func(c ui.C) {
		ui.Column(c).Pad(16).Children(func() { ui.ColorPicker(c, state, Color, "Accent") })
	})
}

func TestColorPickerAppliesATypedHex(t *testing.T) {
	var state ui.ColorPickerState
	color := gfx.RGB(10, 20, 30)
	d := pickerPage(t, &state, &color)
	d.click("Accent")
	if !state.Open() {
		t.Fatal("the trigger did not open the panel")
	}
	d.click("Hex color")
	state.SetHexText("#FF0000")
	d.click("OK")
	if state.Open() || !state.Applied() {
		t.Fatalf("Apply left open=%v", state.Open())
	}
	if color != gfx.RGB(255, 0, 0) {
		t.Fatalf("applied %v, want red", color)
	}
}

func TestColorPickerInvalidHexUsesPanelLabel(t *testing.T) {
	for _, tc := range []struct{ name, label, want string }{
		{"localized", "颜色格式无效", "颜色格式无效"},
		{"default", "", "Invalid color"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var state ui.ColorPickerState
			original := gfx.RGB(10, 20, 30)
			color := original
			d := openPage(t, func(c ui.C) {
				ui.Column(c).Pad(16).Children(func() {
					ui.ColorPicker(c, &state, &color, "Accent").PanelLabels(base.ColorPickerLabels{InvalidColor: tc.label})
				})
			})
			d.click("Accent")
			d.click("Hex color")
			state.SetHexText("#wrong")
			d.click("OK")
			if !state.Open() || state.Applied() || color != original {
				t.Fatal("invalid input changed or applied the color")
			}
			found := false
			for _, label := range d.driver.Snapshot().Labels {
				if label.Value == tc.want {
					found = true
				}
			}
			if !found {
				t.Fatalf("panel did not paint validation message %q", tc.want)
			}
		})
	}
}

func TestColorPickerCancelAndEscapeKeepTheColor(t *testing.T) {
	var state ui.ColorPickerState
	color := gfx.RGB(10, 20, 30)
	d := pickerPage(t, &state, &color)
	d.click("Accent")
	d.click("Cancel")
	if state.Open() || color != gfx.RGB(10, 20, 30) {
		t.Fatalf("Cancel: open=%v color=%v", state.Open(), color)
	}
	d.click("Accent")
	d.key(gfx.KeyEscape)
	if state.Open() || color != gfx.RGB(10, 20, 30) {
		t.Fatalf("Escape: open=%v color=%v", state.Open(), color)
	}
	d.click("Accent")
	d.click("Clear")
	if color.Alpha != 0 || !state.Applied() {
		t.Fatalf("Clear left %v", color)
	}
}

func TestColorPickerPlaneAndHueMoveTheStagedColor(t *testing.T) {
	var state ui.ColorPickerState
	color := gfx.RGB(255, 0, 0)
	d := pickerPage(t, &state, &color)
	d.click("Accent")
	plane := d.hit("Saturation and brightness").Bounds
	if err := d.driver.Click(plane.X+plane.Width*0.5, plane.Y+plane.Height*0.5); err != nil {
		t.Fatal(err)
	}
	d.key(gfx.KeyRight)
	d.click("OK")
	if color == gfx.RGB(255, 0, 0) {
		t.Fatal("a press on the plane and an arrow key did not change the staged color")
	}
}

func TestColorPickerDraftFollowsTheDragBeforeApply(t *testing.T) {
	var state ui.ColorPickerState
	color := gfx.RGB(255, 0, 0)
	d := pickerPage(t, &state, &color)
	if _, ok := state.Draft(); ok {
		t.Fatal("a closed picker has a draft")
	}
	d.click("Accent")
	plane := d.hit("Saturation and brightness").Bounds
	if err := d.driver.Click(plane.X+plane.Width*0.5, plane.Y+plane.Height*0.5); err != nil {
		t.Fatal(err)
	}
	draft, ok := state.Draft()
	if !ok || draft == gfx.RGB(255, 0, 0) {
		t.Fatalf("draft = %v, %v after a press on the plane", draft, ok)
	}
	if color != gfx.RGB(255, 0, 0) {
		t.Fatal("the application's color changed before Apply")
	}
}

// A ColorPicker given WithID names the trigger with the application's identity, so a row's
// words can be its label: clicking them opens the panel.
func TestColorPickerKeepsTheApplicationID(t *testing.T) {
	var state ui.ColorPickerState
	color := gfx.RGB(10, 20, 30)
	var id gfx.ID
	d := openPage(t, func(c ui.C) {
		if id.IsZero() {
			id = ui.ID(c)
		}
		ui.Row(c).Children(func() {
			words := ui.Text(c, "Accent").LabelFor(id)
			_ = words
			ui.ColorPicker(c, &state, &color, "Accent", ui.WithID(id))
		})
	})
	var trigger gfx.Rect
	for _, hit := range d.driver.Snapshot().Hits {
		if hit.ID == id {
			trigger = hit.Bounds
		}
	}
	if trigger.Width == 0 {
		t.Fatal("the trigger did not take the application identity")
	}
	if err := d.driver.Click(center(trigger)); err != nil {
		t.Fatal(err)
	}
	if !state.Open() {
		t.Fatal("a click on the named trigger did not open the panel")
	}
}

// A swatch paints from what its element carries, so a warm paint allocates
// nothing: a Draw callback made for each swatch would make one closure a paint.
func TestColorSwatchPaintsWithoutAllocating(t *testing.T) {
	red, blue := gfx.RGB(220, 38, 38), gfx.RGB(37, 99, 235)
	chosen := red
	d := openPage(t, func(c ui.C) {
		ui.Row(c).Gap(4).Children(func() {
			for _, s := range []struct {
				Name string
				col  gfx.Color
			}{{"Red", red}, {"Blue", blue}, {"Blue2", blue}, {"Red2", red}, {"Red3", red}, {"Blue3", blue}, {"Red4", red}} {
				ui.ColorSwatch(c.Key(s.Name), s.col, chosen == s.col, s.Name)
			}
		})
	})
	d.repaint()
	if allocs := testing.AllocsPerRun(100, d.repaint); allocs != 0 {
		t.Fatalf("a warm swatch row paints with %v allocations, want 0", allocs)
	}
}

// An open picker paints from the frame's own scratch: a context and a draw
// context made at the call would escape through the painter and allocate once
// per picker in each paint.
func TestOpenColorPickerPaintsWithoutAllocating(t *testing.T) {
	var state ui.ColorPickerState
	color := gfx.RGB(10, 20, 30)
	d := pickerPage(t, &state, &color)
	d.click("Accent")
	if !state.Open() {
		t.Fatal("the trigger did not open the panel")
	}
	d.repaint()
	if allocs := testing.AllocsPerRun(100, d.repaint); allocs != 0 {
		t.Fatalf("a warm open picker paints with %v allocations, want 0", allocs)
	}
}

func TestColorPickerLookMethodsApplyToThePickerAndNotToOtherElements(t *testing.T) {
	var state ui.ColorPickerState
	color := gfx.RGB(10, 20, 30)
	pipette := gfx.Icon{ViewBox: 24, Stroke: 1.5}
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(8).Gap(4).Children(func() {
			ui.ColorPicker(c, &state, &color, "Accent").RoundTrigger().TriggerIcon(&pipette).NoColor(false)
			ui.Text(c, "plain").RoundTrigger().NoColor(true) // ignored, as documented
		})
	})
	d.click("Accent")
	if !state.Open() {
		t.Fatal("a picker given look methods did not open")
	}
}

// A swatch is named by the application with Hit, and declares one hit, not two.
func TestColorSwatchTakesTheApplicationsIDWithHit(t *testing.T) {
	id := gfx.Item(77)
	d := openPage(t, func(c ui.C) {
		ui.ColorSwatch(c, gfx.RGB(1, 2, 3), true, "Teal").Hit(id, "Teal")
	})
	n := 0
	for _, h := range d.driver.Snapshot().Hits {
		if h.ID == id {
			n++
			if h.Role != gfx.RoleRadio {
				t.Fatalf("the named swatch is a %v, want a radio", h.Role)
			}
		}
	}
	if n != 1 {
		t.Fatalf("the swatch declares %d hits under its id, want 1", n)
	}
}

// A picker declared inside a Dialog keeps its panel over the dialog: the panel
// is on the layer the picker was declared on, not on the page's layer that the
// dialog's scrim covers, so its controls stay clickable while it is open.
func TestColorPickerPanelStandsAboveItsDialog(t *testing.T) {
	var state ui.ColorPickerState
	color := gfx.RGB(10, 20, 30)
	const dialogLayer = 600
	d := openPage(t, func(c ui.C) {
		dialog := ui.Dialog(c, ui.DialogSpec{Title: "Pick a color", Width: 480, Layer: dialogLayer})
		dialog.Body(func() {
			ui.Column(c).Pad(16).Children(func() {
				ui.ColorPicker(c, &state, &color, "Accent")
			})
		})
	})
	d.click("Accent")
	if !state.Open() {
		t.Fatal("the trigger did not open the panel")
	}
	if panel := d.hit("OK"); panel.Layer <= dialogLayer {
		t.Fatalf("the panel's controls paint at layer %d, over the dialog's %d", panel.Layer, dialogLayer)
	}
	d.click("Hex color")
	state.SetHexText("#FF0000")
	d.click("OK")
	if state.Open() || color != gfx.RGB(255, 0, 0) {
		t.Fatalf("open=%v color=%v, want the panel's Apply to reach the picker", state.Open(), color)
	}
}
