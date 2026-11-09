package text

import "testing"

func BenchmarkLineJournalBatchHistory(b *testing.B) {
	for _, tracked := range []bool{false, true} {
		name := "bytes"
		if tracked {
			name = "lines"
		}
		b.Run(name, func(b *testing.B) {
			editor := NewEditor("a\nb")
			if tracked {
				editor.TrackLineEdits()
			} else {
				editor.TrackEdits()
			}
			for index := range maxHistoryEntries {
				value := "A"
				if index%2 != 0 {
					value = "B"
				}
				if err := editor.ReplaceRanges([]Replacement{
					{Range: Range{End: 1}, Text: value},
					{Range: Range{Start: 2, End: 3}, Text: value},
				}); err != nil {
					b.Fatal(err)
				}
			}
			b.ReportAllocs()
			cycle := func() {
				for range maxHistoryEntries {
					if !editor.Undo() {
						b.Fatal("missing batch Undo")
					}
				}
				for range maxHistoryEntries {
					if !editor.Redo() {
						b.Fatal("missing batch Redo")
					}
				}
			}
			cycle()
			b.ResetTimer()
			for b.Loop() {
				cycle()
			}
			b.ReportMetric(float64(maxHistoryEntries), "units")
		})
	}
}
