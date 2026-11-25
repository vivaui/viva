package ui_test

import (
	"testing"

	"github.com/vivaui/viva/ui"
)

var qualityMarks = []ui.SliderMark{{Value: 0, Label: "Low"}, {Value: 50, Label: "Medium"}, {Value: 100, Label: "High"}}

// The words of the marks are painted under the track, in order across it, and the
// slider is taller by the room they take.
func TestSliderMarksPaintTheirWordsAndMakeRoom(t *testing.T) {
	var s ui.SliderState
	var plain, marked ui.Box
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(8).Gap(8).Children(func() {
			plain = ui.Slider(c, &s).Width(300)
			marked = ui.Slider(c, &s).Width(300).Marks(qualityMarks)
		})
	})
	if marked.Rect().Height <= plain.Rect().Height {
		t.Fatalf("a slider with labelled marks is %v high, not above a plain one (%v)", marked.Rect().Height, plain.Rect().Height)
	}
	// The dots sit on the track at the values they mark: the first where the thumb's
	// center rests at the minimum, the last at the maximum, the middle one half way.
	var centers []float32
	for _, primitive := range d.driver.Snapshot().Primitives {
		if primitive.Radius == 2 && primitive.Bounds.Width == 4 {
			centers = append(centers, primitive.Bounds.X+2)
		}
	}
	r := marked.Rect()
	want := []float32{r.X + 8, r.X + r.Width/2, r.X + r.Width - 8}
	if len(centers) != 3 {
		t.Fatalf("dots = %v, want one for each of the 3 marks", centers)
	}
	for i := range want {
		if centers[i] < want[i]-0.5 || centers[i] > want[i]+0.5 {
			t.Fatalf("dot %d is at %v, want %v", i, centers[i], want[i])
		}
	}
	at := map[string]float32{}
	for _, text := range d.driver.Snapshot().Texts {
		at[text.Value] = text.Bounds.X
	}
	if !(at["Low"] < at["Medium"] && at["Medium"] < at["High"]) {
		t.Fatalf("label origins %v, %v, %v are not in order across the track", at["Low"], at["Medium"], at["High"])
	}
}

// Marks do not move the thumb's travel: a press at the far end still takes the
// maximum.
func TestSliderWithMarksStillTakesThePointer(t *testing.T) {
	var s ui.SliderState
	var box ui.Box
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(8).Children(func() {
			box = ui.Slider(c, &s).Width(300).Marks(qualityMarks).Label("Quality")
		})
	})
	r := box.Rect()
	if err := d.driver.Click(r.X+r.Width-2, r.Y+8); err != nil {
		t.Fatal(err)
	}
	if s.Value < 99 {
		t.Fatalf("value = %v after a press at the far end of a marked slider, want 100", s.Value)
	}
}

func TestMarkedSliderRepaintsWithoutAllocating(t *testing.T) {
	var s ui.SliderState
	d := openPage(t, func(c ui.C) { ui.Slider(c, &s).Width(300).Marks(qualityMarks) })
	d.repaint()
	d.repaint()
	if allocs := testing.AllocsPerRun(100, d.repaint); allocs != 0 {
		t.Fatalf("%v allocations a repaint", allocs)
	}
}
