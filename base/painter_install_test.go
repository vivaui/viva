package base

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
)

// drawButton declares a button and lays the page out, which is enough to reach
// both the measurer and the painter.
func drawButton() {
	var view View
	view.Draw(testCanvas(), gfx.LightTheme(), gfx.Rect{Width: 200, Height: 80}, func(f *Frame) {
		f.Button(gfx.ID{}, "Save")
	})
}

// A control the styled layer did not install stops the page with the kind's
// name and the one call that fixes it, instead of painting or measuring
// nothing. A host driving package base directly installs every kind with
// ui.InstallPainters; the ui constructors install the kinds a page declares.
func TestUninstalledControlFailsFast(t *testing.T) {
	savedPainter, savedMeasurer := controlPainters[KindButton], controlMeasurers[KindButton]
	t.Cleanup(func() {
		controlPainters[KindButton], controlMeasurers[KindButton] = savedPainter, savedMeasurer
	})

	cases := []struct {
		name  string
		setup func()
		want  string
	}{
		{"measurer", func() { controlMeasurers[KindButton] = nil }, "Button has no measurer installed"},
		{"painter", func() { controlMeasurers[KindButton] = savedMeasurer; controlPainters[KindButton] = nil }, "Button has no painter installed"},
	}
	for _, tc := range cases {
		controlPainters[KindButton], controlMeasurers[KindButton] = savedPainter, savedMeasurer
		tc.setup()
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				got := recover()
				msg, ok := got.(string)
				if !ok {
					t.Fatalf("drawing a %s without its %s recovered %v, want the diagnostic panic", controlName(KindButton), tc.name, got)
				}
				if !strings.Contains(msg, tc.want) || !strings.Contains(msg, "ui.InstallPainters") {
					t.Fatalf("diagnostic = %q, want it to name %q and ui.InstallPainters", msg, tc.want)
				}
			}()
			drawButton()
		})
	}
}

// The engine's own kinds are laid out and painted without an installation:
// a Spacer reserves space and a slot takes its size, so requiring an installer
// for them would break a page that only declares layout.
func TestEngineKindsNeedNoInstallation(t *testing.T) {
	savedPainter, savedMeasurer := controlPainters[KindButton], controlMeasurers[KindButton]
	t.Cleanup(func() {
		controlPainters[KindButton], controlMeasurers[KindButton] = savedPainter, savedMeasurer
	})
	controlPainters[KindButton], controlMeasurers[KindButton] = nil, nil

	var view View
	view.Draw(testCanvas(), gfx.LightTheme(), gfx.Rect{Width: 200, Height: 80}, func(f *Frame) {
		f.Row().Children(func() {
			f.Spacer()
			f.Slot().Size(20, 20)
		})
	})
}

// The kind a diagnostic names reads as the Go name of the control.
func TestControlNameIsReadable(t *testing.T) {
	for kind, want := range map[Kind]string{
		KindButton:      "Button",
		KindInputGroup:  "InputGroup",
		KindSplitButton: "SplitButton",
		KindPagination:  "Pagination",
		KindContainer:   "Container",
	} {
		if got := controlName(kind); got != want {
			t.Errorf("controlName(%v) = %q, want %q", kind, got, want)
		}
	}
}
