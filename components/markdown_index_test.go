package components

import (
	"fmt"
	"strings"
	"testing"
	"unsafe"

	"github.com/vivaui/viva/base"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/markdown"
)

// linearOffsetAtReference mirrors the pre-index linear scan over v.pieces so
// tests can verify exact equivalence across dense grids of pointer positions.
func linearOffsetAtReference(v *MarkdownView, point gfx.Point) (int, bool) {
	if !v.painted || v.measure == nil || len(v.pieces) == 0 {
		return 0, false
	}
	x, y := point.X-v.paintX, point.Y-v.paintY
	best, bestScore := -1, float32(0)
	var previous *mdPiece
	for index := range v.pieces {
		piece := &v.pieces[index]
		if piece.plainLo < 0 {
			continue
		}
		if previous != nil && previous.y == piece.y && previous.plainHi < piece.plainLo && y >= piece.y && y <= piece.y+piece.h {
			start := previous.x + v.caretAdvance(previous, int(previous.plainHi-previous.plainLo))
			end := piece.x + v.caretAdvance(piece, 0)
			if end > start && x >= start && x <= end {
				gap := int(piece.plainLo - previous.plainHi)
				position := int(float32(gap)*(x-start)/(end-start) + 0.5)
				return gfx.GraphemeStart(unsafe.String(unsafe.SliceData(v.plain), len(v.plain)), int(previous.plainHi)+min(max(position, 0), gap)), true
			}
		}
		previous = piece
		dx, dy := float32(0), float32(0)
		if x < piece.x {
			dx = piece.x - x
		} else if x > piece.x+piece.w {
			dx = x - piece.x - piece.w
		}
		if y < piece.y {
			dy = piece.y - y
		} else if y > piece.y+piece.h {
			dy = y - piece.y - piece.h
		}
		score := dy*1_000_000 + dx
		if best < 0 || score < bestScore {
			best, bestScore = index, score
		}
	}
	if best < 0 {
		return 0, false
	}
	piece := &v.pieces[best]
	localX := min(max(x-piece.x, 0), piece.w)
	var local int
	if piece.mono {
		local = v.codeOffsetAt(piece, localX)
	} else {
		local = v.measure.TextOffsetAtStyle(piece.text, gfx.TextStyle{Size: piece.size, Weight: piece.weight, Italic: piece.italic}, localX)
	}
	local = min(max(local, 0), int(piece.plainHi-piece.plainLo))
	return int(piece.plainLo) + local, true
}

// linearCaretAtReference mirrors the pre-index linear scan over v.pieces so
// tests can verify exact equivalence for every byte offset.
func linearCaretAtReference(v *MarkdownView, offset int) (gfx.Rect, bool) {
	if !v.painted || v.measure == nil || len(v.pieces) == 0 {
		return gfx.Rect{}, false
	}
	value := unsafe.String(unsafe.SliceData(v.plain), len(v.plain))
	offset = gfx.GraphemeStart(value, offset)
	var chosen, before, after *mdPiece
	for index := range v.pieces {
		piece := &v.pieces[index]
		if piece.plainLo < 0 {
			continue
		}
		switch {
		case offset >= int(piece.plainLo) && offset <= int(piece.plainHi):
			if chosen == nil || piece.plainLo == int32(offset) && chosen.plainLo < int32(offset) {
				chosen = piece
			}
		case int(piece.plainHi) < offset:
			if before == nil || piece.plainHi > before.plainHi {
				before = piece
			}
		case int(piece.plainLo) > offset:
			if after == nil || piece.plainLo < after.plainLo {
				after = piece
			}
		}
	}
	if chosen == nil {
		if before != nil && after != nil && before.y == after.y {
			start := before.x + v.caretAdvance(before, int(before.plainHi-before.plainLo))
			end := after.x + v.caretAdvance(after, 0)
			fraction := float32(offset-int(before.plainHi)) / float32(after.plainLo-before.plainHi)
			return gfx.Rect{X: v.paintX + start + (end-start)*fraction, Y: v.paintY + before.y, Width: 1, Height: before.h}, true
		}
		if before == nil && after == nil {
			return gfx.Rect{}, false
		}
		if before == nil || after != nil && int(after.plainLo)-offset <= offset-int(before.plainHi) {
			chosen = after
			offset = int(after.plainLo)
		} else {
			chosen = before
			offset = int(before.plainHi)
		}
	}
	local := min(max(offset-int(chosen.plainLo), 0), int(chosen.plainHi-chosen.plainLo))
	advance := v.caretAdvance(chosen, local)
	return gfx.Rect{X: v.paintX + chosen.x + advance, Y: v.paintY + chosen.y, Width: 1, Height: chosen.h}, true
}

func TestMarkdownIndexMatchesLinearReferenceOnGrid(t *testing.T) {
	fullSrc := strings.Join([]string{
		"# Overview",
		"",
		"before text with [link](https://example.com) and `code` 中文",
		"",
		"```go",
		"firstLine := 1",
		"",
		"",
		"",
		"lastLine := 2",
		"```",
		"",
		"after text",
		"",
		"---",
		"",
		"```",
		"",
		"",
		"",
		"onlyTrailing()",
		"```",
		"",
		"> outer quote line one",
		">> nested quote line two",
		"",
		"| Col A | Col B | Col C |",
		"| --- | :---: | ---: |",
		"| alpha |  | gamma wrapped across multiple words in a narrow column |",
		"|  |  |  |",
		"| delta | epsilon | zeta |",
		"",
		"1. First item",
		"2. Second item with **bold** and *italic*",
		"",
		"- Unordered bullet",
	}, "\n")

	edgeSrc := strings.Join([]string{
		"lead paragraph",
		"",
		"---",
		"",
		"---",
		"",
		"```",
		"",
		"",
		"",
		"```",
		"",
		"| A | B |",
		"| - | - |",
		"|  |  |",
		"",
		"tail paragraph",
	}, "\n")

	cases := []struct {
		name  string
		src   string
		width float32
	}{
		{name: "parsed-full-features", src: fullSrc, width: 320},
		{name: "parsed-empty-blocks-and-rules", src: edgeSrc, width: 260},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var doc markdown.Document
			doc.Parse(tc.src)

			canvas := markdownCanvas()
			var view MarkdownView
			h := view.Layout(canvas, MarkdownStyle{Size: 13, LineHeight: 20}, &doc, tc.width)
			if h <= 0 {
				t.Fatalf("expected positive height, got %v", h)
			}
			const originX, originY float32 = 16, 24
			canvas.Reset()
			view.Paint(canvas, MarkdownPaint{
				Look: MarkdownLookFor(gfx.Theme{}),
				X:    originX,
				Y:    originY,
			})

			// Verify OffsetAt on a dense grid covering margins, gaps, blank code
			// lines, rules, table cell gaps, and boundaries.
			for y := float32(-10); y <= h+20; y += 5 {
				for x := float32(-10); x <= tc.width+20; x += 17 {
					pt := gfx.Point{X: originX + x, Y: originY + y}
					gotOff, gotOK := view.OffsetAt(pt)
					wantOff, wantOK := linearOffsetAtReference(&view, pt)
					if gotOff != wantOff || gotOK != wantOK {
						t.Fatalf("OffsetAt(%v) = (%d, %v), want linear (%d, %v)", pt, gotOff, gotOK, wantOff, wantOK)
					}
				}
			}

			// Verify CaretAt for every byte offset in [0, TextLen].
			n := view.TextLen()
			for off := 0; off <= n; off++ {
				gotRect, gotOK := view.CaretAt(off)
				wantRect, wantOK := linearCaretAtReference(&view, off)
				if gotRect != wantRect || gotOK != wantOK {
					t.Fatalf("CaretAt(%d) = (%+v, %v), want linear (%+v, %v)", off, gotRect, gotOK, wantRect, wantOK)
				}
			}
		})
	}
}

func TestMarkdownEmptyCodeLinesStayInCodeBlock(t *testing.T) {
	// Direct regression for blank lines inside a code block: clicking an empty
	// line between code lines must resolve to the code block, not before/after text.
	src := "before text\n\n```\nline1\n\n\n\nline5\n```\n\nafter text\n"
	var doc markdown.Document
	doc.Parse(src)

	canvas := markdownCanvas()
	var view MarkdownView
	h := view.Layout(canvas, MarkdownStyle{Size: 13, LineHeight: 20}, &doc, 400)
	canvas.Reset()
	view.Paint(canvas, MarkdownPaint{Look: MarkdownLookFor(gfx.Theme{})})

	plain := view.SelectedText(base.TextRange{Anchor: 0, Focus: view.TextLen()})
	codeLo := strings.Index(plain, "line1")
	codeHi := strings.Index(plain, "line5") + len("line5")
	if codeLo < 0 || codeHi <= codeLo {
		t.Fatalf("expected code block text in plain %q", plain)
	}
	r1, ok1 := view.CaretAt(codeLo)
	r5, ok5 := view.CaretAt(codeHi)
	if !ok1 || !ok5 || r5.Y <= r1.Y+r1.Height {
		t.Fatalf("expected empty lines between line1 (%+v) and line5 (%+v), height=%v", r1, r5, h)
	}
	for y := r1.Y + r1.Height + 2; y < r5.Y; y += 6 {
		off, ok := view.OffsetAt(gfx.Point{X: 40, Y: y})
		if !ok || off < codeLo || off > codeHi {
			t.Fatalf("empty code line at y=%v resolved to offset %d (plain=%q), want inside [%d,%d]", y, off, plain, codeLo, codeHi)
		}
	}
}

func TestMarkdownViewTallTableCellClippedPaintAndHitTest(t *testing.T) {
	// Single-row table where one cell wraps into >500 visual lines: clipped
	// Paint emits only the visible cell lines and OffsetAt/CaretAt round-trip.
	var sb strings.Builder
	sb.WriteString("| Header A | Header B |\n| --- | --- |\n| Short | ")
	for i := 0; i < 1200; i++ {
		if i > 0 {
			sb.WriteByte(' ')
		}
		fmt.Fprintf(&sb, "cellword%04d", i)
	}
	sb.WriteString(" |\n")

	var doc markdown.Document
	doc.Parse(sb.String())

	canvas := markdownCanvas()
	var view MarkdownView
	h := view.Layout(canvas, MarkdownStyle{Size: 13, LineHeight: 20}, &doc, 320)
	if h < 8000 {
		t.Fatalf("expected tall table height >= 8000, got %v", h)
	}

	clipTop := h / 2
	canvas.Reset()
	view.Paint(canvas, MarkdownPaint{
		Look: MarkdownLookFor(gfx.Theme{}),
		X:    0,
		Y:    -clipTop,
		Clip: gfx.Rect{X: 0, Y: 0, Width: 320, Height: 200},
	})
	if drawn := len(canvas.Texts()); drawn == 0 || drawn > 30 {
		t.Fatalf("clipped Paint emitted %d texts for 200px slice of tall cell, want 1..30", drawn)
	}

	off, ok := view.OffsetAt(gfx.Point{X: 200, Y: 100})
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

func TestMarkdownViewLongQuoteBarClippedPaint(t *testing.T) {
	// One continuous quote bar spanning hundreds of blocks must still paint
	// inside a mid-document clip while skipping off-screen paragraphs.
	var sb strings.Builder
	for i := 0; i < 400; i++ {
		fmt.Fprintf(&sb, "> Quoted paragraph %d\n>\n", i)
	}
	var doc markdown.Document
	doc.Parse(sb.String())

	canvas := markdownCanvas()
	var view MarkdownView
	h := view.Layout(canvas, MarkdownStyle{Size: 13, LineHeight: 20}, &doc, 420)
	if h < 6000 {
		t.Fatalf("expected tall quoted doc, got %v", h)
	}

	clipTop := h / 2
	canvas.Reset()
	view.Paint(canvas, MarkdownPaint{
		Look: MarkdownLookFor(gfx.Theme{}),
		X:    0,
		Y:    -clipTop,
		Clip: gfx.Rect{X: 0, Y: 0, Width: 420, Height: 200},
	})
	if drawn := len(canvas.Texts()); drawn == 0 || drawn > 25 {
		t.Fatalf("clipped Paint emitted %d texts for 200px slice of 400-block quote, want 1..25", drawn)
	}
	if rects := len(canvas.Rectangles()); rects != 1 {
		t.Fatalf("expected 1 spanning quote bar rectangle inside clip, got %d", rects)
	}
}

func BenchmarkMarkdownViewClippedPaintSteady(b *testing.B) {
	var sb strings.Builder
	for i := 0; i < 400; i++ {
		sb.WriteString("> Iterators returned by [`iterator`](https://example.com) are *fail-fast* in paragraph.\n\n")
	}
	var doc markdown.Document
	doc.Parse(sb.String())

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
