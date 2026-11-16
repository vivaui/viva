package app

import (
	"runtime"
	"testing"
	"time"
	"unsafe"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/text"
)

func TestTextEventApplyForwardsCompositionFields(t *testing.T) {
	editor := text.NewEditor("abcd")
	if !editor.EnableTentativeComposition() {
		t.Fatal("capability setup refused")
	}
	editor.SetSelection(text.Range{Start: 1, End: 2})
	TextEvent{
		Kind: TextSetMarked, CompositionPhase: text.CompositionUpdate, Text: "\u00e9\U0001d11e",
		MarkedSelection: TextRange{Start: 2, End: 2},
	}.Apply(editor)
	TextEvent{
		Kind: TextInsert, CompositionPhase: text.CompositionResult, Text: "r",
		HasReplacement: true, Replacement: TextRange{Start: 0, End: 1},
		HasReplacementUTF16: true, ReplacementUTF16: text.UTF16Range{Start: 5, Length: 1},
	}.Apply(editor)
	if got := editor.String(); got != "acdr" {
		t.Fatalf("app Apply raw result = %q, want acdr", got)
	}
}

func TestNativeTextEventAdapterPreservesTaggedFields(t *testing.T) {
	event := platform.TextEvent{
		Kind: platform.TextInsert, Text: "native", Command: platform.TextUndo,
		Selection: text.Range{Start: 1, End: 2}, MarkedSelection: text.Range{Start: 3, End: 4},
		Replacement: text.Range{Start: 5, End: 6}, HasReplacement: true,
		CompositionPhase: text.CompositionResult,
		ReplacementUTF16: text.UTF16Range{Start: 7, Length: 8}, HasReplacementUTF16: true,
	}
	want := TextEvent{
		Kind: TextInsert, Text: "native", Command: TextUndo,
		Selection: TextRange{Start: 1, End: 2}, MarkedSelection: TextRange{Start: 3, End: 4},
		Replacement: TextRange{Start: 5, End: 6}, HasReplacement: true,
		CompositionPhase: text.CompositionResult,
		ReplacementUTF16: text.UTF16Range{Start: 7, Length: 8}, HasReplacementUTF16: true,
	}
	if got := appTextEvent(event); got != want {
		t.Fatalf("native event adaptation = %+v, want %+v", got, want)
	}
	paste := platform.TextEvent{Kind: platform.TextPaste, Text: "column\r\n", Rectangular: true}
	if got := appTextEvent(paste); got != (TextEvent{Kind: TextPaste, Text: "column\r\n", Rectangular: true}) {
		t.Fatalf("native column paste adaptation = %+v", got)
	}
}

func TestTextStateOfTentativeCompositionSnapshot(t *testing.T) {
	editor := text.NewEditor("abc")
	if TextStateOf(editor).TentativeComposition {
		t.Fatal("default editor reported the opt-in capability")
	}
	if !editor.EnableTentativeComposition() || !TextStateOf(editor).TentativeComposition {
		t.Fatal("app snapshot did not follow opt-in")
	}
	identity := editor.TentativeCompositionIdentity()
	TextEvent{Kind: TextSetMarked, CompositionPhase: text.CompositionUpdate, Text: "x"}.Apply(editor)
	if state := TextStateOf(editor); !state.Composing || !state.TentativeComposition || platformTextState(state).CompositionIdentity != identity {
		t.Fatalf("live app state = %+v", state)
	}
	TextEvent{Kind: TextInsert, CompositionPhase: text.CompositionCommit}.Apply(editor)
	if state := TextStateOf(editor); state.Composing || !state.TentativeComposition {
		t.Fatalf("idle app state = %+v", state)
	}
	if !editor.DisableTentativeComposition() {
		t.Fatal("disable refused")
	}
	if state := TextStateOf(editor); state.Composing || state.TentativeComposition || platformTextState(state).CompositionIdentity != (text.CompositionIdentity{}) {
		t.Fatalf("passive disabled app state = %+v", state)
	}
}

func TestPainterApplyTextEventCopiesTaggedFieldsAndCapability(t *testing.T) {
	editor := text.NewEditor("abcd")
	if !editor.EnableTentativeComposition() {
		t.Fatal("capability setup refused")
	}
	editor.SetSelection(text.Range{Start: 1, End: 2})
	owner := &painter{activeText: &TextInput{Editor: editor}, nativeText: platform.TextInput{Multiline: true}}
	state, err := owner.applyTextEvent(platform.TextEvent{
		Kind: platform.TextSetMarked, CompositionPhase: text.CompositionUpdate, Text: "\u00e9\U0001d11e",
		MarkedSelection: text.Range{Start: 2, End: 2},
	})
	if err != nil || !state.TentativeComposition || !state.Composing ||
		!owner.activeText.State.TentativeComposition || !owner.nativeText.State.TentativeComposition {
		t.Fatalf("direct Update adaptation lost capability: %+v, %v", state, err)
	}
	state, err = owner.applyTextEvent(platform.TextEvent{
		Kind: platform.TextInsert, CompositionPhase: text.CompositionResult, Text: "r",
		HasReplacementUTF16: true, ReplacementUTF16: text.UTF16Range{Start: 5, Length: 1},
	})
	if err != nil || state.Value != "acdr" || state.Composing || !state.TentativeComposition || owner.nativeText.State != state {
		t.Fatalf("direct Result adaptation lost raw replacement/state: %+v, %v", state, err)
	}
	var received TextEvent
	owner.activeText = &TextInput{OnEvent: func(event TextEvent) (TextState, error) {
		received = event
		return TextState{Value: "handled", TentativeComposition: true, Redraw: true}, nil
	}}
	event := platform.TextEvent{
		Kind: platform.TextInsert, CompositionPhase: text.CompositionResult, Text: "native",
		Selection: text.Range{Start: 7, End: 8}, MarkedSelection: text.Range{Start: 2, End: 3},
		HasReplacement: true, Replacement: text.Range{Start: 4, End: 5},
		HasReplacementUTF16: true, ReplacementUTF16: text.UTF16Range{Start: 9, Length: 11},
	}
	state, err = owner.applyTextEvent(event)
	if err != nil || received.CompositionPhase != event.CompositionPhase || received.Text != event.Text ||
		received.ReplacementUTF16 != event.ReplacementUTF16 || !received.HasReplacementUTF16 ||
		received.Selection != (TextRange{Start: 7, End: 8}) || received.MarkedSelection != (TextRange{Start: 2, End: 3}) ||
		!received.HasReplacement || received.Replacement != (TextRange{Start: 4, End: 5}) {
		t.Fatalf("custom handler adaptation lost fields: %+v, %v", received, err)
	}
	if !state.TentativeComposition || !state.Redraw || !owner.nativeText.State.TentativeComposition || owner.nativeText.State.Redraw {
		t.Fatal("returned/mirrored capability or one-shot redraw changed")
	}
}

func TestPainterTaggedCompositionKeepsSingleLineSanitation(t *testing.T) {
	editor := text.NewEditor("abc")
	if !editor.EnableTentativeComposition() {
		t.Fatal("capability setup refused")
	}
	editor.SetSelection(text.Range{Start: 1, End: 1})
	owner := &painter{activeText: &TextInput{Editor: editor}, nativeText: platform.TextInput{}}
	if _, err := owner.applyTextEvent(platform.TextEvent{
		Kind: platform.TextSetMarked, CompositionPhase: text.CompositionUpdate, Text: "p",
	}); err != nil {
		t.Fatal(err)
	}
	state, err := owner.applyTextEvent(platform.TextEvent{
		Kind: platform.TextInsert, CompositionPhase: text.CompositionResult, Text: "x\ny",
	})
	if err != nil || editor.String() != "ax ybc" || state.Value != "ax ybc" {
		t.Fatalf("tagged single-line result = %q (state %q), error=%v", editor.String(), state.Value, err)
	}
}

func TestPainterTaggedTerminalSurvivesEmptySingleLineSanitation(t *testing.T) {
	for _, scenario := range []struct {
		name  string
		phase text.CompositionPhase
		want  string
	}{
		{name: "result", phase: text.CompositionResult, want: "ac"},
		{name: "cancel", phase: text.CompositionCancel, want: "ac"},
		{name: "commit", phase: text.CompositionCommit, want: "apc"},
		{name: "empty commit", phase: text.CompositionEmptyCommit, want: "ac"},
	} {
		for _, custom := range []bool{false, true} {
			name := scenario.name + "/editor"
			if custom {
				name = scenario.name + "/handler"
			}
			t.Run(name, func(t *testing.T) {
				editor := text.NewEditor("abc")
				if !editor.EnableTentativeComposition() {
					t.Fatal("capability setup refused")
				}
				editor.SetSelection(text.Range{Start: 1, End: 2})
				owner := &painter{activeText: &TextInput{Editor: editor}}
				if custom {
					owner.activeText = &TextInput{State: TextStateOf(editor), OnEvent: func(event TextEvent) (TextState, error) {
						event.Apply(editor)
						return TextStateOf(editor), nil
					}}
				}
				if _, err := owner.applyTextEvent(platform.TextEvent{Kind: platform.TextSetMarked, CompositionPhase: text.CompositionUpdate, Text: "p"}); err != nil {
					t.Fatal(err)
				}
				ack, err := owner.applyTextEvent(platform.TextEvent{Kind: platform.TextInsert, CompositionPhase: scenario.phase, Text: "\n"})
				if err != nil || editor.String() != scenario.want || editor.CompositionActive() ||
					owner.activeText.State != TextStateOf(editor) || owner.nativeText.State != platform.EditorState(editor) || ack.WithoutRedraw() != platform.EditorState(editor) {
					t.Fatalf("sanitized terminal: editor=%+v active=%+v native=%+v ack=%+v err=%v", TextStateOf(editor), owner.activeText.State, owner.nativeText.State, ack, err)
				}
			})
		}
	}
}

func TestPainterSecureTentativeCompositionRetainsOnlyLiveRollback(t *testing.T) {
	for _, scenario := range []struct {
		name  string
		phase text.CompositionPhase
		want  string
	}{
		{name: "result", phase: text.CompositionResult, want: "arc"},
		{name: "cancel", phase: text.CompositionCancel, want: "ac"},
		{name: "commit", phase: text.CompositionCommit, want: "apc"},
		{name: "empty commit", phase: text.CompositionEmptyCommit, want: "ac"},
		{name: "empty update", phase: text.CompositionUpdate, want: "ac"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			editor := text.NewEditor("abc")
			if !editor.EnableTentativeComposition() {
				t.Fatal("capability setup refused")
			}
			editor.SetSelection(text.Range{Start: 1, End: 2})
			owner := &painter{activeText: &TextInput{Editor: editor}, nativeText: platform.TextInput{Secure: true}}
			if _, err := owner.applyTextEvent(platform.TextEvent{Kind: platform.TextSetMarked, CompositionPhase: text.CompositionUpdate, Text: "p"}); err != nil {
				t.Fatal(err)
			}
			if !editor.CanUndo() {
				t.Error("secure Update discarded the live rollback units")
			}
			for _, command := range []platform.TextCommand{platform.TextUndo, platform.TextRedo} {
				if _, err := owner.applyTextEvent(platform.TextEvent{Kind: platform.TextCommandEvent, Command: command}); err != nil {
					t.Fatal(err)
				}
				if editor.String() != "apc" || !editor.CompositionActive() || !editor.CanUndo() {
					t.Error("secure no-op Undo/Redo changed or discarded live rollback")
				}
			}
			ending := platform.TextEvent{Kind: platform.TextInsert, CompositionPhase: scenario.phase}
			if scenario.phase == text.CompositionResult {
				ending.Text = "r"
			}
			ack, err := owner.applyTextEvent(ending)
			if err != nil || editor.String() != scenario.want || editor.CompositionActive() || editor.CanUndo() || editor.CanRedo() ||
				owner.activeText.State != TextStateOf(editor) || owner.nativeText.State != platform.EditorState(editor) || ack.WithoutRedraw() != platform.EditorState(editor) {
				t.Fatalf("secure settlement: editor=%+v active=%+v native=%+v ack=%+v undo=%t redo=%t err=%v", TextStateOf(editor), owner.activeText.State, owner.nativeText.State, ack, editor.CanUndo(), editor.CanRedo(), err)
			}
		})
	}
}

func TestPainterTaggedTerminalSurvivesSingleLineCommandFilter(t *testing.T) {
	for _, custom := range []bool{false, true} {
		for _, command := range []platform.TextCommand{platform.TextInsertNewline, platform.TextInsertLineBreak} {
			for _, phase := range []text.CompositionPhase{text.CompositionResult, text.CompositionCommit, text.CompositionCancel, text.CompositionEmptyCommit} {
				editor := text.NewEditor("abc")
				if !editor.EnableTentativeComposition() {
					t.Fatal("capability setup refused")
				}
				editor.SetSelection(text.Range{Start: 1, End: 2})
				owner := &painter{activeText: &TextInput{Editor: editor}}
				calls := 0
				if custom {
					owner.activeText = &TextInput{State: TextStateOf(editor), OnEvent: func(event TextEvent) (TextState, error) {
						calls++
						event.Apply(editor)
						return TextStateOf(editor), nil
					}}
				}
				if _, err := owner.applyTextEvent(platform.TextEvent{Kind: platform.TextSetMarked, CompositionPhase: text.CompositionUpdate, Text: "p"}); err != nil {
					t.Fatal(err)
				}
				ack, err := owner.applyTextEvent(platform.TextEvent{Kind: platform.TextCommandEvent, Command: command, CompositionPhase: phase})
				want := "ac"
				if phase == text.CompositionCommit {
					want = "apc"
				}
				if err != nil || editor.String() != want || editor.CompositionActive() ||
					owner.activeText.State != TextStateOf(editor) || owner.nativeText.State != platform.EditorState(editor) || ack.WithoutRedraw() != platform.EditorState(editor) || custom && calls != 2 {
					t.Errorf("tagged command suppressed: custom=%t command=%v phase=%v state=%+v calls=%d ack=%+v err=%v", custom, command, phase, TextStateOf(editor), calls, ack, err)
				}
			}
		}
	}
}

func TestPainterSecureRejectedTaggedEventPreservesHistory(t *testing.T) {
	for _, active := range []bool{false, true} {
		name := "idle"
		if active {
			name = "active"
		}
		t.Run(name, func(t *testing.T) {
			editor := text.NewEditor("abc")
			if !editor.EnableTentativeComposition() {
				t.Fatal("capability setup refused")
			}
			cursor, _, ok := editor.TrackHistory()
			if !ok {
				t.Fatal("history setup refused")
			}
			editor.Type("x")
			editor.SetSavePoint()
			if active && !editor.ApplyComposition(text.CompositionEvent{Phase: text.CompositionUpdate, Text: "p"}) {
				t.Fatal("active setup refused")
			}
			_, cursor, ok = editor.HistorySince(cursor)
			if !ok {
				t.Fatal("history prefix unavailable before refusal")
			}
			before, undo, redo, saved := TextStateOf(editor), editor.CanUndo(), editor.CanRedo(), editor.AtSavePoint()
			owner := &painter{activeText: &TextInput{Editor: editor}, nativeText: platform.TextInput{Secure: true}}
			ack, err := owner.applyTextEvent(platform.TextEvent{Kind: platform.TextInsert, CompositionPhase: text.CompositionPhase(255), Text: "refused"})
			if err != nil || TextStateOf(editor) != before || editor.CanUndo() != undo || editor.CanRedo() != redo || editor.AtSavePoint() != saved || ack.WithoutRedraw() != platform.EditorState(editor) {
				t.Errorf("refused secure event changed source/save/undo: state=%+v undo=%t redo=%t saved=%t err=%v", TextStateOf(editor), editor.CanUndo(), editor.CanRedo(), editor.AtSavePoint(), err)
			}
			if frames, latest, ok := editor.HistorySince(cursor); !ok || len(frames) != 0 || latest != cursor {
				t.Error("refused secure event cleared producer metadata")
			}
		})
	}
}

func TestPainterSecureLegacyCompositionStillClearsHistory(t *testing.T) {
	for _, mode := range []string{"disabled legacy", "enabled legacy", "explicit escape"} {
		t.Run(mode, func(t *testing.T) {
			editor := text.NewEditor("abc")
			if mode != "disabled legacy" && !editor.EnableTentativeComposition() {
				t.Fatal("capability setup refused")
			}
			editor.SetSelection(text.Range{Start: 1, End: 2})
			owner := &painter{activeText: &TextInput{Editor: editor}, nativeText: platform.TextInput{Secure: true}}
			if mode == "explicit escape" {
				if _, err := owner.applyTextEvent(platform.TextEvent{Kind: platform.TextSetMarked, CompositionPhase: text.CompositionUpdate, Text: "p"}); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := owner.applyTextEvent(platform.TextEvent{Kind: platform.TextSetMarked, Text: "q"}); err != nil {
				t.Fatal(err)
			}
			if editor.String() != "aqc" || !editor.CompositionActive() || editor.CanUndo() || editor.CanRedo() {
				t.Fatalf("legacy marking retained password history: state=%+v undo=%t redo=%t", TextStateOf(editor), editor.CanUndo(), editor.CanRedo())
			}
			ack, err := owner.applyTextEvent(platform.TextEvent{Kind: platform.TextInsert, Text: "r"})
			if err != nil || editor.String() != "arc" || editor.CompositionActive() || editor.CanUndo() || editor.CanRedo() || ack.WithoutRedraw() != platform.EditorState(editor) {
				t.Fatalf("legacy result changed secure behavior: state=%+v undo=%t redo=%t err=%v", TextStateOf(editor), editor.CanUndo(), editor.CanRedo(), err)
			}
		})
	}
}

func TestPainterSingleLineLegacyControlTextStaysIgnored(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		for _, phase := range []text.CompositionPhase{text.CompositionNone, text.CompositionResult} {
			if enabled && phase != text.CompositionNone {
				continue
			}
			editor := text.NewEditor("abc")
			if enabled && !editor.EnableTentativeComposition() {
				t.Fatal("capability setup refused")
			}
			calls := 0
			owner := &painter{activeText: &TextInput{State: TextStateOf(editor), OnEvent: func(event TextEvent) (TextState, error) {
				calls++
				return TextStateOf(editor), nil
			}}}
			for _, event := range []platform.TextEvent{
				{Kind: platform.TextInsert, Text: "\n"},
				{Kind: platform.TextCommandEvent, Command: platform.TextInsertNewline},
				{Kind: platform.TextCommandEvent, Command: platform.TextInsertLineBreak},
			} {
				event.CompositionPhase = phase
				if _, err := owner.applyTextEvent(event); err != nil || calls != 0 {
					t.Fatalf("legacy control-only input reached the handler: enabled=%t phase=%v calls=%d err=%v", enabled, phase, calls, err)
				}
			}
		}
	}
}

func TestPainterFocusedStateCarriesTentativeCapability(t *testing.T) {
	editor := text.NewEditor("Go")
	if !editor.EnableTentativeComposition() {
		t.Fatal("capability setup refused")
	}
	var canvas gfx.Canvas
	config := Config{Title: "capability snapshot", Width: 200, Height: 80}
	config.Paint = func(canvas *gfx.Canvas, size gfx.Size) {
		canvas.HitWithOptions(gfx.Item(11), gfx.Rect{Width: 120, Height: 24}, "Name", gfx.HitOptions{Role: gfx.RoleTextField})
	}
	config.TextInput = func(context TextInputContext) *TextInput {
		return &TextInput{Bounds: context.Bounds, FontSize: 13, Editor: editor}
	}
	owner := newTestPainter(t, &canvas, config)
	for _, kind := range []platform.PointerEventKind{platform.PointerMove, platform.PointerDown, platform.PointerUp} {
		if _, err := owner.pointerEvent(platform.PointerEvent{Kind: kind, X: 10, Y: 10}); err != nil {
			t.Fatal(err)
		}
	}
	owner.syncTextInput()
	if !owner.nativeText.State.TentativeComposition || owner.nativeText.State.CompositionIdentity != editor.TentativeCompositionIdentity() {
		t.Fatal("focused native state lost capability ownership")
	}
	if !editor.DisableTentativeComposition() {
		t.Fatal("disable refused")
	}
	owner.syncTextInput()
	if owner.nativeText.State.TentativeComposition || TextStateOf(editor).TentativeComposition {
		t.Fatal("passive disabled owner leaked an enabled snapshot")
	}
}

func TestPainterDistinguishesIdenticalTentativeEditorOwners(t *testing.T) {
	for _, custom := range []bool{false, true} {
		name := "editor"
		if custom {
			name = "custom handler"
		}
		t.Run(name, func(t *testing.T) {
			first, second := text.NewEditor("same"), text.NewEditor("same")
			if !first.EnableTentativeComposition() || !second.EnableTentativeComposition() {
				t.Fatal("capability setup refused")
			}
			current := first
			var canvas gfx.Canvas
			config := Config{Title: "composition ownership", Width: 200, Height: 80}
			config.Paint = func(canvas *gfx.Canvas, size gfx.Size) {
				canvas.HitWithOptions(gfx.Item(11), gfx.Rect{Width: 120, Height: 24}, "Name", gfx.HitOptions{Role: gfx.RoleTextField})
			}
			config.TextInput = func(context TextInputContext) *TextInput {
				owner := current
				input := &TextInput{Bounds: context.Bounds, FontSize: 13, Editor: owner}
				if custom {
					input.Editor = nil
					input.State = TextStateOf(owner)
					input.OnEvent = func(event TextEvent) (TextState, error) {
						event.Apply(owner)
						return TextStateOf(owner), nil
					}
				}
				return input
			}
			owner := newTestPainter(t, &canvas, config)
			for _, kind := range []platform.PointerEventKind{platform.PointerMove, platform.PointerDown, platform.PointerUp} {
				if _, err := owner.pointerEvent(platform.PointerEvent{Kind: kind, X: 10, Y: 10}); err != nil {
					t.Fatal(err)
				}
			}
			if update := owner.syncTextInput(); update.Changed {
				t.Fatal("the same owner's identical snapshot forced input replacement")
			}
			if !first.DisableTentativeComposition() || !first.EnableTentativeComposition() {
				t.Fatal("capability restart refused")
			}
			if update := owner.syncTextInput(); !update.Changed || update.Input == nil ||
				update.Input.State.CompositionIdentity != first.TentativeCompositionIdentity() {
				t.Fatal("re-enabled ownership was not published for the identical target and editor")
			}
			current = second
			if update := owner.syncTextInput(); !update.Changed || update.Input == nil {
				t.Fatal("a different tentative owner with identical state was not replaced")
			}
		})
	}
}

func TestTentativeStateSnapshotDoesNotRetainTheEditor(t *testing.T) {
	released := make(chan struct{})
	makeSnapshot := func() TextState {
		editor := text.NewEditor("snapshot remains readable")
		if !editor.EnableTentativeComposition() {
			t.Fatal("capability setup refused")
		}
		runtime.AddCleanup(editor, func(done chan struct{}) { close(done) }, released)
		return TextStateOf(editor)
	}
	snapshot := makeSnapshot()
	deadline := time.Now().Add(5 * time.Second)
	for {
		runtime.GC()
		select {
		case <-released:
			if snapshot.Value != "snapshot remains readable" {
				t.Fatal("reclaiming the Editor invalidated its value snapshot")
			}
			runtime.KeepAlive(snapshot)
			return
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("a retained value snapshot kept the Editor and history alive")
		}
		runtime.Gosched()
	}
}

// A different focused target must end the composition it started, even when
// the application hands the same editor to the next target. A document keeps
// the text the user can see; a terminal preedit surface discards it, because
// that input was never delivered to the application.
func TestPainterSettlesTentativeCompositionAtTargetHandoff(t *testing.T) {
	for _, surface := range []struct {
		name     string
		role     gfx.Role
		terminal bool
	}{
		{"document", gfx.RoleTextField, false},
		{"terminal preedit", gfx.RoleTerminal, true},
	} {
		for _, reuse := range []bool{false, true} {
			name := surface.name
			if reuse {
				name += " same editor"
			}
			t.Run(name, func(t *testing.T) {
				first, second := text.NewEditor("abc"), text.NewEditor("xyz")
				if !first.EnableTentativeComposition() || !second.EnableTentativeComposition() {
					t.Fatal("capability setup refused")
				}
				if !first.ApplyComposition(text.CompositionEvent{Phase: text.CompositionUpdate, Text: "にほん"}) ||
					!first.CompositionActive() {
					t.Fatal("tentative composition did not start")
				}

				var canvas gfx.Canvas
				config := Config{Title: "composition handoff", Width: 200, Height: 100}
				config.Paint = func(canvas *gfx.Canvas, size gfx.Size) {
					canvas.HitWithOptions(gfx.Item(11), gfx.Rect{Width: 120, Height: 20}, "One", gfx.HitOptions{Role: surface.role})
					canvas.HitWithOptions(gfx.Item(12), gfx.Rect{Y: 40, Width: 120, Height: 20}, "Two", gfx.HitOptions{Role: surface.role})
				}
				config.TextInput = func(context TextInputContext) *TextInput {
					editor := second
					if context.ID == gfx.Item(11) || reuse {
						editor = first
					}
					return &TextInput{Bounds: context.Bounds, FontSize: 13, Editor: editor}
				}
				owner := newTestPainter(t, &canvas, config)
				click := func(x, y float32) {
					t.Helper()
					for _, kind := range []platform.PointerEventKind{platform.PointerMove, platform.PointerDown, platform.PointerUp} {
						if _, err := owner.pointerEvent(platform.PointerEvent{Kind: kind, X: x, Y: y}); err != nil {
							t.Fatal(err)
						}
					}
				}

				click(10, 10)
				owner.syncTextInput()
				if owner.nativeText.Target != gfx.Item(11) || owner.nativeText.Terminal != surface.terminal {
					t.Fatalf("first target not published: target=%v terminal=%t", owner.nativeText.Target, owner.nativeText.Terminal)
				}
				if !first.CompositionActive() {
					t.Fatal("composition ended before the target changed")
				}

				click(10, 50)
				owner.syncTextInput()
				if owner.nativeText.Target != gfx.Item(12) {
					t.Fatalf("handoff target = %v, want item 12", owner.nativeText.Target)
				}
				if first.CompositionActive() {
					t.Fatal("the outgoing editor kept its composition across the handoff")
				}
				if surface.terminal {
					if got := first.String(); got != "abc" {
						t.Fatalf("terminal preedit was not discarded: %q", got)
					}
					return
				}
				if got := first.String(); got != "abcにほん" {
					t.Fatalf("document composition was not committed: %q", got)
				}
				if _, composing := first.Composition(); composing {
					t.Fatal("committed document still reports a composition")
				}
				if !first.Undo() || first.String() != "abc" {
					t.Fatalf("undo did not restore the pre-start text: %q", first.String())
				}
			})
		}
	}
}

var nativeStateBenchmarkSink TextState

func BenchmarkPainterTextAcknowledgment(b *testing.B) {
	editor := text.NewEditor("a")
	owner := &painter{activeText: &TextInput{Editor: editor}, nativeText: platform.TextInput{Multiline: true}}
	b.ReportAllocs()
	b.ResetTimer()
	index := 0
	for b.Loop() {
		index ^= 1
		if _, err := owner.applyTextEvent(platform.TextEvent{Kind: platform.TextSetSelection, Selection: text.Range{Start: index, End: index}}); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportMetric(float64(unsafe.Sizeof(painter{})), "painter-B")
}

func BenchmarkNativeStateSnapshot(b *testing.B) {
	for _, enabled := range []bool{false, true} {
		name := "legacy"
		if enabled {
			name = "tentative"
		}
		b.Run(name, func(b *testing.B) {
			editor := text.NewEditor("state snapshot")
			if enabled && !editor.EnableTentativeComposition() {
				b.Fatal("capability setup refused")
			}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				nativeStateBenchmarkSink = TextStateOf(editor)
			}
			b.ReportMetric(float64(unsafe.Sizeof(TextState{})), "app-state-B")
			b.ReportMetric(float64(unsafe.Sizeof(platform.TextState{})), "native-state-B")
			runtime.KeepAlive(editor)
		})
	}
}

func BenchmarkNativeStateFirstSnapshot(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		editor := text.NewEditor("first snapshot")
		if !editor.EnableTentativeComposition() {
			b.Fatal("capability setup refused")
		}
		nativeStateBenchmarkSink = TextStateOf(editor)
	}
}

func BenchmarkNativeStateDefaultConstruction(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		editor := text.NewEditor("first snapshot")
		nativeStateBenchmarkSink = TextStateOf(editor)
	}
}

func TestDefaultStateSnapshotPreservesConstructionAllocations(t *testing.T) {
	allocations := testing.AllocsPerRun(100, func() {
		editor := text.NewEditor("first snapshot")
		nativeStateBenchmarkSink = TextStateOf(editor)
	})
	if allocations > 2 {
		t.Fatalf("default state snapshot construction allocated %.0f times, want at most 2", allocations)
	}
}
