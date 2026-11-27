package components

import (
	"fmt"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/markdown"
)

func TestMarkdownBuilderLabeledAndOpaqueLinks(t *testing.T) {
	var doc markdown.Document
	doc.Reset()
	doc.AppendParagraph(0, 0, []markdown.Span{
		{Text: "Iterators returned by "},
		{Text: "iterator", Style: markdown.Mono, Link: "symbol:#iterator()"},
		{Text: " are "},
		{Text: "fail-fast", Style: markdown.Italic},
		{Text: "."},
	})
	doc.AppendLabeled(0, []markdown.Span{{Text: "Since:"}}, []markdown.Span{{Text: "1.2"}})
	doc.AppendLabeled(0, []markdown.Span{{Text: "See also:"}}, []markdown.Span{
		{Text: "Collection", Style: markdown.Mono, Link: "symbol:Collection"},
		{Text: ", "},
		{Text: "List", Style: markdown.Mono, Link: "symbol:List"},
	})
	doc.AppendLabeled(0, []markdown.Span{{Text: "Type parameters:"}}, []markdown.Span{
		{Text: "<E>", Style: markdown.Mono},
		{Text: " – the type of elements"},
	})

	canvas := markdownCanvas()
	var view MarkdownView
	h := view.Layout(canvas, MarkdownStyle{Size: 13, LineHeight: 20}, &doc, 460)
	if h <= 0 {
		t.Fatalf("expected positive height, got %v", h)
	}
	if view.LinkCount() != 3 {
		t.Fatalf("LinkCount = %d, want 3", view.LinkCount())
	}
	if got := view.Link(0); got != "symbol:#iterator()" {
		t.Fatalf("Link(0) = %q, want opaque symbol target", got)
	}
	if view.AllocatedBytes() <= 0 || doc.AllocatedBytes() <= 0 {
		t.Fatalf("expected positive AllocatedBytes: view=%d doc=%d", view.AllocatedBytes(), doc.AllocatedBytes())
	}
	// Verify aligned Labeled value columns start at the same X.
	var labelEndX, valStartX []float32
	for _, blk := range view.blockSpans {
		if blk.kind != markdown.Labeled {
			continue
		}
		lCell := view.cellSpans[blk.cellLo]
		vCell := view.cellSpans[blk.cellLo+1]
		if lCell.rowLo < lCell.rowHi && vCell.rowLo < vCell.rowHi {
			lp := view.pieces[view.rowSpans[lCell.rowLo].pieceLo]
			vp := view.pieces[view.rowSpans[vCell.rowLo].pieceLo]
			labelEndX = append(labelEndX, lp.x+lp.w)
			valStartX = append(valStartX, vp.x)
		}
	}
	if len(valStartX) != 3 {
		t.Fatalf("expected 3 labeled rows, got %d", len(valStartX))
	}
	for i := 1; i < len(valStartX); i++ {
		if valStartX[i] != valStartX[0] {
			t.Fatalf("labeled value column misaligned: %v (labels end at %v)", valStartX, labelEndX)
		}
	}
}

func TestMarkdownViewTallTableCellFourLevelIndex(t *testing.T) {
	// Single-row table where one cell wraps into >1000 visual lines: clipped
	// Paint and OffsetAt must visit only visible cell lines, not all 1000.
	tallSpans := make([]markdown.Span, 0, 2200)
	for i := 0; i < 1100; i++ {
		if i > 0 {
			tallSpans = append(tallSpans, markdown.Span{Text: markdown.Newline})
		}
		tallSpans = append(tallSpans, markdown.Span{Text: fmt.Sprintf("cell-line-%04d", i)})
	}
	var doc markdown.Document
	doc.AppendTable(0, 2, [][]markdown.TableCell{
		{
			{Spans: []markdown.Span{{Text: "Header A"}}},
			{Spans: []markdown.Span{{Text: "Header B"}}},
		},
		{
			{Spans: []markdown.Span{{Text: "Short"}}},
			{Spans: tallSpans},
		},
	})

	canvas := markdownCanvas()
	var view MarkdownView
	h := view.Layout(canvas, MarkdownStyle{Size: 13, LineHeight: 20}, &doc, 400)
	if h < 15000 {
		t.Fatalf("expected tall table height >= 15000, got %v", h)
	}

	// Count rows visited for a 200px viewport slice in the middle of the tall cell.
	clipTop, clipBottom := float32(10000), float32(10200)
	firstBlk, lastBlk := view.visibleBlockRange(true, clipTop, clipBottom)
	visited := 0
	view.visitVisibleRows(firstBlk, lastBlk, true, clipTop, clipBottom, func(*mdRowSpan) {
		visited++
	})
	if visited == 0 || visited > 20 {
		t.Fatalf("visited %d rows for 200px slice of 1100-line cell, want 1..20", visited)
	}

	// Paint the slice and verify OffsetAt + CaretAt round-trip inside the middle of the cell.
	canvas.Reset()
	view.Paint(canvas, MarkdownPaint{
		Look: MarkdownLookFor(gfx.Theme{}),
		X:    0,
		Y:    -clipTop,
		Clip: gfx.Rect{X: 0, Y: 0, Width: 400, Height: 200},
	})
	off, ok := view.OffsetAt(gfx.Point{X: 220, Y: 100})
	if !ok {
		t.Fatal("OffsetAt in tall cell returned ok=false")
	}
	rect, ok := view.CaretAt(off)
	if !ok {
		t.Fatalf("CaretAt(%d) returned ok=false", off)
	}
	if rect.Y < -25 || rect.Y > 225 {
		t.Fatalf("CaretAt(%d) Y=%v outside painted slice [0, 200]", off, rect.Y)
	}
}

func TestMarkdownViewLongQuoteBarAndShortBlocksIndex(t *testing.T) {
	// Counterexample check: one quote bar spanning many blocks does not force
	// visiting off-screen blocks or off-screen quote bars.
	var doc markdown.Document
	for i := 0; i < 600; i++ {
		doc.AppendParagraph(1, 0, []markdown.Span{{Text: fmt.Sprintf("Quoted paragraph %d", i)}})
	}
	canvas := markdownCanvas()
	var view MarkdownView
	h := view.Layout(canvas, MarkdownStyle{Size: 13, LineHeight: 20}, &doc, 420)
	if h < 10000 {
		t.Fatalf("expected tall quoted doc, got %v", h)
	}

	clipTop, clipBottom := float32(5000), float32(5200)
	firstBlk, lastBlk := view.visibleBlockRange(true, clipTop, clipBottom)
	if count := lastBlk - firstBlk; count == 0 || count > 20 {
		t.Fatalf("visible blocks = %d, want 1..20", count)
	}
	bars := view.visibleQuoteBars(0, true, clipTop, clipBottom)
	if len(bars) != 1 {
		t.Fatalf("visible quote bars at depth 0 = %d, want 1", len(bars))
	}
}

func BenchmarkMarkdownViewClippedPaintSteady(b *testing.B) {
	var doc markdown.Document
	for i := 0; i < 400; i++ {
		doc.AppendParagraph(1, 0, []markdown.Span{
			{Text: "Iterators returned by "},
			{Text: "iterator", Style: markdown.Mono, Link: "symbol:#iterator()"},
			{Text: " are "},
			{Text: "fail-fast", Style: markdown.Italic},
			{Text: " in paragraph."},
		})
	}
	canvas := markdownCanvas()
	var view MarkdownView
	view.Layout(canvas, MarkdownStyle{Size: 13, LineHeight: 20}, &doc, 500)
	look := MarkdownLookFor(gfx.Theme{})
	paint := MarkdownPaint{
		Look: look,
		X:    0,
		Y:    -4000,
		Clip: gfx.Rect{X: 0, Y: 0, Width: 500, Height: 300},
	}
	// Warm canvas command buffer and view scratch slices.
	canvas.Reset()
	view.Paint(canvas, paint)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		canvas.Reset()
		view.Paint(canvas, paint)
	}
}
