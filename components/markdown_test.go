package components

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/markdown"
)

// markdownCanvas measures every rune as half its size wide, and every cell of
// fixed-pitch text as 6 wide at size 12, so widths are counts of characters.
func markdownCanvas() *gfx.Canvas {
	canvas := &gfx.Canvas{}
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(utf8.RuneCountInString(value)) * size / 2
	})
	canvas.SetCellMeasurer(func(size float32, face gfx.MonoFace) (float32, float32) { return size / 2, size * 1.25 })
	return canvas
}

// nonlinearMarkdownCanvas makes a full run wider than the sum of its runes,
// as native shaping can do. It keeps fixed-pitch measurement linear.
func nonlinearMarkdownCanvas() *gfx.Canvas {
	canvas := markdownCanvas()
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		n := float32(utf8.RuneCountInString(value))
		return n*size/2 + max(n-1, 0)
	})
	return canvas
}

func layoutMarkdown(t *testing.T, src string, width float32, style MarkdownStyle) (*MarkdownView, *markdown.Document) {
	t.Helper()
	var doc markdown.Document
	doc.Parse(src)
	view := &MarkdownView{}
	view.Layout(markdownCanvas(), style, &doc, width)
	return view, &doc
}

// words returns the non-blank characters of the pieces, in order.
func (v *MarkdownView) words() string {
	var out strings.Builder
	for _, p := range v.pieces {
		out.WriteString(strings.Join(strings.Fields(p.text), ""))
	}
	return out.String()
}

func TestMarkdownSelectionRangeClampsAndOrders(t *testing.T) {
	const digits = "0123456789" // ASCII, so every offset is a rune boundary
	tests := []struct {
		name  string
		sel   MarkdownSelection
		text  string
		start int
		end   int
	}{
		{name: "empty", sel: MarkdownSelection{Anchor: 4, Focus: 4}, text: digits, start: 4, end: 4},
		{name: "forward", sel: MarkdownSelection{Anchor: 2, Focus: 7}, text: digits, start: 2, end: 7},
		{name: "reversed", sel: MarkdownSelection{Anchor: 7, Focus: 2}, text: digits, start: 2, end: 7},
		{name: "negative", sel: MarkdownSelection{Anchor: -3, Focus: 4}, text: digits, start: 0, end: 4},
		{name: "past the end", sel: MarkdownSelection{Anchor: 4, Focus: 99}, text: digits, start: 4, end: 10},
		{name: "outside both ways", sel: MarkdownSelection{Anchor: -8, Focus: 99}, text: digits, start: 0, end: 10},
		{name: "empty text", sel: MarkdownSelection{Anchor: -1, Focus: 3}, text: "", start: 0, end: 0},
		{name: "start inside a rune", sel: MarkdownSelection{Anchor: 3, Focus: 5}, text: "a 中", start: 2, end: 5},
		{name: "end inside a rune", sel: MarkdownSelection{Anchor: 0, Focus: 4}, text: "a 中", start: 0, end: 2},
		{name: "both inside a rune", sel: MarkdownSelection{Anchor: 4, Focus: 3}, text: "a 中", start: 2, end: 2},
		{name: "clamped into a rune", sel: MarkdownSelection{Anchor: 3, Focus: 6}, text: "a 中", start: 2, end: 5},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			start, end := test.sel.Range([]byte(test.text))
			if start != test.start || end != test.end {
				t.Fatalf("Range(%q) = %d..%d, want %d..%d", test.text, start, end, test.start, test.end)
			}
		})
	}
}

// A selection kept across a layout change used to be clamped to a shorter text
// mid-rune, and SelectedText returned bytes that were not UTF-8. This is the
// Motty streaming case: an unclosed backtick renders as literal text, then its
// closing backtick turns "a `中" into "a 中", one byte shorter.
func TestMarkdownSelectionClampedMidRuneIsValidUTF8(t *testing.T) {
	var doc markdown.Document
	var view MarkdownView
	canvas := markdownCanvas()
	style := MarkdownStyle{Size: 13, LineHeight: 18}
	layout := func(source string) {
		doc.Parse(source)
		view.Layout(canvas, style, &doc, 400)
	}
	layout("a `中")
	sel := MarkdownSelection{Anchor: 3, Focus: view.TextLen()}
	if got := view.SelectedText(sel); got != "中" {
		t.Fatalf("SelectedText before the change = %q, want %q", got, "中")
	}
	layout("a `中`")
	got := view.SelectedText(sel)
	if !utf8.ValidString(got) {
		t.Fatalf("SelectedText after the change = %q, which is not valid UTF-8", got)
	}
	if got != "中" {
		t.Fatalf("SelectedText after the change = %q, want %q", got, "中")
	}
}

func TestEmptyMarkdownViewHasNoTextToSelect(t *testing.T) {
	var view MarkdownView
	if got := view.TextLen(); got != 0 {
		t.Fatalf("TextLen = %d, want 0", got)
	}
	if got := view.SelectedText(MarkdownSelection{Anchor: -2, Focus: 5}); got != "" {
		t.Fatalf("SelectedText = %q, want empty", got)
	}
}

func TestMarkdownSelectionUsesRenderedTextWithoutSoftWraps(t *testing.T) {
	view, _ := layoutMarkdown(t, "alpha beta gamma delta epsilon", 70, MarkdownStyle{Size: 12})
	if len(view.pieces) < 2 {
		t.Fatalf("the paragraph did not wrap: %+v", view.pieces)
	}
	const want = "alpha beta gamma delta epsilon"
	if got := view.SelectedText(MarkdownSelection{Anchor: 0, Focus: view.TextLen()}); got != want {
		t.Fatalf("selected %q, want %q", got, want)
	}
	if got := view.SelectedText(MarkdownSelection{Anchor: 11, Focus: 16}); got != "gamma" {
		t.Fatalf("selected %q, want gamma", got)
	}
}

func TestMarkdownSelectionHasOneBreakBetweenBlocks(t *testing.T) {
	view, _ := layoutMarkdown(t, "first paragraph\n\nsecond paragraph", 200, MarkdownStyle{Size: 12})
	const want = "first paragraph\nsecond paragraph"
	if got := view.SelectedText(MarkdownSelection{Anchor: 0, Focus: view.TextLen()}); got != want {
		t.Fatalf("selected %q, want %q", got, want)
	}
}

func TestMarkdownSelectionUsesVisibleInlineTextAndSpacing(t *testing.T) {
	src := "before **bold** and `code` and a [link](https://example.com/x) after"
	view, _ := layoutMarkdown(t, src, 200, MarkdownStyle{Size: 12})
	const want = "before bold and code and a link after"
	if got := view.SelectedText(MarkdownSelection{Anchor: 0, Focus: view.TextLen()}); got != want {
		t.Fatalf("selected %q, want %q", got, want)
	}
}

func TestMarkdownSelectionReadsTableCellsInRenderedOrder(t *testing.T) {
	src := "| Name | Value |\n|:--|--:|\n| ssh | shell |\n| ftp | old |\n"
	view, _ := layoutMarkdown(t, src, 300, MarkdownStyle{Size: 12})
	const want = "Name Value\nssh shell\nftp old"
	if got := view.SelectedText(MarkdownSelection{Anchor: 0, Focus: view.TextLen()}); got != want {
		t.Fatalf("selected %q, want %q", got, want)
	}
}

func TestInlinePiecesMapToOneRangeOfTheRenderedText(t *testing.T) {
	view, _ := layoutMarkdown(t, "before **bold** and `code` and a [link](https://example.com/x) after", 90, MarkdownStyle{Size: 12})
	var last int32 = -1
	for _, piece := range view.pieces {
		if piece.plainLo < 0 {
			continue
		}
		if piece.plainLo < last || piece.plainHi < piece.plainLo || int(piece.plainHi) > view.TextLen() {
			t.Fatalf("piece %q has range %d..%d after %d", piece.text, piece.plainLo, piece.plainHi, last)
		}
		if got := string(view.plain[piece.plainLo:piece.plainHi]); got != piece.text {
			t.Errorf("piece %q maps to %q", piece.text, got)
		}
		last = piece.plainHi
	}
}

func TestFencedCodeCopiesSourceLinesWithoutVisualWraps(t *testing.T) {
	const line = "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	view, _ := layoutMarkdown(t, "```\n"+line+"\nsecond line\n```\n", 200, MarkdownStyle{Size: 12})
	var mono int
	for _, piece := range view.pieces {
		if piece.mono {
			mono++
		}
	}
	if mono < 3 {
		t.Fatalf("the long code line did not wrap: %d pieces", mono)
	}
	want := line + "\nsecond line"
	if got := view.SelectedText(MarkdownSelection{Anchor: 0, Focus: view.TextLen()}); got != want {
		t.Fatalf("selected %q, want %q", got, want)
	}
}

func TestFencedCodeKeepsTabsAndCopiesThem(t *testing.T) {
	view, _ := layoutMarkdown(t, "```\n\tfoo\tbar\n```\n", 300, MarkdownStyle{Size: 12})
	const want = "\tfoo\tbar"
	if got := view.SelectedText(MarkdownSelection{Anchor: 0, Focus: view.TextLen()}); got != want {
		t.Fatalf("selected %q, want %q", got, want)
	}
	tests := []struct {
		start, end int
		want       string
	}{
		{start: 0, end: 1, want: "\t"},
		{start: 1, end: 4, want: "foo"},
		{start: 0, end: 4, want: "\tfoo"},
		{start: 4, end: 5, want: "\t"},
		{start: 5, end: 8, want: "bar"},
	}
	for _, test := range tests {
		if got := view.SelectedText(MarkdownSelection{Anchor: test.start, Focus: test.end}); got != test.want {
			t.Errorf("select %d..%d = %q, want %q", test.start, test.end, got, test.want)
		}
	}
}

func TestCodeChunkBoundariesDoNotSplitATab(t *testing.T) {
	// Five cells put the tab across the six-cell boundary: the first chunk has
	// to end before it, and the next one owns the whole tab.
	view, _ := layoutMarkdown(t, "```\nabcde\tX\n```\n", 56, MarkdownStyle{Size: 12})
	const want = "abcde\tX"
	if got := view.SelectedText(MarkdownSelection{Anchor: 0, Focus: view.TextLen()}); got != want {
		t.Fatalf("selected %q, want %q", got, want)
	}
	last := int32(0)
	for _, piece := range view.pieces {
		if !piece.mono {
			continue
		}
		if piece.plainLo != last || piece.plainHi <= piece.plainLo {
			t.Fatalf("piece %q has range %d..%d, want it to start at %d", piece.text, piece.plainLo, piece.plainHi, last)
		}
		last = piece.plainHi
	}
	if int(last) != view.TextLen() {
		t.Fatalf("code pieces cover %d bytes of %d", last, view.TextLen())
	}
}

func TestWrappedCodePiecesCoverTheSourceLineInOrder(t *testing.T) {
	const line = "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	view, _ := layoutMarkdown(t, "```\n"+line+"\n```\n", 200, MarkdownStyle{Size: 12})
	last := int32(0)
	for _, piece := range view.pieces {
		if !piece.mono {
			continue
		}
		if piece.plainLo != last || piece.plainHi <= piece.plainLo {
			t.Fatalf("piece %q has range %d..%d, want it to start at %d", piece.text, piece.plainLo, piece.plainHi, last)
		}
		last = piece.plainHi
	}
	if int(last) != view.TextLen() {
		t.Fatalf("code pieces cover %d bytes of %d", last, view.TextLen())
	}
}

func paintedMarkdown(t *testing.T, src string, width float32) (*MarkdownView, *gfx.Canvas) {
	t.Helper()
	canvas := markdownCanvas()
	var doc markdown.Document
	doc.Parse(src)
	var view MarkdownView
	view.Layout(canvas, MarkdownStyle{Size: 12}, &doc, width)
	view.Paint(canvas, MarkdownPaint{X: 10, Y: 20})
	return &view, canvas
}

func TestMarkdownOffsetAtFollowsThePaintedText(t *testing.T) {
	view, _ := paintedMarkdown(t, "abc def", 300)
	tests := []struct {
		point  gfx.Point
		offset int
	}{
		{point: gfx.Point{X: 10, Y: 20}, offset: 0},
		{point: gfx.Point{X: 52, Y: 20}, offset: 7},
		{point: gfx.Point{X: 0, Y: 20}, offset: 0},
		{point: gfx.Point{X: 300, Y: 20}, offset: 7},
		{point: gfx.Point{X: 10, Y: -100}, offset: 0},
		{point: gfx.Point{X: 60, Y: 200}, offset: 7},
	}
	for _, test := range tests {
		offset, ok := view.OffsetAt(test.point)
		if !ok || offset != test.offset {
			t.Errorf("OffsetAt(%v) = %d, %v, want %d", test.point, offset, ok, test.offset)
		}
	}
}

func TestMarkdownOffsetAtSeparatesVisualLines(t *testing.T) {
	view, _ := paintedMarkdown(t, "first\nsecond", 300)
	if len(view.pieces) < 2 {
		t.Fatalf("the line break made %d pieces", len(view.pieces))
	}
	first, second := view.pieces[0], view.pieces[1]
	endOfFirst, ok := view.OffsetAt(gfx.Point{X: 10 + first.x + first.w, Y: 20 + first.y})
	if !ok || endOfFirst != 5 {
		t.Fatalf("end of the first line = %d, %v", endOfFirst, ok)
	}
	startOfSecond, ok := view.OffsetAt(gfx.Point{X: 10 + second.x, Y: 20 + second.y + 2})
	if !ok || startOfSecond != 6 {
		t.Fatalf("start of the second line = %d, %v", startOfSecond, ok)
	}
}

func TestMarkdownOffsetAtStaysOnCharacterBoundaries(t *testing.T) {
	view, _ := paintedMarkdown(t, "中文", 300)
	allowed := map[int]bool{0: true, 3: true, 6: true}
	for x := float32(0); x <= 12; x++ {
		offset, ok := view.OffsetAt(gfx.Point{X: 10 + x, Y: 20})
		if !ok || !allowed[offset] {
			t.Fatalf("x=%v gives offset %d, not a character boundary", x, offset)
		}
	}
	if offset, _ := view.OffsetAt(gfx.Point{X: 16, Y: 20}); offset != 3 {
		t.Errorf("the middle of the text gives %d, want 3", offset)
	}
	if offset, _ := view.OffsetAt(gfx.Point{X: 22, Y: 20}); offset != 6 {
		t.Errorf("the end of the text gives %d, want 6", offset)
	}
}

func TestMarkdownOffsetAtSnapsInsideAnExpandedCodeTab(t *testing.T) {
	view, _ := paintedMarkdown(t, "```\nabcde\tX\n```\n", 56)
	var tabPiece *mdPiece
	for index := range view.pieces {
		if view.pieces[index].mono && view.pieces[index].plainLo == 5 {
			tabPiece = &view.pieces[index]
		}
	}
	if tabPiece == nil {
		t.Fatalf("no piece starts at the tab: %+v", view.pieces)
	}
	originX, originY := float32(10+tabPiece.x), float32(20+tabPiece.y+2)
	left, ok := view.OffsetAt(gfx.Point{X: originX + 3, Y: originY})
	if !ok || left != 5 {
		t.Fatalf("the left half of the tab gives %d, %v, want 5", left, ok)
	}
	right, ok := view.OffsetAt(gfx.Point{X: originX + 15, Y: originY})
	if !ok || right != 6 {
		t.Fatalf("the right half of the tab gives %d, %v, want 6", right, ok)
	}
}

func TestMarkdownCaretBoundsReportsNoCaret(t *testing.T) {
	view, _ := paintedMarkdown(t, "text", 300)
	if bounds, ok := view.CaretBounds(); ok || bounds != (gfx.Rect{}) {
		t.Fatalf("CaretBounds = %+v, %v; a painted Markdown view has no caret", bounds, ok)
	}
}

func markdownSelectionRects(canvas *gfx.Canvas, color gfx.Color) []gfx.Rectangle {
	var found []gfx.Rectangle
	for _, rectangle := range canvas.Rectangles() {
		if rectangle.Color == color {
			found = append(found, rectangle)
		}
	}
	return found
}

func markdownTextRunsCoverValue(canvas *gfx.Canvas, text gfx.Text) bool {
	runs := canvas.TextRuns(text)
	if len(runs) == 0 || text.CellWidth <= 0 {
		return false
	}
	bytes, columns := 0, int32(0)
	for _, run := range runs {
		if run.Column != columns {
			return false
		}
		bytes += int(run.Bytes)
		columns += run.Cells
	}
	return bytes == len(text.Value) && columns == int32(monoCells(text.Value))
}

func TestMarkdownPaintsPaintedSelection(t *testing.T) {
	canvas := markdownCanvas()
	var doc markdown.Document
	doc.Parse("alpha beta gamma")
	var view MarkdownView
	view.Layout(canvas, MarkdownStyle{Size: 12}, &doc, 300)
	selected := gfx.Color{Red: 1, Alpha: 1}
	var painted gfx.Canvas
	view.Paint(&painted, MarkdownPaint{
		Look: MarkdownLook{Selection: selected}, X: 10, Y: 20, Selection: MarkdownSelection{Anchor: 0, Focus: 5},
	})
	rects := markdownSelectionRects(&painted, selected)
	if len(rects) != 1 {
		t.Fatalf("%d selection rectangles, want 1: %+v", len(rects), painted.Rectangles())
	}
	if rects[0].Bounds.Width <= 0 || rects[0].Bounds.Height <= 0 || rects[0].Bounds.Y < 20 {
		t.Errorf("selection rectangle = %+v", rects[0].Bounds)
	}
	if len(painted.Texts()) == 0 {
		t.Fatal("the text was not painted with the selection")
	}
}

func TestMarkdownPaintsNoSelectionForAnEmptyRange(t *testing.T) {
	canvas := markdownCanvas()
	var doc markdown.Document
	doc.Parse("alpha beta gamma")
	var view MarkdownView
	view.Layout(canvas, MarkdownStyle{Size: 12}, &doc, 300)
	selected := gfx.Color{Red: 1, Alpha: 1}
	var painted gfx.Canvas
	view.Paint(&painted, MarkdownPaint{
		Look: MarkdownLook{Selection: selected}, Selection: MarkdownSelection{Anchor: 4, Focus: 4},
	})
	if rects := markdownSelectionRects(&painted, selected); len(rects) != 0 {
		t.Fatalf("empty selection painted %+v", rects)
	}
}

func TestMarkdownSelectionIsClippedToVisibleLines(t *testing.T) {
	canvas := markdownCanvas()
	var doc markdown.Document
	doc.Parse("first\nsecond")
	var view MarkdownView
	view.Layout(canvas, MarkdownStyle{Size: 12}, &doc, 300)
	if len(view.pieces) < 2 {
		t.Fatalf("the document has %d pieces", len(view.pieces))
	}
	second := view.pieces[1]
	selected := gfx.Color{Red: 1, Alpha: 1}
	paint := func(selection MarkdownSelection) *gfx.Canvas {
		var painted gfx.Canvas
		view.Paint(&painted, MarkdownPaint{
			Look: MarkdownLook{Selection: selected}, Selection: selection,
			Clip: gfx.Rect{Y: second.y + 2, Width: 300, Height: second.h - 2},
		})
		return &painted
	}
	if rects := markdownSelectionRects(paint(MarkdownSelection{Anchor: 0, Focus: 5}), selected); len(rects) != 0 {
		t.Fatalf("a selection above the clip painted %+v", rects)
	}
	if rects := markdownSelectionRects(paint(MarkdownSelection{Anchor: 0, Focus: 12}), selected); len(rects) != 1 {
		t.Fatalf("the visible line painted %d selection rectangles, want 1", len(rects))
	}
}

func TestMarkdownCodeSelectionCoversATabAsCells(t *testing.T) {
	canvas := markdownCanvas()
	var doc markdown.Document
	doc.Parse("```\nabcde\tX\n```\n")
	var view MarkdownView
	view.Layout(canvas, MarkdownStyle{Size: 12}, &doc, 56)
	selected := gfx.Color{Red: 1, Alpha: 1}
	behind := gfx.Color{Green: 1, Alpha: 1}
	var painted gfx.Canvas
	view.Paint(&painted, MarkdownPaint{
		Look: MarkdownLook{Selection: selected, CodeFill: behind}, Selection: MarkdownSelection{Anchor: 5, Focus: 6},
	})
	rects := markdownSelectionRects(&painted, selected)
	if len(rects) != 1 {
		t.Fatalf("%d selection rectangles, want 1: %+v", len(rects), rects)
	}
	selectionAt, fillAt := -1, -1
	for index, rectangle := range painted.Rectangles() {
		if rectangle.Color == selected {
			selectionAt = index
		}
		if rectangle.Color == behind {
			fillAt = index
		}
	}
	if fillAt < 0 || selectionAt < fillAt {
		t.Errorf("selection at %d and code fill at %d: selection must be painted over the fill", selectionAt, fillAt)
	}
	cell := float32(0)
	for _, piece := range view.pieces {
		if piece.mono {
			cell = piece.cell
			break
		}
	}
	if want := 3 * cell; cell <= 0 || rects[0].Bounds.Width != want {
		t.Errorf("the tab covers %v, want %v (cell %v)", rects[0].Bounds.Width, want, cell)
	}
}

func TestMarkdownWrapsEveryParagraphIntoItsWidthWithoutLosingText(t *testing.T) {
	src := "A long paragraph with **bold words**, `inline code` and a [link to somewhere](https://example.com/x), 中文也要能换行，而且标点不能出现在行首。\n" +
		"- an item that is long enough to need a second line to be shown\n" +
		"> a quote that is long enough to wrap as well, inside its bar\n" +
		"Averyveryveryverylongwordthatdoesnotfitanywhereatall"
	const width = 200
	view, _ := layoutMarkdown(t, src, width, MarkdownStyle{Size: 12})
	want := strings.Join(strings.Fields(strings.NewReplacer("**", "", "`", "", "[link to somewhere](https://example.com/x)", "linktosomewhere", "- ", "", "> ", "").Replace(src)), "")
	if got := view.words(); strings.ReplaceAll(got, "•", "") != want {
		t.Errorf("text changed while wrapping:\n got  %q\n want %q", got, want)
	}
	for _, p := range view.pieces {
		if p.x < 0 || p.x+p.w > width+12 { // closing punctuation may hang a glyph past the edge
			t.Errorf("piece %q at %v..%v leaves the %d-wide column", p.text, p.x, p.x+p.w, width)
		}
	}
	// Pieces on one line never overlap.
	for i, a := range view.pieces {
		for _, b := range view.pieces[i+1:] {
			if a.y == b.y && a.x < b.x+b.w-0.5 && b.x < a.x+a.w-0.5 {
				t.Errorf("%q and %q overlap on one line", a.text, b.text)
			}
		}
	}
}

func TestMarkdownLinkRegionsFollowTheWrappedText(t *testing.T) {
	view, _ := layoutMarkdown(t, "see [the very long link text here](https://example.com/a) now", 150, MarkdownStyle{Size: 12})
	if len(view.links) < 2 {
		t.Fatalf("a link that wraps should have a region on each line, got %d", len(view.links))
	}
	var canvas gfx.Canvas
	scope := gfx.Scope(7)
	view.Paint(&canvas, MarkdownPaint{Look: MarkdownLook{}, X: 10, Y: 100, Links: scope})
	if len(canvas.Hits()) != len(view.links) {
		t.Fatalf("%d hit regions for %d link fragments", len(canvas.Hits()), len(view.links))
	}
	for _, hit := range canvas.Hits() {
		index, ok := hit.ID.In(scope)
		if !ok || view.Link(index) != "https://example.com/a" || hit.Bounds.X < 10 || hit.Bounds.Y < 100 {
			t.Errorf("hit %+v does not lead to the link", hit)
		}
	}
	// Scrolled out of view, a link has no region.
	canvas.Reset()
	view.Paint(&canvas, MarkdownPaint{X: 10, Y: 100, Links: scope, Clip: gfx.Rect{Y: 1000, Width: 100, Height: 100}})
	if len(canvas.Hits()) != 0 || len(canvas.Texts()) != 0 {
		t.Errorf("painted %d texts and %d hits outside the clip", len(canvas.Texts()), len(canvas.Hits()))
	}
}

func TestMarkdownCodeBlocksAreReportedAndKeepRoomForAButton(t *testing.T) {
	src := "text\n```sh\nls -la /var/log/some/very/long/path/that/must/wrap\necho 你好\n```\n"
	view, _ := layoutMarkdown(t, src, 200, MarkdownStyle{Size: 12, CodeInset: 30})
	codes := view.Codes()
	if len(codes) != 1 || !strings.HasPrefix(codes[0].Text, "ls -la") {
		t.Fatalf("codes = %+v", codes)
	}
	box := codes[0].Bounds
	if box.Width != 200 || box.Y <= 0 {
		t.Errorf("block is at %+v", box)
	}
	var lines int
	for _, p := range view.pieces {
		if !p.mono {
			continue
		}
		lines++
		if p.x+p.w > box.X+box.Width-30 || p.y < box.Y || p.y+p.h > box.Y+box.Height {
			t.Errorf("code line %q at %v..%v is outside the text area of %+v", p.text, p.x, p.x+p.w, box)
		}
	}
	if lines < 3 {
		t.Errorf("the long line should have wrapped: %d lines", lines)
	}
}

func TestMarkdownTableFitsItsWidthAndWrapsItsCells(t *testing.T) {
	src := "| Name | Description |\n|:--|--:|\n| ssh | the secure shell protocol and the program that speaks it to remote hosts |\n| ftp | old |\n"
	view, doc := layoutMarkdown(t, src, 180, MarkdownStyle{Size: 12})
	var frame mdBox // the header band spans the table
	for _, box := range view.boxes {
		if box.kind == mdBoxHead {
			frame = box
		}
	}
	if frame.w == 0 || frame.w > 180.5 {
		t.Fatalf("table is %v wide in a 180 column", frame.w)
	}
	for _, p := range view.pieces {
		if p.x+p.w > frame.x+frame.w+0.5 {
			t.Errorf("%q sticks out of the table", p.text)
		}
	}
	if !strings.Contains(view.words(), "secureshellprotocol"[:6]) {
		t.Errorf("cell text is missing: %q", view.words())
	}
	_ = doc
	// The right-aligned column ends at its cell's right edge.
	var oldRight float32
	for _, p := range view.pieces {
		if p.text == "old" {
			oldRight = p.x + p.w
		}
	}
	if oldRight < frame.x+frame.w-markdownCellPadX-1 {
		t.Errorf("right-aligned cell ends at %v, the table's edge is %v", oldRight, frame.x+frame.w)
	}
}

func TestMarkdownQuoteBarRunsThroughConsecutiveQuotedBlocks(t *testing.T) {
	view, _ := layoutMarkdown(t, "> first\n> - second\n\nplain\n\n> again", 200, MarkdownStyle{Size: 12})
	var bars []mdBox
	for _, box := range view.boxes {
		if box.kind == mdBoxBar {
			bars = append(bars, box)
		}
	}
	if len(bars) != 2 {
		t.Fatalf("want one bar for the first quote and one for the second, got %d", len(bars))
	}
	if bars[0].y+bars[0].h > bars[1].y {
		t.Errorf("bars overlap: %+v %+v", bars[0], bars[1])
	}
}

func TestMarkdownLayoutAndPaintDoNotAllocateOnceWarm(t *testing.T) {
	src := strings.Repeat("## Title\nSome **bold** and `code` with [a link](https://example.com), 中文。\n1. one\n2. two\n> quote\n| a | b |\n|---|---|\n| 1 | 2 |\n\n```go\ncode\n```\n", 8)
	var doc markdown.Document
	doc.Parse(src)
	canvas := markdownCanvas()
	var view MarkdownView
	look := MarkdownLookFor(gfx.DarkTheme())
	selection := MarkdownSelection{Anchor: 2, Focus: 40}
	run := func() {
		canvas.Reset()
		view.Layout(canvas, MarkdownStyle{Size: 13}, &doc, 400)
		view.Paint(canvas, MarkdownPaint{Look: look, Links: gfx.Scope(3), Selection: selection})
	}
	run()
	if allocs := testing.AllocsPerRun(10, run); allocs != 0 {
		t.Errorf("a warm layout and paint allocates %v times", allocs)
	}
}

func TestMarkdownWrappedPunctuationDoesNotAllocateOnceWarm(t *testing.T) {
	var doc markdown.Document
	doc.Parse("x**bcdef。**")
	canvas := nonlinearMarkdownCanvas()
	var view MarkdownView
	run := func() {
		view.Layout(canvas, MarkdownStyle{Size: 12}, &doc, 40)
	}
	run()
	if allocs := testing.AllocsPerRun(10, run); allocs != 0 {
		t.Errorf("a warm layout with shaped punctuation allocates %v times", allocs)
	}
}

func TestMarkdownCandidateLineMeasurementsMatchWhatIsPainted(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{name: "plain", src: "端口绑定在容器地址，外部访问仍要从宿主机转发。"},
		{name: "style boundary", src: "端口绑定在 **容器地址**，外部访问仍要从宿主机转发。"},
		{name: "closing punctuation", src: "端口绑定在容器地址，外部访问仍要从宿主机转发。"},
		{name: "inline code boundary", src: "端口绑定在 `127.0.0.1`，外部访问仍要从宿主机转发。"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var doc markdown.Document
			doc.Parse(test.src)
			var view MarkdownView
			const width = 132
			view.Layout(nonlinearMarkdownCanvas(), MarkdownStyle{Size: 12}, &doc, width)
			want := strings.NewReplacer("**", "", "`", "").Replace(test.src)
			if got := view.SelectedText(MarkdownSelection{Focus: view.TextLen()}); got != want {
				t.Fatalf("rendered text = %q, want %q", got, want)
			}
			for _, piece := range view.pieces {
				if piece.x < 0 || piece.x+piece.w > width {
					t.Errorf("piece %q is at %v..%v in a %v-wide column", piece.text, piece.x, piece.x+piece.w, width)
				}
			}
			for i, a := range view.pieces {
				for _, b := range view.pieces[i+1:] {
					if a.y == b.y && a.x < b.x+b.w && b.x < a.x+a.w {
						t.Errorf("%q at %v..%v overlaps %q at %v..%v", a.text, a.x, a.x+a.w, b.text, b.x, b.x+b.w)
					}
				}
			}
			if !t.Failed() {
				return
			}
			for _, piece := range view.pieces {
				t.Logf("piece %q: x=%v w=%v y=%v mono=%v", piece.text, piece.x, piece.w, piece.y, piece.mono)
			}
		})
	}
}

func TestMarkdownClosingPunctuationMovesWithThePrecedingText(t *testing.T) {
	// The bold word starts after one character. It and the punctuation do not
	// fit after that character, but they fit together on the next line.
	var doc markdown.Document
	doc.Parse("a**bcdef**。")
	var view MarkdownView
	const width = 40
	view.Layout(markdownCanvas(), MarkdownStyle{Size: 12}, &doc, width)
	var moved *mdPiece
	for index := range view.pieces {
		piece := &view.pieces[index]
		if piece.text == "bcdef" {
			moved = piece
		}
	}
	if moved == nil || moved.x != 0 || moved.y == 0 {
		t.Fatalf("punctuation was not moved with its preceding text: %+v", view.pieces)
	}
	if got := view.SelectedText(MarkdownSelection{Focus: view.TextLen()}); got != "abcdef。" {
		t.Fatalf("rendered text = %q", got)
	}
}

func TestMarkdownLongClosingRunKeepsPunctuationInsideTheColumn(t *testing.T) {
	// The bold word starts on its own line but is still too long for the
	// punctuation, so only the shortest suffix that fits moves with it.
	var doc markdown.Document
	doc.Parse("a**bcdefg**。")
	var view MarkdownView
	const width = 40
	view.Layout(markdownCanvas(), MarkdownStyle{Size: 12}, &doc, width)
	if got := view.SelectedText(MarkdownSelection{Focus: view.TextLen()}); got != "abcdefg。" {
		t.Fatalf("rendered text = %q", got)
	}
	for _, piece := range view.pieces {
		if piece.x < 0 || piece.x+piece.w > width {
			t.Errorf("piece %q is at %v..%v in a %v-wide column", piece.text, piece.x, piece.x+piece.w, width)
		}
	}
	var punct *mdPiece
	for index := range view.pieces {
		piece := &view.pieces[index]
		if piece.text == "。" {
			punct = piece
		}
	}
	if punct == nil || punct.x != 6 || punct.y == 0 {
		t.Fatalf("punctuation did not follow a short suffix on a new line: %+v", view.pieces)
	}
}

func TestMarkdownMonoClosingRunMovesItsSuffixWithThePunctuation(t *testing.T) {
	const width = 40
	view, canvas := paintedMarkdown(t, "a`bcdefgh`。", width)
	if got := view.SelectedText(MarkdownSelection{Focus: view.TextLen()}); got != "abcdefgh。" {
		t.Fatalf("rendered text = %q, want %q", got, "abcdefgh。")
	}
	var code []*mdPiece
	var punct *mdPiece
	for index := range view.pieces {
		piece := &view.pieces[index]
		if piece.x < 0 || piece.x+piece.w > width {
			t.Errorf("piece %q is at %v..%v in a %v-wide column", piece.text, piece.x, piece.x+piece.w, width)
		}
		if piece.mono {
			code = append(code, piece)
			found := false
			for _, text := range canvas.Texts() {
				if text.Value == piece.text && markdownTextRunsCoverValue(canvas, text) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("inline-code piece %q has incomplete grid runs", piece.text)
			}
			found = false
			for _, box := range view.boxes {
				if box.kind == mdBoxInlineCode && box.x == piece.x && box.w == piece.w {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("inline-code piece %q has no full-width background", piece.text)
			}
		}
		if piece.text == "。" {
			punct = piece
		}
	}
	if len(code) != 2 || code[0].text != "bcdefg" || code[1].text != "h" {
		t.Fatalf("inline-code fragments = %+v, want bcdefg and h", code)
	}
	if punct == nil || punct.y != code[1].y || punct.x != code[1].x+code[1].w {
		t.Fatalf("punctuation %+v does not follow the moved suffix %+v", punct, code[1])
	}
}

func TestMarkdownClosingPunctuationUsesTheJoinedShapedWidth(t *testing.T) {
	var doc markdown.Document
	doc.Parse("x**bcdef。**")
	var view MarkdownView
	const width = 40
	view.Layout(nonlinearMarkdownCanvas(), MarkdownStyle{Size: 12}, &doc, width)
	if got := view.SelectedText(MarkdownSelection{Focus: view.TextLen()}); got != "xbcdef。" {
		t.Fatalf("rendered text = %q", got)
	}
	for _, piece := range view.pieces {
		if piece.text == "。" {
			t.Fatalf("punctuation starts a line alone: %+v", view.pieces)
		}
	}
	var punctuation *mdPiece
	for index := range view.pieces {
		piece := &view.pieces[index]
		if strings.HasSuffix(piece.text, "。") {
			punctuation = piece
		}
	}
	if punctuation == nil || len(punctuation.text) == len("。") || punctuation.x+punctuation.w > width {
		t.Fatalf("punctuation did not stay with a fitting suffix: %+v", view.pieces)
	}
}

func TestMarkdownClosingPunctuationKeepsThePrecedingSpanStyle(t *testing.T) {
	canvas := &gfx.Canvas{}
	canvas.SetMeasurer(func(value string, _ float32, weight float32) float32 {
		width := float32(utf8.RuneCountInString(value))
		if weight > gfx.WeightRegular {
			width *= 10
		}
		return width
	})
	canvas.SetCellMeasurer(func(size float32, _ gfx.MonoFace) (float32, float32) { return size / 2, size * 1.25 })
	var doc markdown.Document
	doc.Parse("x**abc**。。。。。。。。。。。")
	var view MarkdownView
	const width = 40
	view.Layout(canvas, MarkdownStyle{Size: 12}, &doc, width)
	var punctuation *mdPiece
	for index := range view.pieces {
		if strings.HasPrefix(view.pieces[index].text, "。") {
			punctuation = &view.pieces[index]
		}
	}
	if punctuation == nil {
		t.Fatalf("punctuation did not stay with the styled suffix: %+v", view.pieces)
	}
	for index := range view.pieces {
		piece := &view.pieces[index]
		if piece != punctuation && piece.y == punctuation.y && piece.x+piece.w == punctuation.x {
			return
		}
	}
	t.Fatalf("punctuation has no preceding text on its line: %+v", view.pieces)
}

func TestMarkdownBreakpointRewindPaintsEachPieceBackgroundAndLinkOnce(t *testing.T) {
	src := "a**bcdef**。 [link](https://example.com)"
	view, _ := layoutMarkdown(t, src, 40, MarkdownStyle{Size: 12})
	if got := view.SelectedText(MarkdownSelection{Focus: view.TextLen()}); got != "abcdef。 link" {
		t.Fatalf("rendered text = %q", got)
	}
	paintedFor := func(label string) map[string]int {
		t.Helper()
		var canvas gfx.Canvas
		view.Paint(&canvas, MarkdownPaint{Links: gfx.Scope(1)})
		counts := map[string]int{}
		for _, text := range canvas.Texts() {
			counts[label+":text:"+text.Value]++
		}
		for _, box := range canvas.Rectangles() {
			counts[label+":rectangle"]++
			if !box.HasClip {
				t.Errorf("%s: background is not clipped", label)
			}
		}
		for range canvas.Hits() {
			counts[label+":hit"]++
		}
		return counts
	}
	counts := paintedFor("paint")
	for value, count := range counts {
		if count != 1 {
			t.Errorf("%s painted %d times", value, count)
		}
	}
	if counts["paint:text:a"] != 1 || counts["paint:text:bcdef"] != 1 || counts["paint:text:。"] != 1 ||
		counts["paint:text:link"] != 1 || counts["paint:hit"] != 1 {
		t.Errorf("rewound paint is missing the expected pieces: %+v", counts)
	}
}

func TestMarkdownInlineCodePaddingParticipatesInTheBreak(t *testing.T) {
	var doc markdown.Document
	doc.Parse("ab cd `e`")
	var view MarkdownView
	const width = 20
	view.Layout(markdownCanvas(), MarkdownStyle{Size: 12}, &doc, width)
	for _, box := range view.boxes {
		if box.kind == mdBoxInlineCode {
			if box.x < 0 || box.x+box.w > width {
				t.Errorf("inline-code background is at %v..%v in a %v-wide column", box.x, box.x+box.w, width)
			}
		}
	}
}

func TestMarkdownReflowKeepsShapedTextInsideTheColumn(t *testing.T) {
	var doc markdown.Document
	doc.Parse("端口绑定在 `127.0.0.1`，外部访问仍要从宿主机转发。")
	var view MarkdownView
	for _, width := range []float32{132, 100, 80, 60} {
		view.Layout(nonlinearMarkdownCanvas(), MarkdownStyle{Size: 12}, &doc, width)
		for _, p := range view.pieces {
			if p.x < 0 || p.x+p.w > width {
				t.Errorf("at %v wide, piece %q is at %v..%v", width, p.text, p.x, p.x+p.w)
			}
		}
		if got, want := view.SelectedText(MarkdownSelection{Focus: view.TextLen()}), "端口绑定在 127.0.0.1，外部访问仍要从宿主机转发。"; got != want {
			t.Errorf("at %v wide, rendered text = %q, want %q", width, got, want)
		}
	}
}

func TestLongTokensWrapByCompleteUTF8Characters(t *testing.T) {
	const token = "超级长的不可分割地址docker.1panel.live超级长的不可分割地址"
	var doc markdown.Document
	doc.Parse(token)
	var view MarkdownView
	const width = 72
	view.Layout(nonlinearMarkdownCanvas(), MarkdownStyle{Size: 12}, &doc, width)
	if got := view.SelectedText(MarkdownSelection{Focus: view.TextLen()}); got != token {
		t.Fatalf("rendered text = %q, want %q", got, token)
	}
	for _, p := range view.pieces {
		if !utf8.ValidString(p.text) {
			t.Errorf("piece %q is not valid UTF-8", p.text)
		}
		if p.x < 0 || p.x+p.w > width {
			t.Errorf("piece %q is at %v..%v", p.text, p.x, p.x+p.w)
		}
	}
}

func TestInlineCodePaddingIsTheSameInHeadingsTablesAndAlignedText(t *testing.T) {
	src := "## Heading `code`\n\n| center | right |\n|:-:|--:|\n| `中` | `127.0.0.1` |\n"
	view, _ := layoutMarkdown(t, src, 220, MarkdownStyle{Size: 12})
	var boxes []mdBox
	for _, box := range view.boxes {
		if box.kind == mdBoxInlineCode {
			boxes = append(boxes, box)
		}
	}
	if len(boxes) != 3 {
		t.Fatalf("want inline-code backgrounds in the heading and both table cells, got %+v", boxes)
	}
	for _, box := range boxes {
		var text *mdPiece
		for index := range view.pieces {
			p := &view.pieces[index]
			center := p.y + p.h/2
			if p.mono && center >= box.y && center <= box.y+box.h && p.x >= box.x && p.x+p.w <= box.x+box.w {
				text = p
				break
			}
		}
		if text == nil {
			t.Fatalf("no text belongs to background %+v", box)
		}
		if text.padX != markdownInlinePad || text.x != box.x || text.w != box.w {
			t.Errorf("background %+v and text %+v do not share padding %v", box, text, markdownInlinePad)
		}
	}
}

func TestMarkdownTableAlignsDecoratedInlineCodeInEveryColumn(t *testing.T) {
	src := "| left | center | right |\n|:--|:-:|--:|\n| `a` | `bb` | `ccc` |\n"
	view, _ := layoutMarkdown(t, src, 300, MarkdownStyle{Size: 12})
	var head mdBox
	for _, box := range view.boxes {
		if box.kind == mdBoxHead {
			head = box
			break
		}
	}
	if head.w == 0 {
		t.Fatal("the table has no header frame")
	}
	cellWidth := head.w / 3
	for column, text := range []string{"a", "bb", "ccc"} {
		piece, background := markdownInlineCode(t, view, text)
		left := head.x + float32(column)*cellWidth + markdownCellPadX
		right := head.x + float32(column+1)*cellWidth - markdownCellPadX
		switch column {
		case 1:
			if background.x < left || background.x+background.w > right {
				t.Errorf("centered code %q is at %+v in %v..%v", text, background, left, right)
			}
		case 2:
			if abs32(background.x+background.w-right) > 0.5 {
				t.Errorf("right-aligned code %q ends at %v, want %v", text, background.x+background.w, right)
			}
		default:
			if abs32(background.x-left) > 0.5 {
				t.Errorf("left-aligned code %q starts at %v, want %v", text, background.x, left)
			}
		}
		if piece.x != background.x || piece.w != background.w || piece.padX != markdownInlinePad {
			t.Errorf("code %q has text %+v and background %+v", text, piece, background)
		}
	}
}

func markdownInlineCode(t *testing.T, view *MarkdownView, text string) (*mdPiece, mdBox) {
	t.Helper()
	var piece *mdPiece
	for index := range view.pieces {
		p := &view.pieces[index]
		if p.mono && p.text == text {
			piece = p
			break
		}
	}
	if piece == nil {
		t.Fatalf("no inline-code piece %q", text)
	}
	for _, box := range view.boxes {
		if box.kind == mdBoxInlineCode && box.x == piece.x && box.w == piece.w &&
			box.y <= piece.y+piece.h/2 && piece.y+piece.h/2 <= box.y+box.h {
			return piece, box
		}
	}
	t.Fatalf("no inline-code background for %q", text)
	return nil, mdBox{}
}

func abs32(value float32) float32 {
	if value < 0 {
		return -value
	}
	return value
}

func TestMarkdownMonoPiecesAlwaysUseFixedGridRuns(t *testing.T) {
	for _, face := range []gfx.MonoFace{gfx.MonoSystem, gfx.MonoClassic} {
		t.Run(fmt.Sprintf("face %d", face), func(t *testing.T) {
			canvas := markdownCanvas()
			var doc markdown.Document
			doc.Parse("text `inline` and\n\n## Heading `heading`\n\n```sh\necho 你好\n```\n")
			var view MarkdownView
			view.Layout(canvas, MarkdownStyle{Size: 12, MonoFace: face}, &doc, 220)
			view.Paint(canvas, MarkdownPaint{})
			mono := 0
			for _, piece := range view.pieces {
				if !piece.mono {
					continue
				}
				mono++
				found := false
				for _, text := range canvas.Texts() {
					if text.Value == piece.text {
						runs, bytes := canvas.TextRuns(text), 0
						columns := int32(0)
						for _, run := range runs {
							bytes += int(run.Bytes)
							if run.Column != columns {
								t.Errorf("piece %q run starts at cell %d, want %d", piece.text, run.Column, columns)
							}
							columns += run.Cells
						}
						found = bytes == len(piece.text) && text.CellWidth == piece.cell &&
							text.Style.MonoFace == face && columns == int32(monoCells(piece.text))
						if found {
							break
						}
					}
				}
				if !found {
					t.Errorf("mono piece %q was not painted through complete grid runs", piece.text)
				}
			}
			if mono < 3 {
				t.Errorf("test fixture has %d mono pieces", mono)
			}
		})
	}
}

func TestMarkdownInlineCodeKeepsItsWeightInGridRuns(t *testing.T) {
	view, canvas := paintedMarkdown(t, "## Heading `code`", 220)
	var piece *mdPiece
	for index := range view.pieces {
		if view.pieces[index].mono && view.pieces[index].text == "code" {
			piece = &view.pieces[index]
		}
	}
	if piece == nil || piece.weight <= gfx.WeightRegular {
		t.Fatalf("inline-code piece has no styled weight: %+v", piece)
	}
	var label *gfx.Text
	for index := range canvas.Texts() {
		if canvas.Texts()[index].Value == piece.text {
			label = &canvas.Texts()[index]
		}
	}
	if label == nil {
		t.Fatal("inline code painted no label")
	}
	if label.Style.Weight != piece.weight {
		t.Errorf("label weight = %v, want %v", label.Style.Weight, piece.weight)
	}
	runs := canvas.TextRuns(*label)
	if len(runs) == 0 {
		t.Fatal("inline code painted no grid runs")
	}
	for _, run := range runs {
		if run.Weight != piece.weight {
			t.Errorf("run weight = %v, want %v", run.Weight, piece.weight)
		}
	}
}

func TestMarkdownStrikeOnInlineCodeCoversOnlyTheTextArea(t *testing.T) {
	var doc markdown.Document
	doc.Parse("~~`code`~~")
	var view MarkdownView
	view.Layout(markdownCanvas(), MarkdownStyle{Size: 12}, &doc, 220)
	var canvas gfx.Canvas
	strike := gfx.Color{Red: 1, Alpha: 1}
	view.Paint(&canvas, MarkdownPaint{Look: MarkdownLook{Text: strike}})
	var label *gfx.Text
	for index := range canvas.Texts() {
		if canvas.Texts()[index].Value == "code" {
			label = &canvas.Texts()[index]
		}
	}
	if label == nil {
		t.Fatal("inline code painted no label")
	}
	found := false
	for _, rectangle := range canvas.Rectangles() {
		if rectangle.Color != strike {
			continue
		}
		found = true
		if rectangle.Bounds.X != label.Bounds.X || rectangle.Bounds.Width != label.Bounds.Width {
			t.Errorf("strike %+v does not match text area %+v", rectangle.Bounds, label.Bounds)
		}
	}
	if !found {
		t.Fatal("inline code painted no strike")
	}
}

func TestInlineCodeCopyDoesNotIncludeVisualPadding(t *testing.T) {
	src := "端口绑`127.0.0.1`还是"
	view, _ := layoutMarkdown(t, src, 220, MarkdownStyle{Size: 12})
	if got := view.SelectedText(MarkdownSelection{Focus: view.TextLen()}); got != "端口绑127.0.0.1还是" {
		t.Errorf("copied text = %q", got)
	}
	var piece *mdPiece
	for index := range view.pieces {
		if view.pieces[index].mono {
			piece = &view.pieces[index]
			break
		}
	}
	if piece == nil {
		t.Fatal("no inline code piece")
	}
	selected := view.SelectedText(MarkdownSelection{Anchor: int(piece.plainLo), Focus: int(piece.plainHi)})
	if selected != "127.0.0.1" {
		t.Errorf("selected inline code = %q", selected)
	}
}

func TestMarkdownAdjacentInlineCodeAndPunctuationKeepFullFragments(t *testing.T) {
	view, _ := layoutMarkdown(t, "`a` `bb`，前`127.0.0.1`后", 220, MarkdownStyle{Size: 12})
	const want = "a bb，前127.0.0.1后"
	if got := view.SelectedText(MarkdownSelection{Focus: view.TextLen()}); got != want {
		t.Fatalf("rendered text = %q, want %q", got, want)
	}
	for _, text := range []string{"a", "bb", "127.0.0.1"} {
		piece, background := markdownInlineCode(t, view, text)
		if piece.padX != markdownInlinePad || piece.x != background.x || piece.w != background.w {
			t.Errorf("code %q has text %+v and background %+v", text, piece, background)
		}
	}
	for i, current := range view.pieces {
		for _, next := range view.pieces[i+1:] {
			if current.y == next.y && current.x < next.x+next.w && next.x < current.x+current.w {
				t.Errorf("%q at %v..%v overlaps %q at %v..%v", current.text, current.x, current.x+current.w, next.text, next.x, next.x+next.w)
			}
		}
	}
	code, _ := markdownInlineCode(t, view, "127.0.0.1")
	for _, piece := range view.pieces {
		if piece.text == "，" && piece.y == code.y && piece.x < code.x+code.w {
			t.Errorf("punctuation at %v starts inside code background ending at %v", piece.x, code.x+code.w)
		}
		if piece.text == "后" && piece.y == code.y && piece.x < code.x+code.w {
			t.Errorf("following text at %v starts inside code background ending at %v", piece.x, code.x+code.w)
		}
	}
}

func TestMarkdownTinyWidthsTerminateWithValidText(t *testing.T) {
	tests := []struct {
		name, src, want string
	}{
		{name: "quoted code", src: "> `中中中中中中`，尾", want: "中中中中中中，尾"},
		{name: "nested quote", src: "> - `中中中中中中`。后面", want: "中中中中中中。后面"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var doc markdown.Document
			doc.Parse(test.src)
			var view MarkdownView
			view.Layout(nonlinearMarkdownCanvas(), MarkdownStyle{Size: 12}, &doc, 40)
			got := view.SelectedText(MarkdownSelection{Focus: view.TextLen()})
			if got != test.want {
				t.Fatalf("rendered text = %q, want %q", got, test.want)
			}
			if !utf8.ValidString(got) {
				t.Fatalf("rendered text is not valid UTF-8: %q", got)
			}
			if len(view.pieces) == 0 {
				t.Fatal("tiny layout produced no pieces")
			}
		})
	}
}

func TestInlineCodeSelectionRectsStayInsideThePaddedBackground(t *testing.T) {
	view, canvas := paintedMarkdown(t, "alpha `code` beta", 300)
	var piece *mdPiece
	var background mdBox
	for index := range view.pieces {
		if view.pieces[index].mono {
			piece = &view.pieces[index]
		}
	}
	for _, box := range view.boxes {
		if box.kind == mdBoxInlineCode {
			background = box
		}
	}
	if piece == nil || background.w == 0 {
		t.Fatalf("fixture has piece %+v and background %+v", piece, background)
	}
	selected := gfx.Color{Red: 1, Alpha: 1}
	var painted gfx.Canvas
	view.Paint(&painted, MarkdownPaint{
		Look:      MarkdownLook{Selection: selected},
		Selection: MarkdownSelection{Anchor: int(piece.plainLo), Focus: int(piece.plainHi)},
	})
	rects := markdownSelectionRects(&painted, selected)
	if len(rects) != 1 {
		t.Fatalf("%d selection rectangles, want 1: %+v", len(rects), rects)
	}
	start := piece.x + piece.padX
	if rects[0].Bounds.X != start || rects[0].Bounds.X+rects[0].Bounds.Width > background.x+background.w-piece.padX {
		t.Errorf("selection %+v is outside text part of background %+v", rects[0].Bounds, background)
	}
	_ = canvas
}

func TestMarkdownPaintAppliesTheFullClipRectangle(t *testing.T) {
	var canvas gfx.Canvas
	var doc markdown.Document
	doc.Parse("first\nsecond")
	var view MarkdownView
	view.Layout(markdownCanvas(), MarkdownStyle{Size: 12}, &doc, 300)
	clip := gfx.Rect{X: 0, Y: 20, Width: 40, Height: 18}
	view.Paint(&canvas, MarkdownPaint{Clip: clip})
	for _, text := range canvas.Texts() {
		if !text.HasClip || text.ClipOf != clip {
			t.Errorf("text %q did not retain clip %+v", text.Value, clip)
		}
	}
	for _, box := range canvas.Rectangles() {
		if !box.HasClip || box.ClipOf != clip {
			t.Errorf("background %+v did not retain clip %+v", box, clip)
		}
	}
}

func TestMarkdownPaintNestsItsClipAndRestoresTheCallerStack(t *testing.T) {
	var canvas gfx.Canvas
	outer := gfx.Rect{X: 5, Y: 10, Width: 120, Height: 80}
	inner := gfx.Rect{X: 20, Y: 20, Width: 50, Height: 100}
	canvas.Clip(outer)
	var doc markdown.Document
	doc.Parse("first `code` [link](https://example.com)")
	var view MarkdownView
	view.Layout(markdownCanvas(), MarkdownStyle{Size: 12}, &doc, 300)
	selection := gfx.Color{Red: 1, Alpha: 1}
	view.Paint(&canvas, MarkdownPaint{
		Look: MarkdownLook{Selection: selection}, Selection: MarkdownSelection{Anchor: 0, Focus: view.TextLen()},
		X: inner.X, Y: inner.Y, Clip: inner, Links: gfx.Scope(1),
	})
	if clips := canvas.Clips(); len(clips) != 1 || clips[0].Bounds != outer {
		t.Fatalf("the caller's clip stack was not restored: %+v", clips)
	}
	effective := inner
	if effective.X < outer.X {
		effective.X = outer.X
	}
	if effective.Y < outer.Y {
		effective.Y = outer.Y
	}
	if effective.X+effective.Width > outer.X+outer.Width {
		effective.Width = outer.X + outer.Width - effective.X
	}
	if effective.Y+effective.Height > outer.Y+outer.Height {
		effective.Height = outer.Y + outer.Height - effective.Y
	}
	if len(canvas.Texts()) == 0 || len(canvas.Rectangles()) == 0 || view.LinkCount() == 0 {
		t.Fatal("the clipped Markdown painted no text or background, or has no link")
	}
	for _, text := range canvas.Texts() {
		if !text.HasClip || text.ClipOf != effective {
			t.Errorf("text %q has clip %+v, want %+v", text.Value, text.ClipOf, effective)
		}
	}
	for _, rectangle := range canvas.Rectangles() {
		if !rectangle.HasClip || rectangle.ClipOf != effective {
			t.Errorf("background %+v has clip %+v, want %+v", rectangle.Bounds, rectangle.ClipOf, effective)
		}
	}
	for _, hit := range canvas.Hits() {
		if hit.Bounds.X < effective.X || hit.Bounds.X+hit.Bounds.Width > effective.X+effective.Width ||
			hit.Bounds.Y < effective.Y || hit.Bounds.Y+hit.Bounds.Height > effective.Y+effective.Height {
			t.Errorf("link hit %+v escapes the nested clip %+v", hit.Bounds, effective)
		}
	}
	canvas.Fill(gfx.Rect{X: 60, Y: 40, Width: 20, Height: 10}, gfx.Color{Blue: 1, Alpha: 1})
	if rectangle := canvas.Rectangles()[len(canvas.Rectangles())-1]; rectangle.ClipOf != outer {
		t.Errorf("a later control was clipped by Markdown: %+v", rectangle)
	}
}

func BenchmarkMarkdownLayout(b *testing.B) {
	src := strings.Repeat("## Title\nSome **bold** and `code` with [a link](https://example.com), 中文。\n- item one\n- item two\n> quote\n\n```go\ncode\n```\n", 20)
	var doc markdown.Document
	doc.Parse(src)
	canvas := markdownCanvas()
	var view MarkdownView
	b.ReportAllocs()
	for b.Loop() {
		view.Layout(canvas, MarkdownStyle{Size: 13}, &doc, 400)
	}
}

func BenchmarkMarkdownLongChineseLayout(b *testing.B) {
	src := strings.Repeat("端口绑定在容器地址，外部访问仍要从宿主机转发。", 200)
	var doc markdown.Document
	doc.Parse(src)
	canvas := markdownCanvas()
	var view MarkdownView
	b.ReportAllocs()
	for b.Loop() {
		view.Layout(canvas, MarkdownStyle{Size: 13}, &doc, 400)
	}
}

func BenchmarkMarkdownLongCodeLayout(b *testing.B) {
	src := "```sh\n" + strings.Repeat("docker run --rm -p 127.0.0.1:8080:8080 docker.1panel.live/library/example:2026.10.28\n", 200) + "```\n"
	var doc markdown.Document
	doc.Parse(src)
	canvas := markdownCanvas()
	var view MarkdownView
	b.ReportAllocs()
	for b.Loop() {
		view.Layout(canvas, MarkdownStyle{Size: 13}, &doc, 400)
	}
}

func BenchmarkMarkdownSteadyPaint(b *testing.B) {
	src := strings.Repeat("## Title\nSome **bold** and `code` with [a link](https://example.com), 中文。\n- item one\n- item two\n> quote\n\n```go\ncode\n```\n", 20)
	var doc markdown.Document
	doc.Parse(src)
	canvas := markdownCanvas()
	var view MarkdownView
	look := MarkdownLookFor(gfx.DarkTheme())
	view.Layout(canvas, MarkdownStyle{Size: 13}, &doc, 400)
	canvas.Reset()
	b.ReportAllocs()
	for b.Loop() {
		canvas.Reset()
		view.Paint(canvas, MarkdownPaint{Look: look, Links: gfx.Scope(3), Clip: gfx.Rect{Width: 400, Height: 600}})
	}
}

func TestHighlightedCodeIsDrawnInColoredRunsThatKeepTheText(t *testing.T) {
	// A toy highlighter: every "#" to the end of the line is a comment, and the
	// quoted words are strings.
	highlight := func(language, line string, dst []MarkdownCodeRun) []MarkdownCodeRun {
		if language != "sh" {
			return dst
		}
		if at := strings.Index(line, "#"); at >= 0 {
			dst = append(dst, MarkdownCodeRun{Start: at, End: len(line), Class: CodeComment})
		}
		return dst
	}
	src := "```sh\necho 你好 # 注释 and a very long comment that has to wrap somewhere\n```\n```text\nplain\n```\n"
	var doc markdown.Document
	doc.Parse(src)
	var view MarkdownView
	view.Layout(markdownCanvas(), MarkdownStyle{Size: 12, Highlight: highlight}, &doc, 220)
	var canvas gfx.Canvas
	look := MarkdownLookFor(gfx.DarkTheme())
	look.Code[CodeComment] = gfx.Color{Red: 1, Alpha: 1}
	view.Paint(&canvas, MarkdownPaint{Look: look})

	var joined strings.Builder
	coloured := 0
	for _, text := range canvas.Texts() {
		runs := canvas.TextRuns(text)
		if len(runs) == 0 {
			continue
		}
		joined.WriteString(text.Value)
		bytes := 0
		for _, run := range runs {
			bytes += int(run.Bytes)
			if run.Color == look.Code[CodeComment] {
				coloured++
			}
		}
		if bytes != len(text.Value) {
			t.Errorf("runs cover %d of %d bytes of %q", bytes, len(text.Value), text.Value)
		}
	}
	if coloured == 0 || !strings.Contains(joined.String(), "echo") {
		t.Fatalf("no colored comment runs: coloured=%d text=%q", coloured, joined.String())
	}
	// A block the highlighter ignores still uses the fixed grid: its runs cover
	// every byte even though they all have the body color.
	plain := 0
	for _, text := range canvas.Texts() {
		if text.Value != "plain" {
			continue
		}
		runs := canvas.TextRuns(text)
		bytes := 0
		for _, run := range runs {
			bytes += int(run.Bytes)
		}
		if bytes == len(text.Value) && text.CellWidth > 0 {
			plain++
		}
	}
	if plain != 1 {
		t.Errorf("the unhighlighted block is not drawn with complete grid runs")
	}
}

func TestTabsInCodeBecomeBlanksSoColumnsAreTrue(t *testing.T) {
	var doc markdown.Document
	doc.Parse("```go\n\tfmt.Println(1)\n\t\tx\tend\n```\n")
	var view MarkdownView
	view.Layout(markdownCanvas(), MarkdownStyle{Size: 12}, &doc, 300)
	var lines []string
	for _, piece := range view.pieces {
		if piece.mono {
			lines = append(lines, piece.text)
		}
	}
	want := []string{"    fmt.Println(1)", "        x   end"}
	if len(lines) != 2 || lines[0] != want[0] || lines[1] != want[1] {
		t.Fatalf("code lines = %q, want %q", lines, want)
	}
	if codes := view.Codes(); len(codes) != 1 || !strings.Contains(codes[0].Text, "\t") {
		t.Errorf("the copied text lost its tabs: %+v", codes)
	}
}

func TestFencedCodeBlankLinesLayOut(t *testing.T) {
	tests := []struct{ name, src, want string }{
		{name: "blank line", src: "```\na\n\nb\n```\n", want: "a\n\nb"},
		{name: "empty block", src: "```\n```\n", want: ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			view, _ := layoutMarkdown(t, test.src, 300, MarkdownStyle{Size: 12})
			if got := view.SelectedText(MarkdownSelection{Anchor: 0, Focus: view.TextLen()}); got != test.want {
				t.Fatalf("selected %q, want %q", got, test.want)
			}
		})
	}
}

func TestFencedCodeToleratesInvalidUTF8(t *testing.T) {
	view, _ := layoutMarkdown(t, "```\n\xff\n```\n", 300, MarkdownStyle{Size: 12})
	const want = "\xff"
	if got := view.SelectedText(MarkdownSelection{Anchor: 0, Focus: view.TextLen()}); got != want {
		t.Fatalf("selected %q, want %q", got, want)
	}
}

func TestInlineCodeSelectionUsesItsOwnCellWidth(t *testing.T) {
	canvas := markdownCanvas()
	var doc markdown.Document
	doc.Parse("alpha `code` beta")
	var view MarkdownView
	view.Layout(canvas, MarkdownStyle{Size: 12}, &doc, 300)
	selected := gfx.Color{Red: 1, Alpha: 1}
	var painted gfx.Canvas
	// "alpha " is six bytes, then the four bytes of the inline code.
	view.Paint(&painted, MarkdownPaint{
		Look: MarkdownLook{Selection: selected}, Selection: MarkdownSelection{Anchor: 6, Focus: 10},
	})
	rects := markdownSelectionRects(&painted, selected)
	if len(rects) != 1 {
		t.Fatalf("%d selection rectangles, want 1: %+v", len(rects), rects)
	}
	// Inline code is one point smaller than the body, so its cell is not a
	// fenced block's cell and there may be no fenced block at all.
	const cell = float32(11) / 2
	if want := 4 * cell; rects[0].Bounds.Width != want {
		t.Errorf("inline code selection is %v wide, want %v", rects[0].Bounds.Width, want)
	}
}

func TestMarkdownOffsetAtInsideInlineCode(t *testing.T) {
	view, _ := layoutMarkdown(t, "alpha `code` beta", 300, MarkdownStyle{Size: 12})
	var painted gfx.Canvas
	view.Paint(&painted, MarkdownPaint{})
	index := -1
	for i := range view.pieces {
		if view.pieces[i].mono {
			index = i
		}
	}
	if index < 0 {
		t.Fatal("no inline code piece")
	}
	piece := &view.pieces[index]
	const cell = float32(11) / 2
	offset, ok := view.OffsetAt(gfx.Point{X: piece.x + cell*2.1, Y: piece.y + 1})
	if want := int(piece.plainLo) + 2; !ok || offset != want {
		t.Fatalf("offset in inline code = %d, %v; want %d", offset, ok, want)
	}
}

func TestMarkdownCaretAtStyledWrappedPiece(t *testing.T) {
	canvas := markdownCanvas()
	var doc markdown.Document
	doc.Parse("plain **bold words** *italic ending* [link](https://example.invalid) tail")
	var view MarkdownView
	view.Layout(canvas, MarkdownStyle{Size: 16, LineHeight: 26}, &doc, 145)
	var painted gfx.Canvas
	view.Paint(&painted, MarkdownPaint{X: 17, Y: 31})
	for _, piece := range view.pieces {
		if !piece.italic || piece.y == 0 || len(piece.text) < 2 {
			continue
		}
		offset := int(piece.plainLo) + 1
		caret, ok := view.CaretAt(offset)
		want := float32(17) + piece.x + canvas.TextCaretStyle(piece.text, gfx.TextStyle{Size: piece.size, Weight: piece.weight, Italic: true}, 1)
		if !ok || caret.X != want || caret.Y != 31+piece.y || caret.Height != piece.h {
			t.Fatalf("wrapped italic caret=%+v ok=%v, want x=%v y=%v h=%v", caret, ok, want, 31+piece.y, piece.h)
		}
		if hit, ok := view.OffsetAt(gfx.Point{X: caret.X, Y: caret.Y + caret.Height/2}); !ok || hit != offset {
			t.Fatalf("caret hit=%d ok=%v, want=%d", hit, ok, offset)
		}
		return
	}
	t.Fatal("fixture did not wrap an italic piece")
}

func TestMarkdownCaretAtInlineCodeAndUnpaintedView(t *testing.T) {
	canvas := markdownCanvas()
	var doc markdown.Document
	doc.Parse("alpha `code` beta")
	var view MarkdownView
	if _, ok := view.CaretAt(0); ok {
		t.Fatal("empty view reported a caret")
	}
	view.Layout(canvas, MarkdownStyle{Size: 12}, &doc, 300)
	if _, ok := view.CaretAt(0); ok {
		t.Fatal("unpainted view reported a caret")
	}
	var painted gfx.Canvas
	view.Paint(&painted, MarkdownPaint{X: 9, Y: 13})
	for _, piece := range view.pieces {
		if !piece.mono || piece.text != "code" {
			continue
		}
		caret, ok := view.CaretAt(int(piece.plainLo) + 2)
		want := float32(9) + piece.x + piece.padX + 2*piece.cell
		if !ok || caret.X != want || caret.Y != 13+piece.y {
			t.Fatalf("inline code caret=%+v ok=%v, want x=%v", caret, ok, want)
		}
		view.Layout(canvas, MarkdownStyle{Size: 12}, &doc, 300)
		if _, ok := view.CaretAt(int(piece.plainLo) + 2); ok {
			t.Fatal("new layout retained stale painted geometry")
		}
		return
	}
	t.Fatal("inline code piece missing")
}

func TestMarkdownCaretAtSnapsGraphemeBoundary(t *testing.T) {
	view, _ := layoutMarkdown(t, "a👩‍💻b", 300, MarkdownStyle{Size: 16})
	var painted gfx.Canvas
	view.Paint(&painted, MarkdownPaint{})
	start, ok := view.CaretAt(len("a"))
	inside, insideOK := view.CaretAt(len("a") + len("👩") + 1)
	if !ok || !insideOK || start != inside {
		t.Fatalf("grapheme start=%+v inside=%+v, ok=%v/%v", start, inside, ok, insideOK)
	}
}

func TestMarkdownCaretAtPrefersTheWrappedLineStart(t *testing.T) {
	canvas := markdownCanvas()
	var doc markdown.Document
	doc.Parse("one two three four five six seven")
	var view MarkdownView
	view.Layout(canvas, MarkdownStyle{Size: 12, LineHeight: 20}, &doc, 64)
	var painted gfx.Canvas
	view.Paint(&painted, MarkdownPaint{X: 10, Y: 15})
	for _, piece := range view.pieces {
		if piece.y == 0 || piece.plainLo < 0 {
			continue
		}
		caret, ok := view.CaretAt(int(piece.plainLo))
		if !ok || caret.Y != 15+piece.y || caret.X != 10+piece.x {
			t.Fatalf("wrapped start caret=%+v ok=%v, want x=%v y=%v", caret, ok, 10+piece.x, 15+piece.y)
		}
		return
	}
	t.Fatal("fixture did not wrap")
}

func TestMarkdownCaretAtWithoutTextPiecesReportsNothing(t *testing.T) {
	view, _ := paintedMarkdown(t, "plain", 200)
	for index := range view.pieces {
		view.pieces[index].plainLo, view.pieces[index].plainHi = -1, -1
	}
	if _, ok := view.CaretAt(0); ok {
		t.Fatal("a view whose pieces carry no text offsets must not report a caret")
	}
}

// Every offset must map to a caret the pointer can reach again: hitting the
// middle of the caret lands on an offset that paints the same caret. Wrapped
// lines and style changes are the places this can go wrong.
func TestMarkdownCaretAtIsStableUnderOffsetAt(t *testing.T) {
	view, _ := paintedMarkdown(t, "plain **bold words** *italic ending* `code` tail that wraps over several lines", 150)
	for offset := 0; offset <= len(view.plain); offset++ {
		caret, ok := view.CaretAt(offset)
		if !ok {
			t.Fatalf("offset %d has no caret", offset)
		}
		hit, ok := view.OffsetAt(gfx.Point{X: caret.X, Y: caret.Y + caret.Height/2})
		if !ok {
			t.Fatalf("offset %d: caret %+v is not hittable", offset, caret)
		}
		again, ok := view.CaretAt(hit)
		if !ok || again.Y != caret.Y {
			t.Fatalf("offset %d -> hit %d: caret moved from %+v to %+v", offset, hit, caret, again)
		}
	}
}

func TestMarkdownCaretAtWarmDoesNotAllocate(t *testing.T) {
	canvas := markdownCanvas()
	var doc markdown.Document
	doc.Parse("plain **bold** *italic* tail")
	var view MarkdownView
	view.Layout(canvas, MarkdownStyle{Size: 16}, &doc, 145)
	var painted gfx.Canvas
	view.Paint(&painted, MarkdownPaint{})
	offset := len("plain bold ita")
	if _, ok := view.CaretAt(offset); !ok {
		t.Fatal("fixture has no caret")
	}
	if allocations := testing.AllocsPerRun(100, func() { view.CaretAt(offset) }); allocations != 0 {
		t.Fatalf("warm caret allocated %v times", allocations)
	}
}

func TestMarkdownCaretAtCoversSpacesBetweenStyles(t *testing.T) {
	canvas := markdownCanvas()
	var doc markdown.Document
	doc.Parse("alpha   **bold** tail")
	var view MarkdownView
	view.Layout(canvas, MarkdownStyle{Size: 16}, &doc, 300)
	var painted gfx.Canvas
	view.Paint(&painted, MarkdownPaint{})
	for offset := 0; offset <= view.TextLen(); offset++ {
		caret, ok := view.CaretAt(offset)
		if !ok {
			t.Fatalf("no caret for visible byte %d in %q", offset, view.plain)
		}
		if offset > len("alpha") && offset < len("alpha   ") {
			if hit, hitOK := view.OffsetAt(gfx.Point{X: caret.X, Y: caret.Y + caret.Height/2}); !hitOK || hit != offset {
				t.Fatalf("space byte %d hit %d, ok=%v", offset, hit, hitOK)
			}
		}
	}
}

func BenchmarkMarkdownCaretAtHit(b *testing.B) {
	canvas := markdownCanvas()
	var doc markdown.Document
	doc.Parse("plain **bold** *italic* tail")
	var view MarkdownView
	view.Layout(canvas, MarkdownStyle{Size: 16}, &doc, 145)
	var painted gfx.Canvas
	view.Paint(&painted, MarkdownPaint{})
	const offset = len("plain bold ita")
	b.ReportAllocs()
	for b.Loop() {
		view.CaretAt(offset)
	}
}

func TestHeadingInlineCodeSelectionUsesItsOwnCellWidth(t *testing.T) {
	canvas := markdownCanvas()
	var doc markdown.Document
	doc.Parse("# alpha `code` beta\n\n```\nx\n```\n")
	var view MarkdownView
	view.Layout(canvas, MarkdownStyle{Size: 12}, &doc, 300)
	index := -1
	for i := range view.pieces {
		if piece := &view.pieces[i]; piece.mono && !piece.spread && piece.text == "code" {
			index = i
		}
	}
	if index < 0 {
		t.Fatal("no inline code piece in the heading")
	}
	piece := &view.pieces[index]
	selected := gfx.Color{Red: 1, Alpha: 1}
	var painted gfx.Canvas
	view.Paint(&painted, MarkdownPaint{
		Look:      MarkdownLook{Selection: selected},
		Selection: MarkdownSelection{Anchor: int(piece.plainLo), Focus: int(piece.plainHi)},
	})
	rects := markdownSelectionRects(&painted, selected)
	if len(rects) != 1 {
		t.Fatalf("%d selection rectangles, want 1: %+v", len(rects), rects)
	}
	if want := float32(monoCells(piece.text)) * piece.cell; rects[0].Bounds.Width != want {
		t.Errorf("heading inline code selection is %v wide, want %v", rects[0].Bounds.Width, want)
	}
}

func TestMarkdownImagesAreScaledToTheColumnAndPainted(t *testing.T) {
	wide := gfx.Bitmap{Pix: make([]byte, 400*100*4), Width: 400, Height: 100, Stride: 400 * 4, ID: 1}
	small := gfx.Bitmap{Pix: make([]byte, 20*10*4), Width: 20, Height: 10, Stride: 20 * 4, ID: 2}
	style := MarkdownStyle{Size: 12, Image: func(dest string) (gfx.Bitmap, bool) {
		switch dest {
		case "wide.png":
			return wide, true
		case "small.png":
			return small, true
		}
		return gfx.Bitmap{}, false
	}}
	view, _ := layoutMarkdown(t, "![a](wide.png)\n\n[![b](small.png)](https://example.com/) ![missing](gone.png)", 200, style)
	if len(view.images) != 2 {
		t.Fatalf("images = %+v", view.images)
	}
	if got := view.images[0]; got.w != 200 || got.h != 50 {
		t.Errorf("a wide image is %vx%v, want 200x50", got.w, got.h)
	}
	if got := view.images[1]; got.w != 20 || got.h != 10 {
		t.Errorf("a small image is %vx%v, want its own 20x10", got.w, got.h)
	}
	if view.Height() < 50 {
		t.Errorf("height %v leaves no room for the image", view.Height())
	}
	if view.LinkCount() != 1 || view.Link(0) != "https://example.com/" {
		t.Errorf("the linked badge is not a link: %d", view.LinkCount())
	}
	if !strings.Contains(string(view.plain), "missing") {
		t.Errorf("an unresolved image shows no alt text: %q", view.plain)
	}
	var canvas gfx.Canvas
	view.Paint(&canvas, MarkdownPaint{X: 0, Y: 0})
	if len(canvas.Images()) != 2 {
		t.Errorf("painted %d images", len(canvas.Images()))
	}
}

// markdownPieceByText returns the first laid-out piece that draws text, or nil.
func markdownPieceByText(view *MarkdownView, text string) *mdPiece {
	for index := range view.pieces {
		if view.pieces[index].text == text {
			return &view.pieces[index]
		}
	}
	return nil
}

func TestMarkdownTallImageMakesItsLineTall(t *testing.T) {
	tall := gfx.Bitmap{Pix: make([]byte, 40*100*4), Width: 40, Height: 100, Stride: 40 * 4}
	short := gfx.Bitmap{Pix: make([]byte, 40*20*4), Width: 40, Height: 20, Stride: 40 * 4}
	style := MarkdownStyle{Size: 12, Image: func(dest string) (gfx.Bitmap, bool) {
		if dest == "tall.png" {
			return tall, true
		}
		return short, true
	}}
	view, _ := layoutMarkdown(t, "![tall](tall.png) ![short](short.png)\n\nafter", 300, style)
	if len(view.images) != 2 {
		t.Fatalf("images = %+v", view.images)
	}
	var tallest float32
	for _, image := range view.images {
		tallest = max(tallest, image.h)
	}
	if tallest != 100 {
		t.Fatalf("the tall image is %v high, want its own 100", tallest)
	}
	after := markdownPieceByText(view, "after")
	if after == nil {
		t.Fatal("the text after the images was not laid out")
	}
	if after.y < tallest {
		t.Errorf("the block after the image line starts at %v, above the %v image", after.y, tallest)
	}
}

func TestMarkdownImageWithoutABitmapLaysOutItsAltText(t *testing.T) {
	const src = "before ![the diagram](docs/arch.png) after"
	view, _ := layoutMarkdown(t, src, 300, MarkdownStyle{Size: 12})
	if len(view.images) != 0 {
		t.Fatalf("a view without a resolver laid out %+v", view.images)
	}
	if markdownPieceByText(view, "the diagram") == nil {
		t.Fatalf("the alt text was not laid out: %q", view.words())
	}
	var canvas gfx.Canvas
	view.Paint(&canvas, MarkdownPaint{})
	if len(canvas.Images()) != 0 || len(canvas.Texts()) == 0 {
		t.Errorf("painted %d images and %d texts for an unresolved image", len(canvas.Images()), len(canvas.Texts()))
	}

	// A resolver that cannot answer yet leaves the alt text in place.
	asked := 0
	style := MarkdownStyle{Size: 12, Image: func(string) (gfx.Bitmap, bool) {
		asked++
		return gfx.Bitmap{}, false
	}}
	pending, _ := layoutMarkdown(t, src, 300, style)
	if asked == 0 {
		t.Error("the view never asked the application for the bitmap")
	}
	if len(pending.images) != 0 || markdownPieceByText(pending, "the diagram") == nil {
		t.Errorf("a false answer left %+v and %q", pending.images, pending.words())
	}
}

func TestMarkdownLinkedImageKeepsItsLinkOverTheImage(t *testing.T) {
	badge := gfx.Bitmap{Pix: make([]byte, 30*20*4), Width: 30, Height: 20, Stride: 30 * 4}
	style := MarkdownStyle{Size: 12, Image: func(string) (gfx.Bitmap, bool) { return badge, true }}
	view, _ := layoutMarkdown(t, "[![badge](badge.png)](https://example.com/ci)", 200, style)
	if len(view.images) != 1 {
		t.Fatalf("images = %+v", view.images)
	}
	if view.LinkCount() != 1 || view.Link(0) != "https://example.com/ci" {
		t.Fatalf("the linked badge is not a link: %d %q", view.LinkCount(), view.Link(0))
	}
	var canvas gfx.Canvas
	scope := gfx.Scope(9)
	view.Paint(&canvas, MarkdownPaint{X: 5, Y: 7, Links: scope})
	hits := canvas.Hits()
	if len(hits) != 1 {
		t.Fatalf("%d hit regions for a linked image", len(hits))
	}
	image := view.images[0]
	if want := (gfx.Rect{X: 5 + image.x, Y: 7 + image.y, Width: image.w, Height: image.h}); hits[0].Bounds != want {
		t.Errorf("the link region is %+v, want the image at %+v", hits[0].Bounds, want)
	}
	if index, ok := hits[0].ID.In(scope); !ok || view.Link(index) != "https://example.com/ci" {
		t.Errorf("hit %+v does not lead to the link", hits[0])
	}
}

func TestMarkdownImageWiderThanTheColumnStartsItsOwnLine(t *testing.T) {
	wide := gfx.Bitmap{Pix: make([]byte, 400*100*4), Width: 400, Height: 100, Stride: 400 * 4}
	style := MarkdownStyle{Size: 12, Image: func(string) (gfx.Bitmap, bool) { return wide, true }}
	view, _ := layoutMarkdown(t, "before ![diagram](wide.png)", 100, style)
	if len(view.images) != 1 {
		t.Fatalf("images = %+v", view.images)
	}
	image := view.images[0]
	if image.w != 100 || image.h != 25 {
		t.Errorf("a wide image is %vx%v, want 100x25 in a 100 column", image.w, image.h)
	}
	if image.x != 0 || image.x+image.w > 100 {
		t.Errorf("the image is at %v..%v, outside a line of its own", image.x, image.x+image.w)
	}
	before := markdownPieceByText(view, "before")
	if before == nil {
		t.Fatal("the text before the image was not laid out")
	}
	if image.y < before.y+before.h {
		t.Errorf("the image starts at %v on the line above, which ends at %v", image.y, before.y+before.h)
	}
}

func TestMarkdownImagesInsideHeadingsListsQuotesAndTables(t *testing.T) {
	icon := gfx.Bitmap{Pix: make([]byte, 40*20*4), Width: 40, Height: 20, Stride: 40 * 4}
	style := MarkdownStyle{Size: 12, Image: func(string) (gfx.Bitmap, bool) { return icon, true }}
	src := "# ![heading](x.png)\n\n- ![item](x.png)\n\n> ![quote](x.png)\n\n| ![cell](x.png) | h |\n|---|---|\n| a | b |\n"
	view, _ := layoutMarkdown(t, src, 200, style)
	if len(view.images) != 4 {
		t.Fatalf("images in a heading, a list, a quote and a table = %+v", view.images)
	}
	for _, image := range view.images {
		if image.x < 0 || image.x+image.w > 200.5 {
			t.Errorf("an image at %v..%v is outside the 200 column", image.x, image.x+image.w)
		}
	}
}

func TestMarkdownImageWithNoDestinationIsOnlyItsAltText(t *testing.T) {
	asked := 0
	style := MarkdownStyle{Size: 12, Image: func(string) (gfx.Bitmap, bool) {
		asked++
		return gfx.Bitmap{}, false
	}}
	view, _ := layoutMarkdown(t, "![alt]()", 200, style)
	if asked != 0 {
		t.Errorf("the view asked for the bitmap of an empty destination %d times", asked)
	}
	if len(view.images) != 0 || markdownPieceByText(view, "alt") == nil {
		t.Errorf("an empty destination laid out %+v and %q", view.images, view.words())
	}
}
