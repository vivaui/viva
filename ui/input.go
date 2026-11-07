package ui

// Key is one normalized, platform-independent keyboard identity.
type Key uint16

const (
	KeyUnknown Key = iota
	KeyTab
	KeyEnter
	KeySpace
	KeyEscape
	KeyLeft
	KeyRight
	KeyUp
	KeyDown
	KeyHome
	KeyEnd
	KeyBackspace
	KeyDelete
	KeyPageUp
	KeyPageDown
	KeyInsert
	KeyF1
	KeyF2
	KeyF3
	KeyF4
	KeyF5
	KeyF6
	KeyF7
	KeyF8
	KeyF9
	KeyF10
	KeyF11
	KeyF12
	// The modifier keys themselves. They are delivered only to a terminal that
	// asked for key releases (see app.TextInput.KeyReleases), as a press when
	// the key goes down and a release when it comes up; KeyEvent.Modifiers
	// then holds the modifiers in effect after the change.
	KeyShiftLeft
	KeyShiftRight
	KeyControlLeft
	KeyControlRight
	KeyAltLeft
	KeyAltRight
	KeyMetaLeft
	KeyMetaRight
)

// IsModifier reports whether the key is one of the modifier keys.
func (key Key) IsModifier() bool { return key >= KeyShiftLeft && key <= KeyMetaRight }

// Modifier is the modifier bit the key sets while held, zero for other keys.
func (key Key) Modifier() Modifiers {
	if !key.IsModifier() {
		return 0
	}
	return Modifiers(1) << ((key - KeyShiftLeft) / 2)
}

// Modifiers is a bitset of active keyboard modifiers.
type Modifiers uint8

const (
	ModifierShift Modifiers = 1 << iota
	ModifierControl
	ModifierAlt
	ModifierMeta
)

// PointerEventKind identifies one normalized pointer phase.
type PointerEventKind uint8

const (
	PointerDown PointerEventKind = iota + 1
	PointerMove
	PointerUp
	PointerExit
)

// PointerButton identifies the pointer button responsible for an event.
type PointerButton uint8

const (
	PointerButtonNone PointerButton = iota
	PointerButtonPrimary
	PointerButtonSecondary
	PointerButtonMiddle
	// PointerButtonBack and PointerButtonForward are the side buttons of a mouse,
	// which browsers and file managers use to move through history.
	PointerButtonBack
	PointerButtonForward
)

// PointerEvent is delivered in logical window pixels. Target is the stable hit
// ID under the pointer, or the press target while Captured is true. Viewport is
// the current logical window size, allowing coordinate-driven controls to
// derive responsive geometry without mutating application state during Paint.
//
// ClickCount numbers consecutive presses of one button close together in time
// and space, as the operating system defines a double click: 1 for a lone
// press, 2 for the second press of a double click, 3 for a triple click. Down
// and Up events carry the count of their press; other events carry 0.
type PointerEvent struct {
	Kind      PointerEventKind
	Button    PointerButton
	Modifiers Modifiers
	X         float32
	Y         float32
	// ScreenX and ScreenY are the same point in screen coordinates: logical
	// pixels from the top-left corner of the primary display, y down, the
	// coordinates app.Window.Frame is in. A press that is held keeps reporting
	// them when the pointer leaves the window, which is how a drag finds the
	// window it ends over. Exit events carry none.
	ScreenX, ScreenY float32
	Viewport         Size
	Target           ID
	Captured         bool
	ClickCount       int
}

// Win32Key is the Windows console INPUT_RECORD data of a key event: the
// virtual-key and scan codes, the UTF-16 code unit, the control-key state and
// the repeat count. It is set only on Windows, and a terminal forwards it only
// when the program enabled win32-input-mode (CSI ? 9001 h). Other platforms
// leave it zero.
type Win32Key struct {
	VirtualKey   uint16
	ScanCode     uint16
	Unicode      uint16
	ControlState uint16
	Repeat       uint16
}

// KeyEvent is delivered to the currently focused hit target.
//
// While a RoleTerminal hit, or a hit declared with HitOptions.TypeAhead, has
// focus the keyboard is raw: printable characters
// and text-service input (input methods, dead keys, paste) arrive as text
// events (see app.TextInput), and every other key arrives here. Key names the
// navigation, editing and function keys; for a chord such as Ctrl+C, Key is
// KeyUnknown and Text holds the character without the modifiers applied ("c").
// Text is otherwise empty on OnKey events. app.Config.OnShortcut separately
// receives modified printable chords before native text handling, except that a
// focused ordinary text field on Windows claims the standard editing chords.
//
// A terminal that sets app.TextInput.KeyReleases gets the whole keyboard as
// presses and releases instead: every key that is not consumed by input-method
// composition arrives as a press (printable keys with Text, the character
// without modifiers, and Key KeyUnknown), a press that repeats has Repeat set,
// and the same Key and Text come back once with Release set when the key goes
// up, so the pair identifies the key even if the modifiers changed in between.
// Modifier keys are keys of their own then (KeyShiftLeft and so on). A release
// is never sent for a press the application did not receive, and every key
// still down when the terminal loses the keyboard, the window loses focus or
// the terminal stops asking is released first, so a remote never keeps a key.
// Handlers that did not ask for releases never see a Release event. A window
// that only wants to know when a modifier was pressed and released on its own
// sets app.Config.OnModifierTap instead (see ModifierTapEvent).
type KeyEvent struct {
	Key       Key
	Modifiers Modifiers
	Target    ID
	Repeat    bool
	// Release marks a key going up (see above).
	Release bool
	Text    string
	// Typed is the text a printable key types with the modifiers in effect,
	// "A" for Shift+a, "!" for Shift+1 on a US layout, on the press of a
	// terminal that asked for key releases (app.TextInput.KeyReleases). Text
	// stays the character without modifiers and identifies the key. Typed is
	// empty for a key that types nothing, for chords with Control, Command or
	// Alt, and when the platform cannot say. A terminal that shows another
	// machine's desktop uses it where the remote names characters.
	Typed string
	// Win32 is the console key record on Windows; other platforms leave it zero.
	Win32 Win32Key
}

// ModifierTapEvent reports a modifier key that was pressed and released on its
// own: no other key, modifier, text, pointer press or scroll came between, and
// the window kept the keyboard. Key names the physical key (KeyShiftLeft and so
// on). Target is the hit that had the keyboard when the key went up.
//
// Viva reports each tap and leaves gestures made of several, such as pressing
// Shift twice, to the application, which knows how long it allows between them.
type ModifierTapEvent struct {
	Key    Key
	Target ID
}

// ScrollEvent is one normalized scroll gesture in logical window pixels.
// Target is the stable hit ID at X,Y. Viewport is the current logical window
// size, allowing coordinate-driven controls to derive their local geometry.
type ScrollEvent struct {
	Modifiers Modifiers
	X         float32
	Y         float32
	DeltaX    float32
	DeltaY    float32
	Viewport  Size
	Target    ID
	Precise   bool
}

// StepY is DeltaY for content that moves in steps of unit pixels, such as the
// rows of a list or terminal. A classic wheel never moves less than one unit per
// event in its direction: macOS reports a slow notch as a fraction of a line,
// which the pixel scale turns into a distance far below one row, so without the
// floor several notches would pass before anything moves. A precise delta is
// the distance the finger moved and is returned as it is.
func (e ScrollEvent) StepY(unit float32) float32 {
	return wheelStep(e.DeltaY, unit, e.Precise)
}

// StepX is StepY for DeltaX.
func (e ScrollEvent) StepX(unit float32) float32 {
	return wheelStep(e.DeltaX, unit, e.Precise)
}

func wheelStep(delta, unit float32, precise bool) float32 {
	switch {
	case precise || delta == 0 || unit <= 0:
		return delta
	case delta > 0:
		return max(delta, unit)
	default:
		return min(delta, -unit)
	}
}

// FileDropPhase says where a drag of files from the system is.
type FileDropPhase uint8

const (
	// FileDropOver reports files being dragged over the window, at a new point.
	// It is sent as the pointer moves, and again when it enters. Paths is empty:
	// systems do not always say what is dragged until it is dropped.
	FileDropOver FileDropPhase = iota
	// FileDropLeave reports that the drag left the window or was abandoned.
	FileDropLeave
	// FileDropDrop reports files dropped, with their paths.
	FileDropDrop
)

// FileDropFile is a file made available by a drag and drop. Name, Size, MIME
// and IsDir are known without reading; Read delivers bytes asynchronously, so a
// browser can settle its Promise without blocking the event loop. A directory
// cannot be read.
type FileDropFile interface {
	Name() string
	Size() int64
	MIME() string
	IsDir() bool
	Read(offset int64, length int, done func([]byte, error)) error
}

// FileDropEvent is files dragged from the system over, or dropped on, a window.
// Target is the hit under the point, as for a pointer event, so the application
// need not hit-test itself. Return Handled for FileDropOver and FileDropDrop when
// the target takes files: it is what the system shows to the user as an accepted
// drop. Repaint paints the window again, to show where the files would land.
type FileDropEvent struct {
	Phase  FileDropPhase
	X, Y   float32
	Target ID
	// Paths are absolute file system paths, in the order the system gave them;
	// set only for FileDropDrop, and only on a native target. Browsers do not
	// expose paths; use Files there.
	Paths []string
	// Files are the dropped files, set only for FileDropDrop. Native targets
	// also provide them, opened from Paths, so an application can read the same
	// way on every platform. Folders have IsDir set and cannot be read.
	Files []FileDropFile
}

// EventResult reports how an application handled an input event. Repaint asks
// for one synchronous paint. Focus moves focus to one focusable hit, including
// a hit the event reveals in that paint; Blur clears focus and takes precedence
// over Focus. Handled suppresses the runtime's click (what Clicked reads), activation,
// escape, or window-scroll behavior for that event. ClipboardText, when
// non-empty, asks the active runtime to copy the text without making
// application code depend on a platform clipboard.
type EventResult struct {
	Focus   ID
	Handled bool
	Repaint bool
	Blur    bool
	// ContinueTraversal asks the runtime to settle the repaint this result
	// requests and then carry the same key on to normal traversal. A menu closes
	// on Tab this way: the popup is gone before the focus moves through the page.
	// It has no effect on a modifier key or a key release.
	ContinueTraversal bool
	FocusVisible      bool
	ClipboardText     string
}
