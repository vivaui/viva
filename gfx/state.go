package gfx

// State is how a control stands this frame: what the pointer and the keyboard do to
// it, and what the application has declared about it. It is the one description of
// that for every control, whatever draws it: a component, a ui element's look, or
// the callback of a custom drawing.
//
// The first four fields are the canvas's: they come from the control's identity and
// change without the application doing anything. The rest are the application's own
// model of the control and are never read from the canvas; a caller that draws a
// checked box says so by setting Checked, and WithDeclared is how a frame adds them.
type State struct {
	// Hovered is the pointer over the control.
	Hovered bool
	// Pressed is the control held down.
	Pressed bool
	// Focused is the keyboard on the control, however it got there.
	Focused bool
	// FocusVisible is the keyboard on the control after the keyboard moved it there
	// and not after a click: when a focus ring is to be seen.
	FocusVisible bool

	// Disabled is a control that does not react and is drawn dimmed.
	Disabled bool
	// ReadOnly is a field that can be read and selected but not edited.
	ReadOnly bool
	// Invalid is a value that failed validation.
	Invalid bool
	// Checked is a checkbox, switch or toggle that is on.
	Checked bool
	// Mixed is a checkbox between on and off. It takes precedence over Checked.
	Mixed bool
	// Selected is an item chosen from several.
	Selected bool
	// Open is a disclosure, menu or popup that is showing what it controls.
	Open bool
}

// StateOf returns the pointer and keyboard state of the control named id. The zero
// ID is never hovered, pressed or focused.
func (canvas *Canvas) StateOf(id ID) State {
	if canvas == nil || id.IsZero() {
		return State{}
	}
	return State{
		Hovered:      canvas.hovered == id,
		Pressed:      canvas.pressed == id,
		Focused:      canvas.focused == id,
		FocusVisible: canvas.focusVisible && canvas.focused == id,
	}
}

// Interacting reports whether the pointer or the keyboard is doing anything to the
// control: the states a hover, pressed or focus look is chosen by.
func (s State) Interacting() bool { return s.Hovered || s.Pressed || s.Focused }
