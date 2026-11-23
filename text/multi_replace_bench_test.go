package text

import (
	"strings"
	"testing"
)

// BenchmarkReplaceRangesAtTheEndOfALongText replaces a range at the end of a text of
// 20000 lines, as a log bound to a field grows: the cost is the edit's, not the text's.
func BenchmarkReplaceRangesAtTheEndOfALongText(b *testing.B) {
	editor := NewEditor(strings.Repeat("line of a log with 中文 in it\n", 20000))
	editor.ClearHistory()
	end := editor.RuneCount()
	edits := [2][1]Replacement{{{Range: Range{Start: end, End: end}, Text: "appended\n"}}, {{Range: Range{Start: end, End: end + 9}}}}
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		if err := editor.ReplaceRanges(edits[i%2][:]); err != nil {
			b.Fatal(err)
		}
	}
}
