package text

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func mustTrackHistory(t *testing.T, editor *Editor) HistoryCursor {
	t.Helper()
	cursor, size, ok := editor.TrackHistory()
	if !ok || cursor == (HistoryCursor{}) || size != len(editor.String()) {
		t.Fatalf("TrackHistory = %+v, %d, %t", cursor, size, ok)
	}
	return cursor
}

func historySample(current, save, detach, tail int) HistoryContext {
	return HistoryContext{Current: current, Save: save, Detach: detach, RetainedTo: tail, Collecting: true}
}

func expectHistoryFrames(t *testing.T, editor *Editor, from HistoryCursor, want ...[]HistoryStep) HistoryCursor {
	t.Helper()
	frames, latest, ok := editor.HistorySince(from)
	if !ok || len(frames) != len(want) {
		t.Fatalf("HistorySince = %d frames, %t; want %d: %+v", len(frames), ok, len(want), frames)
	}
	for index, frame := range frames {
		if frame.From != from || frame.To == from || !reflect.DeepEqual(frame.Steps, want[index]) {
			t.Fatalf("frame %d = %+v; want steps %+v", index, frame, want[index])
		}
		if len(frame.Steps) == 0 || frame.BeforeBytes != frame.Steps[0].BeforeBytes || frame.AfterBytes != frame.Steps[len(frame.Steps)-1].AfterBytes {
			t.Fatalf("frame %d has inconsistent byte boundaries", index)
		}
		from = frame.To
	}
	if latest != from {
		t.Fatalf("latest cursor %+v does not end the chain %+v", latest, from)
	}
	return latest
}

func TestHistoryFramesPreservePrimitivesAcrossCoalescingSaveAndReplay(t *testing.T) {
	editor := NewEditor("a\u00e9")
	from := mustTrackHistory(t, editor)
	if editor.AtSavePoint() {
		t.Fatal("producer unexpectedly enabled save-point tracking")
	}
	editor.MoveTo(0, false)
	editor.Insert("\U0001f600")
	editor.Insert("z")
	editor.SetSavePoint()
	from = expectHistoryFrames(t, editor, from,
		[]HistoryStep{{HistoryForward, HistoryInsert, 0, 4, 3, 7, historySample(1, 0, -1, 1)}},
		[]HistoryStep{{HistoryForward, HistoryInsert, 4, 1, 7, 8, historySample(2, 0, -1, 2)}},
		[]HistoryStep{{Operation: HistorySave, BeforeBytes: 8, AfterBytes: 8, Context: historySample(2, 2, -1, 2)}},
	)
	if !editor.Undo() || editor.String() != "a\u00e9" || editor.CanUndo() {
		t.Fatal("producer changed ordinary coalesced Undo")
	}
	from = expectHistoryFrames(t, editor, from, []HistoryStep{
		{HistoryUndo, HistoryDelete, 4, 1, 8, 7, historySample(2, 2, -1, 2)},
		{HistoryUndo, HistoryDelete, 0, 4, 7, 3, historySample(1, 2, -1, 2)},
	})
	if !editor.Redo() || editor.String() != "\U0001f600za\u00e9" || !editor.AtSavePoint() {
		t.Fatal("producer changed ordinary Redo or save point")
	}
	from = expectHistoryFrames(t, editor, from, []HistoryStep{
		{HistoryRedo, HistoryInsert, 0, 4, 3, 7, historySample(0, 2, -1, 2)},
		{HistoryRedo, HistoryInsert, 4, 1, 7, 8, historySample(1, 2, -1, 2)},
	})
	if frames, latest, ok := editor.HistorySince(from); !ok || frames != nil || latest != from {
		t.Fatal("empty read is not distinct from a saved operation")
	}
}

func TestHistoryFramesSaveOnlyOwnedReadsAndCursorRefusal(t *testing.T) {
	editor := NewEditor("abc")
	from := mustTrackHistory(t, editor)
	for range 3 {
		editor.SetSavePoint()
	}
	want := []HistoryStep{{Operation: HistorySave, BeforeBytes: 3, AfterBytes: 3, Context: historySample(0, 0, -1, 0)}}
	latest := expectHistoryFrames(t, editor, from, want, want, want)
	frames, _, _ := editor.HistorySince(from)
	frames[0].Steps[0].Context.Save = 99
	frames[0].Steps[0].Position = 99
	frames[0].From = HistoryCursor{}
	expectHistoryFrames(t, editor, from, want, want, want)
	foreign := mustTrackHistory(t, NewEditor("abc"))
	future, generation := latest, latest
	future.sequence++
	generation.generation++
	for _, cursor := range []HistoryCursor{{}, foreign, future, generation} {
		if got, _, ok := editor.HistorySince(cursor); ok || got != nil {
			t.Fatalf("unproven cursor %+v returned partial frames", cursor)
		}
	}
	var absent *Editor
	if cursor, size, ok := absent.TrackHistory(); ok || size != 0 || cursor != (HistoryCursor{}) {
		t.Fatal("nil producer accepted")
	}
	if got, _, ok := absent.HistorySince(from); ok || got != nil {
		t.Fatal("nil producer returned history")
	}
	editor.ClearHistory()
	if got, _, ok := editor.HistorySince(latest); ok || got != nil {
		t.Fatal("reset retained a stale generation")
	}
	reset := mustTrackHistory(t, editor)
	if reset == from || reset == latest {
		t.Fatal("reset reused an earlier cursor")
	}
	expectHistoryFrames(t, editor, reset)
}

func TestHistoryFramesSaveAndDetachArePrimitivePositions(t *testing.T) {
	editor := NewEditor("abc")
	from := mustTrackHistory(t, editor)
	editor.Insert("1")
	editor.BreakUndoGroup()
	editor.Insert("2")
	editor.SetSavePoint()
	editor.Undo()
	editor.Insert("X")
	frames, latest, ok := editor.HistorySince(from)
	if !ok || len(frames) != 5 || editor.String() != "abc1X" || editor.AtSavePoint() {
		t.Fatal("saved-branch fixture did not execute")
	}
	want := HistoryStep{HistoryForward, HistoryInsert, 4, 1, 4, 5, historySample(2, -1, 1, 2)}
	if !reflect.DeepEqual(frames[4].Steps, []HistoryStep{want}) {
		t.Fatalf("saved branch context = %+v", frames[4].Steps)
	}
	editor.Undo()
	editor.Undo()
	editor.Insert("Y")
	frames, latest, ok = editor.HistorySince(latest)
	if !ok || len(frames) != 3 || editor.String() != "abcY" || frames[2].Steps[0].Context != historySample(1, -1, 0, 1) {
		t.Fatalf("earlier detach = %+v", frames)
	}
	editor.SetSavePoint()
	expectHistoryFrames(t, editor, latest, []HistoryStep{{Operation: HistorySave, BeforeBytes: 4, AfterBytes: 4, Context: historySample(1, 1, -1, 1)}})
}

func TestHistoryFramesAtomicActionExceedsByteJournalWindow(t *testing.T) {
	editor := NewEditor(strings.Repeat("x", 2049))
	byteCursor := editor.TrackEdits()
	from := mustTrackHistory(t, editor)
	edits := make([]Replacement, 2049)
	for index := range edits {
		edits[index] = Replacement{Range: Range{Start: index, End: index}, Text: "q"}
	}
	if err := editor.ReplaceRanges(edits); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := editor.EditsSince(byteCursor, nil); ok {
		t.Fatal("ordinary byte window unexpectedly retains the whole large action")
	}
	for _, operation := range []HistoryOperation{HistoryForward, HistoryUndo, HistoryRedo} {
		frames, latest, ok := editor.HistorySince(from)
		if !ok || len(frames) != 1 || len(frames[0].Steps) != 2049 {
			t.Fatalf("atomic %v delivered a partial action: %d frames, %t", operation, len(frames), ok)
		}
		first, last := frames[0].Steps[0], frames[0].Steps[2048]
		if first.Operation != operation || last.Operation != operation || frames[0].AfterBytes != len(editor.String()) {
			t.Fatal("atomic operation is not tied to the accepted editor state")
		}
		if operation == HistoryUndo {
			if first.Primitive != HistoryDelete || first.Position != 0 || first.BeforeBytes != 4098 || last.Position != 2048 || last.AfterBytes != 2049 {
				t.Fatalf("inverse order = %+v ... %+v", first, last)
			}
		} else if first.Primitive != HistoryInsert || first.Position != 2048 || first.BeforeBytes != 2049 || last.Position != 0 || last.AfterBytes != 4098 {
			t.Fatalf("forward order = %+v ... %+v", first, last)
		}
		from = latest
		if operation == HistoryForward && !editor.Undo() || operation == HistoryUndo && !editor.Redo() {
			t.Fatal("atomic action lost physical Undo/Redo")
		}
	}
}

func TestHistoryFramesKeepEqualNetTransactionsAndPrivateDrafts(t *testing.T) {
	editor := NewEditor("abc")
	from := mustTrackHistory(t, editor)
	if err := editor.Transact(func(draft *Editor) error {
		if cursor, _, ok := draft.TrackHistory(); ok || cursor != (HistoryCursor{}) {
			t.Fatal("private transaction draft accepted a public producer")
		}
		draft.ReplaceRange(Range{Start: 0, End: 1}, "x")
		draft.ReplaceRange(Range{Start: 0, End: 1}, "a")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if editor.String() != "abc" || !editor.CanUndo() {
		t.Fatal("equal-net accepted primitives were discarded")
	}
	frames, latest, ok := editor.HistorySince(from)
	if !ok || len(frames) != 1 || len(frames[0].Steps) != 4 || frames[0].BeforeBytes != 3 || frames[0].AfterBytes != 3 {
		t.Fatalf("equal-net frame = %+v, %t", frames, ok)
	}
	for index, step := range frames[0].Steps {
		primitive, before, after := HistoryDelete, 3, 2
		if index%2 != 0 {
			primitive, before, after = HistoryInsert, 2, 3
		}
		want := HistoryStep{HistoryForward, primitive, 0, 1, before, after, historySample(index+1, 0, -1, index+1)}
		if step != want {
			t.Fatalf("equal-net step %d = %+v, want %+v", index, step, want)
		}
	}
	if err := editor.Transact(func(draft *Editor) error {
		draft.Insert("refused")
		return fmt.Errorf("cancel transaction")
	}); err == nil || editor.String() != "abc" {
		t.Fatal("failed draft changed the source")
	}
	expectHistoryFrames(t, editor, latest)
	if !editor.Undo() || editor.String() != "abc" || !editor.Redo() || editor.String() != "abc" {
		t.Fatal("equal-net physical history did not replay")
	}
}

func TestHistoryFramesDeliveryExpiryDiffersFromHistoryLoss(t *testing.T) {
	editor := NewEditor("abc")
	old := mustTrackHistory(t, editor)
	for range 50 {
		editor.SetSavePoint()
	}
	_, retained, ok := editor.HistorySince(old)
	if !ok {
		t.Fatal("early delivery expired")
	}
	for range 250 {
		editor.SetSavePoint()
	}
	if got, _, ok := editor.HistorySince(old); ok || got != nil {
		t.Fatal("expired prefix returned a suffix")
	}
	if _, _, ok := editor.TrackHistory(); ok {
		t.Fatal("expired bootstrap was recreated from current text")
	}
	if got, _, ok := editor.HistorySince(retained); !ok || len(got) != 250 {
		t.Fatal("bootstrap expiry invalidated a retained consumer")
	}
	trimmed := NewEditor("")
	cursor := mustTrackHistory(t, trimmed)
	for range 257 {
		trimmed.BreakUndoGroup()
		trimmed.Insert("x")
	}
	if got, _, ok := trimmed.HistorySince(cursor); ok || got != nil || !trimmed.Undo() || trimmed.RuneCount() != 256 {
		t.Fatal("unprovable trim either invented history or broke physical Undo")
	}
	trimmed.ClearHistory()
	if _, _, ok := trimmed.TrackHistory(); ok {
		t.Fatal("unavailable history silently rebootstrapped")
	}
}

func TestHistoryFramesNoOpBatchKeepsSelectionAndRedo(t *testing.T) {
	editor := NewEditor("abc")
	from := mustTrackHistory(t, editor)
	editor.Insert("x")
	if !editor.Undo() {
		t.Fatal("fixture did not establish Redo")
	}
	_, from, ok := editor.HistorySince(from)
	if !ok {
		t.Fatal("fixture lost its cursor")
	}
	editor.SetSelection(Range{Start: 2, End: 0})
	selection, caret := editor.Selection(), editor.Caret()
	if err := editor.ReplaceRanges([]Replacement{{Range: Range{Start: 1, End: 1}}}); err != nil {
		t.Fatal(err)
	}
	if editor.String() != "abc" || editor.Selection() != selection || editor.Caret() != caret || !editor.CanRedo() || editor.CanUndo() {
		t.Fatal("empty replacement changed selection, text or history")
	}
	expectHistoryFrames(t, editor, from)
}

func TestHistoryFramesLateAttachAndLegacyCompositionRefuse(t *testing.T) {
	late := NewEditor("abc")
	late.Insert("x")
	late.Undo()
	if _, _, ok := late.TrackHistory(); ok || !late.CanRedo() {
		t.Fatal("redo-only source attached or was mutated")
	}
	plain, tracked := NewEditor("abc"), NewEditor("abc")
	cursor := mustTrackHistory(t, tracked)
	for _, editor := range []*Editor{plain, tracked} {
		editor.Insert("X")
		editor.SetSelection(Range{Start: 1, End: 2})
		editor.SetMarkedText("q", Range{})
		if !editor.CancelComposition() {
			t.Fatal("ordinary native cancellation was changed")
		}
	}
	if plain.String() != tracked.String() || plain.Selection() != tracked.Selection() || plain.CanUndo() != tracked.CanUndo() {
		t.Fatal("history tracking changed upstream marked-composition behavior")
	}
	if got, _, ok := tracked.HistorySince(cursor); ok || got != nil {
		t.Fatal("legacy composition guessed metadata frames")
	}
	if !plain.Undo() || !tracked.Undo() || plain.String() != "abc" || tracked.String() != "abc" {
		t.Fatal("metadata invalidation destroyed ordinary Undo")
	}
}

func TestHistoryFramesClearPreservesSavedTruth(t *testing.T) {
	for _, dirty := range []bool{false, true} {
		t.Run(fmt.Sprint(dirty), func(t *testing.T) {
			editor := NewEditor("abc")
			editor.SetSavePoint()
			old := mustTrackHistory(t, editor)
			if dirty {
				editor.ReplaceRange(Range{Start: 0, End: 1}, "a")
			}
			editor.ClearChangeHistory()
			if editor.String() != "abc" || editor.AtSavePoint() == dirty || editor.CanUndo() || editor.CanRedo() {
				t.Fatal("history clear changed text or laundered an unsaved boundary")
			}
			if got, _, ok := editor.HistorySince(old); ok || got != nil {
				t.Fatal("change-history clear retained the prior generation")
			}
			from := mustTrackHistory(t, editor)
			editor.Insert("!")
			frames, _, ok := editor.HistorySince(from)
			wantSave := 0
			if dirty {
				wantSave = -1
			}
			if !ok || len(frames) != 1 || frames[0].Steps[0].Context.Save != wantSave {
				t.Fatal("clear bootstrap invented a reachable saved boundary")
			}
		})
	}
}

func TestHistoryFramesSparseBatchRetainedCapacity(t *testing.T) {
	for _, lines := range []bool{false, true} {
		t.Run(fmt.Sprint(lines), func(t *testing.T) {
			value := strings.Repeat("x\n", 2048)
			editor := NewEditor(value)
			from := mustTrackHistory(t, editor)
			var lineCursor uint64
			if lines {
				lineCursor = editor.TrackLineEdits()
			}
			edits := make([]Replacement, len(value))
			edits[0] = Replacement{Range: Range{End: 1}, Text: "x"}
			for index := 1; index < len(edits); index++ {
				edits[index].Range = Range{Start: index, End: index}
			}
			if err := editor.ReplaceRanges(edits); err != nil {
				t.Fatal(err)
			}
			if editor.String() != value || !editor.CanUndo() || len(editor.history) != 1 {
				t.Fatal("equal sparse batch did not retain its accepted action")
			}
			unit := &editor.history[0]
			minimum := len(unit.removed) + len(unit.inserted) + cap(unit.primitives)*historyPrimitiveRunes
			if lines {
				contexts := editor.lineBatchContexts(unit.primitives)
				if len(contexts) != 1 {
					t.Fatal("sparse batch lost retained line contexts")
				}
				minimum += cap(contexts) * lineContextRunes
			}
			if editor.historyRunes < minimum {
				t.Errorf("retained charge %d is below backing lower bound %d (primitive len=%d cap=%d)", editor.historyRunes, minimum, len(unit.primitives), cap(unit.primitives))
			}
			from = expectHistoryFrames(t, editor, from, []HistoryStep{
				{HistoryForward, HistoryDelete, 0, 1, 4096, 4095, historySample(1, 0, -1, 1)},
				{HistoryForward, HistoryInsert, 0, 1, 4095, 4096, historySample(2, 0, -1, 2)},
			})
			if lines {
				lineCursor = requireLineEdits(t, editor, lineCursor, []LineEdit{{Edit: Edit{Removed: 1, Inserted: 1}, AtLineStart: true}})
			}
			if !editor.Undo() || editor.String() != value {
				t.Fatal("sparse batch Undo failed")
			}
			if lines {
				lineCursor = requireLineEdits(t, editor, lineCursor, []LineEdit{{Edit: Edit{Removed: 1, Inserted: 1}, AtLineStart: true}})
			}
			if !editor.Redo() || editor.String() != value {
				t.Fatal("sparse batch Redo failed")
			}
			if lines {
				requireLineEdits(t, editor, lineCursor, []LineEdit{{Edit: Edit{Removed: 1, Inserted: 1}, AtLineStart: true}})
			}
			if frames, _, ok := editor.HistorySince(from); !ok || len(frames) != 2 || len(frames[0].Steps) != 2 || len(frames[1].Steps) != 2 {
				t.Fatal("sparse batch lost complete inverse frames")
			}
			editor.Undo()
			editor.Insert("z")
			if editor.CanRedo() || editor.historyRunes != 1+historyPrimitiveRunes {
				t.Fatal("branch retained sparse batch storage or accounting")
			}
			editor.ClearHistory()
			if editor.historyRunes != 0 || editor.historyCost() != historyHeaderRunes {
				t.Fatal("clear retained sparse batch storage or accounting")
			}
		})
	}
}

func TestHistoryFramesClearWithoutSavePointPreservesBoundary(t *testing.T) {
	for _, name := range []string{"initial", "dirty", "undone", "dirty-cleared"} {
		t.Run(name, func(t *testing.T) {
			editor := NewEditor("abc")
			old := mustTrackHistory(t, editor)
			wantSave, size := 0, 3
			if name != "initial" {
				editor.Insert("x")
				if name == "undone" {
					editor.Undo()
				} else {
					wantSave, size = -1, 4
				}
			}
			if name == "dirty-cleared" {
				editor.ClearChangeHistory()
			}
			editor.ClearChangeHistory()
			if editor.CanUndo() || editor.CanRedo() || editor.AtSavePoint() || editor.savePointIndex() != nil {
				t.Fatal("clear enabled save-point tracking or retained Undo/Redo")
			}
			if frames, _, ok := editor.HistorySince(old); ok || frames != nil {
				t.Fatal("clear retained the old generation")
			}
			from := mustTrackHistory(t, editor)
			editor.Insert("y")
			from = expectHistoryFrames(t, editor, from, []HistoryStep{{HistoryForward, HistoryInsert, size, 1, size, size + 1, historySample(1, wantSave, -1, 1)}})
			if !editor.Undo() {
				t.Fatal("post-clear Undo failed")
			}
			from = expectHistoryFrames(t, editor, from, []HistoryStep{{HistoryUndo, HistoryDelete, size, 1, size + 1, size, historySample(1, wantSave, -1, 1)}})
			if editor.AtSavePoint() || editor.savePointIndex() != nil {
				t.Fatal("producer implicitly enabled AtSavePoint")
			}
			editor.SetSavePoint()
			expectHistoryFrames(t, editor, from, []HistoryStep{{Operation: HistorySave, BeforeBytes: size, AfterBytes: size, Context: historySample(0, 0, -1, 1)}})
			if !editor.AtSavePoint() {
				t.Fatal("explicit Save did not establish the new saved boundary")
			}
		})
	}
}

func TestHistoryFramesPartialBudgetEvictionThenGrowth(t *testing.T) {
	editor := NewEditor("")
	epoch := mustTrackHistory(t, editor)
	from := epoch
	known := make(map[uint64]HistoryFrame)
	remember := func(steps ...HistoryStep) {
		t.Helper()
		next := expectHistoryFrames(t, editor, from, steps)
		known[next.sequence] = HistoryFrame{From: from, To: next, BeforeBytes: steps[0].BeforeBytes, AfterBytes: steps[len(steps)-1].AfterBytes, Steps: steps}
		from = next
	}
	check := func() {
		t.Helper()
		history := editor.historyState()
		if history == nil {
			t.Fatal("delivery eviction invalidated the physical history")
		}
		backing := history.frames[:cap(history.frames)]
		occupied := make([]bool, len(backing))
		cost := historyHeaderRunes + len(backing)*historyFrameRunes
		var want []HistoryFrame
		for index := range len(history.frames) {
			at := (int(history.oldest) + index) % len(backing)
			occupied[at] = true
			frame := backing[at]
			expected := known[history.windowStart+uint64(index)+1]
			if !reflect.DeepEqual(frame, expected) {
				t.Fatalf("logical slot %d = %+v; want %+v", index, frame, expected)
			}
			want = append(want, expected)
			cost += cap(frame.Steps) * historyStepRunes
		}
		for index, frame := range backing {
			if !occupied[index] && !reflect.DeepEqual(frame, HistoryFrame{}) {
				t.Fatalf("evicted slot %d retains a frame: %+v", index, frame)
			}
		}
		if history.cost != cost {
			t.Fatalf("ring charge = %d; live backing = %d", history.cost, cost)
		}
		frames, latest, ok := editor.HistorySince(want[0].From)
		if !ok || latest != from || !reflect.DeepEqual(frames, want) {
			t.Fatal("wrapped read differs from the complete retained frame chain")
		}
		frames[0].Steps[0].Length++
		again, _, ok := editor.HistorySince(want[0].From)
		if !ok || !reflect.DeepEqual(again, want) {
			t.Fatal("wrapped read exposes retained step backing")
		}
	}
	size := maxHistoryRunes - 2500
	editor.Insert(strings.Repeat("x", size))
	remember(HistoryStep{HistoryForward, HistoryInsert, 0, size, 0, size, historySample(1, 0, -1, 1)})
	for range 128 {
		editor.SetSavePoint()
		remember(HistoryStep{Operation: HistorySave, BeforeBytes: size, AfterBytes: size, Context: historySample(1, 1, -1, 1)})
		check()
		history := editor.historyState()
		if history.oldest > 0 && len(history.frames) < cap(history.frames) {
			break
		}
	}
	history := editor.historyState()
	if history.oldest == 0 || len(history.frames) == cap(history.frames) {
		t.Fatal("fixture did not reach partial budget eviction")
	}
	capacity := cap(history.frames)
	if frames, _, ok := editor.HistorySince(epoch); ok || frames != nil {
		t.Fatal("budget eviction served an incomplete bootstrap")
	}
	if !editor.Undo() || editor.String() != "" {
		t.Fatal("budget eviction broke physical Undo")
	}
	remember(HistoryStep{HistoryUndo, HistoryDelete, 0, size, size, 0, historySample(1, 1, -1, 1)})
	check()
	editor.Insert("z")
	remember(HistoryStep{HistoryForward, HistoryInsert, 0, 1, 0, 1, historySample(1, -1, 0, 1)})
	check()
	if editor.CanRedo() || editor.historyRunes != 1+historyPrimitiveRunes {
		t.Fatal("branch did not release the large redo unit")
	}
	for range capacity + 1 {
		editor.SetSavePoint()
		remember(HistoryStep{Operation: HistorySave, BeforeBytes: 1, AfterBytes: 1, Context: historySample(1, 1, -1, 1)})
		check()
	}
	if cap(history.frames) <= capacity {
		t.Fatal("fixture did not grow after freeing retained history")
	}
	editor.ClearHistory()
	if history.frames != nil || history.cost != historyHeaderRunes || editor.historyRunes != 0 {
		t.Fatal("clear retained delivery or physical-history storage")
	}
}

func TestHistoryFramesClearResetsDetachedBoundary(t *testing.T) {
	editor := NewEditor("abc")
	mustTrackHistory(t, editor)
	editor.Insert("x")
	editor.SetSavePoint()
	editor.Insert("y")
	editor.Undo()
	editor.Undo()
	editor.Insert("z")
	if history := editor.historyState(); history == nil || history.detach != 0 || history.save != -1 {
		t.Fatalf("fixture history = %+v, want save -1 and detach 0", history)
	}
	editor.ClearChangeHistory()
	from := mustTrackHistory(t, editor)
	editor.Insert("q")
	frames, _, ok := editor.HistorySince(from)
	if !ok || len(frames) != 1 || len(frames[0].Steps) != 1 {
		t.Fatalf("frames = %+v, ok = %t", frames, ok)
	}
	if got := frames[0].Steps[0].Context.Detach; got != -1 {
		t.Fatalf("post-clear detach = %d, want -1", got)
	}
}
