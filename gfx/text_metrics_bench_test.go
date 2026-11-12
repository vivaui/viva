// Wrapping and measuring with the metrics every platform shares (internal/textmetrics):
// no installed measurer, no cache, no allocation.
package gfx

import "testing"

const benchParagraph = "Viva lays text out in lines: a paragraph breaks at spaces, between East Asian characters, after the separators of a long path such as /usr/local/share/applications/example.desktop, and as a last resort inside a word like pneumonoultramicroscopicsilicovolcanoconiosis. 文本按宽度折行：东亚文字可以在任意两个字符之间断开。"

func BenchmarkWrapParagraphReferenceMetrics(b *testing.B) {
	var canvas Canvas
	var dst []string
	b.ReportAllocs()
	for b.Loop() {
		dst = canvas.WrapLines(dst[:0], benchParagraph, 13, WeightRegular, 360, 20)
	}
}

func BenchmarkTextWidthReferenceMetrics(b *testing.B) {
	var canvas Canvas
	b.ReportAllocs()
	var sink float32
	for b.Loop() {
		sink += canvas.TextWidthWeight("Connect to host-01", 13, WeightMedium)
	}
	_ = sink
}
