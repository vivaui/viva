package components

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// The text-entry family shares one contract: focus is the border alone, in the
// accent color, and an invalid field's border is destructive and wins over
// focus. There is no halo around the field, and border and fill stay opaque so
// they never tint its interior.
func TestTextEntryFamilySharesOneFocusAndInvalidContract(t *testing.T) {
	bounds := gfx.Rect{Width: 160}
	fields := map[string]func(canvas *gfx.Canvas, theme gfx.Theme, state InputSnapshot){
		"input": func(canvas *gfx.Canvas, theme gfx.Theme, state InputSnapshot) {
			PaintInput(canvas, theme, bounds, InputSnapshot{ID: gfx.Item(1), Label: "x", Value: "v", Focused: state.Focused, Invalid: state.Invalid})
		},
		"textarea": func(canvas *gfx.Canvas, theme gfx.Theme, state InputSnapshot) {
			PaintTextarea(canvas, theme, gfx.Rect{Width: 160, Height: 80}, TextareaSnapshot{ID: gfx.Item(1), Label: "x", Value: "v", Focused: state.Focused, Invalid: state.Invalid})
		},
		"input group": func(canvas *gfx.Canvas, theme gfx.Theme, state InputSnapshot) {
			PaintInputGroup(canvas, theme, bounds, InputGroupSnapshot{ID: gfx.Item(1), Label: "x", Value: "v", Focused: state.Focused, Invalid: state.Invalid})
		},
		"native select": func(canvas *gfx.Canvas, theme gfx.Theme, state InputSnapshot) {
			PaintNativeSelect(canvas, theme, bounds, NativeSelectSnapshot{ID: gfx.Item(1), Label: "x", Focused: state.Focused, Invalid: state.Invalid})
		},
		"select": func(canvas *gfx.Canvas, theme gfx.Theme, state InputSnapshot) {
			if state.Focused {
				canvas.SetInteraction(gfx.ID{}, gfx.ID{}, gfx.Item(1), true)
			}
			BeginSelect(canvas, theme, bounds, SelectOptions{TriggerID: gfx.Item(1), Label: "x", Invalid: state.Invalid})
		},
		"combobox": func(canvas *gfx.Canvas, theme gfx.Theme, state InputSnapshot) {
			if state.Focused {
				canvas.SetInteraction(gfx.ID{}, gfx.ID{}, gfx.Item(1), true)
			}
			BeginCombobox(canvas, theme, bounds, ComboboxOptions{InputID: gfx.Item(1), Label: "x", Invalid: state.Invalid})
		},
	}
	for name, paint := range fields {
		for themeName, theme := range map[string]gfx.Theme{"dark": gfx.DarkTheme(), "light": gfx.LightTheme()} {
			t.Run(name+"/"+themeName, func(t *testing.T) {
				var idle, focused, invalid gfx.Canvas
				paint(&idle, theme, InputSnapshot{})
				paint(&focused, theme, InputSnapshot{Focused: true})
				paint(&invalid, theme, InputSnapshot{Focused: true, Invalid: true})

				if len(focused.Rectangles()) != len(idle.Rectangles()) || len(invalid.Rectangles()) != len(idle.Rectangles()) {
					t.Fatalf("focus or invalid changed the shapes painted (%d, %d, was %d): the state is the border alone",
						len(focused.Rectangles()), len(invalid.Rectangles()), len(idle.Rectangles()))
				}
				border, fill := focused.Rectangles()[0], focused.Rectangles()[1]
				if border.Color != theme.Primary {
					t.Fatalf("focused border = %+v, want the accent color %+v", border.Color, theme.Primary)
				}
				if fill != idle.Rectangles()[1] {
					t.Fatalf("focus changed the fill: %+v, was %+v", fill, idle.Rectangles()[1])
				}
				if border.Color.Alpha != 1 || fill.Color.Alpha != 1 {
					t.Fatalf("border alpha %v, fill alpha %v; both must be opaque or they tint the interior", border.Color.Alpha, fill.Color.Alpha)
				}
				if got := invalid.Rectangles()[0].Color; got != theme.Danger {
					t.Fatalf("invalid border = %+v, want the destructive color %+v, which wins over focus", got, theme.Danger)
				}
			})
		}
	}
}

// A disabled field cannot take focus, so it never gets the focus halo, and it
// fades as a whole (opacity-50), border and fill included.
func TestTextEntryDisabledIsFadedAndNeverRinged(t *testing.T) {
	theme := gfx.DarkTheme()
	var enabled, disabled gfx.Canvas
	PaintInput(&enabled, theme, gfx.Rect{Width: 160}, InputSnapshot{ID: gfx.Item(1), Label: "x", Value: "v"})
	PaintInput(&disabled, theme, gfx.Rect{Width: 160}, InputSnapshot{ID: gfx.Item(1), Label: "x", Value: "v", Focused: true, Disabled: true})
	if len(disabled.Rectangles()) != len(enabled.Rectangles()) {
		t.Fatalf("a disabled field painted %d rectangles, an enabled one %d: it must have no halo", len(disabled.Rectangles()), len(enabled.Rectangles()))
	}
	if disabled.Rectangles()[0].Color == enabled.Rectangles()[0].Color {
		t.Fatal("a disabled field's border did not fade")
	}
}
