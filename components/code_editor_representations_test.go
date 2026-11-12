package components

import (
	"sort"
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
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
		if offset, ok := fixture.editor.OffsetAt(gfx.Point{X: sample.x, Y: fixture.editor.content.Y + fixture.editor.lineHeight/2}); !ok || offset != sample.want {
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
	accent := gfx.RGB(180, 40, 60)
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
	assertBlob := func(background, foreground gfx.Color) {
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
	accent := gfx.RGB(180, 40, 60)
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
	if offset, ok := fixture.editor.OffsetAt(gfx.Point{X: blobX + 3*cell, Y: fixture.editor.content.Y + fixture.editor.lineHeight/2}); !ok || offset != len("A\u200b") {
		t.Fatalf("blob hit = %d, %v; want source byte %d", offset, ok, len("A\u200b"))
	}
}

func TestCodeEditorPlainAndHiddenRepresentationsDoNotPaintBlob(t *testing.T) {
	fixture := newCodeFixture("A\u200bB\x01C")
	fixture.snapshot.InvisibleColor = gfx.RGB(48, 96, 144)
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

func TestCodeEditorRepresentedWhitespaceDoesNotPaintOrdinaryMarkers(t *testing.T) {
	for _, sample := range []struct {
		name      string
		character rune
		markers   int
	}{
		{name: "space", character: ' ', markers: 1},
		{name: "tab", character: '\t', markers: 3},
	} {
		t.Run(sample.name, func(t *testing.T) {
			fixture := newCodeFixture("A" + string(sample.character) + "B")
			fixture.snapshot.ShowWhitespace = true
			fixture.snapshot.InvisibleColor = gfx.RGB(48, 96, 144)
			fixture.paint()
			if marks := fixture.fills(fixture.snapshot.InvisibleColor); len(marks) != sample.markers {
				t.Fatalf("ordinary whitespace markers = %d; want %d", len(marks), sample.markers)
			}
			for _, label := range []string{"", "LABEL"} {
				fixture.snapshot.Representations = []CodeRepresentation{{Character: sample.character, Label: label}}
				fixture.paint()
				if marks := fixture.fills(fixture.snapshot.InvisibleColor); len(marks) != 0 {
					t.Errorf("representation %q still paints ordinary whitespace markers: %+v", label, marks)
				}
			}
			fixture.snapshot.Representations = nil
			fixture.paint()
			if marks := fixture.fills(fixture.snapshot.InvisibleColor); len(marks) != sample.markers {
				t.Fatalf("removing representations left %d markers; want %d", len(marks), sample.markers)
			}
		})
	}
}

func TestCodeEditorDisabledBlobLabelsFadeOnce(t *testing.T) {
	for _, sample := range []struct {
		name      string
		highlight bool
		selection bool
		multiple  bool
	}{
		{name: "normal"},
		{name: "highlight", highlight: true},
		{name: "selection", selection: true},
		{name: "multiple selections", selection: true, multiple: true},
	} {
		t.Run(sample.name, func(t *testing.T) {
			fixture := newCodeFixture("A\u200bB")
			fixture.snapshot.Disabled = true
			fixture.snapshot.Representations = []CodeRepresentation{{Character: '\u200b', Label: "ZWSP", Blob: true}}
			want := fixture.theme.Background
			if sample.highlight {
				fixture.snapshot.HighlightLine = func(_ string, _ int, _ string, dst []CodeEditorRun) []CodeEditorRun {
					return append(dst, CodeEditorRun{Start: 1, End: 4, Color: gfx.RGB(180, 40, 60)})
				}
			}
			if sample.selection {
				want = fixture.theme.Selection
				if sample.multiple {
					fixture.snapshot.SelectionRanges = []text.Span{{Start: 1, End: 4}}
				} else {
					fixture.snapshot.SelectionStart, fixture.snapshot.SelectionEnd = 1, 4
				}
			}
			want.Alpha *= 0.5
			fixture.paint()
			found := false
			for _, label := range fixture.canvas.Texts() {
				if !strings.Contains(label.Value, "ZWSP") {
					continue
				}
				for _, run := range fixture.canvas.TextRuns(label) {
					if run.Bytes == 4 && run.Cells == 4 {
						found = true
						if run.Color != want {
							t.Errorf("disabled blob label color = %+v; want %+v", run.Color, want)
						}
					}
				}
			}
			if !found {
				t.Fatal("disabled blob label was not painted")
			}
		})
	}
}

func TestCodeEditorDisabledPlainRepresentationFades(t *testing.T) {
	fixture := newCodeFixture("A\u200bB")
	fixture.snapshot.Disabled = true
	fixture.snapshot.InvisibleColor = gfx.RGB(48, 96, 144)
	fixture.snapshot.Representations = []CodeRepresentation{{Character: '\u200b', Label: "ZWSP"}}
	fixture.paint()
	want := fixture.snapshot.InvisibleColor
	want.Alpha *= 0.5
	for _, label := range fixture.canvas.Texts() {
		if !strings.Contains(label.Value, "ZWSP") {
			continue
		}
		for _, run := range fixture.canvas.TextRuns(label) {
			if run.Bytes == 4 && run.Cells == 4 {
				if run.Color != want {
					t.Fatalf("disabled plain label color = %+v; want %+v", run.Color, want)
				}
				return
			}
		}
	}
	t.Fatal("disabled plain representation was not painted")
}

func BenchmarkCodeEditorPaintRepresentations(b *testing.B) {
	for _, sample := range []struct {
		name string
		reps []CodeRepresentation
	}{
		{name: "default"},
		{name: "labels", reps: []CodeRepresentation{{Character: '\t', Label: "TAB"}, {Character: '\u200b', Label: "ZWSP"}}},
		{name: "blobs", reps: []CodeRepresentation{{Character: '\t', Label: "TAB", Blob: true}, {Character: '\u200b', Label: "ZWSP", Blob: true}}},
	} {
		b.Run(sample.name, func(b *testing.B) {
			fixture := newCodeFixture(strings.Repeat("A\u200b\tB C\n", 100))
			fixture.snapshot.Focused = false
			fixture.snapshot.Representations = sample.reps
			fixture.paint()
			b.ReportAllocs()
			b.ResetTimer()
			for index := 0; index < b.N; index++ {
				fixture.paint()
			}
		})
	}
}

// TestCodeEditorIndentGuidesFollowDisplayedIndentation puts a representation and
// an indentation guide on the same leading whitespace: the guides must be placed
// in the displayed columns the layout used, not in the source tab stops, or a
// hidden or widened leading tab leaves a guide on the first text column.
func TestCodeEditorIndentGuidesFollowDisplayedIndentation(t *testing.T) {
	accent := gfx.RGB(48, 96, 144)
	guideColumns := func(fixture *codeFixture) []float32 {
		guide := fixture.snapshot.InvisibleColor
		guide.Alpha *= 0.5
		var columns []float32
		for _, mark := range fixture.fills(guide) {
			columns = append(columns, (mark.X-fixture.editor.content.X)/fixture.editor.cellWidth)
		}
		sort.Slice(columns, func(i, j int) bool { return columns[i] < columns[j] })
		return columns
	}
	caretColumn := func(fixture *codeFixture, offset int) float32 {
		fixture.snapshot.Caret = offset
		fixture.snapshot.SelectionStart, fixture.snapshot.SelectionEnd = 0, 0
		fixture.paint()
		caret, visible := fixture.editor.CaretBounds()
		if !visible {
			t.Fatalf("caret at source byte %d is not painted", offset)
		}
		return (caret.X - fixture.editor.content.X) / fixture.editor.cellWidth
	}
	hit := func(fixture *codeFixture, column float32) int {
		offset, ok := fixture.editor.OffsetAt(gfx.Point{
			X: fixture.editor.content.X + column*fixture.editor.cellWidth,
			Y: fixture.editor.content.Y + fixture.editor.lineHeight/2,
		})
		if !ok {
			t.Fatalf("no hit at displayed column %v", column)
		}
		return offset
	}

	// A representation that hides the leading tab leaves the text at displayed
	// column 0, so there is no complete indentation level to mark.
	hidden := newCodeFixture("\tX")
	hidden.snapshot.TabWidth = 4
	hidden.snapshot.InvisibleColor = accent
	hidden.snapshot.Representations = []CodeRepresentation{{Character: '\t', Label: ""}}
	hidden.snapshot.ShowIndentGuides = true
	hidden.paint()
	if columns := guideColumns(hidden); len(columns) != 0 {
		t.Fatalf("hidden leading tab drew guides at displayed columns %v; want none", columns)
	}
	if row := hidden.editor.rows[0]; row.cells != 1 || hidden.editor.rowText(row) != "X" {
		t.Fatalf("hidden tab layout = %q in %d cells; want X in 1 cell", hidden.editor.rowText(row), row.cells)
	}
	if column := caretColumn(hidden, 1); !near(column, 0) {
		t.Fatalf("caret for X = displayed column %v; want 0", column)
	}
	if offset := hit(hidden, 0.1); offset != 1 {
		t.Fatalf("hit on the first text column = source byte %d; want 1", offset)
	}

	// Labelled tabs widen each leading tab, so the text starts at displayed
	// column 4: only the level at column 0 fits before the text.
	labelled := newCodeFixture("\t\tX")
	labelled.snapshot.TabWidth = 4
	labelled.snapshot.InvisibleColor = accent
	labelled.snapshot.Representations = []CodeRepresentation{{Character: '\t', Label: "  "}}
	labelled.snapshot.ShowIndentGuides = true
	labelled.paint()
	columns := guideColumns(labelled)
	if len(columns) != 1 || !near(columns[0], 0) {
		t.Fatalf("labelled tabs drew guides at displayed columns %v; want one at 0", columns)
	}
	if row := labelled.editor.rows[0]; row.cells != 5 || labelled.editor.rowText(row) != "    X" {
		t.Fatalf("labelled tab layout = %q in %d cells; want \"    X\" in 5 cells", labelled.editor.rowText(row), row.cells)
	}
	textColumn := caretColumn(labelled, 2)
	if !near(textColumn, 4) {
		t.Fatalf("caret for X = displayed column %v; want 4", textColumn)
	}
	for _, column := range guideColumns(labelled) {
		if column+0.01 >= textColumn {
			t.Fatalf("guide at displayed column %v reaches the text at %v", column, textColumn)
		}
	}
	if offset := hit(labelled, textColumn+0.1); offset != 2 {
		t.Fatalf("hit on the first text column = source byte %d; want 2", offset)
	}

	// A label wider than the tab stop widens the displayed indentation in turn:
	// the guides follow the displayed tab stops (0, 2 and 4 cells for 6
	// displayed cells), not the two source-space levels.
	wider := newCodeFixture("\t\tX")
	wider.snapshot.TabWidth = 2
	wider.snapshot.InvisibleColor = accent
	wider.snapshot.Representations = []CodeRepresentation{{Character: '\t', Label: "   "}}
	wider.snapshot.ShowIndentGuides = true
	wider.paint()
	if columns := guideColumns(wider); len(columns) != 3 ||
		!near(columns[0], 0) || !near(columns[1], 2) || !near(columns[2], 4) {
		t.Fatalf("widened tabs drew guides at displayed columns %v; want 0, 2 and 4", columns)
	}
	if column := caretColumn(wider, 2); !near(column, 6) {
		t.Fatalf("caret for X = displayed column %v; want 6", column)
	}

	// The guides are the only difference: rows, caret placement and the hit
	// mapping are the same with them off.
	before := strings.Join(labelled.rowTexts(), "|")
	labelled.snapshot.ShowIndentGuides = false
	labelled.paint()
	if columns := guideColumns(labelled); len(columns) != 0 {
		t.Fatalf("disabled guides painted at displayed columns %v", columns)
	}
	if after := strings.Join(labelled.rowTexts(), "|"); after != before {
		t.Fatalf("guides changed the rows: %q -> %q", before, after)
	}
	if row := labelled.editor.rows[0]; row.cells != 5 {
		t.Fatalf("guides changed the row cells: %d; want 5", row.cells)
	}
	if column := caretColumn(labelled, 2); !near(column, 4) {
		t.Fatalf("caret with guides off = displayed column %v; want 4", column)
	}
	if offset := hit(labelled, 4.1); offset != 2 {
		t.Fatalf("hit with guides off = source byte %d; want 2", offset)
	}
}
