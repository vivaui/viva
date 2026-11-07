//go:build darwin || windows || linux

package app

import (
	"testing"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/ui"
)

var (
	labelTree      = ui.Tree{}
	labelPage      = labelTree.Feature("label")
	labelSwitch    = labelPage.ID(labelTree.Name("switch"))
	labelCheckbox  = labelPage.ID(labelTree.Name("checkbox"))
	labelOff       = labelPage.ID(labelTree.Name("off"))
	labelSwitchRow = ui.Rect{X: 10, Y: 10, Width: 200, Height: 20}
	labelBoxRow    = ui.Rect{X: 10, Y: 50, Width: 200, Height: 20}
	labelOffRow    = ui.Rect{X: 10, Y: 90, Width: 200, Height: 20}
)

// labelApp is a form whose switch and checkbox have a label beside them, as
// `<label for>` has, and a third that is disabled.
type labelApp struct {
	switched, checked, off bool
	clicks                 int
}

func (a *labelApp) config() Config {
	theme := ui.LightTheme()
	return Config{
		Title: "label", Width: 400, Height: 200, Tree: &labelTree,
		Paint: func(canvas *ui.Canvas, size ui.Size) {
			components.FieldTitle(canvas, theme, labelSwitchRow, "Notifications", labelSwitch, false)
			if components.BeginSwitch(canvas, theme, labelSwitch, ui.Rect{X: 300, Y: 10, Width: 32, Height: 18}, a.switched).Paint().Clicked() {
				a.switched = !a.switched
				a.clicks++
			}
			components.FieldTitle(canvas, theme, labelBoxRow, "Accept terms", labelCheckbox, false)
			if components.BeginCheckbox(canvas, theme, labelCheckbox, ui.Rect{X: 300, Y: 50}, a.checked).Label("terms").Paint().Clicked() {
				a.checked = !a.checked
				a.clicks++
			}
			components.FieldTitle(canvas, theme, labelOffRow, "Unavailable", labelOff, true)
			if components.BeginSwitch(canvas, theme, labelOff, ui.Rect{X: 300, Y: 90, Width: 32, Height: 18}, a.off).Disabled().Paint().Clicked() {
				a.off = !a.off
			}
		},
	}
}

func TestPressingALabelFocusesAndClicksItsControl(t *testing.T) {
	a := &labelApp{}
	var canvas ui.Canvas
	driver, err := NewDriver(a.config(), &canvas)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	if err := driver.Click(30, 20); err != nil { // the words "Notifications"
		t.Fatal(err)
	}
	if !a.switched || a.clicks != 1 {
		t.Fatalf("after pressing the label: switched=%v clicks=%d, want the switch on after one click", a.switched, a.clicks)
	}
	if focused := driver.painter.focus.Focused(); focused != labelSwitch {
		t.Fatalf("focus is %v, want the switch", focused)
	}
	if err := driver.Click(30, 60); err != nil {
		t.Fatal(err)
	}
	if !a.checked || driver.painter.focus.Focused() != labelCheckbox {
		t.Fatalf("after pressing the second label: checked=%v focus=%v", a.checked, driver.painter.focus.Focused())
	}
	// Pressing past the words, on the rest of the row, is not pressing the label.
	if err := driver.Click(180, 60); err != nil {
		t.Fatal(err)
	}
	if !a.checked {
		t.Fatal("empty space beside the label toggled the checkbox")
	}
	// A disabled control's label does nothing.
	if err := driver.Click(30, 100); err != nil {
		t.Fatal(err)
	}
	if a.off || driver.painter.focus.Focused() == labelOff {
		t.Fatalf("a disabled control's label acted: off=%v focus=%v", a.off, driver.painter.focus.Focused())
	}
	// The keyboard visits each control once, not once more for its label.
	for _, want := range []ui.ID{labelSwitch, labelCheckbox, labelSwitch} {
		if err := driver.Key(ui.KeyTab, 0); err != nil {
			t.Fatal(err)
		}
		if driver.painter.focus.Focused() != want {
			t.Fatalf("Tab reached %v, want %v", driver.painter.focus.Focused(), want)
		}
	}
}

func TestALabelNamesItsControlForAssistiveTechnology(t *testing.T) {
	a := &labelApp{}
	var canvas ui.Canvas
	driver, err := NewDriver(a.config(), &canvas)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	var named []string
	for _, node := range driver.painter.accessibility {
		named = append(named, node.Label)
	}
	want := []string{"Notifications", "terms", "Unavailable"}
	if len(named) != 3 || named[0] != want[0] || named[2] != want[2] {
		t.Fatalf("accessible nodes %q: a label is no node of its own and names the control it labels", named)
	}
}
