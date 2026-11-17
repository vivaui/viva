package text

import "testing"

func BenchmarkLFLabelsTentativeWorkflow(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		editor := NewEditor("a\nbc")
		if !editor.TrackLFLabels([]byte("W"), 'L', nil) || !editor.EnableTentativeComposition() {
			b.Fatal("capability setup refused")
		}
		editor.MoveTo(1, false)
		editor.ApplyComposition(CompositionEvent{Phase: CompositionUpdate, Text: "x"})
		editor.ApplyComposition(CompositionEvent{Phase: CompositionResult, Text: "yz"})
		editor.Undo()
		editor.Redo()
		tentativeCoreEditorSink = editor
	}
	labels, ok := tentativeCoreEditorSink.LFLabels(nil)
	if tentativeCoreEditorSink.String() != "ayz\nbc" || tentativeCoreEditorSink.CompositionActive() || !ok || string(labels) != "W" {
		b.Fatal("workflow did not retain its committed result")
	}
}

func BenchmarkLFLabelsTentativeRekey(b *testing.B) {
	for _, legacy := range []bool{false, true} {
		name := "prestart-deletion"
		if legacy {
			name = "legacy-held-history"
		}
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				editor := NewEditor("a\nb")
				if !editor.TrackLFLabels([]byte("W"), 'M', nil) {
					b.Fatal("label setup refused")
				}
				editor.SetSelection(Range{Start: 1, End: 2})
				if legacy {
					editor.Insert("")
				}
				if !editor.EnableTentativeComposition() {
					b.Fatal("capability setup refused")
				}
				if legacy {
					editor.SetMarkedText("q\n", Range{End: 2})
					if !editor.CancelComposition() {
						b.Fatal("legacy cancellation refused")
					}
				} else if !editor.ApplyComposition(CompositionEvent{Phase: CompositionUpdate, Text: "q\n"}) ||
					!editor.ApplyComposition(CompositionEvent{Phase: CompositionCancel}) {
					b.Fatal("tentative event refused")
				}
				if !editor.Undo() || !editor.Redo() {
					b.Fatal("deletion replay refused")
				}
				tentativeCoreEditorSink = editor
			}
			// Both versions replay the deletion text; baseline LF metadata is
			// broken. The regression tests verify labels through Undo and Redo.
			if tentativeCoreEditorSink.String() != "ab" || tentativeCoreEditorSink.CompositionActive() {
				b.Fatal("workflow did not retain its replayed deletion")
			}
		})
	}
}
