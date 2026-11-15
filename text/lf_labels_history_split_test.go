package text

import "testing"

func TestLFLabelCompositionSplitHistoryUnitsPreserveLabels(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		finish func(*Editor)
		value  string
		labels []byte
	}{
		{"paste", func(editor *Editor) { editor.Paste("c") }, "a\nbc", []byte{'W'}},
		{"separate insert", func(editor *Editor) { editor.BreakUndoGroup(); editor.Insert("c") }, "a\nbc", []byte{'W'}},
		{"save point", func(editor *Editor) { editor.SetSavePoint(); editor.SetMarkedText("\n\n", Range{}) }, "a\nb\n\n", []byte{'W', 'M', 'M'}},
		{"save point then merge", func(editor *Editor) {
			editor.SetSavePoint()
			editor.SetMarkedText("\n\n", Range{})
			editor.SetMarkedText("\n\n\n", Range{})
		}, "a\nb\n\n\n", []byte{'W', 'M', 'M', 'M'}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			editor := NewEditor("a\nb")
			if !editor.TrackLFLabels([]byte{'W'}, 'M', lfLabelPreserveReplacement) {
				t.Fatal("label setup refused")
			}
			editor.MoveTo(editor.RuneCount(), false)
			editor.SetMarkedText("\n", Range{})
			scenario.finish(editor)
			editor.Unmark()
			if editor.String() != scenario.value {
				t.Fatalf("final text=%q, want %q", editor.String(), scenario.value)
			}
			assertLFLabels(t, editor, scenario.labels)
			if !editor.Undo() || editor.String() != "a\nb\n" {
				t.Fatalf("split unit Undo=%q", editor.String())
			}
			assertLFLabels(t, editor, []byte{'W', 'M'})
			if !editor.SetLFLabels([]byte{'W', 'Z'}, 'D') {
				t.Fatal("restored LF identities cannot be relabeled")
			}
			if !editor.Redo() || editor.String() != scenario.value {
				t.Fatalf("split unit Redo=%q", editor.String())
			}
			// Replacement LFs have their own identities; replay keeps their labels,
			// while the relabeled removed LF must return on the next Undo.
			assertLFLabels(t, editor, scenario.labels)
			if !editor.Undo() || editor.String() != "a\nb\n" {
				t.Fatal("repeated split Undo lost source")
			}
			assertLFLabels(t, editor, []byte{'W', 'Z'})
			editor.MoveTo(editor.RuneCount(), false)
			editor.Insert("\n")
			assertLFLabels(t, editor, []byte{'W', 'Z', 'D'})
			if !editor.Undo() || editor.String() != "a\nb\n" {
				t.Fatal("new branch insertion lost its Undo")
			}
			assertLFLabels(t, editor, []byte{'W', 'Z'})
		})
	}
}

func TestLFLabelCompositionSplitWithoutLFForgetsPreviousUnitLabels(t *testing.T) {
	editor := NewEditor("a\nb")
	if !editor.TrackLFLabels([]byte{'W'}, 'M', lfLabelPreserveReplacement) {
		t.Fatal("label setup refused")
	}
	editor.SetSelection(Range{Start: 1, End: 2})
	editor.SetMarkedText("x", Range{})
	editor.SetSavePoint()
	editor.SetMarkedText("y", Range{})
	editor.Unmark()
	assertLFLabels(t, editor, nil)
	if !editor.Undo() || editor.String() != "axb" {
		t.Fatalf("LF-free split Undo=%q", editor.String())
	}
	assertLFLabels(t, editor, nil)
	if !editor.Redo() || editor.String() != "ayb" {
		t.Fatalf("LF-free split Redo=%q", editor.String())
	}
	assertLFLabels(t, editor, nil)
	if !editor.Undo() || !editor.Undo() || editor.String() != "a\nb" {
		t.Fatal("original LF source lost Undo")
	}
	assertLFLabels(t, editor, []byte{'W'})
}
