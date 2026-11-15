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
