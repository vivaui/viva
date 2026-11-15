package text

import (
	"reflect"
	"runtime"
	"strings"
	"testing"
	"unsafe"
)

func enableTentativeCore(t *testing.T, editor *Editor) {
	t.Helper()
	if !editor.EnableTentativeComposition() || !editor.TentativeCompositionEnabled() {
		t.Fatal("fresh editor refused tentative composition")
	}
}

func applyTentativeCore(t *testing.T, editor *Editor, event CompositionEvent) {
	t.Helper()
	if !editor.ApplyComposition(event) {
		t.Fatalf("event refused: %+v", event)
	}
}

func TestTentativeCoreSelectedCancelKeepsOrdinaryDeletion(t *testing.T) {
	editor := NewEditor("abc")
	from := mustTrackHistory(t, editor)
	enableTentativeCore(t, editor)
	editor.SetSelection(Range{Start: 1, End: 2})
	applyTentativeCore(t, editor, CompositionEvent{Phase: CompositionUpdate, Text: "x", Selected: Range{End: 1}})
	if editor.String() != "axc" || editor.CommittedString() != "ac" || !editor.CompositionActive() {
		t.Fatal("selected tentative lost its ordinary deletion or committed view")
	}
	from = expectHistoryFrames(t, editor, from, []HistoryStep{
		{HistoryForward, HistoryDelete, 1, 1, 3, 2, historySample(1, 0, -1, 1)},
		{HistoryForward, HistoryInsert, 1, 1, 2, 3, historySample(2, 0, -1, 2)},
	})
	applyTentativeCore(t, editor, CompositionEvent{Phase: CompositionCancel})
	if editor.String() != "ac" || editor.CompositionActive() || editor.CanRedo() {
		t.Fatal("Cancel restored ordinary selected text or retained tentative Redo")
	}
	from = expectHistoryFrames(t, editor, from, []HistoryStep{{HistoryUndo, HistoryDelete, 1, 1, 3, 2, historySample(2, 0, -1, 2)}})
	if !editor.Undo() || editor.String() != "abc" || editor.CanUndo() {
		t.Fatal("ordinary deletion is not the one remaining Undo unit")
	}
	expectHistoryFrames(t, editor, from, []HistoryStep{{HistoryUndo, HistoryInsert, 1, 1, 2, 3, historySample(1, 0, -1, 1)}})
}

func TestTentativeCoreUTF16ReplacementAfterRollback(t *testing.T) {
	editor := NewEditor("\U0001f600bc")
	from := mustTrackHistory(t, editor)
	enableTentativeCore(t, editor)
	editor.SetSelection(Range{Start: 1, End: 2})
	applyTentativeCore(t, editor, CompositionEvent{Phase: CompositionUpdate, Text: "x"})
	from = expectHistoryFrames(t, editor, from, []HistoryStep{
		{HistoryForward, HistoryDelete, 4, 1, 6, 5, historySample(1, 0, -1, 1)},
		{HistoryForward, HistoryInsert, 4, 1, 5, 6, historySample(2, 0, -1, 2)},
	})
	applyTentativeCore(t, editor, CompositionEvent{Phase: CompositionResult, Text: "r", HasReplacementUTF16: true, ReplacementUTF16: UTF16Range{Start: 0, Length: 2}})
	if editor.String() != "rc" || editor.CompositionActive() {
		t.Fatal("raw UTF16 replacement was not applied to the rolled-back buffer")
	}
	expectHistoryFrames(t, editor, from, []HistoryStep{
		{HistoryUndo, HistoryDelete, 4, 1, 6, 5, historySample(2, 0, -1, 2)},
		{HistoryForward, HistoryDelete, 0, 4, 5, 1, historySample(2, 0, -1, 2)},
		{HistoryForward, HistoryInsert, 0, 1, 1, 2, historySample(3, 0, -1, 3)},
	})
	for _, want := range []string{"c", "\U0001f600c", "\U0001f600bc"} {
		if !editor.Undo() || editor.String() != want {
			t.Fatalf("Result Undo = %q; want %q", editor.String(), want)
		}
	}
}

func TestTentativeCoreOverwriteUnitsAndLiveModeToggle(t *testing.T) {
	editor := NewEditor("a\u00e9b\nc")
	from := mustTrackHistory(t, editor)
	editor.SetOverwrite(true)
	enableTentativeCore(t, editor)
	editor.MoveTo(1, false)
	applyTentativeCore(t, editor, CompositionEvent{Phase: CompositionResult, Text: "XY"})
	if editor.String() != "aXY\nc" {
		t.Fatal("overwrite did not consume one code point per scalar")
	}
	expectHistoryFrames(t, editor, from, []HistoryStep{
		{HistoryForward, HistoryDelete, 1, 2, 6, 4, historySample(1, 0, -1, 1)},
		{HistoryForward, HistoryInsert, 1, 1, 4, 5, historySample(2, 0, -1, 2)},
		{HistoryForward, HistoryDelete, 2, 1, 5, 4, historySample(3, 0, -1, 3)},
		{HistoryForward, HistoryInsert, 2, 1, 4, 5, historySample(4, 0, -1, 4)},
	})
	for _, want := range []string{"aXb\nc", "a\u00e9b\nc"} {
		if !editor.Undo() || editor.String() != want {
			t.Fatalf("scalar Undo = %q; want %q", editor.String(), want)
		}
	}
	applyTentativeCore(t, editor, CompositionEvent{Phase: CompositionUpdate, Text: "x"})
	if editor.CommittedString() != "a\u00e9b\nc" {
		t.Fatal("committed view lost overwritten source text")
	}
	editor.SetOverwrite(false)
	applyTentativeCore(t, editor, CompositionEvent{Phase: CompositionUpdate, Text: "yz"})
	if editor.String() != "ayz\u00e9b\nc" || editor.CommittedString() != "a\u00e9b\nc" {
		t.Fatal("mode toggle changed rollback or the next insertion-mode update")
	}
	applyTentativeCore(t, editor, CompositionEvent{Phase: CompositionCancel})
	if editor.String() != "a\u00e9b\nc" || editor.CanUndo() || editor.CanRedo() {
		t.Fatal("Cancel retained tentative or discarded old Redo incorrectly")
	}
}

func TestTentativeCoreCommitAndEmptyCommit(t *testing.T) {
	for _, phase := range []CompositionPhase{CompositionCommit, CompositionCancel, CompositionEmptyCommit} {
		t.Run(string(rune('0'+phase)), func(t *testing.T) {
			editor := NewEditor("abc")
			from := mustTrackHistory(t, editor)
			enableTentativeCore(t, editor)
			applyTentativeCore(t, editor, CompositionEvent{Phase: CompositionUpdate, Text: "x"})
			_, from, _ = editor.HistorySince(from)
			applyTentativeCore(t, editor, CompositionEvent{Phase: phase})
			if editor.CompositionActive() {
				t.Fatal("terminal event left live rollback ownership")
			}
			if phase == CompositionCommit {
				if editor.String() != "abcx" || !editor.CanUndo() {
					t.Fatal("Commit lost its visible ordinary unit")
				}
				expectHistoryFrames(t, editor, from)
				if !editor.Undo() || editor.String() != "abc" {
					t.Fatal("committed tentative was not Undoable")
				}
			} else if editor.String() != "abc" || editor.CanUndo() || editor.CanRedo() {
				t.Fatal("rollback terminal event retained tentative units")
			}
		})
	}
	editor := NewEditor("abc")
	enableTentativeCore(t, editor)
	editor.SetSelection(Range{Start: 1, End: 2})
	applyTentativeCore(t, editor, CompositionEvent{Phase: CompositionResult})
	if editor.String() != "abc" || editor.Selection() != (Range{Start: 1, End: 2}) || editor.CanUndo() {
		t.Fatal("empty implicit Result deleted its selection")
	}
	applyTentativeCore(t, editor, CompositionEvent{Phase: CompositionEmptyCommit})
	if editor.String() != "ac" || !editor.Undo() || editor.String() != "abc" {
		t.Fatal("inactive EmptyCommit did not retain ordinary selection deletion")
	}
}

func TestTentativeCoreRefusalPreservesLiveSource(t *testing.T) {
	editor := NewEditor("abc")
	from := mustTrackHistory(t, editor)
	enableTentativeCore(t, editor)
	applyTentativeCore(t, editor, CompositionEvent{Phase: CompositionUpdate, Text: "x", Selected: Range{End: 1}})
	before, _, _ := editor.HistorySince(from)
	selection, caret := editor.Selection(), editor.Caret()
	for _, event := range []CompositionEvent{
		{Phase: CompositionNone},
		{Phase: CompositionResult, Text: "\xff"},
		{Phase: CompositionResult, Text: "q", HasReplacementUTF16: true, ReplacementUTF16: UTF16Range{Start: -1}},
		{Phase: CompositionUpdate, Text: "q", HasReplacement: true, Replacement: Range{End: 1}},
	} {
		if editor.ApplyComposition(event) {
			t.Fatalf("invalid event handled: %+v", event)
		}
		after, _, ok := editor.HistorySince(from)
		if !ok || editor.String() != "abcx" || editor.Selection() != selection || editor.Caret() != caret || !editor.CompositionActive() || !reflect.DeepEqual(before, after) {
			t.Fatal("refusal partially changed live text, selection, ownership or frames")
		}
	}
	if editor.DisableTentativeComposition() {
		t.Fatal("active Disable stranded rollback ownership")
	}
	applyTentativeCore(t, editor, CompositionEvent{Phase: CompositionCancel})
	if !editor.DisableTentativeComposition() || editor.TentativeCompositionEnabled() || !editor.DisableTentativeComposition() {
		t.Fatal("inactive Disable is not idempotent")
	}
}

func TestTentativeCoreCopiedReceiverCannotMutateOwner(t *testing.T) {
	editor := NewEditor("abc")
	enableTentativeCore(t, editor)
	copy := *editor
	if copy.EnableTentativeComposition() || copy.TentativeCompositionEnabled() || copy.ApplyComposition(CompositionEvent{Phase: CompositionUpdate, Text: "x"}) || copy.DisableTentativeComposition() {
		t.Fatal("copied receiver borrowed another editor's capability")
	}
	if editor.String() != "abc" || editor.CompositionActive() || !editor.TentativeCompositionEnabled() {
		t.Fatal("foreign capability operation changed its actual owner")
	}
}

func TestTentativeCoreNewUpdateAfterUndoOwnsCurrentBoundary(t *testing.T) {
	editor := NewEditor("abc")
	from := mustTrackHistory(t, editor)
	enableTentativeCore(t, editor)
	applyTentativeCore(t, editor, CompositionEvent{Phase: CompositionResult, Text: "x"})
	if !editor.Undo() || editor.String() != "abc" || !editor.CanRedo() {
		t.Fatal("fixture did not create a Redo suffix")
	}
	_, from, _ = editor.HistorySince(from)
	applyTentativeCore(t, editor, CompositionEvent{Phase: CompositionUpdate, Text: "y"})
	if editor.String() != "abcy" || editor.CommittedString() != "abc" || editor.CanRedo() {
		t.Fatalf("new tentative after Undo = %q, committed %q, Redo %t", editor.String(), editor.CommittedString(), editor.CanRedo())
	}
	from = expectHistoryFrames(t, editor, from, []HistoryStep{{HistoryForward, HistoryInsert, 3, 1, 3, 4, historySample(1, 0, -1, 1)}})
	applyTentativeCore(t, editor, CompositionEvent{Phase: CompositionCancel})
	if editor.String() != "abc" || editor.CanUndo() || editor.CanRedo() || editor.CompositionActive() {
		t.Fatal("Cancel did not remove the new suffix from its actual current boundary")
	}
	expectHistoryFrames(t, editor, from, []HistoryStep{{HistoryUndo, HistoryDelete, 3, 1, 4, 3, historySample(1, 0, -1, 1)}})
}

func TestTentativeCoreSharedHeaderChargedOnce(t *testing.T) {
	for _, historyFirst := range []bool{false, true} {
		t.Run(map[bool]string{false: "composition-first", true: "history-first"}[historyFirst], func(t *testing.T) {
			editor := NewEditor("abc")
			if historyFirst {
				mustTrackHistory(t, editor)
			}
			enableTentativeCore(t, editor)
			shared := int(unsafe.Sizeof(editorSidecar{})) / 4
			composition := int(unsafe.Sizeof(compositionState{})) / 4
			history := int(unsafe.Sizeof(historyJournal{})) / 4
			want := shared + composition
			if historyFirst {
				want += history
			}
			if editor.historyCost() != want {
				t.Fatalf("first capability headers charged %d; retained headers %d", editor.historyCost(), want)
			}
			if !historyFirst {
				mustTrackHistory(t, editor)
			}
			want = shared + composition + history
			if editor.historyCost() != want {
				t.Fatalf("joint headers charged %d; retained headers %d", editor.historyCost(), want)
			}
			editor.ClearHistory()
			if editor.historyCost() != want {
				t.Fatal("idle Clear duplicated or lost a retained header charge")
			}
			if !editor.DisableTentativeComposition() || editor.historyCost() != shared+history {
				t.Fatal("Disable did not release only composition-owned storage")
			}
		})
	}
}

func TestTentativeCorePhysicalPreflightIncludesSharedHeader(t *testing.T) {
	editor := NewEditor("")
	enableTentativeCore(t, editor)
	state := editor.compositionState()
	unitAndOwner := int(unsafe.Sizeof(historyEdit{})+unsafe.Sizeof(compositionState{})) / 4
	plan := compositionPlan{units: 1, payload: maxSingleEditRunes - unitAndOwner}
	if editor.compositionPlanFits(state, plan) {
		t.Fatal("physical preflight accepted a plan whose shared header exceeds the bound")
	}
	plan.payload -= int(unsafe.Sizeof(editorSidecar{})) / 4
	if !editor.compositionPlanFits(state, plan) {
		t.Fatal("physical preflight refused the exact retained-capacity bound")
	}
}

func TestTentativeCoreLargeOverwriteFrameAndPassiveRetention(t *testing.T) {
	const count = 2049
	editor := NewEditor(strings.Repeat("a", count))
	from := mustTrackHistory(t, editor)
	editor.SetOverwrite(true)
	editor.MoveTo(0, false)
	enableTentativeCore(t, editor)
	applyTentativeCore(t, editor, CompositionEvent{Phase: CompositionUpdate, Text: strings.Repeat("z", count)})
	frames, latest, ok := editor.HistorySince(from)
	if !ok || len(frames) != 1 || len(frames[0].Steps) != 2*count || len(editor.history) != count {
		t.Fatal("large overwrite lost complete frame delivery or scalar units")
	}
	for index := range count {
		wantDelete := HistoryStep{HistoryForward, HistoryDelete, index, 1, count, count - 1, historySample(2*index+1, 0, -1, 2*index+1)}
		wantInsert := HistoryStep{HistoryForward, HistoryInsert, index, 1, count - 1, count, historySample(2*index+2, 0, -1, 2*index+2)}
		if frames[0].Steps[2*index] != wantDelete || frames[0].Steps[2*index+1] != wantInsert {
			t.Fatalf("scalar %d does not preserve its delete/insert sample timing", index)
		}
		unit := &editor.history[index]
		if cap(unit.removed) != 1 || cap(unit.inserted) != 1 {
			t.Fatal("scalar unit retains uncharged source-buffer or allocator slack")
		}
	}
	applyTentativeCore(t, editor, CompositionEvent{Phase: CompositionCommit})
	if !editor.DisableTentativeComposition() || editor.TentativeCompositionEnabled() {
		t.Fatal("completed Disable failed")
	}
	editor.SetSelection(Range{})
	editor.Insert("")
	if len(editor.history) != count || !editor.DisableTentativeComposition() {
		t.Fatal("selection/no-op/repeated Disable retired complete physical work")
	}
	expectHistoryFrames(t, editor, latest)
	if !editor.Undo() || editor.String() != strings.Repeat("z", count-1)+"a" || !editor.Redo() {
		t.Fatal("passive completed action lost ordinary Undo/Redo")
	}
	state := editor.compositionState()
	minimum := (cap(editor.history) - maxHistoryEntries) * int(unsafe.Sizeof(historyEdit{})) / 4
	minimum += int(unsafe.Sizeof(compositionState{})) / 4
	if state == nil || state.cost < minimum {
		t.Fatal("retiring protection dropped ownership of retained extra capacity")
	}
	editor.Insert("q")
	if len(editor.history) != maxHistoryEntries || cap(editor.history) != maxHistoryEntries || editor.compositionState() != nil {
		t.Fatal("ordinary mutation did not settle and release retired composition storage")
	}
	if frames, _, ok := editor.HistorySince(from); ok || frames != nil {
		t.Fatal("unprovable ordinary trim served a complete-looking suffix")
	}
}

func TestTentativeCoreMetadataLossKeepsPhysicalCancel(t *testing.T) {
	const count = 60000
	value := strings.Repeat("a", count)
	editor := NewEditor(value)
	from := mustTrackHistory(t, editor)
	editor.SetOverwrite(true)
	editor.MoveTo(0, false)
	enableTentativeCore(t, editor)
	applyTentativeCore(t, editor, CompositionEvent{Phase: CompositionUpdate, Text: "z"})
	before, _, _ := editor.HistorySince(from)
	selection, caret := editor.Selection(), editor.Caret()
	text := editor.String()
	if editor.ApplyComposition(CompositionEvent{Phase: CompositionUpdate, Text: strings.Repeat("q", 140000)}) {
		t.Fatal("over-budget physical event was accepted")
	}
	after, _, ok := editor.HistorySince(from)
	if !ok || editor.String() != text || editor.Selection() != selection || editor.Caret() != caret || !reflect.DeepEqual(before, after) {
		t.Fatal("hard preflight refusal rolled back or changed the old live event")
	}
	var allocatedBefore, allocatedAfter runtime.MemStats
	runtime.ReadMemStats(&allocatedBefore)
	applyTentativeCore(t, editor, CompositionEvent{Phase: CompositionUpdate, Text: strings.Repeat("z", count)})
	runtime.ReadMemStats(&allocatedAfter)
	allocated := allocatedAfter.TotalAlloc - allocatedBefore.TotalAlloc
	t.Logf("60000-unit replacement allocated %d bytes", allocated)
	if allocated > 256<<20 {
		t.Fatalf("metadata-only exhaustion allocated %d bytes for a linear-size event", allocated)
	}
	if editor.String() != strings.Repeat("z", count) || editor.CommittedString() != value || len(editor.history) != count {
		t.Fatal("metadata exhaustion destroyed accepted physical work")
	}
	if frames, _, ok := editor.HistorySince(from); ok || frames != nil {
		t.Fatal("metadata exhaustion served a partial atomic frame")
	}
	applyTentativeCore(t, editor, CompositionEvent{Phase: CompositionCancel})
	if editor.String() != value || editor.CanUndo() || editor.CanRedo() || editor.CompositionActive() {
		t.Fatal("metadata loss stranded the physical rollback boundary")
	}
}

func TestTentativeCoreOverwriteCodePointsAndLineEnds(t *testing.T) {
	for _, test := range []struct {
		name, initial, inserted, want string
		caret                         int
	}{
		{"base-before-combining", "a\u0301b", "X", "X\u0301b", 0},
		{"combining-code-point", "a\u0301b", "X", "aXb", 1},
		{"astral-code-point", "a\U0001f600b", "X", "aXb", 1},
		{"crlf", "ab\r\ncd", "XY", "abXY\r\ncd", 2},
		{"lf", "ab\ncd", "XY", "abXY\ncd", 2},
		{"end", "ab", "XY", "abXY", 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			editor := NewEditor(test.initial)
			editor.SetOverwrite(true)
			editor.MoveTo(test.caret, false)
			enableTentativeCore(t, editor)
			applyTentativeCore(t, editor, CompositionEvent{Phase: CompositionUpdate, Text: test.inserted})
			if editor.String() != test.want || editor.CommittedString() != test.initial {
				t.Fatalf("overwrite = %q, committed %q; want %q/%q", editor.String(), editor.CommittedString(), test.want, test.initial)
			}
			applyTentativeCore(t, editor, CompositionEvent{Phase: CompositionCancel})
			if editor.String() != test.initial || editor.CanUndo() {
				t.Fatal("code-point/line-end rollback did not restore source")
			}
		})
	}
}

func TestTentativeCoreRawUTF16BoundaryPolicy(t *testing.T) {
	for _, test := range []struct {
		name string
		raw  UTF16Range
		want string
	}{
		{"split-start", UTF16Range{Start: 1}, "\U0001f600aX"},
		{"split-end", UTF16Range{Length: 1}, "X"},
		{"huge-relative-length", UTF16Range{Start: 2, Length: int(^uint(0) >> 1)}, "\U0001f600X"},
		{"huge-start", UTF16Range{Start: int(^uint(0) >> 1), Length: int(^uint(0) >> 1)}, "\U0001f600aX"},
	} {
		t.Run(test.name, func(t *testing.T) {
			editor := NewEditor("\U0001f600a")
			enableTentativeCore(t, editor)
			applyTentativeCore(t, editor, CompositionEvent{Phase: CompositionResult, Text: "X", HasReplacementUTF16: true, ReplacementUTF16: test.raw})
			if editor.String() != test.want {
				t.Fatalf("UTF16 replacement = %q; want %q", editor.String(), test.want)
			}
			for editor.Undo() {
			}
			if editor.String() != "\U0001f600a" {
				t.Fatal("UTF16 replacement lost ordinary Undo units")
			}
		})
	}
}

func TestTentativeCoreLegacyCommandsRetireRollback(t *testing.T) {
	for _, test := range []struct {
		name  string
		apply func(*Editor)
	}{
		{"replace-range", func(editor *Editor) { editor.ReplaceRange(Range{End: 1}, "q") }},
		{"selection", func(editor *Editor) { editor.SetSelection(Range{}) }},
		{"move", func(editor *Editor) { editor.MoveTo(0, false) }},
		{"unmark", func(editor *Editor) { editor.Unmark() }},
		{"insert", func(editor *Editor) { editor.Insert("r") }},
		{"paste", func(editor *Editor) { editor.Paste("r") }},
		{"backspace", func(editor *Editor) { editor.DeleteBackward() }},
		{"delete", func(editor *Editor) { editor.DeleteForward() }},
		{"word", func(editor *Editor) { editor.DeleteWord(false) }},
		{"line", func(editor *Editor) { editor.DeleteToLineBoundary(false) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			editor, legacy := NewEditor("abc"), NewEditor("abc")
			from := mustTrackHistory(t, editor)
			enableTentativeCore(t, editor)
			applyTentativeCore(t, editor, CompositionEvent{Phase: CompositionUpdate, Text: "x"})
			legacy.SetMarkedText("x", Range{})
			test.apply(editor)
			test.apply(legacy)
			if editor.String() != legacy.String() || editor.Selection() != legacy.Selection() || editor.CanUndo() != legacy.CanUndo() || editor.CanRedo() != legacy.CanRedo() {
				t.Fatal("direct legacy command changed the established visible or ordinary history behavior")
			}
			if editor.CompositionActive() || editor.CancelComposition() {
				t.Fatal("direct legacy command left a stale owned rollback boundary")
			}
			if frames, _, ok := editor.HistorySince(from); ok || frames != nil {
				t.Fatal("direct legacy escape served unproven metadata")
			}
		})
	}
}

func TestTentativeCoreLegacyInsertReleasesLiveProtection(t *testing.T) {
	const count = 2049
	editor := NewEditor(strings.Repeat("a", count))
	editor.SetOverwrite(true)
	editor.MoveTo(0, false)
	enableTentativeCore(t, editor)
	applyTentativeCore(t, editor, CompositionEvent{Phase: CompositionUpdate, Text: strings.Repeat("z", count)})
	editor.Insert("r")
	if editor.String() != "r" || editor.CompositionActive() || len(editor.history) > maxHistoryEntries {
		t.Fatalf("legacy Insert kept %d units after ending composition", len(editor.history))
	}
	if state := editor.compositionState(); state != nil && (state.live || state.active) {
		t.Fatal("legacy Insert retained the former tentative's physical protection")
	}
	for range 300 {
		editor.BreakUndoGroup()
		editor.Insert("q")
	}
	if len(editor.history) != maxHistoryEntries || cap(editor.history) != maxHistoryEntries || editor.historyCost() > maxHistoryRunes {
		t.Fatal("ordinary edits after legacy Insert retained unbounded or uncharged storage")
	}
}

func TestTentativeCoreNewResultRetiresPreviousLargeEvent(t *testing.T) {
	const count = 80000
	editor := NewEditor(strings.Repeat("a", count))
	from := mustTrackHistory(t, editor)
	editor.SetOverwrite(true)
	editor.MoveTo(0, false)
	enableTentativeCore(t, editor)
	applyTentativeCore(t, editor, CompositionEvent{Phase: CompositionUpdate, Text: strings.Repeat("z", count)})
	applyTentativeCore(t, editor, CompositionEvent{Phase: CompositionCommit})
	applyTentativeCore(t, editor, CompositionEvent{Phase: CompositionResult, Text: strings.Repeat("q", count)})
	if editor.String() != strings.Repeat("q", count) || len(editor.history) != count || editor.historyCost() > maxSingleEditRunes {
		t.Fatalf("new event retained previous protection: units=%d cost=%d", len(editor.history), editor.historyCost())
	}
	if frames, _, ok := editor.HistorySince(from); ok || frames != nil {
		t.Fatal("over-budget metadata presented an incomplete event suffix")
	}
	if !editor.Undo() || editor.String() != strings.Repeat("q", count-1)+"z" {
		t.Fatal("retiring the previous event removed the new event's ordinary units")
	}
}

func TestTentativeCoreResultRetainsPlannedCapacity(t *testing.T) {
	for _, mode := range []string{"physical-only", "invalidated-history"} {
		t.Run(mode, func(t *testing.T) {
			editor := NewEditor(strings.Repeat("a", 80000))
			var from HistoryCursor
			if mode == "invalidated-history" {
				from = mustTrackHistory(t, editor)
			}
			editor.SetOverwrite(true)
			editor.MoveTo(0, false)
			enableTentativeCore(t, editor)
			applyTentativeCore(t, editor, CompositionEvent{Phase: CompositionUpdate, Text: strings.Repeat("z", 80000)})
			applyTentativeCore(t, editor, CompositionEvent{Phase: CompositionCommit})
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			applyTentativeCore(t, editor, CompositionEvent{Phase: CompositionResult, Text: strings.Repeat("q", 130000)})
			runtime.ReadMemStats(&after)
			allocated := after.TotalAlloc - before.TotalAlloc
			t.Logf("Result allocated %d bytes", allocated)
			if allocated > 256<<20 {
				t.Fatal("accepted event repeatedly grew/compacted its planned history backing")
			}
			if editor.String() != strings.Repeat("q", 130000) || len(editor.history) != 130000 || cap(editor.history) != 130000 || editor.historyCost() > maxSingleEditRunes {
				t.Fatalf("new event lost physical units or retained old backing: units=%d capacity=%d cost=%d", len(editor.history), cap(editor.history), editor.historyCost())
			}
			if mode != "physical-only" {
				if frames, _, ok := editor.HistorySince(from); ok || frames != nil {
					t.Fatal("over-budget optional metadata exposed a partial event")
				}
			}
			if !editor.Undo() || editor.String() != strings.Repeat("q", 129999) || !editor.Redo() || editor.String() != strings.Repeat("q", 130000) {
				t.Fatal("new event lost ordinary Undo/Redo after evicting the old prefix")
			}
		})
	}
}

func coalescedTentativeResult(mode string, caret int, value string) CompositionEvent {
	event := CompositionEvent{Phase: CompositionResult, Text: value}
	switch mode {
	case "rune-empty":
		event.HasReplacement = true
		event.Replacement = Range{Start: caret, End: caret}
	case "utf16-empty":
		event.HasReplacementUTF16 = true
		event.ReplacementUTF16 = UTF16Range{Start: caret}
	}
	return event
}

func TestTentativeCoreResultOwnsCoalescedOrdinaryUnit(t *testing.T) {
	for _, mode := range []string{"implicit", "rune-empty", "utf16-empty"} {
		t.Run(mode, func(t *testing.T) {
			editor := NewEditor("")
			editor.Insert("a")
			enableTentativeCore(t, editor)
			applyTentativeCore(t, editor, coalescedTentativeResult(mode, editor.Caret(), "bc"))
			if editor.String() != "abc" || len(editor.history) != 1 {
				t.Fatal("inactive Result changed ordinary insertion coalescing")
			}
			state := editor.compositionState()
			unit := editor.history[0]
			if state == nil || !state.completed || state.completedStart != 0 || cap(unit.inserted) != len(unit.inserted) {
				t.Fatal("coalesced Result did not own its complete retained unit and exact text capacity")
			}
			if !editor.Undo() || editor.String() != "" || !editor.Redo() || editor.String() != "abc" {
				t.Fatal("coalesced Result lost its source Undo grouping")
			}
		})
	}
}

func TestTentativeCoreCoalescedResultFrameContainsOnlyNewScalars(t *testing.T) {
	for _, mode := range []string{"implicit", "rune-empty", "utf16-empty"} {
		t.Run(mode, func(t *testing.T) {
			editor := NewEditor("")
			from := mustTrackHistory(t, editor)
			editor.Insert("a")
			from = expectHistoryFrames(t, editor, from, []HistoryStep{{HistoryForward, HistoryInsert, 0, 1, 0, 1, historySample(1, 0, -1, 1)}})
			enableTentativeCore(t, editor)
			applyTentativeCore(t, editor, coalescedTentativeResult(mode, editor.Caret(), "bc"))
			expectHistoryFrames(t, editor, from, []HistoryStep{
				{HistoryForward, HistoryInsert, 1, 1, 1, 2, historySample(2, 0, -1, 2)},
				{HistoryForward, HistoryInsert, 2, 1, 2, 3, historySample(3, 0, -1, 3)},
			})
			if !editor.Undo() || editor.String() != "" || !editor.Redo() || editor.String() != "abc" {
				t.Fatal("metadata tracking changed the coalesced ordinary unit")
			}
		})
	}
}

func TestTentativeCoreCoalescedResultPreflightIncludesPreviousText(t *testing.T) {
	for _, mode := range []string{"implicit", "rune-empty", "utf16-empty"} {
		t.Run(mode, func(t *testing.T) {
			editor := NewEditor("")
			editor.Insert(strings.Repeat("a", maxSingleEditRunes-128))
			enableTentativeCore(t, editor)
			before, selection := editor.String(), editor.Selection()
			if editor.ApplyComposition(coalescedTentativeResult(mode, editor.Caret(), strings.Repeat("b", 512))) {
				t.Fatalf("oversized coalesced Result accepted: retained cost=%d", editor.historyCost())
			}
			if editor.String() != before || editor.Selection() != selection || len(editor.history) != 1 || editor.CompositionActive() {
				t.Fatal("refused coalesced Result partially changed source or history")
			}
			applyTentativeCore(t, editor, coalescedTentativeResult(mode, editor.Caret(), strings.Repeat("b", 32)))
			if editor.String() != before+strings.Repeat("b", 32) || len(editor.history) != 1 || editor.historyCost() > maxSingleEditRunes {
				t.Fatal("fitting coalesced Result lost its complete physically bounded unit")
			}
			if !editor.Undo() || editor.String() != "" || editor.CanUndo() {
				t.Fatal("coalesced Result broke ordinary Undo grouping")
			}
		})
	}
}

func TestTentativeCoreDisableKeepsRetiredStorageOwner(t *testing.T) {
	const count = 2049
	for _, history := range []bool{false, true} {
		for _, redo := range []bool{false, true} {
			name := map[bool]string{false: "physical", true: "history"}[history] + map[bool]string{false: "-undo", true: "-redo"}[redo]
			t.Run(name, func(t *testing.T) {
				editor := NewEditor(strings.Repeat("a", count))
				var from HistoryCursor
				if history {
					from = mustTrackHistory(t, editor)
				}
				editor.SetOverwrite(true)
				editor.MoveTo(0, false)
				enableTentativeCore(t, editor)
				applyTentativeCore(t, editor, CompositionEvent{Phase: CompositionUpdate, Text: strings.Repeat("z", count)})
				applyTentativeCore(t, editor, CompositionEvent{Phase: CompositionCommit})
				if !editor.Undo() || redo && !editor.Redo() {
					t.Fatal("fixture could not retire completed protection through replay")
				}
				before, latest, available := editor.HistorySince(from)
				if !editor.DisableTentativeComposition() || editor.TentativeCompositionEnabled() {
					t.Fatal("inactive Disable failed")
				}
				if len(editor.history) != count || editor.compositionState() == nil {
					t.Fatal("Disable released the remaining owner of oversized ordinary storage")
				}
				minimum := (cap(editor.history) - maxHistoryEntries) * int(unsafe.Sizeof(historyEdit{})) / 4
				for _, unit := range editor.history {
					minimum += cap(unit.removed) + cap(unit.inserted)
				}
				if editor.historyCost() < minimum || !editor.DisableTentativeComposition() {
					t.Fatal("disabled nonprotecting owner lost accounting or idempotence")
				}
				after, next, ok := editor.HistorySince(from)
				if available != ok || latest != next || !reflect.DeepEqual(before, after) {
					t.Fatal("Disable changed existing metadata frames or cursors")
				}
				editor.Insert("q")
				if len(editor.history) != maxHistoryEntries || cap(editor.history) != maxHistoryEntries || editor.compositionState() != nil || editor.CanRedo() {
					t.Fatal("next ordinary mutation did not settle and release disabled storage")
				}
				editor.ClearHistory()
				if len(editor.history) != 0 || editor.historyRunes != 0 || editor.compositionState() != nil {
					t.Fatal("Clear retained retired physical storage")
				}
			})
		}
	}
}
