package ui_test

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/style"
	"github.com/vivaui/viva/ui"
)

var checkValue, switchValue bool

// The calls a page makes on a Checkbox style its caption and box.
func TestCheckboxPagePropertiesReachTheCaptionAndBox(t *testing.T) {
	checkValue = true
	ink, fill := gfx.Color{Red: 1, Alpha: 1}, gfx.Color{Blue: 1, Alpha: 1}
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(20).Children(func() {
			ui.Checkbox(c, &checkValue, "Sync").FontSize(18).Weight(gfx.WeightBold).Color(ink).Background(fill)
		})
	})
	snapshot := d.driver.Snapshot()
	var caption gfx.Text
	for _, text := range snapshot.Texts {
		if text.Value == "Sync" {
			caption = text
		}
	}
	if caption.Value != "Sync" {
		t.Fatalf("the caption was not painted: %+v", snapshot.Texts)
	}
	if caption.Style.Size != 18 || caption.Style.Weight != gfx.WeightBold || caption.Style.Color != ink {
		t.Errorf("caption style = %+v, want 18, bold, red", caption.Style)
	}
	found := false
	for _, primitive := range snapshot.Primitives {
		if primitive.Kind == app.RectanglePrimitive && primitive.Color == fill {
			found = true
		}
	}
	if !found {
		t.Errorf("no checkbox fill %v in %+v", fill, snapshot.Primitives)
	}
	marked := false
	for _, primitive := range snapshot.Primitives {
		if primitive.Kind == app.TrianglePrimitive {
			marked = true
		}
	}
	if !marked {
		t.Error("the checkbox painted no mark")
	}
	checkHit := false
	for _, hit := range snapshot.Hits {
		if hit.Role == gfx.RoleCheckbox {
			checkHit = true
			if hit.Label != "Sync" {
				t.Errorf("checkbox hit label = %q, want the caption", hit.Label)
			}
		}
	}
	if !checkHit {
		t.Errorf("no checkbox hit: %+v", snapshot.Hits)
	}
}

// A Switch's track takes an explicit height and its caption the element's style.
func TestSwitchPagePropertiesReachTheTrackAndCaption(t *testing.T) {
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(20).Children(func() {
			ui.Switch(c, &switchValue, "Running").Height(40).FontSize(18).Weight(gfx.WeightBold)
		})
	})
	snapshot := d.driver.Snapshot()
	var caption gfx.Text
	for _, text := range snapshot.Texts {
		if text.Value == "Running" {
			caption = text
		}
	}
	if caption.Value != "Running" {
		t.Fatalf("the caption was not painted: %+v", snapshot.Texts)
	}
	if caption.Style.Size != 18 || caption.Style.Weight != gfx.WeightBold {
		t.Errorf("caption style = %+v, want 18, bold", caption.Style)
	}
	track := gfx.Rect{}
	found := false
	for _, primitive := range snapshot.Primitives {
		if primitive.Kind == app.RectanglePrimitive && primitive.Bounds.Height == 40 && primitive.Radius == 20 {
			track, found = primitive.Bounds, true
		}
	}
	if !found {
		t.Fatalf("no 40-tall pill track: %+v", snapshot.Primitives)
	}
	var hit gfx.Hit
	for _, h := range snapshot.Hits {
		if h.Role == gfx.RoleSwitch {
			hit = h
		}
	}
	if hit.Role != gfx.RoleSwitch {
		t.Fatalf("no switch hit: %+v", snapshot.Hits)
	}
	if hit.Bounds.X > track.X || hit.Bounds.Y > track.Y {
		t.Errorf("switch hit %v does not reach the track %v", hit.Bounds, track)
	}
}

// A page's Styles.Checkbox and Styles.Switch dress the controls.
func TestPageStylesDressCheckboxesAndSwitches(t *testing.T) {
	fill := gfx.Color{Red: 0.2, Green: 0.6, Blue: 0.3, Alpha: 1}
	track := gfx.Color{Red: 0.7, Green: 0.1, Blue: 0.1, Alpha: 1}
	styles := &ui.Styles{
		Checkbox: func(gfx.Theme, gfx.State) style.CheckboxLook {
			return style.CheckboxLook{Fill: fill, Radius: 3}
		},
		Switch: func(gfx.Theme, gfx.State, float32) style.SwitchLook {
			return style.SwitchLook{Track: track}
		},
	}
	d := openPageWith(t, ui.PageConfig{
		View: func(c ui.C) {
			ui.Column(c).Pad(20).Children(func() {
				ui.Checkbox(c, &checkValue, "Sync")
				ui.Switch(c, &switchValue, "Run")
			})
		},
		Styles: styles,
	}, app.Config{})
	snapshot := d.driver.Snapshot()
	checkboxFill, switchTrack := false, false
	for _, primitive := range snapshot.Primitives {
		if primitive.Kind != app.RectanglePrimitive {
			continue
		}
		if primitive.Color == fill {
			checkboxFill = true
		}
		if primitive.Color == track {
			switchTrack = true
		}
	}
	if !checkboxFill {
		t.Errorf("no checkbox fill %v from the page recipe", fill)
	}
	if !switchTrack {
		t.Errorf("no switch track %v from the page recipe", track)
	}
}

// A steady page of checkboxes and switches allocates nothing.
func TestPageOfChoicesAllocatesNothing(t *testing.T) {
	page, canvas := ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas()
	for range 3 {
		drawPage(page, canvas, denseChoices)
	}
	if allocs := testing.AllocsPerRun(50, func() { drawPage(page, canvas, denseChoices) }); allocs != 0 {
		t.Fatalf("a steady page of choices allocates %v times", allocs)
	}
}

// BenchmarkPageChoices is the page-level cost of checkboxes and switches, for
// the stage-2 regression budget.
func BenchmarkPageChoices(b *testing.B) {
	page, canvas := ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas()
	b.ReportAllocs()
	for b.Loop() {
		drawPage(page, canvas, denseChoices)
	}
}

var choiceValues [16]bool

// denseChoices is a settings page of checkboxes and switches with captions.
func denseChoices(c ui.C) {
	ui.Column(c).Fill().Pad(12).Gap(6).Children(func() {
		for row := 0; row < 8; row++ {
			ui.Row(c).FillWidth().Gap(12).Children(func() {
				ui.Checkbox(c, &choiceValues[row*2], "Sync over SSH")
				ui.Switch(c, &choiceValues[row*2+1], "Running")
			})
		}
	})
}
