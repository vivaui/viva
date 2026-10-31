package linux

import (
	"unicode/utf8"
	"unsafe"

	"github.com/vivaui/viva/internal/ibus"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/text"
)

// linuxIME connects one window's focused text field to an input method running
// in the IBus daemon (or in Fcitx5, which speaks the same protocol): the keys the
// field would receive go to the engine first, and what the engine composes comes
// back as the same text events a native editor produces, so the field shows the
// marked text and commits the choice exactly as it does on the other platforms.
type imeHost interface {
	fail(err error)
	checkInputMethod()
}

// The engine owns the candidate window. All this side does is say when the field
// has the keyboard and where the caret is.
type linuxIME struct {
	app    imeHost
	text   *linuxTextInput
	router eventRouter
	// origin is where the window's top-left corner is on the screen.
	origin  func() (x, y int32)
	context inputMethodContext

	// focused is whether the window has the keyboard; active is whether the
	// engine has been told the field does.
	focused bool
	active  bool
	// engineComposing is whether the engine believes a composition is under way.
	engineComposing bool
	// cursor is the last caret location sent, and haveCursor says it was sent.
	cursor     [4]int32
	haveCursor bool
	// dead is set once the daemon is gone; keys then go straight to the field.
	dead bool

	// sources is the field state the engine's window was built from, window is
	// that window and sent says it is what the engine was last told.
	sources surroundingState
	window  surrounding
	sent    bool
	// hasSources says sources is current; it is cleared when the engine must be
	// told again, as after it gains the focus.
	hasSources bool
}

// inputMethodContext is the input context as the window uses it. *ibus.Context
// is the real one; the interface is the seam tests replace it at.
type inputMethodContext interface {
	ProcessKey(keysym, keycode, state uint32, done func(handled bool)) error
	FocusIn() error
	FocusOut() error
	Reset() error
	SetCursorLocation(x, y, width, height int32) error
	SetSurroundingText(text string, cursor, anchor uint32) error
	Close()
}

func newIMEForHost(host imeHost, bus *ibus.Bus, origin func() (x, y int32), textInput *linuxTextInput, router eventRouter) *linuxIME {
	if bus == nil {
		return nil
	}
	ime := &linuxIME{app: host, text: textInput, router: router, origin: origin}
	context, err := bus.NewContext("viva", ibus.Handler{
		Commit:  ime.commit,
		Preedit: ime.preedit,
		Forward: ime.forward,

		DeleteSurrounding: ime.deleteSurrounding,
	})
	if err != nil {
		host.checkInputMethod()
		return nil
	}
	ime.context = context
	return ime
}

// newIME gives a window an input context, or nil when there is no daemon.
func (application *linuxApplication) newIME(win *window, textInput *linuxTextInput, router eventRouter) *linuxIME {
	return newIMEForHost(application, application.inputMethodBus(), win.contentOrigin, textInput, router)
}

func (app *waylandApplication) newIME(win *waylandWindow, textInput *linuxTextInput, router eventRouter) *linuxIME {
	bus, _ := app.inputMethodBus().(*ibus.Bus)
	return newIMEForHost(app, bus, win.contentOrigin, textInput, router)
}

// inputMethodBus connects to the daemon the first time a window can take text,
// and not again: a machine without one is the usual case, and a daemon that
// starts later is picked up by the next run.
func (application *linuxApplication) inputMethodBus() *ibus.Bus {
	if !application.ibusTried {
		application.ibusTried = true
		if bus, err := ibus.Dial(); err == nil {
			application.ibus = bus
		}
	}
	return application.ibus
}

// serveInputMethod reads what the daemon sent.
func (application *linuxApplication) serveInputMethod() {
	if application.ibus != nil {
		_ = application.ibus.Dispatch()
		application.checkInputMethod()
	}
}

// checkInputMethod lets go of a daemon that has failed, or has stopped answering:
// every window goes back to receiving its keys directly.
func (application *linuxApplication) checkInputMethod() {
	if application.ibus == nil || application.ibus.Err() == nil {
		return
	}
	application.ibus = nil
	for _, state := range application.windows {
		if state.router.ime != nil {
			state.router.ime.lost()
		}
	}
	application.rebuildPoll()
}

// lost abandons the engine. Text it left marked is kept as typed.
func (ime *linuxIME) lost() {
	ime.dead, ime.active, ime.engineComposing = true, false, false
	ime.context = nil
	if ime.text.state.Composing {
		ime.apply(platform.TextEvent{Kind: platform.TextUnmark})
	}
}

func (ime *linuxIME) close() {
	if ime.context != nil {
		ime.context.Close()
		ime.context = nil
	}
	ime.dead = true
}

// setFocused records whether the window has the keyboard.
func (ime *linuxIME) setFocused(focused bool) {
	ime.focused = focused
	ime.sync()
}

// sync brings the engine's idea of focus and caret in line with the field. It
// runs after every frame, which is when the field changes, and does nothing
// when nothing has.
func (ime *linuxIME) sync() {
	if ime.dead || ime.context == nil {
		return
	}
	want := ime.focused && ime.text.enabled
	switch {
	case want && !ime.active:
		ime.active, ime.haveCursor, ime.hasSources, ime.sent = true, false, false, false
		_ = ime.context.FocusIn()
	case !want && ime.active:
		ime.active = false
		if ime.engineComposing {
			ime.engineComposing = false
			_ = ime.context.Reset()
		}
		_ = ime.context.FocusOut()
	}
	if !ime.active {
		return
	}
	// The field stopped composing without the engine (a click moved the caret,
	// the application replaced the text): the engine must forget its composition
	// or its next key continues one the user no longer sees.
	if ime.engineComposing && !ime.text.state.Composing {
		ime.engineComposing = false
		_ = ime.context.Reset()
	}
	ime.updateCursor()
	ime.updateSurrounding()
}

// updateSurrounding tells the engine the text around the caret when it has changed.
// A password, and a terminal whose text is not a document, tell it nothing.
func (ime *linuxIME) updateSurrounding() {
	var sources surroundingState
	if !ime.text.input.Secure && !ime.text.input.Terminal {
		state := ime.text.state
		sources = surroundingState{Value: state.Value, Selection: state.Selection, Composition: state.Composition, Composing: state.Composing}
	}
	if ime.hasSources && sources == ime.sources {
		return
	}
	previous := ime.window
	ime.sources, ime.hasSources = sources, true
	ime.window = surroundingOf(sources)
	if ime.sent && ime.window.Text == previous.Text && ime.window.Cursor == previous.Cursor && ime.window.Anchor == previous.Anchor {
		return
	}
	ime.sent = true
	_ = ime.context.SetSurroundingText(ime.window.Text, uint32(ime.window.Cursor), uint32(ime.window.Anchor))
}

// deleteSurrounding deletes the text around the caret that the engine is about to
// convert again.
func (ime *linuxIME) deleteSurrounding(offset, count int) {
	if ime.dead || !ime.active {
		return
	}
	// The window is as of the last frame, and the field has not changed since:
	// the engine's request arrives between the keys that caused it and the next frame.
	if !ime.hasSources {
		return
	}
	if r, ok := ime.window.fieldRange(offset, count); ok {
		ime.apply(platform.TextEvent{Kind: platform.TextInsert, Replacement: r, HasReplacement: true})
	}
}

// updateCursor tells the engine's panel where to put the candidate window.
func (ime *linuxIME) updateCursor() {
	x, y, width, height := ime.caretRectangle()
	originX, originY := ime.origin()
	// The caret is logical, the engine wants screen pixels.
	scale := ime.router.displayScale()
	location := [4]int32{originX + int32(x*scale), originY + int32(y*scale), int32(width * scale), int32(height * scale)}
	if ime.haveCursor && location == ime.cursor {
		return
	}
	ime.cursor, ime.haveCursor = location, true
	_ = ime.context.SetCursorLocation(location[0], location[1], location[2], location[3])
}

// caretRectangle is the caret in window coordinates: the line the field draws,
// or for a terminal, which draws none, the cell where its input goes.
func (ime *linuxIME) caretRectangle() (x, y, width, height float32) {
	input := &ime.text.input
	if input.Placed && input.CaretHeight > 0 {
		// The field laid its rows out itself: the caret is where it painted it.
		return input.CaretX, input.CaretY, input.CaretWidth, input.CaretHeight
	}
	if input.Terminal || ime.text.caret.Bounds.Height == 0 {
		return input.X, input.Y, 1, input.Height
	}
	caret := ime.text.caret.Bounds
	return caret.X, caret.Y, caret.Width, caret.Height
}

// processKeyDirect gives a key press to the engine directly using keysym, keycode, and modifier state.
func (ime *linuxIME) processKeyDirect(keysym uint32, keycode uint32, state uint32, keyEvent platform.KeyEvent) bool {
	if ime.dead || !ime.active || ime.context == nil || keysym == 0 {
		return false
	}
	err := ime.context.ProcessKey(keysym, keycode, state, func(handled bool) {
		if handled {
			return
		}
		if err := ime.router.deliverTyped(keyEvent, keysym); err != nil {
			ime.app.fail(err)
		}
	})
	if err != nil {
		ime.app.checkInputMethod()
		return false
	}
	return true
}

// processKey gives a key press to the engine. It reports whether the engine has
// it, in which case the key reaches the field only if the engine declines it.
func (ime *linuxIME) processKey(keysym uint32, event *xcbInputEvent, keyEvent platform.KeyEvent) bool {
	if event.Detail < 8 {
		return false
	}
	return ime.processKeyDirect(keysym, uint32(event.Detail)-8, uint32(event.State), keyEvent)
}

// release tells the engine a key came up. Some engines act on that alone, as a
// lone Shift that switches between Chinese and English does.
func (ime *linuxIME) release(keymap *xcbKeymap, event *xcbInputEvent) {
	if ime.dead || !ime.active || ime.context == nil || event.Detail < 8 {
		return
	}
	keysym := keymap.keysym(event.Detail, event.State)
	ime.releaseDirect(keysym, uint32(event.Detail)-8, uint32(event.State))
}

func (ime *linuxIME) releaseDirect(keysym, keycode, state uint32) {
	if ime.dead || !ime.active || ime.context == nil || keysym == 0 {
		return
	}
	if err := ime.context.ProcessKey(keysym, keycode, state|ibus.ReleaseMask, nil); err != nil {
		ime.app.checkInputMethod()
	}
}

func (ime *linuxIME) commit(value string) {
	ime.engineComposing = false
	ime.apply(platform.TextEvent{Kind: platform.TextInsert, Text: value})
}

// preedit shows the text being composed as marked text, or removes it.
func (ime *linuxIME) preedit(value string, cursor int, visible bool, attrs []ibus.Attribute) {
	if !visible || value == "" {
		ime.engineComposing = false
		if ime.text.state.Composing {
			ime.apply(platform.TextEvent{Kind: platform.TextSetMarked})
			ime.apply(platform.TextEvent{Kind: platform.TextUnmark})
		}
		return
	}
	ime.engineComposing = true
	length := utf8.RuneCountInString(value)
	cursor = min(max(cursor, 0), length)
	ime.apply(platform.TextEvent{Kind: platform.TextSetMarked, Text: value, MarkedSelection: preeditSelection(length, cursor, attrs)})
}

// preeditSelection is the part of the marked text to show as selected: the clause
// the engine is converting, which it marks with a background. Without one it is
// the caret. (The underline an engine puts under the whole preedit is what marked
// text is already drawn with.)
func preeditSelection(length, cursor int, attrs []ibus.Attribute) text.Range {
	for _, attribute := range attrs {
		if attribute.Kind != ibus.AttrBackground {
			continue
		}
		start, end := min(max(attribute.Start, 0), length), min(max(attribute.End, 0), length)
		if end > start {
			return text.Range{Start: start, End: end}
		}
	}
	return text.Range{Start: cursor, End: cursor}
}

// forward delivers a key the engine took apart and gave back, such as the Enter
// that ends a composition, past the engine.
func (ime *linuxIME) forward(keysym, _ uint32, state uint32) {
	if state&ibus.ReleaseMask != 0 {
		return
	}
	event := platform.KeyEvent{Key: normalizedKeysym(keysym), Modifiers: xcbModifiers(uint16(state))}
	if character, ok := keysymRune(keysym); ok {
		event.Text = string(character)
	}
	if err := ime.router.deliverTyped(event, keysym); err != nil {
		ime.app.fail(err)
	}
}

// apply runs one text event through the field and presents the result.
func (ime *linuxIME) apply(event platform.TextEvent) {
	frame, err := ime.text.textEvent(event)
	if err == nil {
		err = ime.router.presentFrame(frame, nil)
	}
	if err != nil {
		ime.app.fail(err)
	}
}

// rootOrigin is where the window's top-left corner is on the screen. The engine
// wants the caret in screen coordinates, and a window manager that decorates or
// moves windows makes that differ from anything the window can know itself.
func (win *window) rootOrigin() (int32, int32) {
	cookie := win.api.translateCoordinates(win.connection, win.id, win.screen.Root, 0, 0)
	var nativeError unsafe.Pointer
	reply := win.api.translateCoordinatesReply(win.connection, cookie, &nativeError)
	if nativeError != nil {
		win.api.free(nativeError)
	}
	if reply == nil {
		return 0, 0
	}
	defer win.api.free(unsafe.Pointer(reply))
	return int32(reply.DstX), int32(reply.DstY)
}
