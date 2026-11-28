package ui_test

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"testing"
)

func TestColorPickerLiveEditsBoundDraft(t *testing.T) {
	var state ui.ColorPickerState
	color := gfx.RGB(255, 0, 0)
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(16).Children(func() {
			ui.ColorPicker(c, &state, &color, "Accent").Live(true)
		})
	})
	d.click("Accent")
	for _, hit := range d.driver.Snapshot().Hits {
		if hit.Label == "OK" || hit.Label == "Cancel" || hit.Label == "Clear" {
			t.Fatalf("live picker exposes confirmation action %q", hit.Label)
		}
	}
	d.click("Saturation and brightness")
	if color == gfx.RGB(255, 0, 0) || !state.Applied() || !state.Open() {
		t.Fatal("plane did not update the bound draft while keeping the picker open")
	}
	before := color
	d.key(gfx.KeyRight)
	if color == before || !state.Applied() {
		t.Fatal("keyboard adjustment did not update the draft")
	}
	d.click("Hex color")
	state.SetHexText("#123456")
	d.key(gfx.KeyEnter)
	if uint8(color.Red*255+.5) != 18 || uint8(color.Green*255+.5) != 52 || uint8(color.Blue*255+.5) != 86 || state.Open() {
		t.Fatalf("hex edit = %v, open = %v", color, state.Open())
	}
	d.click("Accent")
	d.click("Hue")
	before = color
	d.key(gfx.KeyEscape)
	if state.Open() || color != before {
		t.Fatal("Escape did not retain the live edit")
	}
}
