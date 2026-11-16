package text

import (
	"fmt"
	"testing"
)

func TestLFLabelTentativePrestartDeletionReplay(t *testing.T) {
	for _, phase := range []CompositionPhase{CompositionCancel, CompositionResult, CompositionCommit} {
		for _, save := range []bool{false, true} {
			t.Run(fmt.Sprintf("phase-%d/save-%t", phase, save), func(t *testing.T) {
				editor := NewEditor("a\nb\nc\nd")
				if !editor.TrackLFLabels([]byte("WLM"), 'W', nil) || !editor.EnableTentativeComposition() {
					t.Fatal("capability setup refused")
				}
				editor.SetSelection(Range{Start: 1, End: 2})
				if !editor.ApplyComposition(CompositionEvent{Phase: CompositionUpdate, Text: "x\n"}) ||
					!editor.ApplyComposition(CompositionEvent{Phase: phase, Text: "x\n"}) {
					t.Fatal("tentative event refused")
				}
				if editor.CompositionActive() {
					t.Fatal("terminal event left a live composition")
				}
				if save {
					editor.SetSavePoint()
				}
				if phase != CompositionCancel {
					if editor.String() != "ax\nb\nc\nd" {
						t.Fatalf("committed text = %q", editor.String())
					}
					assertLFLabels(t, editor, []byte("WLM"))
					if !editor.Undo() {
						t.Fatal("result Undo refused")
					}
				}
				if editor.String() != "ab\nc\nd" {
					t.Fatalf("post-preedit text = %q", editor.String())
				}
				assertLFLabels(t, editor, []byte("LM"))
				if !editor.Undo() || editor.String() != "a\nb\nc\nd" || editor.CanUndo() {
					t.Fatalf("prestart deletion Undo = %q", editor.String())
				}
				assertLFLabels(t, editor, []byte("WLM"))
				if !editor.SetLFLabels([]byte("ZLM"), 'W') {
					t.Fatal("restored LF identities cannot be relabeled")
				}
				if !editor.Redo() || editor.String() != "ab\nc\nd" {
					t.Fatalf("prestart deletion Redo = %q", editor.String())
				}
				assertLFLabels(t, editor, []byte("LM"))
				if phase != CompositionCancel {
					if !editor.Redo() || editor.String() != "ax\nb\nc\nd" {
						t.Fatalf("result Redo = %q", editor.String())
					}
					assertLFLabels(t, editor, []byte("WLM"))
					if !editor.Undo() {
						t.Fatal("repeated result Undo refused")
					}
				}
				if !editor.Undo() || editor.String() != "a\nb\nc\nd" {
					t.Fatalf("repeated deletion Undo = %q", editor.String())
				}
				assertLFLabels(t, editor, []byte("ZLM"))
			})
		}
	}
}

func TestLFLabelTentativeCoalescedInsertedKeysReplayRelabel(t *testing.T) {
	editor := NewEditor("a\nb")
	calls := 0
	if !editor.TrackLFLabels([]byte("W"), 'M', func(LFLabelReplacement) []byte {
		calls++
		return []byte("M")
	}) {
		t.Fatal("label setup refused")
	}
	editor.MoveTo(editor.RuneCount(), false)
	if !editor.EnableTentativeComposition() {
		t.Fatal("capability setup refused")
	}
	for _, want := range []string{"a\nb\n", "a\nb\n\n", "a\nb\n\n\n"} {
		if !editor.ApplyComposition(CompositionEvent{Phase: CompositionResult, Text: "\n"}) || editor.String() != want {
			t.Fatalf("direct result = %q, want %q", editor.String(), want)
		}
	}
	assertLFLabels(t, editor, []byte("WMMM"))
	if !editor.SetLFLabels([]byte("WXYZ"), 'M') {
		t.Fatal("inserted LF identities cannot be relabeled")
	}
	// Explicit insertion-mode Result events preserve source coalescing,
	// including their LF scalars, so these three results share one unit.
	for range 3 {
		if !editor.Undo() || editor.String() != "a\nb" || editor.CanUndo() {
			t.Fatalf("coalesced result Undo = %q, want original text in one unit", editor.String())
		}
		assertLFLabels(t, editor, []byte("W"))
		if !editor.Redo() || editor.String() != "a\nb\n\n\n" {
			t.Fatalf("coalesced result Redo = %q", editor.String())
		}
		assertLFLabels(t, editor, []byte("WXYZ"))
	}
	if calls != 3 {
		t.Fatalf("compaction or replay reran the label mapper: %d calls, want 3", calls)
	}
}

func TestLFLabelTentativeCompactionPreservesLegacyHeldHistory(t *testing.T) {
	editor := NewEditor("a\nb")
	if !editor.TrackLFLabels([]byte("W"), 'M', nil) {
		t.Fatal("label setup refused")
	}
	editor.SetSelection(Range{Start: 1, End: 2})
	editor.Insert("")
	if !editor.EnableTentativeComposition() {
		t.Fatal("capability setup refused")
	}
	editor.SetMarkedText("q\n", Range{End: 2})
	if !editor.CancelComposition() || editor.String() != "ab" {
		t.Fatalf("legacy cancellation = %q, want ab", editor.String())
	}
	assertLFLabels(t, editor, nil)
	if !editor.Undo() || editor.String() != "a\nb" || editor.CanUndo() {
		t.Fatalf("held deletion Undo = %q", editor.String())
	}
	assertLFLabels(t, editor, []byte("W"))
	if !editor.Redo() || editor.String() != "ab" {
		t.Fatalf("held deletion Redo = %q", editor.String())
	}
	assertLFLabels(t, editor, nil)
}
