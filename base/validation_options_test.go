package base_test

import (
	"testing"

	"github.com/vivaui/viva/base"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/style"
)

var (
	validationCheckbox = tree.ID(page, "validation-checkbox")
	validationToggle   = tree.ID(page, "validation-toggle")
	validationOption   = tree.Sub(page, tree.Name("validation-option"))
)

// Invalid marks a Checkbox, Toggle and Select as failing validation, and only
// those controls: the flag is element state the painter reads.
func TestInvalidMarksValidationControls(t *testing.T) {
	var view base.View
	canvas := testCanvas()
	var checkbox, cleared, toggle, selectBox, text base.Box
	view.Draw(canvas, gfx.LightTheme(), gfx.Rect{Width: 400, Height: 200}, func(f *base.Frame) {
		f.Column().Children(func() {
			checkbox = f.Checkbox(validationCheckbox, false).Invalid(true)
			cleared = f.Checkbox(validationCheckbox, false).Invalid(true).Invalid(false)
			toggle = f.Toggle(validationToggle, "Bold", false).Invalid(true)
			selectBox = f.SelectBase(page, new(base.SelectState), []string{"A"}, -1).Invalid(true)
			text = f.Text("note").Invalid(true)
		})
	})
	for name, box := range map[string]base.Box{
		"checkbox": checkbox, "toggle": toggle, "select": selectBox,
	} {
		if box.N().Flags&base.FlagInvalid == 0 {
			t.Errorf("%s did not take the invalid flag", name)
		}
	}
	if cleared.N().Flags&base.FlagInvalid != 0 {
		t.Error("Invalid(false) did not clear the invalid flag")
	}
	if text.N().Flags&base.FlagInvalid != 0 {
		t.Error("an element that is not a validation control took the invalid flag")
	}
}

// DisabledOptions marks a segmented control's options without touching the
// caller's slice.
func TestSegmentedDisabledOptionsMarksThePageOptions(t *testing.T) {
	scope := tree.Sub(page, tree.Name("validation-segmented"))
	var view base.View
	canvas := testCanvas()
	segDisabled := []bool{false, true, false}
	view.Draw(canvas, gfx.LightTheme(), gfx.Rect{Width: 300, Height: 80}, func(f *base.Frame) {
		choice := f.Choice(scope, []string{"A", "B", "C"}, 0).DisabledOptions(segDisabled)
		widget := base.WidgetsOf(f).GetSegmented()[choice.N().Aux]
		for index, option := range widget.GetOptions() {
			if got, want := option.Disabled, index == 1; got != want {
				t.Errorf("option %d Disabled = %v, want %v", index, got, want)
			}
		}
	})

	caller := []style.SegmentOption{
		{ID: validationOption.At(0), Label: "A"},
		{ID: validationOption.At(1), Label: "B"},
	}
	canvas.Reset()
	view.Draw(canvas, gfx.LightTheme(), gfx.Rect{Width: 300, Height: 80}, func(f *base.Frame) {
		choice := f.Segmented(caller, validationOption.At(0)).DisabledOptions([]bool{true, false})
		widget := base.WidgetsOf(f).GetSegmented()[choice.N().Aux]
		if options := widget.GetOptions(); !options[0].Disabled || options[1].Disabled {
			t.Errorf("segmented options = %+v", options)
		}
	})
	if caller[0].Disabled || caller[1].Disabled {
		t.Errorf("the caller's options were changed: %+v", caller)
	}
}

// The chained options cost nothing once the page is warm.
func TestValidationOptionsDeclareWithoutAllocating(t *testing.T) {
	var view base.View
	canvas := testCanvas()
	theme := gfx.LightTheme()
	labels := []string{"A", "B", "C"}
	disabled := []bool{false, true, false}
	scope := tree.Sub(page, tree.Name("validation-alloc"))
	state := new(base.SelectState)
	draw := func() {
		canvas.Reset()
		view.Draw(canvas, theme, gfx.Rect{Width: 400, Height: 200}, func(f *base.Frame) {
			f.Column().Children(func() {
				f.Checkbox(validationCheckbox, false).Invalid(true)
				f.Toggle(validationToggle, "Bold", false).Invalid(true)
				f.SelectBase(page, state, labels, -1).Invalid(true)
				f.Choice(scope, labels, 0).DisabledOptions(disabled)
			})
		})
	}
	draw()
	if allocations := testing.AllocsPerRun(50, draw); allocations != 0 {
		t.Fatalf("a warm paint with the validation options allocated %v times", allocations)
	}
}
