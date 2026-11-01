package components

import (
	"slices"
	"testing"

	"github.com/vivaui/viva/ui"
)

var splitButtonTestOptions = SplitButtonOptions{
	PrimaryID: ui.Item(71), MenuID: ui.Item(72),
	Label: "Save", MenuLabel: "More save options",
	Hint: "Save the file", MenuHint: "Choose another save action",
	Size: ButtonSizeSmall, TriggerWidth: 32, Open: true,
}

func TestSplitButtonPaintsConnectedIndependentActions(t *testing.T) {
	theme := ui.LightTheme()
	var canvas ui.Canvas
	result := BeginSplitButton(&canvas, theme, ui.Rect{Width: 112, Height: 32}, splitButtonTestOptions).Paint()
	geometry := BeginSplitButton(nil, theme, ui.Rect{Width: 112, Height: 32}, splitButtonTestOptions).Geometry()
	if geometry.Primary != (ui.Rect{Width: 80, Height: 32}) ||
		geometry.Menu != (ui.Rect{X: 80, Width: 32, Height: 32}) {
		t.Fatalf("split geometry = %+v", geometry)
	}
	if len(canvas.Hits()) != 2 {
		t.Fatalf("split hits = %+v", canvas.Hits())
	}
	primary, menu := canvas.Hits()[0], canvas.Hits()[1]
	if primary.ID != splitButtonTestOptions.PrimaryID || primary.Label != "Save" || primary.Hint != splitButtonTestOptions.Hint {
		t.Fatalf("primary hit = %+v", primary)
	}
	if menu.ID != splitButtonTestOptions.MenuID || menu.Label != splitButtonTestOptions.MenuLabel ||
		!menu.Expanded || !menu.Expandable || menu.Hint != splitButtonTestOptions.MenuHint {
		t.Fatalf("menu hit = %+v", menu)
	}
	if primary.Bounds.X+primary.Bounds.Width != menu.Bounds.X {
		t.Fatalf("halves are not connected: primary=%+v menu=%+v", primary.Bounds, menu.Bounds)
	}
	if !hasRectangleColor(&canvas, theme.Primary) {
		t.Fatal("split button did not use the primary button variant")
	}
	_ = result
}

func TestSplitButtonReportsEachActionAndDisabledState(t *testing.T) {
	for _, test := range []struct {
		name         string
		target       ui.ID
		wantPrimary  bool
		wantMenu     bool
		disabled     bool
		hitDisabled  bool
		expectedHits int
	}{
		{name: "primary", target: splitButtonTestOptions.PrimaryID, wantPrimary: true, expectedHits: 2},
		{name: "menu", target: splitButtonTestOptions.MenuID, wantMenu: true, expectedHits: 2},
		{name: "disabled", disabled: true, hitDisabled: true, expectedHits: 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			options := splitButtonTestOptions
			options.Disabled = test.disabled
			var canvas ui.Canvas
			canvas.BeginEventPass(test.target)
			result := BeginSplitButton(&canvas, ui.LightTheme(), ui.Rect{Width: 112, Height: 32}, options).Paint()
			primary, menu := result.Clicked()
			canvas.EndEventPass()
			if primary != test.wantPrimary || menu != test.wantMenu {
				t.Fatalf("Clicked() = (%t, %t)", primary, menu)
			}
			if len(canvas.Hits()) != test.expectedHits || canvas.Hits()[0].Disabled != test.hitDisabled ||
				canvas.Hits()[1].Disabled != test.hitDisabled {
				t.Fatalf("disabled split hits = %+v", canvas.Hits())
			}
		})
	}
}

func TestSplitButtonOutlineHasOneOuterBorderAndStablePaint(t *testing.T) {
	options := splitButtonTestOptions
	options.Variant = ButtonOutline
	theme := ui.DarkTheme()
	var canvas ui.Canvas
	paint := func() {
		canvas.Reset()
		BeginSplitButton(&canvas, theme, ui.Rect{X: 12, Y: 15, Width: 132, Height: 32}, options).Paint()
	}
	paint()
	rectangles := slices.Clone(canvas.Rectangles())
	texts := slices.Clone(canvas.Texts())
	hits := slices.Clone(canvas.Hits())
	border := rectangles[0]
	if border.Bounds != (ui.Rect{X: 12, Y: 15, Width: 132, Height: 32}) || border.Radius == 0 {
		t.Fatalf("outer border = %+v", border)
	}
	paint()
	if !slices.Equal(rectangles, canvas.Rectangles()) ||
		!slices.Equal(texts, canvas.Texts()) ||
		!slices.Equal(hits, canvas.Hits()) {
		t.Fatal("identical split buttons produced different command streams")
	}
	if allocations := testing.AllocsPerRun(100, paint); allocations != 0 {
		t.Fatalf("split button paint allocations = %v", allocations)
	}
}
