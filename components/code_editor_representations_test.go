package components

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

func TestCodeEditorRepresentationExpandsDisplayButKeepsSourceCaretAndSelection(t *testing.T) {
	const source = "A\u200b\tZ"
	fixture := newCodeFixture(source)
	fixture.snapshot.TabWidth = 4
	fixture.paint()
	baseline := fixture.editor.rows[0].cells
	fixture.snapshot.Representations = []CodeRepresentation{{Character: '\u200b', Label: "ZWSP"}}
	fixture.snapshot.Caret = len("A\u200b")
	fixture.snapshot.SelectionStart, fixture.snapshot.SelectionEnd = 1, len("A\u200b")
	fixture.paint()
	row := fixture.editor.rows[0]
	if row.cells != 9 || row.cells <= baseline || fixture.editor.rowText(row) != "AZWSP   Z" || fixture.snapshot.Value != source {
		t.Fatalf("display expansion = %q (%d cells), baseline=%d", fixture.editor.rowText(row), row.cells, baseline)
	}
	left := fixture.editor.content.X
	cell := fixture.editor.cellWidth
	for _, sample := range []struct {
		x    float32
		want int
	}{{left + 2*cell, 1}, {left + 4*cell, len("A\u200b")}} {
		if offset, ok := fixture.editor.OffsetAt(ui.Point{X: sample.x, Y: fixture.editor.content.Y + fixture.editor.lineHeight/2}); !ok || offset != sample.want {
			t.Errorf("representation hit at %v = %d, %v; want source byte %d", sample.x, offset, ok, sample.want)
		}
	}
	selected := false
	for _, rectangle := range fixture.canvas.Rectangles() {
		if rectangle.Color == fixture.theme.Selection && rectangle.Bounds.Width >= 4*cell {
			selected = true
		}
	}
	if !selected {
		t.Fatal("selection did not span the representation's visual cells")
	}
	fixture.snapshot.SelectionStart, fixture.snapshot.SelectionEnd = 0, 0
	fixture.paint()
	if caret, visible := fixture.editor.CaretBounds(); !visible || caret.X < left+5*cell || caret.X > left+6*cell {
		t.Fatalf("caret after source U+200B = %+v, %v", caret, visible)
	}
}

func TestCodeEditorRepresentationChangesWrapWithoutMakingUnicodeEOLALineBreak(t *testing.T) {
	const source = "A\u200bX\u2028Z\nlast"
	fixture := newCodeFixture(source)
	fixture.bounds.Width = fixture.widthFor(4)
	fixture.snapshot.Wrap = true
	fixture.snapshot.Representations = []CodeRepresentation{
		{Character: '\u200b', Label: "ZWSP"},
		{Character: '\u2028', Label: "LS"},
	}
	accent := ui.RGB(180, 40, 60)
	fixture.snapshot.HighlightLine = func(_ string, line int, _ string, dst []CodeEditorRun) []CodeEditorRun {
		if line == 0 {
			return append(dst, CodeEditorRun{Start: 1, End: len("A\u200b"), Color: accent})
		}
		return dst
	}
	fixture.paint()
	if fixture.editor.lineCount != 2 || len(fixture.editor.rows) < 4 || !strings.Contains(strings.Join(fixture.rowTexts(), "|"), "\u2028") {
		t.Fatalf("representation changed logical lines or failed to wrap: %+v", fixture.rowTexts())
	}
	wraps := fixture.editor.SoftWrapOffsets(nil)
	if len(wraps) < 2 || wraps[0] != 1 {
		t.Fatalf("soft-wrap offsets do not address source bytes: %+v", wraps)
	}
	found := false
	for _, label := range fixture.canvas.Texts() {
		if !strings.Contains(label.Value, "ZWSP") {
			continue
		}
		for _, run := range fixture.canvas.TextRuns(label) {
			if run.Color == accent && run.Bytes == 4 && run.Cells == 4 {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("source-byte highlight did not color the expanded representation")
	}
}

func TestCodeEditorHiddenRepresentationAndToggleInvalidateLayout(t *testing.T) {
	const source = "A\x01B"
	fixture := newCodeFixture(source)
	fixture.snapshot.Representations = []CodeRepresentation{{Character: '\x01', Label: ""}}
	fixture.paint()
	if row := fixture.editor.rows[0]; row.cells != 2 || fixture.editor.rowText(row) != "AB" || fixture.snapshot.Value != source {
		t.Fatalf("hidden control text = %q, cells=%d", fixture.editor.rowText(row), row.cells)
	}
	fixture.snapshot.Representations = []CodeRepresentation{{Character: '\x01', Label: "SOH"}}
	fixture.paint()
	if row := fixture.editor.rows[0]; row.cells != 5 || fixture.editor.rowText(row) != "ASOHB" {
		t.Fatalf("representation change kept stale layout: %q, cells=%d", fixture.editor.rowText(row), row.cells)
	}
	fixture.snapshot.Representations = nil
	fixture.paint()
	if row := fixture.editor.rows[0]; fixture.editor.rowText(row) != source || row.cells == 5 {
		t.Fatalf("disabling representations kept stale text: %q, cells=%d", fixture.editor.rowText(row), row.cells)
	}
}

func TestCodeEditorRepresentationsStayClippedAndMapVerticalOffsets(t *testing.T) {
	fixture := newCodeFixture(strings.Repeat("A\u200b", 5000))
	fixture.bounds.Width = fixture.widthFor(6)
	fixture.snapshot.Caret = len(fixture.snapshot.Value)
	fixture.snapshot.Representations = []CodeRepresentation{{Character: '\u200b', Label: "ZWSP"}}
	fixture.paint()
	if fixture.editor.scrollX <= 0 {
		t.Fatal("long represented line did not scroll horizontally")
	}
	for _, label := range fixture.canvas.Texts() {
		if label.RunCount == 0 {
			continue
		}
		if !label.HasClip || label.ClipOf.X < fixture.editor.content.X || len(label.Value) > 100 {
			t.Fatalf("representation escaped visible columns: %+v", label)
		}
	}
	fixture.snapshot.Value = "A\u200bX\nlast"
	fixture.snapshot.Caret = len("A\u200b")
	fixture.snapshot.Wrap = true
	fixture.paint()
	position, ok := fixture.editor.YAtOffset(len("A\u200b"))
	if !ok {
		t.Fatal("represented source byte has no visual position")
	}
	if offset, ok := fixture.editor.OffsetAtY(position); !ok || offset != len("A\u200b") {
		t.Fatalf("represented vertical roundtrip = %d, %v; want %d", offset, ok, len("A\u200b"))
	}
}

func TestCodeEditorBlobRepresentationInvertsNormalHighlightAndSelectionColors(t *testing.T) {
	const value = "A\u200bB"
	fixture := newCodeFixture(value)
	fixture.snapshot.Representations = []CodeRepresentation{{Character: '\u200b', Label: "ZWSP", Blob: true}}
	fixture.paint()
	cell := fixture.editor.cellWidth
	blobX := fixture.editor.content.X + cell
	assertBlob := func(background, foreground ui.Color) {
		t.Helper()
		foundFill, foundRun := false, false
		for _, rectangle := range fixture.canvas.Rectangles() {
			if rectangle.Color == background && rectangle.Bounds.X >= blobX && rectangle.Bounds.X < blobX+cell &&
				rectangle.Bounds.Width >= 4*cell-2 && rectangle.HasClip && rectangle.ClipOf.X >= fixture.editor.content.X {
				foundFill = true
			}
		}
		for _, label := range fixture.canvas.Texts() {
			if !strings.Contains(label.Value, "ZWSP") {
				continue
			}
			for _, run := range fixture.canvas.TextRuns(label) {
				if run.Bytes == 4 && run.Cells == 4 && run.Color == foreground {
					foundRun = true
				}
			}
		}
		if !foundFill || !foundRun {
			t.Fatalf("blob inverse colors missing: fill=%v run=%v, want %+v / %+v", foundFill, foundRun, background, foreground)
		}
	}
	assertBlob(fixture.theme.Foreground, fixture.theme.Background)
	accent := ui.RGB(180, 40, 60)
	fixture.snapshot.HighlightLine = func(_ string, _ int, _ string, dst []CodeEditorRun) []CodeEditorRun {
		return append(dst, CodeEditorRun{Start: 1, End: len("A\u200b"), Color: accent})
	}
	fixture.paint()
	assertBlob(accent, fixture.theme.Background)
	fixture.snapshot.SelectionStart, fixture.snapshot.SelectionEnd = 1, len("A\u200b")
	fixture.paint()
	assertBlob(fixture.theme.SelectionForeground, fixture.theme.Selection)
	fixture.snapshot.SelectionStart, fixture.snapshot.SelectionEnd = 0, 0
	fixture.snapshot.SelectionRanges = []text.Span{{Start: 1, End: len("A\u200b")}}
	fixture.paint()
	assertBlob(fixture.theme.SelectionForeground, fixture.theme.Selection)
	if fixture.snapshot.Value != value {
		t.Fatal("blob paint changed the source text")
	}
	if offset, ok := fixture.editor.OffsetAt(ui.Point{X: blobX + 3*cell, Y: fixture.editor.content.Y + fixture.editor.lineHeight/2}); !ok || offset != len("A\u200b") {
		t.Fatalf("blob hit = %d, %v; want source byte %d", offset, ok, len("A\u200b"))
	}
}

func TestCodeEditorPlainAndHiddenRepresentationsDoNotPaintBlob(t *testing.T) {
	fixture := newCodeFixture("A\u200bB\x01C")
	fixture.snapshot.InvisibleColor = ui.RGB(48, 96, 144)
	fixture.snapshot.Representations = []CodeRepresentation{
		{Character: '\x01', Label: "", Blob: true},
		{Character: '\u200b', Label: "ZWSP"},
	}
	fixture.paint()
	for _, rectangle := range fixture.canvas.Rectangles() {
		if rectangle.Color == fixture.theme.Foreground && rectangle.Radius > 0 && rectangle.Bounds.Width >= 3*fixture.editor.cellWidth-2 {
			t.Fatalf("plain or hidden representation painted a blob: %+v", rectangle)
		}
	}
	foundPlain := false
	for _, label := range fixture.canvas.Texts() {
		for _, run := range fixture.canvas.TextRuns(label) {
			foundPlain = foundPlain || run.Bytes == 4 && run.Color == fixture.snapshot.InvisibleColor
		}
	}
	if !foundPlain {
		t.Fatal("plain representation lost its marker color")
	}
}
