package linux

import (
	"errors"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/ibus"
	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/text"
)

type tentativeIMEHarness struct {
	*imeHarness
	events []platform.TextEvent
}

func newTentativeIMEHarness(t *testing.T) *tentativeIMEHarness {
	t.Helper()
	h := &tentativeIMEHarness{imeHarness: newIMEHarness(t)}
	h.editor = text.NewEditor("abc")
	h.editor.SetSelection(text.Range{Start: 1, End: 2})
	if !h.editor.EnableTentativeComposition() {
		t.Fatal("tentative capability refused")
	}
	field := h.input.input
	field.Target = gfx.Item(42)
	field.State = platform.EditorState(h.editor)
	field.OnEvent = func(event platform.TextEvent) (platform.TextState, error) {
		h.events = append(h.events, event)
		event.Apply(h.editor)
		return platform.EditorState(h.editor), nil
	}
	if _, _, err := h.input.prepare(platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: &field}}); err != nil {
		t.Fatal(err)
	}
	owner, ok := any(h.ime).(interface{ bindOwner() })
	if !ok {
		t.Fatal("IBus frontend has no native composition owner binding")
	}
	owner.bindOwner()
	h.focus(t)
	return h
}

func (h *tentativeIMEHarness) preedit(t *testing.T, event ibus.PreeditEvent) {
	t.Helper()
	frontend, ok := any(h.ime).(interface{ nativePreedit(ibus.PreeditEvent) })
	if !ok {
		t.Fatal("IBus frontend drops native preedit provenance")
	}
	frontend.nativePreedit(event)
}

func nativeMode(value string, mode uint32) ibus.PreeditEvent {
	return ibus.PreeditEvent{Text: value, Cursor: len([]rune(value)), Visible: true, HasMode: true, Mode: mode}
}

func (h *tentativeIMEHarness) reset(t *testing.T) error {
	t.Helper()
	frontend, ok := any(h.ime).(interface{ reset() error })
	if !ok {
		t.Fatal("IBus frontend cannot settle native Reset mode")
	}
	return frontend.reset()
}

func TestIBusTentativeResultsRollbackAndPreserveOrdinarySelectionDeletion(t *testing.T) {
	for _, empty := range []bool{false, true} {
		h := newTentativeIMEHarness(t)
		h.preedit(t, nativeMode("你好", ibus.PreeditModeClear))
		if empty {
			h.preedit(t, nativeMode("", ibus.PreeditModeClear))
			if !h.ime.composing() || h.input.state.Composing {
				t.Fatal("empty Update lost its live native session")
			}
		}
		h.ime.commit("好")
		if h.editor.String() != "a好c" || h.editor.CompositionActive() {
			t.Fatalf("result=%q active=%t", h.editor.String(), h.editor.CompositionActive())
		}
		for index, event := range h.events {
			want := text.CompositionUpdate
			if index == len(h.events)-1 {
				want = text.CompositionResult
			}
			if event.CompositionPhase != want {
				t.Fatalf("event[%d]=%+v want phase=%d", index, event, want)
			}
		}
		if !h.editor.Undo() || h.editor.String() != "ac" || !h.editor.Undo() || h.editor.String() != "abc" {
			t.Fatalf("result Undo sequence ended at %q", h.editor.String())
		}
	}
}

func TestIBusTentativeHideShowIsVisibilityWithoutATerminal(t *testing.T) {
	h := newTentativeIMEHarness(t)
	h.preedit(t, nativeMode("你好", ibus.PreeditModeClear))
	h.preedit(t, ibus.PreeditEvent{Kind: ibus.PreeditHide})
	if !h.ime.composing() || h.editor.String() != "ac" {
		t.Fatalf("hidden session=%q active=%t", h.editor.String(), h.ime.composing())
	}
	h.ime.sync()
	h.preedit(t, ibus.PreeditEvent{Kind: ibus.PreeditShow, Visible: true})
	if h.editor.String() != "a你好c" || !h.editor.CompositionActive() {
		t.Fatal("Show did not restore the retained native preedit")
	}
	for _, event := range h.events {
		if event.CompositionPhase != text.CompositionUpdate {
			t.Fatalf("visibility fabricated a terminal: %+v", event)
		}
	}
}

func TestIBusTentativeEscapeWaitsForNativeClearingAndTheKeyAnswer(t *testing.T) {
	for _, clears := range []bool{false, true} {
		h := newTentativeIMEHarness(t)
		h.preedit(t, nativeMode("你好", ibus.PreeditModeClear))
		if !h.ime.processKey(0xff1b, &xcbInputEvent{Detail: 9}, platform.KeyEvent{}) {
			t.Fatal("Escape was not sent")
		}
		if clears {
			h.preedit(t, ibus.PreeditEvent{Kind: ibus.PreeditHide})
			if !h.ime.composing() {
				t.Fatal("Hide canceled before the native key answer")
			}
		}
		h.context.keys[0].done(true)
		if h.ime.composing() == clears {
			t.Fatalf("Escape clearing=%t active=%t", clears, h.ime.composing())
		}
		if clears && (h.editor.String() != "ac" || h.events[len(h.events)-1].CompositionPhase != text.CompositionCancel || !h.editor.Undo() || h.editor.String() != "abc") {
			t.Fatalf("canceled Escape state=%q events=%+v", h.editor.String(), h.events)
		}
	}
}

func TestIBusTentativeResetAndFocusUseTheNegotiatedNativeMode(t *testing.T) {
	for _, focusOut := range []bool{false, true} {
		for _, mode := range []uint32{ibus.PreeditModeClear, ibus.PreeditModeCommit} {
			h := newTentativeIMEHarness(t)
			h.preedit(t, nativeMode("你好", mode))
			if focusOut {
				h.ime.setFocused(false)
			} else if err := h.reset(t); err != nil {
				t.Fatal(err)
			}
			want, phase := "ac", text.CompositionCancel
			if mode == ibus.PreeditModeCommit {
				want, phase = "a你好c", text.CompositionResult
			}
			if h.editor.String() != want || h.editor.CompositionActive() || h.events[len(h.events)-1].CompositionPhase != phase {
				t.Fatalf("mode=%d focusOut=%t text=%q events=%+v", mode, focusOut, h.editor.String(), h.events)
			}
		}
	}
}

func TestIBusTentativeUnknownModeRefusesSettlementWithoutGuessing(t *testing.T) {
	h := newTentativeIMEHarness(t)
	h.preedit(t, nativeMode("你好", 99))
	if err := h.reset(t); err == nil || h.editor.String() != "a你好c" || !h.editor.CompositionActive() {
		t.Fatalf("unknown mode reset=%v text=%q active=%t", err, h.editor.String(), h.editor.CompositionActive())
	}
}

func TestIBusTentativeCallbacksCannotEditAReplacementOwner(t *testing.T) {
	h := newTentativeIMEHarness(t)
	h.preedit(t, nativeMode("你好", ibus.PreeditModeClear))
	h.ime.processKey('x', &xcbInputEvent{Detail: 53}, platform.KeyEvent{Text: "x"})
	h.editor = text.NewEditor("replacement")
	h.editor.EnableTentativeComposition()
	field := h.input.input
	field.State = platform.EditorState(h.editor)
	if _, _, err := h.input.prepare(platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: &field}}); err != nil {
		t.Fatal(err)
	}
	h.ime.commit("stale")
	h.preedit(t, nativeMode("old", ibus.PreeditModeClear))
	h.context.keys[0].done(false)
	if h.editor.String() != "replacement" || h.editor.CompositionActive() {
		t.Fatalf("old native context edited replacement: %q", h.editor.String())
	}
	h.ime.sync()
	if !h.context.closed {
		t.Fatal("replacement owner retained the old native context")
	}
}

func TestIBusTentativeHandlerFailureStopsNativeTailCallbacks(t *testing.T) {
	h := newTentativeIMEHarness(t)
	err := errors.New("handler failed")
	h.input.input.OnEvent = func(platform.TextEvent) (platform.TextState, error) { return platform.TextState{}, err }
	h.preedit(t, nativeMode("你好", ibus.PreeditModeClear))
	h.ime.commit("tail")
	if h.ime.app.firstErr != err || h.editor.String() != "abc" {
		t.Fatalf("failure=%v text=%q", h.ime.app.firstErr, h.editor.String())
	}
}

func TestIBusTentativeEmptySessionKeepsControlShortcutsOnTheEnginePath(t *testing.T) {
	h := newTentativeIMEHarness(t)
	h.preedit(t, nativeMode("你好", ibus.PreeditModeClear))
	h.preedit(t, nativeMode("", ibus.PreeditModeClear))
	calls := 0
	h.ime.router.config.OnShortcut = func(platform.KeyEvent) (platform.Frame, bool, error) {
		calls++
		return platform.Frame{}, true, nil
	}
	keymap, event := letterKey(0x4)
	if err := h.ime.router.key(keymap, event, false); err != nil {
		t.Fatal(err)
	}
	if calls != 0 || len(h.context.keys) != 1 {
		t.Fatalf("empty native session bypassed engine: shortcuts=%d keys=%d", calls, len(h.context.keys))
	}
	h.context.keys[0].done(true)
	if calls != 0 {
		t.Fatal("consumed composition chord reached a shortcut")
	}
	if err := h.ime.router.deliverTypedWithShortcut(platform.KeyEvent{}, 0, platform.KeyEvent{Key: input.KeyUnknown, Text: "n"}); err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatal("a hidden native session ran a plain shortcut")
	}
}

func TestIBusTentativeDaemonLossKeepsAcceptedVisibleText(t *testing.T) {
	h := newTentativeIMEHarness(t)
	h.preedit(t, nativeMode("你好", ibus.PreeditModeClear))
	h.ime.lost()
	if h.editor.String() != "a你好c" || h.editor.CompositionActive() || !h.ime.dead || h.events[len(h.events)-1].CompositionPhase != text.CompositionCommit {
		t.Fatalf("lost daemon state=%q active=%t events=%+v", h.editor.String(), h.editor.CompositionActive(), h.events)
	}
	h.ime.commit("stale")
	h.preedit(t, nativeMode("old", ibus.PreeditModeClear))
	if h.editor.String() != "a你好c" {
		t.Fatal("lost daemon callback edited the field")
	}
}

func TestIBusTentativeDisabledFieldRetiresWithoutKillingTheWindowIME(t *testing.T) {
	h := newTentativeIMEHarness(t)
	if _, _, err := h.input.prepare(platform.Frame{TextInput: platform.TextInputUpdate{Changed: true}}); err != nil {
		t.Fatal(err)
	}
	h.ime.sync()
	if !h.context.closed || h.ime.dead || h.ime.context != nil {
		t.Fatalf("temporary disable: closed=%t dead=%t context=%v", h.context.closed, h.ime.dead, h.ime.context)
	}
}

func TestIBusTentativeFocusOutWaitsForTheLastNativeKeyResult(t *testing.T) {
	h := newTentativeIMEHarness(t)
	h.preedit(t, nativeMode("nihao", ibus.PreeditModeClear))
	h.ime.processKey(' ', &xcbInputEvent{Detail: 65}, platform.KeyEvent{Text: " "})
	calls := len(h.context.calls)
	h.ime.setFocused(false)
	if len(h.context.calls) != calls || !h.ime.active {
		t.Fatalf("focus-out raced pending result: calls=%v active=%t", h.context.calls[calls:], h.ime.active)
	}
	h.ime.commit("你好")
	h.context.keys[0].done(true)
	if h.editor.String() != "a你好c" || h.ime.active || h.context.calls[len(h.context.calls)-1] != "FocusOut" {
		t.Fatalf("pending result lost: text=%q active=%t calls=%v", h.editor.String(), h.ime.active, h.context.calls)
	}
	if h.events[len(h.events)-1].CompositionPhase != text.CompositionResult {
		t.Fatal("pending native Result was replaced by Cancel")
	}
}

func TestIBusTentativeReentrantRefocusSupersedesTheOuterFocusOut(t *testing.T) {
	h := newTentativeIMEHarness(t)
	h.preedit(t, nativeMode("你好", ibus.PreeditModeClear))
	original := h.input.input.OnEvent
	h.input.input.OnEvent = func(event platform.TextEvent) (platform.TextState, error) {
		state, err := original(event)
		if event.CompositionPhase == text.CompositionCancel {
			h.ime.setFocused(true)
		}
		return state, err
	}
	h.ime.setFocused(false)
	if !h.ime.active || !h.ime.focused || h.context.calls[len(h.context.calls)-1] == "FocusOut" {
		t.Fatalf("outer FocusOut undid refocus: active=%t focused=%t calls=%v", h.ime.active, h.ime.focused, h.context.calls)
	}
}

func TestIBusTentativeSyncStopsWhenACursorSendRetiresTheContext(t *testing.T) {
	h := newTentativeIMEHarness(t)
	h.preedit(t, nativeMode("你好", ibus.PreeditModeClear))
	h.ime.haveCursor, h.ime.hasSources = false, false
	h.context.onCall = func(name string) {
		if name == "SetCursorLocation" {
			h.ime.lost()
		}
	}
	h.ime.sync()
	if !h.ime.dead || h.ime.context != nil || h.editor.String() != "a你好c" {
		t.Fatalf("retired cursor send: dead=%t context=%v text=%q", h.ime.dead, h.ime.context, h.editor.String())
	}
}

func TestIBusTentativeReentrantAcknowledgmentRetiresTheSupersededNativeEvent(t *testing.T) {
	for _, kind := range []string{"update", "result", "direct"} {
		t.Run(kind, func(t *testing.T) { reentrantIBusAcknowledgment(t, kind) })
	}
}

func reentrantIBusAcknowledgment(t *testing.T, kind string) {
	t.Helper()
	h := newTentativeIMEHarness(t)
	if kind == "result" {
		h.preedit(t, nativeMode("old", ibus.PreeditModeClear))
	}
	want := h.editor.String()
	h.input.input.OnEvent = func(platform.TextEvent) (platform.TextState, error) {
		end := h.editor.RuneCount()
		h.editor.SetSelection(text.Range{Start: end, End: end})
		field := h.input.input
		field.State = platform.EditorState(h.editor)
		if _, _, err := h.input.prepare(platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: &field}}); err != nil {
			t.Fatal(err)
		}
		return field.State, nil
	}
	if kind == "update" {
		h.preedit(t, nativeMode("old", ibus.PreeditModeClear))
	} else {
		h.ime.commit("ignored")
	}
	h.ime.sync()
	h.input.input.OnEvent = func(event platform.TextEvent) (platform.TextState, error) {
		event.Apply(h.editor)
		return platform.EditorState(h.editor), nil
	}
	h.ime.commit("stale")
	if h.editor.String() != want || !h.context.closed {
		t.Fatalf("superseded native edit survived acknowledgment: %q closed=%t", h.editor.String(), h.context.closed)
	}
}

func TestIBusTentativeDeferredFocusOutStopsAfterAHandlerError(t *testing.T) {
	h := newTentativeIMEHarness(t)
	h.preedit(t, nativeMode("old", ibus.PreeditModeClear))
	h.ime.processKey('n', &xcbInputEvent{Detail: 57}, platform.KeyEvent{Text: "n"})
	h.ime.setFocused(false)
	calls := 0
	err := errors.New("pending handler failed")
	h.input.input.OnEvent = func(platform.TextEvent) (platform.TextState, error) {
		calls++
		return platform.TextState{}, err
	}
	h.preedit(t, nativeMode("new", ibus.PreeditModeClear))
	h.context.keys[0].done(true)
	if calls != 1 || h.ime.app.firstErr != err {
		t.Fatalf("error tail ran another handler: calls=%d error=%v", calls, h.ime.app.firstErr)
	}
}

func TestIBusPublicBaseTentativePreeditUsesExplicitPhases(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		name := "legacy"
		if enabled {
			name = "tentative"
		}
		t.Run(name, func(t *testing.T) {
			h := newIMEHarness(t)
			h.editor = text.NewEditor("abc")
			h.editor.SetSelection(text.Range{Start: 1, End: 2})
			if enabled && !h.editor.EnableTentativeComposition() {
				t.Fatal("tentative capability refused")
			}
			var events []platform.TextEvent
			field := h.input.input
			field.Target, field.State = gfx.Item(42), platform.EditorState(h.editor)
			field.OnEvent = func(event platform.TextEvent) (platform.TextState, error) {
				events = append(events, event)
				event.Apply(h.editor)
				return platform.EditorState(h.editor), nil
			}
			if _, _, err := h.input.prepare(platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: &field}}); err != nil {
				t.Fatal(err)
			}
			if owner, ok := any(h.ime).(interface{ bindOwner() }); ok {
				owner.bindOwner()
			}
			h.focus(t)
			if frontend, ok := any(h.ime).(interface{ nativePreedit(ibus.PreeditEvent) }); ok {
				frontend.nativePreedit(ibus.PreeditEvent{Text: "ni", Cursor: 2, Visible: true, HasMode: true, Mode: ibus.PreeditModeClear})
			} else {
				h.ime.preedit("ni", 2, true, text.Range{Start: 2, End: 2})
			}
			h.ime.commit("z")
			if len(events) != 2 || h.editor.String() != "azc" {
				t.Fatalf("native text=%q events=%+v", h.editor.String(), events)
			}
			for index, event := range events {
				want := text.CompositionNone
				if enabled {
					want = text.CompositionUpdate
					if index == 1 {
						want = text.CompositionResult
					}
				}
				if event.CompositionPhase != want {
					t.Errorf("native phase=%d, want %d for event %d", event.CompositionPhase, want, index)
				}
			}
			if enabled && (!h.editor.Undo() || h.editor.String() != "ac" || !h.editor.Undo() || h.editor.String() != "abc") {
				t.Fatalf("native Result Undo sequence=%q", h.editor.String())
			}
		})
	}
}

// refusingPreeditContext stands in for a daemon that does not implement the
// optional ClientCommitPreedit property.
type refusingPreeditContext struct{}

func (refusingPreeditContext) SetClientCommitPreedit(bool) error {
	return errors.New("ibus: ClientCommitPreedit is not supported")
}

func TestIBusTentativeNegotiationRefusalKeepsLegacyInput(t *testing.T) {
	h := newTentativeIMEHarness(t)
	if !h.ime.tentative || h.ime.phase == nil {
		t.Fatal("tentative owner did not start with a native phase")
	}
	if h.ime.negotiateTentative(refusingPreeditContext{}) {
		t.Fatal("refused negotiation reported explicit phases")
	}
	if h.ime.phase != nil {
		t.Fatal("refused negotiation kept the native phase")
	}
	if !h.ime.sameOwner() || !h.ime.current() {
		t.Fatal("refusing the optional property orphaned the field")
	}
	h.preedit(t, ibus.PreeditEvent{Text: "ni", Cursor: 2, Visible: true})
	if !h.editor.CompositionActive() {
		t.Fatal("legacy preedit did not reach the field")
	}
	last := h.events[len(h.events)-1]
	if last.CompositionPhase != text.CompositionNone {
		t.Fatalf("fallback preedit used phase %v", last.CompositionPhase)
	}
}

func TestIBusTentativeUnsupportedResetModeRetiresWithoutFailingTheApp(t *testing.T) {
	h := newTentativeIMEHarness(t)
	h.preedit(t, ibus.PreeditEvent{Text: "你好", Cursor: 2, Visible: true})
	if !h.ime.composing() {
		t.Fatal("native session did not start")
	}
	h.ime.setFocused(false)
	if h.ime.app.firstErr != nil {
		t.Fatalf("a reset error was escalated to the application: %v", h.ime.app.firstErr)
	}
	if h.ime.context != nil || !h.context.closed {
		t.Fatal("the native session was not retired")
	}
	if got := h.editor.String(); got != "a你好c" {
		t.Fatalf("retiring the session dropped the visible text: %q", got)
	}
}
