package linux

import (
	"errors"
	"reflect"
	"testing"

	"github.com/vivaui/viva/internal/ibus"
	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/text"
)

// fakeIMEContext stands in for an IBus input context: it records the calls and
// lets a test answer the keys it was given.
type fakeIMEContext struct {
	calls   []string
	keys    []fakeIMEKey
	cursors [][4]int32
	// surrounds is every SetSurroundingText, kept apart from calls.
	surrounds []fakeSurround
	err       error
	closed    bool
	closes    int
}

type fakeIMEKey struct {
	keysym, keycode, state uint32
	done                   func(bool)
}

func (f *fakeIMEContext) record(name string) error {
	f.calls = append(f.calls, name)
	return f.err
}

func (f *fakeIMEContext) ProcessKey(keysym, keycode, state uint32, done func(bool)) error {
	if f.err != nil {
		return f.err
	}
	f.keys = append(f.keys, fakeIMEKey{keysym, keycode, state, done})
	return nil
}
func (f *fakeIMEContext) FocusIn() error  { return f.record("FocusIn") }
func (f *fakeIMEContext) FocusOut() error { return f.record("FocusOut") }
func (f *fakeIMEContext) Reset() error    { return f.record("Reset") }
func (f *fakeIMEContext) SetCursorLocation(x, y, width, height int32) error {
	f.cursors = append(f.cursors, [4]int32{x, y, width, height})
	return f.record("SetCursorLocation")
}
func (f *fakeIMEContext) SetSurroundingText(text string, cursor, anchor uint32) error {
	f.surrounds = append(f.surrounds, fakeSurround{text, cursor, anchor})
	return f.err
}
func (f *fakeIMEContext) Close() { f.closed, f.closes = true, f.closes+1 }

type fakeSurround struct {
	text           string
	cursor, anchor uint32
}

// imeHarness is a window's text field and its input method, without a window.
type imeHarness struct {
	ime       *linuxIME
	context   *fakeIMEContext
	editor    *text.Editor
	input     *linuxTextInput
	delivered []platform.KeyEvent
	origin    [2]int32
	presents  int
}

func newIMEHarness(t *testing.T) *imeHarness {
	t.Helper()
	harness := &imeHarness{context: &fakeIMEContext{}, editor: text.NewEditor(""), origin: [2]int32{300, 200}}
	config := &platform.Config{
		DynamicTextInput: true,
		OnKey: func(event platform.KeyEvent) (platform.Frame, error) {
			harness.delivered = append(harness.delivered, event)
			return platform.Frame{}, nil
		},
	}
	harness.input = &linuxTextInput{
		config: config,
		engine: fixedAdvanceEngine{},
	}
	field := platform.TextInput{
		X: 24, Y: 40, Width: 300, Height: 28, FontSize: 16,
		Color: platform.Color{Red: 0, Green: 0, Blue: 0, Alpha: 1},
		OnEvent: func(event platform.TextEvent) (platform.TextState, error) {
			event.Apply(harness.editor)
			composition, composing := harness.editor.Composition()
			return platform.TextState{Value: harness.editor.String(), Selection: harness.editor.Selection(), Composition: composition, Composing: composing}, nil
		},
	}
	if _, _, err := harness.input.prepare(platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: &field}}); err != nil {
		t.Fatal(err)
	}
	router := eventRouter{config: *config, text: harness.input}
	router.present = func(frame platform.Frame) error {
		harness.presents++
		defer harness.ime.sync()
		_, _, err := harness.input.prepare(frame)
		return err
	}
	harness.ime = &linuxIME{
		app: &linuxApplication{}, text: harness.input, router: router, context: harness.context,
		origin: func() (int32, int32) { return harness.origin[0], harness.origin[1] },
	}
	router.ime = harness.ime
	harness.ime.router = router
	return harness
}

func (h *imeHarness) focus(t *testing.T) {
	t.Helper()
	h.ime.setFocused(true)
	if !h.ime.active {
		t.Fatal("a focused window with a text field did not activate the input method")
	}
}

// letterKey is the X event for pressing the letter n (keycode 57), and its keymap.
func letterKey(state uint16) (*xcbKeymap, *xcbInputEvent) {
	return &xcbKeymap{first: 57, last: 57, perKeycode: 2, keysyms: []uint32{'n', 'N'}}, &xcbInputEvent{Detail: 57, State: state}
}

func TestIMEActivatesOnlyWhileTheWindowAndFieldBothHaveTheKeyboard(t *testing.T) {
	h := newIMEHarness(t)
	h.input.enabled = false
	h.ime.setFocused(true)
	if h.ime.active || len(h.context.calls) != 0 {
		t.Fatalf("no field is up, yet the input method was told: %v", h.context.calls)
	}
	h.input.enabled = true
	h.ime.sync()
	if !h.ime.active || !reflect.DeepEqual(h.context.calls[:1], []string{"FocusIn"}) {
		t.Fatalf("calls = %v", h.context.calls)
	}
	h.ime.setFocused(false)
	if h.ime.active || h.context.calls[len(h.context.calls)-1] != "FocusOut" {
		t.Fatalf("losing window focus left the input method active: %v", h.context.calls)
	}
}

func TestIMEReportsTheCaretInScreenCoordinatesOnlyWhenItMoves(t *testing.T) {
	h := newIMEHarness(t)
	h.focus(t)
	if len(h.context.cursors) != 1 {
		t.Fatalf("cursor sends = %v", h.context.cursors)
	}
	first := h.context.cursors[0]
	// The caret is the field's own origin plus the window's place on the screen.
	if first[0] < 300+24 || first[1] < 200+40 || first[3] <= 0 {
		t.Fatalf("caret %v is not placed relative to the window at (300,200) and the field at (24,40)", first)
	}
	h.ime.sync()
	h.ime.sync()
	if len(h.context.cursors) != 1 {
		t.Fatalf("an unchanged caret was sent again: %v", h.context.cursors)
	}
	h.origin = [2]int32{500, 200}
	h.ime.updateCursor()
	if got := h.context.cursors[len(h.context.cursors)-1]; got[0] != first[0]+200 {
		t.Fatalf("moving the window by 200 moved the caret by %d", got[0]-first[0])
	}
}

func TestIMEKeyTheEngineTakesNeverReachesTheField(t *testing.T) {
	h := newIMEHarness(t)
	h.focus(t)
	keymap, event := letterKey(0x1) // Shift held
	if !h.ime.processKey(keymap.keysym(event.Detail, event.State), event, platform.KeyEvent{Text: "N"}) {
		t.Fatal("the key was not given to the engine")
	}
	key := h.context.keys[0]
	if key.keysym != 'N' || key.keycode != 57-8 || key.state != 0x1 {
		t.Fatalf("key = %+v; IBus counts keycodes from the X keycode less 8, and wants the shifted symbol", key)
	}
	key.done(true)
	if len(h.delivered) != 0 || h.editor.String() != "" {
		t.Fatalf("a key the engine took was delivered anyway: %v %q", h.delivered, h.editor.String())
	}
}

func TestIMEKeyTheEngineDeclinesIsTypedIntoTheField(t *testing.T) {
	h := newIMEHarness(t)
	h.focus(t)
	keymap, event := letterKey(0)
	h.ime.processKey(keymap.keysym(event.Detail, event.State), event, platform.KeyEvent{Text: "n", Key: input.KeyUnknown})
	if h.editor.String() != "" {
		t.Fatal("the key reached the field before the engine answered")
	}
	h.context.keys[0].done(false)
	if h.editor.String() != "n" {
		t.Fatalf("field = %q after the engine declined the key", h.editor.String())
	}
}

func TestIMEKeysAreDeliveredInTypedOrderWhateverTheAnswerOrder(t *testing.T) {
	h := newIMEHarness(t)
	h.focus(t)
	keymap, event := letterKey(0)
	for _, character := range []string{"a", "b", "c"} {
		h.ime.processKey(keymap.keysym(event.Detail, event.State), event, platform.KeyEvent{Text: character})
	}
	// The ibus package releases answers oldest first; each done runs in turn.
	for _, key := range h.context.keys {
		key.done(false)
	}
	if h.editor.String() != "abc" {
		t.Fatalf("field = %q", h.editor.String())
	}
}

func TestIMEPreeditIsMarkedTextAndCommitReplacesIt(t *testing.T) {
	h := newIMEHarness(t)
	h.focus(t)
	h.ime.preedit("nǐhǎo", 3, true, nil)
	composition, composing := h.editor.Composition()
	if !composing || composition != (text.Range{Start: 0, End: 5}) || h.editor.String() != "nǐhǎo" {
		t.Fatalf("composition = %+v, %v, text %q", composition, composing, h.editor.String())
	}
	if caret := h.editor.Selection(); caret != (text.Range{Start: 3, End: 3}) {
		t.Fatalf("the engine put its cursor at 3 characters, the field has %+v", caret)
	}
	h.ime.preedit("你好", 2, true, nil)
	if h.editor.String() != "你好" {
		t.Fatalf("a new preedit must replace the last, got %q", h.editor.String())
	}
	h.ime.commit("你好")
	if _, composing := h.editor.Composition(); composing || h.editor.String() != "你好" {
		t.Fatalf("after commit: text %q composing %v", h.editor.String(), composing)
	}
	if h.ime.engineComposing {
		t.Fatal("the field still thinks the engine is composing after a commit")
	}
}

func TestIMEHidingThePreeditRemovesTheMarkedText(t *testing.T) {
	h := newIMEHarness(t)
	h.focus(t)
	h.editor.Insert("keep ")
	h.ime.preedit("nihao", 5, true, nil)
	h.ime.preedit("nihao", 5, false, nil)
	if _, composing := h.editor.Composition(); composing || h.editor.String() != "keep " {
		t.Fatalf("text %q composing %v; cancelling must leave only what was typed before", h.editor.String(), composing)
	}
	// An empty preedit with nothing composing changes nothing, and in particular
	// does not leave an empty composition behind.
	h.ime.preedit("", 0, true, nil)
	if _, composing := h.editor.Composition(); composing {
		t.Fatal("an empty preedit started a composition")
	}
}

func TestIMEPreeditCursorIsClampedToTheText(t *testing.T) {
	h := newIMEHarness(t)
	h.focus(t)
	h.ime.preedit("ab", 99, true, nil)
	if caret := h.editor.Selection(); caret.End != 2 {
		t.Fatalf("caret = %+v for a cursor beyond the end", caret)
	}
	h.ime.preedit("ab", -4, true, nil)
	if caret := h.editor.Selection(); caret.Start != 0 {
		t.Fatalf("caret = %+v for a negative cursor", caret)
	}
}

func TestIMEResetsTheEngineWhenTheFieldStopsComposingWithoutIt(t *testing.T) {
	h := newIMEHarness(t)
	h.focus(t)
	h.ime.preedit("nihao", 5, true, nil)
	// A click puts the caret elsewhere, which ends the field's composition.
	h.editor.SetSelection(text.Range{Start: 0, End: 0})
	h.input.state.Composing = false
	h.ime.sync()
	if got := h.context.calls[len(h.context.calls)-1]; got != "Reset" {
		t.Fatalf("calls = %v; the engine must be told to forget the composition", h.context.calls)
	}
	if h.ime.engineComposing {
		t.Fatal("engineComposing was not cleared")
	}
}

func TestIMELeavingTheFieldAbandonsAnOpenComposition(t *testing.T) {
	h := newIMEHarness(t)
	h.focus(t)
	h.ime.preedit("nihao", 5, true, nil)
	h.input.enabled = false
	h.ime.sync()
	calls := h.context.calls
	if len(calls) < 2 || calls[len(calls)-2] != "Reset" || calls[len(calls)-1] != "FocusOut" {
		t.Fatalf("calls = %v; want Reset then FocusOut", calls)
	}
}

func TestIMEForwardedKeyBypassesTheEngine(t *testing.T) {
	h := newIMEHarness(t)
	h.focus(t)
	h.ime.forward(0xff0d, 28, 0) // Return
	if len(h.delivered) != 1 || h.delivered[0].Key != input.KeyEnter {
		t.Fatalf("delivered = %+v", h.delivered)
	}
	if len(h.context.keys) != 0 {
		t.Fatal("a forwarded key went back to the engine, which would loop")
	}
	h.ime.forward(0xff0d, 28, ibus.ReleaseMask)
	if len(h.delivered) != 1 {
		t.Fatal("a forwarded release was delivered as a key press")
	}
	h.ime.forward('a', 38, 0)
	if h.editor.String() != "a" {
		t.Fatalf("a forwarded character was not typed: %q", h.editor.String())
	}
}

func TestIMEReleaseGoesToTheEngineWithTheReleaseMask(t *testing.T) {
	h := newIMEHarness(t)
	keymap, event := letterKey(0)
	h.ime.release(keymap, event)
	if len(h.context.keys) != 0 {
		t.Fatal("a release was sent to an input method that is not active")
	}
	h.focus(t)
	h.ime.release(keymap, event)
	if got := h.context.keys[0]; got.state != ibus.ReleaseMask || got.done != nil || got.keysym != 'n' {
		t.Fatalf("release = %+v", got)
	}
}

func TestIMEKeysAreNotGivenToTheEngineWithoutAnActiveField(t *testing.T) {
	h := newIMEHarness(t)
	keymap, event := letterKey(0)
	if h.ime.processKey(keymap.keysym(event.Detail, event.State), event, platform.KeyEvent{Text: "n"}) {
		t.Fatal("a key was claimed while no field had the keyboard")
	}
	h.focus(t)
	if h.ime.processKey(0, event, platform.KeyEvent{}) {
		t.Fatal("a key the keyboard mapping does not know was claimed")
	}
	if h.ime.processKey('n', &xcbInputEvent{Detail: 3}, platform.KeyEvent{}) {
		t.Fatal("a keycode below 8 cannot be an IBus key")
	}
}

func TestIMELostDaemonKeepsTheTextAndReturnsKeysToTheField(t *testing.T) {
	h := newIMEHarness(t)
	h.focus(t)
	h.ime.preedit("nihao", 5, true, nil)
	callsBefore := len(h.context.calls)
	h.ime.lost()
	composition, composing := h.editor.Composition()
	if composing || h.editor.String() != "nihao" {
		t.Fatalf("text %q, composition %+v (%v); the marked text is kept as typed", h.editor.String(), composition, composing)
	}
	keymap, event := letterKey(0)
	if h.ime.processKey(keymap.keysym(event.Detail, event.State), event, platform.KeyEvent{Text: "n"}) {
		t.Fatal("a lost input method still claims keys")
	}
	h.ime.sync() // must not touch the dead context
	if len(h.context.calls) != callsBefore {
		t.Fatalf("the dead context was called again: %v", h.context.calls[callsBefore:])
	}
}

func TestIMEFailedSendFallsBackToTheField(t *testing.T) {
	h := newIMEHarness(t)
	h.focus(t)
	h.context.err = errors.New("broken pipe")
	keymap, event := letterKey(0)
	if h.ime.processKey(keymap.keysym(event.Detail, event.State), event, platform.KeyEvent{Text: "n"}) {
		t.Fatal("a key whose send failed was claimed")
	}
}

func TestIMECloseDestroysTheContext(t *testing.T) {
	h := newIMEHarness(t)
	h.ime.close()
	if !h.context.closed || !h.ime.dead {
		t.Fatalf("closed=%v dead=%v", h.context.closed, h.ime.dead)
	}
}

func TestKeymapKeysymFollowsShiftAndCapsLock(t *testing.T) {
	keymap := &xcbKeymap{first: 10, last: 11, perKeycode: 2, keysyms: []uint32{'a', 'A', '1', '!'}}
	for _, test := range []struct {
		keycode uint8
		state   uint16
		want    uint32
	}{
		{10, 0, 'a'}, {10, xcbModShift, 'A'}, {10, xcbModLock, 'A'}, {10, xcbModShift | xcbModLock, 'a'},
		{11, 0, '1'}, {11, xcbModShift, '!'}, {11, xcbModLock, '1'}, // Caps Lock does not shift a digit
		{9, 0, 0}, {12, 0, 0},
	} {
		if got := keymap.keysym(test.keycode, test.state); got != test.want {
			t.Errorf("keysym(%d, %#x) = %#x, want %#x", test.keycode, test.state, got, test.want)
		}
	}
}

// The caret is logical; the engine's candidate window is placed in screen pixels.
func TestIMECaretIsScaledToScreenPixels(t *testing.T) {
	h := newIMEHarness(t)
	h.focus(t)
	one := h.context.cursors[0]
	h.ime.router.scale = func() float32 { return 2 }
	h.ime.haveCursor = false
	h.ime.updateCursor()
	two := h.context.cursors[len(h.context.cursors)-1]
	// The window is at 300,200 on the screen whatever the scale; the caret's offset
	// from it, and its size, double.
	if two[0]-300 != 2*(one[0]-300) || two[1]-200 != 2*(one[1]-200) || two[3] < 2*one[3]-1 || two[3] > 2*one[3]+1 {
		t.Fatalf("caret at 1x %v, at 2x %v", one, two)
	}
}

func TestIMETellsTheEngineTheTextAroundTheCaretAndOnlyWhenItChanges(t *testing.T) {
	h := newIMEHarness(t)
	h.focus(t)
	if got := h.context.surrounds; len(got) != 1 || got[0] != (fakeSurround{"", 0, 0}) {
		t.Fatalf("first surrounding text = %+v", got)
	}
	h.ime.commit("你好")
	if got := h.context.surrounds[len(h.context.surrounds)-1]; got != (fakeSurround{"你好", 2, 2}) {
		t.Fatalf("surrounding text after typing = %+v", got)
	}
	before := len(h.context.surrounds)
	h.ime.sync()
	if len(h.context.surrounds) != before {
		t.Fatal("unchanged text was told to the engine again")
	}
	h.ime.preedit("ni", 2, true, nil)
	if got := h.context.surrounds[len(h.context.surrounds)-1]; got != (fakeSurround{"你好", 2, 2}) {
		t.Fatalf("the engine was told its own preedit: %+v", got)
	}
}

func TestIMEPasswordFieldsTellTheEngineNothing(t *testing.T) {
	h := newIMEHarness(t)
	h.editor.Insert("hunter2")
	h.input.state.Value, h.input.state.Selection = h.editor.String(), h.editor.Selection()
	h.input.input.Secure = true
	h.focus(t)
	for _, got := range h.context.surrounds {
		if got.text != "" {
			t.Fatalf("a password reached the input method: %+v", got)
		}
	}
}

func TestIMEDeleteSurroundingRemovesTheTextTheEngineAsksFor(t *testing.T) {
	h := newIMEHarness(t)
	h.focus(t)
	h.ime.commit("春天来了")
	h.ime.deleteSurrounding(-2, 2)
	if h.editor.String() != "春天" {
		t.Fatalf("field = %q", h.editor.String())
	}
	h.ime.deleteSurrounding(-9, 1) // outside what the engine was told
	if h.editor.String() != "春天" {
		t.Fatalf("a request outside the window changed the field: %q", h.editor.String())
	}
}

func TestIMEPreeditShowsTheClauseBeingConvertedAsSelected(t *testing.T) {
	h := newIMEHarness(t)
	h.focus(t)
	h.ime.preedit("きょうは", 4, true, []ibus.Attribute{{Kind: ibus.AttrUnderline, Value: 1, Start: 0, End: 4}, {Kind: ibus.AttrBackground, Start: 0, End: 3}})
	if selection := h.editor.Selection(); selection != (text.Range{Start: 0, End: 3}) {
		t.Fatalf("selection = %+v, want the converted clause", selection)
	}
	h.ime.preedit("きょうは", 4, true, []ibus.Attribute{{Kind: ibus.AttrUnderline, Value: 1, Start: 0, End: 4}})
	if selection := h.editor.Selection(); selection != (text.Range{Start: 4, End: 4}) {
		t.Fatalf("selection = %+v, want the caret", selection)
	}
}

func TestDestroyingAWaylandWindowClosesItsIMEContextOnce(t *testing.T) {
	h := newIMEHarness(t)
	h.focus(t)
	win := &waylandWindow{state: &waylandWindowState{router: eventRouter{ime: h.ime}}}

	win.destroy()
	if h.context.closes != 1 {
		t.Fatalf("destroying the window closed the context %d times, want 1", h.context.closes)
	}
	if !h.ime.dead || h.ime.context != nil {
		t.Fatalf("closed input method: dead=%v context=%v", h.ime.dead, h.ime.context)
	}

	// Destroy runs again when the same window is closed twice; it must not close
	// the context a second time.
	win.destroy()
	if h.context.closes != 1 {
		t.Fatalf("destroying the closed window again closed the context %d times, want 1", h.context.closes)
	}
}

func TestDestroyingAWaylandWindowWithoutAnIMEIsSafe(t *testing.T) {
	win := &waylandWindow{state: &waylandWindowState{}}
	win.destroy()
	win.destroy()
}

func TestClosingAWaylandWindowsIMEContextLeavesAnotherWindowUsable(t *testing.T) {
	closed, live := newIMEHarness(t), newIMEHarness(t)
	closed.focus(t)
	live.focus(t)

	closedWin := &waylandWindow{state: &waylandWindowState{router: eventRouter{ime: closed.ime}}}
	liveWin := &waylandWindow{state: &waylandWindowState{router: eventRouter{ime: live.ime}}}
	closedWin.destroy()

	if closed.context.closes != 1 {
		t.Fatalf("the closed window's context was closed %d times, want 1", closed.context.closes)
	}
	if live.context.closed || live.ime.dead || live.ime.context == nil {
		t.Fatalf("closing one window disturbed another's input method: closed=%v dead=%v context=%v", live.context.closed, live.ime.dead, live.ime.context)
	}

	// The surviving window keeps its own context, and still sends its keys there.
	keymap, event := letterKey(0)
	if !live.ime.processKey(keymap.keysym(event.Detail, event.State), event, platform.KeyEvent{Text: "n"}) {
		t.Fatal("the surviving window's input method no longer takes keys")
	}
	if len(live.context.keys) != 1 {
		t.Fatalf("the surviving window's context received %d keys, want 1", len(live.context.keys))
	}

	liveWin.destroy()
	if live.context.closes != 1 {
		t.Fatalf("the surviving window's context was closed %d times, want 1", live.context.closes)
	}
}
