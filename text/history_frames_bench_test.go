package text

import "testing"

var historyFramesEditorSink *Editor
var historyFramesReadSink []HistoryFrame

func BenchmarkHistoryFramesClient(b *testing.B) {
	for _, tracked := range []bool{false, true} {
		mode := "default"
		if tracked {
			mode = "enabled"
		}
		b.Run(mode, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				editor := NewEditor("")
				if tracked {
					editor.TrackHistory()
				}
				for _, value := range []string{"h", "e", "\n", "l", "l", "o"} {
					editor.Insert(value)
				}
				editor.SetSavePoint()
				editor.Undo()
				editor.Redo()
				historyFramesEditorSink = editor
			}
		})
	}
}

func BenchmarkHistoryFramesReadPending(b *testing.B) {
	editor := NewEditor("")
	from, _, ok := editor.TrackHistory()
	if !ok {
		b.Fatal("producer unavailable")
	}
	for range 16 {
		editor.Insert("x")
	}
	b.ReportAllocs()
	for b.Loop() {
		frames, _, ok := editor.HistorySince(from)
		if !ok {
			b.Fatal("retained prefix unavailable")
		}
		historyFramesReadSink = frames
	}
}
