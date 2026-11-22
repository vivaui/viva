package linux

import (
	"unicode/utf8"
	"unsafe"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/ibus"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/text"
)

// linuxIME connects one window's focused text field to an input method daemon
// (IBus, Fcitx5 through its IBus frontend, or Fcitx 4): the keys the field would
// receive go to the engine first, and what the engine composes comes back as the
// same text events a native editor produces, so the field shows the marked text
// and commits the choice exactly as it does on the other platforms.
//
// The engine owns the candidate window. All this side does is say when the field
// has the keyboard and where the caret is.
type linuxIME struct {
	app    *linuxApplication
	text   *linuxTextInput
	router eventRouter
	// origin is where the window's top-left corner is on the screen.
	origin  func() (x, y int32)
	context inputMethodContext
	target  gfx.ID
	owner   text.CompositionIdentity
	epoch   uint64
	phase   *ibusComposition
	// tentative records that the field opted into explicit phases, even when
	// the daemon refused the optional negotiation and phase stayed nil.
	tentative   bool
	delivering  int
	pendingKeys int

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

// inputMethodKey is one key press or release as the X server reported it: the
// symbol and keycode, the modifier state, the event time and whether the key
// came up.
type inputMethodKey struct {
	keysym  uint32
	keycode uint32
	state   uint32
	time    uint32
	release bool
}

// inputMethodContext is the input context as the window uses it. *fcitx.Context
// and *ibus.Context (behind a wrapper) are the real ones; the interface is the
// seam tests replace it at.
type inputMethodContext interface {
	ProcessKey(key inputMethodKey, done func(handled bool)) error
	FocusIn() error
	FocusOut() error
	Reset() error
	SetCursorLocation(x, y, width, height int32) error
	SetSurroundingText(text string, cursor, anchor uint32) error
	Close()
}

// newIME gives a window an input context, or nil when there is no daemon.
func (application *linuxApplication) newIME(win *window, textInput *linuxTextInput, router eventRouter) *linuxIME {
	if application.inputMethodDaemon() == nil {
		return nil
	}
	ime := &linuxIME{app: application, text: textInput, router: router, origin: win.contentOrigin}
	ime.router.ime = ime
	if !ime.open() {
		return nil
	}
	return ime
}

// serveInputMethod reads what the daemon sent.
func (application *linuxApplication) serveInputMethod() {
	if application.inputMethod != nil {
		_ = application.inputMethod.Dispatch()
		application.checkInputMethod()
	}
}

// checkInputMethod lets go of a daemon that has failed, or has stopped answering:
// every window goes back to receiving its keys directly.
func (application *linuxApplication) checkInputMethod() {
	if application.inputMethod == nil || application.inputMethod.Err() == nil {
		return
	}
	application.inputMethod = nil
	for _, state := range application.windows {
		if state.router.ime != nil {
			state.router.ime.lost()
		}
	}
	application.rebuildPoll()
}

// lost abandons the engine. Text it left marked is kept as typed.
func (ime *linuxIME) lost() {
	keep := ime.phase != nil && ime.phase.session && ime.sameOwner() && !ime.text.closed
	ime.clearComposition()
	ime.dead, ime.active, ime.engineComposing = true, false, false
	ime.context = nil
	if keep {
		ime.apply(platform.TextEvent{Kind: platform.TextUnmark, CompositionPhase: text.CompositionCommit})
	} else if ime.phase == nil && ime.text.state.Composing {
		ime.apply(platform.TextEvent{Kind: platform.TextUnmark})
	}
}

func (ime *linuxIME) close() {
	ime.dead = true
	ime.retire()
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
	if ime.dead || ime.text.closed || ime.app.firstErr != nil || ime.app.quit {
		return
	}
	if ime.context != nil && !ime.sameOwner() {
		// The shared app layer settles the outgoing editor's tentative
		// composition before publishing a different target; this only drops
		// the native session that belonged to the old one.
		ime.retire()
	}
	if ime.context == nil {
		if !ime.text.enabled {
			return
		}
		if !ime.open() {
			ime.dead = true
			return
		}
	}
	context, epoch := ime.context, ime.epoch
	want := ime.focused && ime.text.enabled
	switch {
	case want && !ime.active:
		ime.active, ime.haveCursor, ime.hasSources, ime.sent = true, false, false, false
		if err := context.FocusIn(); err != nil {
			ime.app.checkInputMethod()
			return
		}
	case !want && ime.active:
		// Every key's text precedes its answer. Keep accepting those results
		// before settling the native mode and sending FocusOut.
		if ime.pendingKeys != 0 {
			return
		}
		ime.active = false
		if ime.engineComposing || ime.phase != nil && ime.phase.session {
			if err := ime.reset(); err != nil {
				if ime.context != context || ime.epoch != epoch || !ime.sameOwner() {
					return
				}
				// A reset the engine cannot describe is not an application
				// failure: some engines keep sending legacy modes or an
				// unknown one, and refusing to guess is deliberate. Retire
				// this native session, keep the text the field already
				// shows, and let sync open a fresh context next frame.
				keep := ime.phase != nil && ime.phase.session
				// Retire before settling: the settlement is delivered through
				// the router, and a reentrant sync must not find the session it
				// just retired still current.
				ime.retire()
				if keep {
					ime.apply(platform.TextEvent{Kind: platform.TextUnmark, CompositionPhase: text.CompositionCommit})
				}
				ime.app.checkInputMethod()
				return
			}
		}
		if ime.context == context && ime.epoch == epoch && !ime.dead && ime.app.firstErr == nil &&
			!(ime.focused && ime.text.enabled) {
			_ = context.FocusOut()
		}
	}
	if !ime.active || !ime.current() || ime.context != context || ime.epoch != epoch {
		return
	}
	// The field stopped composing without the engine (a click moved the caret,
	// the application replaced the text): the engine must forget its composition
	// or its next key continues one the user no longer sees.
	stopped := ime.phase == nil && ime.engineComposing && !ime.text.state.Composing ||
		ime.phase != nil && ime.phase.session && ime.delivering == 0 && ime.phase.revision != ime.text.stateRevision
	if stopped {
		if ime.phase != nil {
			ime.retire()
			ime.sync()
			return
		}
		ime.clearComposition()
		ime.engineComposing = false
		if err := context.Reset(); err != nil {
			ime.app.checkInputMethod()
			return
		}
	}
	if !ime.current() || ime.context != context || ime.epoch != epoch {
		return
	}
	ime.updateCursor()
	if !ime.current() || ime.context != context || ime.epoch != epoch {
		return
	}
	ime.updateSurrounding()
}

// updateSurrounding tells the engine the text around the caret when it has changed.
// A password, and a terminal whose text is not a document, tell it nothing.
func (ime *linuxIME) updateSurrounding() {
	if !ime.current() || !ime.active {
		return
	}
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
	if !ime.current() || !ime.active {
		return
	}
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

// processKey gives a key press to the engine. It reports whether the engine has
// it, in which case the key reaches the field only if the engine declines it.
func (ime *linuxIME) processKey(keysym uint32, event *xcbInputEvent, keyEvent platform.KeyEvent) bool {
	return ime.processKeyWithShortcut(keysym, event, keyEvent, keyEvent)
}

func (ime *linuxIME) processKeyWithShortcut(keysym uint32, event *xcbInputEvent, keyEvent, shortcut platform.KeyEvent) bool {
	if !ime.current() || !ime.active || !ime.focused || event.Detail < 8 || keysym == 0 {
		return false
	}
	epoch := ime.epoch
	ime.pendingKeys++
	err := ime.context.ProcessKey(inputMethodKey{keysym: keysym, keycode: uint32(event.Detail), state: uint32(event.State), time: event.Time}, func(handled bool) {
		if !ime.answerKey(epoch) {
			return
		}
		defer ime.finishKey(epoch)
		if !ime.keyCurrent(epoch) {
			return
		}
		if keysym == 0xff1b && ime.phase != nil && ime.phase.session && !ime.phase.native.Visible {
			ime.clearComposition()
			ime.engineComposing = false
			ime.apply(platform.TextEvent{Kind: platform.TextCancelComposition, CompositionPhase: text.CompositionCancel})
		}
		if !ime.keyCurrent(epoch) {
			return
		}
		if handled {
			return
		}
		if err := ime.router.deliverTypedWithShortcut(keyEvent, keysym, shortcut); err != nil {
			ime.app.fail(err)
		}
	})
	if err != nil {
		if ime.epoch == epoch {
			ime.pendingKeys--
		}
		ime.app.checkInputMethod()
		return ime.epoch != epoch || !ime.sameOwner()
	}
	return true
}

// pressModifier tells the engine a modifier key went down. The engine never gives
// one back, so there is nothing to wait for.
func (ime *linuxIME) pressModifier(keysym uint32, event *xcbInputEvent) {
	if !ime.current() || !ime.active || !ime.focused || event.Detail < 8 || keysym == 0 {
		return
	}
	if err := ime.sendKey(inputMethodKey{keysym: keysym, keycode: uint32(event.Detail), state: uint32(event.State), time: event.Time}); err != nil {
		ime.app.checkInputMethod()
	}
}

// release tells the engine a key came up. Some engines act on that alone, as a
// lone Shift that switches between Chinese and English does.
func (ime *linuxIME) release(keymap *xcbKeymap, event *xcbInputEvent) {
	if !ime.current() || !ime.active || !ime.focused || event.Detail < 8 {
		return
	}
	keysym := keymap.keysym(event.Detail, event.State)
	if keysym == 0 {
		return
	}
	if err := ime.sendKey(inputMethodKey{keysym: keysym, keycode: uint32(event.Detail), state: uint32(event.State), time: event.Time, release: true}); err != nil {
		ime.app.checkInputMethod()
	}
}

func (ime *linuxIME) commit(value string) {
	if !ime.current() || ime.phase != nil && !ime.active {
		return
	}
	result := ime.phase != nil && ime.phase.session
	ime.clearComposition()
	ime.engineComposing = false
	event := platform.TextEvent{Kind: platform.TextInsert, Text: value}
	if result {
		event.CompositionPhase = text.CompositionResult
	}
	ime.apply(event)
}

// preedit shows the text being composed as marked text, or removes it. Selection
// is the clause being converted, in characters.
func (ime *linuxIME) preedit(value string, cursor int, visible bool, selection text.Range) {
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
	selection.Start = min(max(selection.Start, 0), length)
	selection.End = min(max(selection.End, 0), length)
	if selection.End < selection.Start {
		selection.Start, selection.End = selection.End, selection.Start
	}
	ime.apply(platform.TextEvent{Kind: platform.TextSetMarked, Text: value, MarkedSelection: selection})
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
func (ime *linuxIME) forward(keysym uint32, release bool, state uint32) {
	if release {
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
	revision, epoch := ime.text.stateRevision, ime.epoch
	ime.delivering++
	frame, accepted, err := ime.text.dispatchTextEvent(event)
	ime.delivering--
	if !accepted && ime.epoch == epoch && ime.text.stateRevision != revision {
		ime.retire()
		if err == nil {
			ime.sync()
		}
	}
	if accepted && ime.phase != nil && ime.epoch == epoch && ime.text.stateRevision == revision+1 {
		ime.phase.revision = ime.text.stateRevision
	}
	if err == nil {
		if ime.epoch == epoch && ime.sameOwner() {
			err = ime.router.presentFrame(frame, nil)
		}
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
