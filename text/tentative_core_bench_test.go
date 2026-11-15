package text

import (
	"strings"
	"testing"
)

var tentativeCoreEditorSink *Editor
var tentativeCoreFramesSink []HistoryFrame

func BenchmarkTentativeCoreWorkflow(b *testing.B) {
	for _, history := range []bool{false, true} {
		name := "physical"
		if history {
			name = "with-history"
		}
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				editor := NewEditor("abc")
				var cursor HistoryCursor
				if history {
					cursor, _, _ = editor.TrackHistory()
				}
				editor.EnableTentativeComposition()
				editor.MoveTo(1, false)
				editor.ApplyComposition(CompositionEvent{Phase: CompositionUpdate, Text: "x"})
				editor.ApplyComposition(CompositionEvent{Phase: CompositionResult, Text: "yz"})
				editor.Undo()
				editor.Redo()
				if history {
					tentativeCoreFramesSink, _, _ = editor.HistorySince(cursor)
				}
				tentativeCoreEditorSink = editor
			}
		})
	}
}

func BenchmarkTentativeCoreLargeOverwrite(b *testing.B) {
	value, marked := strings.Repeat("a", 2049), strings.Repeat("z", 2049)
	b.ReportAllocs()
	for b.Loop() {
		editor := NewEditor(value)
		editor.TrackHistory()
		editor.SetOverwrite(true)
		editor.MoveTo(0, false)
		editor.EnableTentativeComposition()
		editor.ApplyComposition(CompositionEvent{Phase: CompositionUpdate, Text: marked})
		editor.ApplyComposition(CompositionEvent{Phase: CompositionCancel})
		tentativeCoreEditorSink = editor
	}
}
