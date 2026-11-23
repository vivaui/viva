package gfx

import "testing"

// The estimate is what a canvas without a text engine wraps to: both measure with the
// reference metrics, so a view that estimates its rows before it shows them gets the rows
// it would have shown wherever the platform measures as the reference does.
func TestWrapRowEstimateIsTheReferenceWrap(t *testing.T) {
	texts := []string{
		"", "\n", "a\r\nb\n",
		"the quick brown fox jumps over the lazy dog and keeps running through the forest",
		"中文混排的一段文字，用来检查东亚文字在行尾的断行规则。and some English words",
		"averyveryverylongwordwithoutanyspacesthatmustbreakinthemiddle/of/a/path",
		"  leading spaces and trailing ones   \n\nthen a blank line",
	}
	var canvas Canvas
	for _, text := range texts {
		for _, width := range []float32{0, 40, 120, 300} {
			for from := 0; from >= 0; {
				want, wantNext := canvas.WrapRow(text, from, 13, WeightRegular, width)
				got, next := WrapRowEstimate(text, from, 13, WeightRegular, width)
				if got != want || next != wantNext {
					t.Fatalf("%q at %v from %d: estimate %v next %d, canvas %v next %d", text, width, from, got, next, want, wantNext)
				}
				from = next
			}
		}
	}
}
