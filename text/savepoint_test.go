package text

import (
	"encoding/csv"
	"encoding/hex"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"
	"unsafe"
)

type savePointEditor interface {
	SetSavePoint()
	AtSavePoint() bool
}

func savePoints(t *testing.T, editor *Editor) savePointEditor {
	t.Helper()
	points, ok := any(editor).(savePointEditor)
	if !ok {
		t.Fatal("editor has no optional save-point tracking")
	}
	return points
}

func TestSavePointsMatchPinnedOriginalUndoHistory(t *testing.T) {
	file, err := os.Open("testdata/scintilla-savepoints.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma, reader.Comment, reader.FieldsPerRecord = '\t', '#', 10
	if _, err := reader.Read(); err != nil {
		t.Fatal(err)
	}
	decode := func(value string) string {
		t.Helper()
		if value == "-" {
			return ""
		}
		decoded, err := hex.DecodeString(value)
		if err != nil {
			t.Fatal(err)
		}
		return string(decoded)
	}
	var editor *Editor
	var points savePointEditor
	checked := 0
	for {
		fields, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		at, atErr := strconv.Atoi(fields[3])
		removed, removedErr := strconv.Atoi(fields[4])
		if atErr != nil || removedErr != nil {
			t.Fatal("invalid save-point oracle position")
		}
		inserted := decode(fields[5])
		switch fields[2] {
		case "start":
			editor = NewEditor(inserted)
			points = savePoints(t, editor)
			points.SetSavePoint()
		case "save":
			points.SetSavePoint()
		case "clear":
			editor.ClearHistory()
		case "undo":
			editor.Undo()
		case "redo":
			editor.Redo()
		case "type":
			position := editor.RuneIndex(at)
			if editor.Selection().Start != editor.Selection().End || editor.Caret() != position {
				editor.MoveTo(position, false)
			}
			editor.Insert(inserted)
		case "backspace":
			position := editor.RuneIndex(at + removed)
			if editor.Selection().Start != editor.Selection().End || editor.Caret() != position {
				editor.MoveTo(position, false)
			}
			editor.DeleteBackward()
		case "replace":
			editor.ReplaceRange(Range{Start: editor.RuneIndex(at), End: editor.RuneIndex(at + removed)}, inserted)
		case "zero-transaction":
			if err := editor.Transact(func(draft *Editor) error { draft.Insert("x"); draft.DeleteBackward(); return nil }); err != nil {
				t.Fatal(err)
			}
		case "equal-batch":
			if err := editor.ReplaceRangesWith([]Replacement{{Range: Range{End: 1}, Text: "a"}, {Range: Range{Start: 2, End: 3}, Text: "a"}}, ReplaceRangesOptions{Forward: true, ApplyUnchanged: true}); err != nil {
				t.Fatal(err)
			}
		default:
			t.Fatalf("unknown save-point operation %q", fields[2])
		}
		if editor.String() != decode(fields[6]) || points.AtSavePoint() != (fields[7] == "1") || editor.CanUndo() != (fields[8] == "1") || editor.CanRedo() != (fields[9] == "1") {
			t.Fatalf("oracle %s step %s %s: text=%q saved=%v undo=%v redo=%v, want text=%q saved=%s undo=%s redo=%s", fields[0], fields[1], fields[2], editor.String(), points.AtSavePoint(), editor.CanUndo(), editor.CanRedo(), decode(fields[6]), fields[7], fields[8], fields[9])
		}
		checked++
	}
	if checked != 73 {
		t.Fatalf("checked %d save-point states, want 73", checked)
	}
	t.Logf("checked %d original UndoHistory states", checked)
}

func TestSavePointModeIsOptionalAndDoesNotEnableByteTracking(t *testing.T) {
	editor := NewEditor("cat")
	points := savePoints(t, editor)
	if points.AtSavePoint() {
		t.Fatal("unmarked editor claimed a save point")
	}
	editor.ReplaceRange(Range{End: 3}, "cat")
	if editor.CanUndo() {
		t.Fatal("untracked editor changed its default equal-text policy")
	}
	points.SetSavePoint()
	if _, _, complete := editor.EditsSince(0, nil); complete {
		t.Fatal("save-point tracking enabled byte history")
	}
	editor.ReplaceRange(Range{End: 3}, "cat")
	if points.AtSavePoint() || !editor.CanUndo() {
		t.Fatal("tracked equal replacement did not leave its saved boundary")
	}
	if _, _, complete := editor.EditsSince(0, nil); complete {
		t.Fatal("tracked save-point edit enabled byte tracking")
	}
	cursor := editor.TrackEdits()
	if !editor.Undo() || !points.AtSavePoint() {
		t.Fatal("tracked equal edit did not Undo to its point")
	}
	if edits, _, complete := editor.EditsSince(cursor, nil); !complete || len(edits) == 0 {
		t.Fatal("late byte tracking lost the equal Undo")
	}
}

func TestSavePointPruningDoesNotClaimDiscardedDataIsSaved(t *testing.T) {
	editor := NewEditor("a")
	points := savePoints(t, editor)
	points.SetSavePoint()
	for range maxHistoryEntries + 4 {
		editor.Paste("!")
	}
	for editor.Undo() {
	}
	if points.AtSavePoint() || editor.String() == "a" {
		t.Fatal("pruned history still claimed the original point was reachable")
	}
	points.SetSavePoint()
	if !points.AtSavePoint() {
		t.Fatal("new point after pruning was not marked")
	}
	editor.ReplaceRange(Range{End: editor.RuneCount()}, strings.Repeat("x", maxSingleEditRunes+1))
	if editor.CanUndo() || points.AtSavePoint() {
		t.Fatal("oversized history reset falsely marked modified data saved")
	}
	editor.ClearHistory()
	if !points.AtSavePoint() {
		t.Fatal("explicit history clear did not rebase the engine point")
	}
}

func TestSavePointDraftModeDoesNotMarkTheSourceSaved(t *testing.T) {
	editor := NewEditor("base")
	points := savePoints(t, editor)
	points.SetSavePoint()
	if err := editor.Transact(func(draft *Editor) error {
		if !savePoints(t, draft).AtSavePoint() {
			t.Fatal("draft did not inherit save-point editing mode")
		}
		draft.Insert("!")
		savePoints(t, draft).SetSavePoint()
		return nil
	}); err != nil || points.AtSavePoint() || !editor.CanUndo() {
		t.Fatal("draft save point escaped onto the source")
	}
	if !editor.Undo() || !points.AtSavePoint() || editor.String() != "base" {
		t.Fatal("draft commit lost the original source point")
	}
}

func TestSavePointInsideRetainedPruningWindowRemainsReachable(t *testing.T) {
	editor := NewEditor("base")
	points := savePoints(t, editor)
	for range maxHistoryEntries {
		editor.Paste("!")
	}
	points.SetSavePoint()
	for range 4 {
		editor.Paste("?")
	}
	for range 4 {
		if !editor.Undo() {
			t.Fatal("retained point lost an adjacent history unit")
		}
	}
	if !points.AtSavePoint() || editor.String() != "base"+strings.Repeat("!", maxHistoryEntries) {
		t.Fatal("pruning older history shifted a retained save point incorrectly")
	}
	if !editor.Redo() || points.AtSavePoint() || !editor.Undo() || !points.AtSavePoint() {
		t.Fatal("retained save point did not survive Undo/Redo")
	}
}

var savePointBenchmarkEditor *Editor

func BenchmarkSavePointMetadata(b *testing.B) {
	for _, enabled := range []bool{false, true} {
		name := "default"
		if enabled {
			name = "savepoint"
		}
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			b.ReportMetric(float64(unsafe.Sizeof(Editor{})), "editor-B")
			b.ReportMetric(float64(unsafe.Sizeof(editJournal{})), "metadata-B")
			for range b.N {
				editor := NewEditor("base")
				if enabled {
					editor.SetSavePoint()
				}
				savePointBenchmarkEditor = editor
			}
		})
	}
}
