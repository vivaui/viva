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

// Every control that can fail validation publishes that state on its hit, so
// the runtime can announce it to assistive technology.
func TestInvalidControlsCarryTheSemanticState(t *testing.T) {
	theme := gfx.LightTheme()
	fields := map[string]func(canvas *gfx.Canvas){
		"checkbox": func(canvas *gfx.Canvas) {
			BeginCheckbox(canvas, theme, gfx.Item(1), gfx.Rect{}, false).Invalid().Paint()
		},
		"switch": func(canvas *gfx.Canvas) {
			BeginSwitch(canvas, theme, gfx.Item(1), gfx.Rect{}, false).Invalid().Paint()
		},
		"toggle": func(canvas *gfx.Canvas) {
			BeginToggle(canvas, theme, gfx.Item(1), gfx.Rect{Width: 80, Height: 32}, false).Invalid().Default("Bold")
		},
		"button": func(canvas *gfx.Canvas) {
			BeginButton(canvas, theme, gfx.Item(1), gfx.Rect{Width: 100, Height: 32}).Invalid().Default("Save")
		},
		"radio group": func(canvas *gfx.Canvas) {
			BeginRadioGroup(canvas, theme, gfx.Rect{Width: 160, Height: 48}, []RadioOption{{ID: gfx.Item(1), Label: "One", Invalid: true}}, gfx.ID{}).Paint()
		},
		"select": func(canvas *gfx.Canvas) {
			BeginSelect(canvas, theme, gfx.Rect{Width: 160, Height: 32}, SelectOptions{TriggerID: gfx.Item(1), Label: "Size", Invalid: true})
		},
		"combobox": func(canvas *gfx.Canvas) {
			BeginCombobox(canvas, theme, gfx.Rect{Width: 160, Height: 32}, ComboboxOptions{InputID: gfx.Item(1), Label: "Framework", Invalid: true})
		},
		"command": func(canvas *gfx.Canvas) {
			BeginCommand(canvas, theme, gfx.Rect{Width: 320, Height: 200}, CommandOptions{InputID: gfx.Item(1), Label: "Commands", Invalid: true})
		},
		"input": func(canvas *gfx.Canvas) {
			PaintInput(canvas, theme, gfx.Rect{Width: 160}, InputSnapshot{ID: gfx.Item(1), Label: "Email", Value: "x", Invalid: true})
		},
		"textarea": func(canvas *gfx.Canvas) {
			PaintTextarea(canvas, theme, gfx.Rect{Width: 160, Height: 80}, TextareaSnapshot{ID: gfx.Item(1), Label: "Notes", Value: "x", Invalid: true})
		},
		"input group": func(canvas *gfx.Canvas) {
			PaintInputGroup(canvas, theme, gfx.Rect{Width: 160}, InputGroupSnapshot{ID: gfx.Item(1), Label: "Email", Value: "x", Invalid: true})
		},
		"otp": func(canvas *gfx.Canvas) {
			PaintInputOTP(canvas, theme, gfx.Rect{Width: 160, Height: 48}, InputOTPSnapshot{ID: gfx.Item(1), Label: "Code", Slots: 4, Invalid: true})
		},
		"native select": func(canvas *gfx.Canvas) {
			PaintNativeSelect(canvas, theme, gfx.Rect{Width: 160}, NativeSelectSnapshot{ID: gfx.Item(1), Label: "Size", Invalid: true})
		},
		"questionnaire": func(canvas *gfx.Canvas) {
			questionnaire := BeginQuestionnaire(canvas, theme, gfx.Rect{Width: 300, Height: 200}, QuestionnaireData{ChoiceType: QuestionnaireRadio, Invalid: true})
			questionnaire.AddChoice(QuestionnaireChoice{ID: gfx.Item(1), Label: "One"})
			questionnaire.Input(gfx.Item(2), "", "Something else", false)
		},
	}
	for name, paint := range fields {
		t.Run(name, func(t *testing.T) {
			var canvas gfx.Canvas
			paint(&canvas)
			hits := canvas.Hits()
			if len(hits) == 0 {
				t.Fatal("no hit was declared")
			}
			checked := 0
			for _, hit := range hits {
				if hit.ID.IsZero() {
					continue // structural blocks and regions are not published controls
				}
				checked++
				if !hit.Invalid {
					t.Fatalf("hit %d has no invalid state: %+v", hit.ID, hit)
				}
			}
			if checked == 0 {
				t.Fatal("no identifiable control hit was declared")
			}
		})
	}

	// A FormField hands its invalid state to the control painted into the slot;
	// it draws no control hit itself.
	var canvas gfx.Canvas
	layout := PaintFormField(&canvas, theme, gfx.Rect{Width: 180}, FormFieldSnapshot{Label: "Email", Invalid: true})
	if !layout.Semantics.Invalid {
		t.Fatalf("form semantics = %+v", layout.Semantics)
	}
	PaintNativeSelect(&canvas, theme, layout.Control, NativeSelectSnapshot{ID: gfx.Item(1), Label: "Email", Invalid: layout.Semantics.Invalid})
	if hits := canvas.Hits(); len(hits) != 1 || !hits[0].Invalid {
		t.Fatalf("form control hit = %+v", hits)
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
