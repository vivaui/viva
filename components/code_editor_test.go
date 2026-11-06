package components

import (
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

// codeFixture paints one editor repeatedly and reads back what it laid out.
type codeFixture struct {
	editor   CodeEditor
	canvas   ui.Canvas
	theme    ui.Theme
	snapshot CodeEditorSnapshot
	bounds   ui.Rect
}

func newCodeFixture(value string) *codeFixture {
	theme := ui.LightTheme()
	theme.Selection = ui.RGB(1, 2, 3)
	theme.SelectionForeground = ui.RGB(4, 5, 6)
	f := &codeFixture{theme: theme, bounds: ui.Rect{X: 10, Y: 20, Width: 400, Height: 200}}
	f.snapshot = CodeEditorSnapshot{ID: ui.Item(1), Label: "Source", Value: value, Focused: true, ShowCaret: true}
	return f
}

func (f *codeFixture) paint() {
	f.canvas.Reset()
	f.editor.Paint(&f.canvas, f.theme, f.bounds, f.snapshot)
}

func TestCodeEditorCaretBlinksByDefaultAndStopsWhenInactive(t *testing.T) {
	fixture := newCodeFixture("hello")
	var now time.Duration
	fixture.canvas.SetClock(func() time.Duration { return now })
	frame := func(at time.Duration) (bool, bool) {
		now = at
		fixture.canvas.BeginFrame()
		fixture.paint()
		fixture.canvas.EndFrame()
		_, scheduled := fixture.canvas.NextFrame()
		return len(fixture.fills(fixture.theme.Foreground)) > 0, scheduled
	}
	if lit, scheduled := frame(0); !lit || !scheduled {
		t.Fatal("a focused caret must start lit and schedule its first turnover")
	}
	if lit, scheduled := frame(CaretBlinkHalf + time.Millisecond); lit || !scheduled {
		t.Fatal("a focused caret must turn dark and schedule its next turnover")
	}
	fixture.snapshot.Value = "hello!"
	if lit, _ := frame(CaretBlinkHalf + 2*time.Millisecond); !lit {
		t.Fatal("editing without moving the caret must restart it lit")
	}
	if lit, _ := frame(2*CaretBlinkHalf + 3*time.Millisecond); lit {
		t.Fatal("an unchanged caret must turn dark again")
	}
	fixture.snapshot.Caret = 2
	if lit, _ := frame(2*CaretBlinkHalf + 4*time.Millisecond); !lit {
		t.Fatal("moving the caret must immediately light it")
	}
	fixture.snapshot.Focused = false
	if lit, scheduled := frame(2*CaretBlinkHalf + 5*time.Millisecond); lit || scheduled {
		t.Fatal("an unfocused caret must stop requesting frames")
	}
	fixture.snapshot.Focused = true
	if lit, _ := frame(2*CaretBlinkHalf + 6*time.Millisecond); !lit {
		t.Fatal("regaining focus must restart the caret lit")
	}
	if lit, scheduled := frame(CaretBlinkSpan + 3*CaretBlinkHalf); !lit || scheduled {
		t.Fatal("a caret left alone must settle lit without idle frames")
	}
}

func BenchmarkCodeEditorPaintBlink(b *testing.B) {
	fixture := newCodeFixture("hello\nworld")
	var now time.Duration
	fixture.canvas.SetClock(func() time.Duration { return now })
	b.ReportAllocs()
	for index := 0; index < b.N; index++ {
		now += CaretBlinkHalf
		fixture.canvas.BeginFrame()
		fixture.paint()
		fixture.canvas.EndFrame()
	}
}

// cell is the size of one character cell and the height of a row.
func (f *codeFixture) cell() (width, height, line float32) {
	size := f.theme.Metrics.FontSizeM
	width, height = f.canvas.MonoCellSize(size, f.snapshot.MonoFace)
	return width, height, max(height, float32(int(size*1.5+0.5)))
}

// widthFor is a box width that gives cols cells of text and nothing else.
func (f *codeFixture) widthFor(cols int) float32 {
	cell, _, _ := f.cell()
	return 2*f.theme.Metrics.TextInsetX() + (float32(cols)+0.5)*cell
}

func (f *codeFixture) rowTexts() []string {
	var out []string
	for _, row := range f.editor.rows {
		out = append(out, f.editor.text[row.start:row.end])
	}
	return out
}

func (f *codeFixture) fills(color ui.Color) []ui.Rect {
	var out []ui.Rect
	for _, rectangle := range f.canvas.Rectangles() {
		if rectangle.Color == color {
			out = append(out, rectangle.Bounds)
		}
	}
	return out
}

func TestCodeEditorGutterMarkersUseFirstWrappedRowAndSeparateLane(t *testing.T) {
	fixture := newCodeFixture("a very long first line that wraps\nsecond\nthird")
	fixture.bounds.Width = fixture.widthFor(9)
	fixture.bounds.Height = 600
	fixture.snapshot.Wrap = true
	fixture.snapshot.LineNumbers = true
	fixture.snapshot.MarkerGutter = true
	first, third := ui.RGB(20, 80, 160), ui.RGB(180, 40, 60)
	fixture.paint()
	emptyContentX := fixture.editor.content.X
	fixture.snapshot.GutterMarkers = []CodeEditorGutterMarker{{Line: 0, Color: first}, {Line: 2, Color: third}}
	fixture.paint()
	if fixture.editor.content.X != emptyContentX {
		t.Fatalf("adding a mark moved code from %v to %v", emptyContentX, fixture.editor.content.X)
	}
	_, _, lineHeight := fixture.cell()
	firstMarks, thirdMarks := fixture.fills(first), fixture.fills(third)
	if len(firstMarks) != 1 || len(thirdMarks) != 1 {
		t.Fatalf("marks on wrapped lines = %+v / %+v", firstMarks, thirdMarks)
	}
	thirdRow := -1
	for index, row := range fixture.editor.rows {
		if row.line == 2 && fixture.editor.firstOfLine(index) {
			thirdRow = index
			break
		}
	}
	if !near(firstMarks[0].Y, fixture.editor.content.Y+(lineHeight-firstMarks[0].Height)/2) ||
		thirdRow < 0 || !near(thirdMarks[0].Y, fixture.editor.content.Y+float32(thirdRow)*lineHeight+(lineHeight-thirdMarks[0].Height)/2) {
		t.Fatalf("markers are not aligned to first rows: %+v / %+v", firstMarks, thirdMarks)
	}
	if firstMarks[0].X+firstMarks[0].Width >= fixture.editor.content.X {
		t.Fatalf("marker overlaps code: %+v, content %+v", firstMarks[0], fixture.editor.content)
	}

	fixture.snapshot.LineNumbers = false
	fixture.paint()
	if len(fixture.fills(first)) != 1 || fixture.editor.content.X <= fixture.bounds.X+fixture.theme.Metrics.TextInsetX() {
		t.Fatal("markers should keep a gutter with line numbers hidden")
	}
}

func TestCodeEditorGutterMarkersIgnoreOutOfContractInput(t *testing.T) {
	fixture := newCodeFixture("one\ntwo\nthree")
	fixture.bounds.Height = 600
	fixture.snapshot.MarkerGutter = true
	color := ui.RGB(20, 80, 160)
	// Past the end of the document, and a repeated line, paint nothing extra.
	fixture.snapshot.GutterMarkers = []CodeEditorGutterMarker{{Line: 1, Color: color}, {Line: 1, Color: color}, {Line: 99, Color: color}}
	fixture.paint()
	if marks := fixture.fills(color); len(marks) != 1 {
		t.Fatalf("marks = %+v, want only the one on line 1", marks)
	}
}

func TestCodeEditorGutterMarkersFadeWhenDisabled(t *testing.T) {
	fixture := newCodeFixture("one")
	fixture.bounds.Height = 600
	color := ui.RGB(20, 80, 160)
	fixture.snapshot.GutterMarkers = []CodeEditorGutterMarker{{Line: 0, Color: color}}
	fixture.paint()
	enabled := fixture.fills(color)
	fixture.snapshot.Disabled = true
	fixture.paint()
	if len(enabled) != 1 || len(fixture.fills(color)) != 0 {
		t.Fatal("a disabled editor must paint its markers dimmer than an enabled one")
	}
}

func TestCodeEditorGutterMarkersOnlyPaintVisibleLines(t *testing.T) {
	value := strings.Repeat("row\n", 30) + "last"
	fixture := newCodeFixture(value)
	fixture.bounds.Height = 80
	fixture.snapshot.Caret = len(value)
	first, last := ui.RGB(20, 80, 160), ui.RGB(180, 40, 60)
	fixture.snapshot.GutterMarkers = []CodeEditorGutterMarker{{Line: 0, Color: first}, {Line: 30, Color: last}}
	fixture.paint()
	if len(fixture.fills(first)) != 0 || len(fixture.fills(last)) != 1 {
		t.Fatalf("offscreen markers painted: first=%+v last=%+v", fixture.fills(first), fixture.fills(last))
	}
}

// near compares coordinates that came out of float32 arithmetic.
func near(a, b float32) bool {
	d := a - b
	return d > -0.01 && d < 0.01
}

func TestCodeEditorRowsWrapAtWordsAndKeepSpaces(t *testing.T) {
	cases := []struct {
		name, value string
		cols        int
		wrap        bool
		rows        []string
		cells       []int
	}{
		{"words wrap, the space hangs", "the quick brown fox", 10, true, []string{"the quick ", "brown fox"}, []int{10, 9}},
		{"a long word is cut", "abcdefghijkl", 5, true, []string{"abcde", "fghij", "kl"}, []int{5, 5, 2}},
		{"a wide character does not straddle the edge", "中文字", 5, true, []string{"中文", "字"}, []int{4, 2}},
		{"no wrap keeps a line whole", "long long long", 3, false, []string{"long long long"}, []int{14}},
		{"line breaks", "a\n\nb\n", 10, true, []string{"a", "", "b", ""}, []int{1, 0, 1, 0}},
		{"a tab runs to the next stop", "a\tb", 10, true, []string{"a\tb"}, []int{5}},
		{"combining marks take no cell", "éé", 10, true, []string{"éé"}, []int{2}},
		{"empty text is one empty row", "", 10, true, []string{""}, []int{0}},
	}
	for _, c := range cases {
		f := newCodeFixture(c.value)
		f.editor.fit(c.value, c.cols, c.wrap, 4)
		got := f.rowTexts()
		if strings.Join(got, "|") != strings.Join(c.rows, "|") || len(got) != len(c.rows) {
			t.Errorf("%s: rows %q, want %q", c.name, got, c.rows)
			continue
		}
		for i, row := range f.editor.rows {
			if int(row.cells) != c.cells[i] {
				t.Errorf("%s: row %d spans %d cells, want %d", c.name, i, row.cells, c.cells[i])
			}
		}
	}
	f := newCodeFixture("a\tb")
	f.editor.fit("a\tb", 10, true, 4)
	if shown := f.editor.rowText(f.editor.rows[0]); shown != "a   b" {
		t.Errorf("tab shown as %q", shown)
	}
	f.editor.fit("x\ny\n\nz", 10, true, 4)
	if f.editor.LineCount() != 4 {
		t.Errorf("LineCount = %d", f.editor.LineCount())
	}
}

func TestCodeEditorRowsCoverEveryByteOnce(t *testing.T) {
	value := "func main() {\n\tfmt.Println(\"你好, world — a very long line that has to wrap more than once because it is long\")\n}\n\n"
	for _, cols := range []int{1, 2, 7, 20, 80} {
		var f codeFixture
		f.editor.fit(value, cols, true, 4)
		covered := 0
		previousEnd := -1
		for _, row := range f.editor.rows {
			if int(row.start) < previousEnd {
				t.Fatalf("cols %d: row %+v starts before the previous ended at %d", cols, row, previousEnd)
			}
			if previousEnd >= 0 && int(row.start) != previousEnd && value[previousEnd] != '\n' {
				t.Fatalf("cols %d: bytes %d..%d belong to no row", cols, previousEnd, row.start)
			}
			covered += int(row.end - row.start)
			previousEnd = int(row.end)
		}
		if want := len(value) - strings.Count(value, "\n"); covered != want {
			t.Fatalf("cols %d: rows cover %d bytes, want %d", cols, covered, want)
		}
	}
}

func TestCodeEditorPlacesTheCaretOnTheCellGrid(t *testing.T) {
	value := "ab\tc中d\nxyz"
	f := newCodeFixture(value)
	f.paint()
	cell, _, line := f.cell()
	content := f.editor.content
	cases := []struct {
		offset, row, column int
	}{{0, 0, 0}, {1, 0, 1}, {2, 0, 2}, {3, 0, 4}, {4, 0, 5}, {7, 0, 7}, {8, 0, 8}, {9, 1, 0}, {12, 1, 3}}
	for _, c := range cases {
		f.snapshot.Caret = c.offset
		f.paint()
		bounds, ok := f.editor.CaretBounds()
		if !ok {
			t.Fatalf("offset %d: no caret", c.offset)
		}
		if want := content.X + float32(c.column)*cell; bounds.X != want {
			t.Errorf("offset %d: caret at x %v, want %v", c.offset, bounds.X, want)
		}
		if want := content.Y + float32(c.row)*line; bounds.Y != want {
			t.Errorf("offset %d: caret at y %v, want %v", c.offset, bounds.Y, want)
		}
		// A press on the caret's own edge puts it back where it was.
		got, ok := f.editor.OffsetAt(ui.Point{X: bounds.X + 0.01, Y: bounds.Y + line/2})
		if !ok || got != c.offset {
			t.Errorf("offset %d: a press at its caret gave %d, %v", c.offset, got, ok)
		}
	}
	// Inside a wide character the nearer edge wins.
	row := content.Y + line/2
	if got, _ := f.editor.OffsetAt(ui.Point{X: content.X + 5.9*cell, Y: row}); got != 4 {
		t.Errorf("left half of 中 = %d, want 4", got)
	}
	if got, _ := f.editor.OffsetAt(ui.Point{X: content.X + 6.1*cell, Y: row}); got != 7 {
		t.Errorf("right half of 中 = %d, want 7", got)
	}
	if got, _ := f.editor.OffsetAt(ui.Point{X: content.X + 50*cell, Y: row}); got != 8 {
		t.Errorf("past the end of the line = %d, want 8", got)
	}
	if got, _ := f.editor.OffsetAt(ui.Point{X: content.X, Y: content.Y + 900}); got != 9 {
		t.Errorf("below the text = %d, want the last row's start", got)
	}
	if got, _ := f.editor.OffsetAt(ui.Point{X: content.X - 30, Y: content.Y - 30}); got != 0 {
		t.Errorf("above the text = %d", got)
	}
}

func TestCodeEditorMoveKeepsTheColumnAcrossShortLines(t *testing.T) {
	value := "abcdef\nxy\nabcdefgh"
	f := newCodeFixture(value)
	f.paint()
	steps := []struct {
		movement TextareaMovement
		want     int
	}{
		{TextareaMoveDown, 9},  // line 1 ends at column 2
		{TextareaMoveDown, 15}, // back to column 5
		{TextareaMoveUp, 9},
		{TextareaMoveUp, 5},
		{TextareaMoveUp, 0}, // the first row: the start of the text
		{TextareaMoveDown, 0 + 0},
	}
	caret := 5
	for index, step := range steps {
		caret = f.editor.Move(value, caret, step.movement)
		if index == len(steps)-1 {
			break
		}
		if caret != step.want {
			t.Fatalf("step %d: caret %d, want %d", index, caret, step.want)
		}
	}
	if end := f.editor.Move(value, 16, TextareaMoveDown); end != len(value) {
		t.Fatalf("Down on the last row = %d, want the end", end)
	}
	if got := f.editor.Move(value, 3, TextareaMoveTextEnd); got != len(value) {
		t.Fatalf("TextEnd = %d", got)
	}
	if got := f.editor.Move(value, 3, TextareaMoveTextStart); got != 0 {
		t.Fatalf("TextStart = %d", got)
	}
	if got := f.editor.Move(value, 9, TextareaMoveLineEnd); got != 9 {
		t.Fatalf("End = %d", got)
	}
	if got := f.editor.Move(value, 12, TextareaMoveLineStart); got != 10 {
		t.Fatalf("Home = %d", got)
	}
	if got := f.editor.Move("other", 2, TextareaMoveDown); got != 2 {
		t.Fatalf("a move in text that was not painted = %d, want the caret unchanged", got)
	}
}

func TestCodeEditorHomeGoesToTheFirstCharacterFirst(t *testing.T) {
	value := "x\n  ab"
	f := newCodeFixture(value)
	f.paint()
	caret := 6
	for _, want := range []int{4, 2, 4, 2} {
		caret = f.editor.Move(value, caret, TextareaMoveLineStart)
		if caret != want {
			t.Fatalf("Home gave %d, want %d", caret, want)
		}
	}
}

func TestCodeEditorPageMovesGoAScreen(t *testing.T) {
	value := strings.Repeat("line\n", 100)
	f := newCodeFixture(value)
	f.paint()
	_, _, line := f.cell()
	rows := int(f.editor.content.Height/line) - 1
	got := f.editor.Move(value, 0, TextareaMovePageDown)
	if want := rows * 5; got != want {
		t.Fatalf("PageDown = %d, want %d", got, want)
	}
	if back := f.editor.Move(value, got, TextareaMovePageUp); back != 0 {
		t.Fatalf("PageUp = %d", back)
	}
}

func TestCodeEditorEndAtASoftWrapStaysOnItsRow(t *testing.T) {
	value := "aaaa bbbb cccc"
	f := newCodeFixture(value)
	f.snapshot.Wrap = true
	f.bounds.Width = f.widthFor(5)
	f.paint()
	if got := f.rowTexts(); len(got) != 3 || got[0] != "aaaa " {
		t.Fatalf("rows = %q", got)
	}
	end := f.editor.Move(value, 1, TextareaMoveLineEnd)
	if end != 5 {
		t.Fatalf("End = %d, want the wrap", end)
	}
	f.snapshot.Caret = end
	f.paint()
	first, _ := f.editor.CaretBounds()
	_, _, line := f.cell()
	if first.Y != f.editor.content.Y {
		t.Fatalf("a caret that got to the wrap by End is on y %v, want the first row", first.Y)
	}
	// The same offset reached any other way is the start of the next row.
	f.editor.setUpstream(0, false)
	f.paint()
	next, _ := f.editor.CaretBounds()
	if next.Y != f.editor.content.Y+line || next.X != f.editor.content.X {
		t.Fatalf("a caret at the wrap is at %+v, want the start of the second row", next)
	}
}

func TestCodeEditorPaintsOneLabelPerRowWhateverTheHighlight(t *testing.T) {
	value := "key: 中文 # note\n\tindented x\nplain"
	f := newCodeFixture(value)
	red, blue := ui.RGB(200, 20, 20), ui.RGB(20, 20, 200)
	f.snapshot.Language = "yaml"
	f.snapshot.Highlight = func(language, line string, dst []CodeEditorRun) []CodeEditorRun {
		if language != "yaml" {
			t.Fatalf("language %q", language)
		}
		if i := strings.Index(line, "#"); i >= 0 {
			dst = append(dst, CodeEditorRun{Start: i, End: len(line), Color: blue})
		}
		return append([]CodeEditorRun{{Start: 0, End: 3, Color: red}}, dst...)
	}
	f.paint()
	var texts []ui.Text
	for _, text := range f.canvas.Texts() {
		if text.RunCount > 0 {
			texts = append(texts, text)
		}
	}
	if len(texts) != 3 {
		t.Fatalf("%d grid labels for 3 rows", len(texts))
	}
	cell, _, _ := f.cell()
	for index, text := range texts {
		runs := f.canvas.TextRuns(text)
		bytes, column := 0, int32(0)
		for _, run := range runs {
			if run.Column != column {
				t.Fatalf("label %d: run starts at column %d, want %d", index, run.Column, column)
			}
			bytes += int(run.Bytes)
			column += run.Cells
		}
		if bytes != len(text.Value) {
			t.Fatalf("label %d: runs cover %d of %d bytes of %q", index, bytes, len(text.Value), text.Value)
		}
		if !text.Style.Mono || text.CellWidth != cell {
			t.Fatalf("label %d: not on the cell grid: %+v", index, text)
		}
	}
	if texts[1].Value != "    indented x" {
		t.Errorf("a tab is drawn as spaces to the stop: %q", texts[1].Value)
	}
	first := f.canvas.TextRuns(texts[0])
	if first[0].Color != red || first[0].Cells != 3 {
		t.Errorf("first run = %+v, want 3 red cells", first[0])
	}
	var wide []ui.TextRun
	for _, run := range first {
		if run.Cells == 2 && run.Bytes == 3 {
			wide = append(wide, run)
		}
	}
	if len(wide) != 2 {
		t.Errorf("wide characters should be runs of 2 cells and 3 bytes, got %+v", wide)
	}
	last := first[len(first)-1]
	if last.Color != blue {
		t.Errorf("the comment is %+v, want blue", last)
	}
}

func TestCodeEditorHighlightLineReceivesLogicalIndexAfterScrolling(t *testing.T) {
	value := "same\n" + strings.Repeat("padding\n", 25) + "same"
	fixture := newCodeFixture(value)
	firstColor, lastColor := ui.RGB(195, 40, 45), ui.RGB(30, 90, 190)
	var indexes []int
	legacyCalls := 0
	fixture.snapshot.Language = "go"
	fixture.snapshot.Highlight = func(_ string, _ string, runs []CodeEditorRun) []CodeEditorRun {
		legacyCalls++
		return runs
	}
	fixture.snapshot.HighlightLine = func(language string, lineIndex int, line string, runs []CodeEditorRun) []CodeEditorRun {
		if language != "go" {
			t.Fatalf("language=%q", language)
		}
		indexes = append(indexes, lineIndex)
		if line != "same" {
			return runs
		}
		color := firstColor
		if lineIndex == 26 {
			color = lastColor
		}
		return append(runs, CodeEditorRun{Start: 0, End: len(line), Color: color})
	}
	fixture.paint()
	firstPainted := false
	for _, drawn := range fixture.canvas.Texts() {
		if drawn.Value == "same" {
			runs := fixture.canvas.TextRuns(drawn)
			firstPainted = len(runs) > 0 && runs[0].Color == firstColor
		}
	}
	if !firstPainted || legacyCalls != 0 {
		t.Fatalf("first logical line missing color or old callback used: indexes=%v old=%d", indexes, legacyCalls)
	}
	fixture.editor.SetScrollY(fixture.editor.MaxScrollY())
	fixture.paint()
	lastPainted := false
	for _, drawn := range fixture.canvas.Texts() {
		if drawn.Value == "same" {
			runs := fixture.canvas.TextRuns(drawn)
			lastPainted = len(runs) > 0 && runs[0].Color == lastColor
		}
	}
	if !lastPainted || legacyCalls != 0 || !slices.Contains(indexes, 0) || !slices.Contains(indexes, 26) || fixture.snapshot.Value != value {
		t.Fatalf("scrolled line index/color wrong: indexes=%v old=%d last=%v", indexes, legacyCalls, lastPainted)
	}
}

func TestCodeEditorHighlightLineRunsOncePerWrappedLogicalLine(t *testing.T) {
	fixture := newCodeFixture("wrapped words repeated until the line folds\nsecond")
	fixture.bounds.Width = fixture.widthFor(12)
	fixture.bounds.Height = 600
	fixture.snapshot.Wrap = true
	counts := map[int]int{}
	fixture.snapshot.HighlightLine = func(_ string, lineIndex int, _ string, runs []CodeEditorRun) []CodeEditorRun {
		counts[lineIndex]++
		return runs
	}
	fixture.paint()
	if counts[0] != 1 || counts[1] != 1 || len(counts) != 2 {
		t.Fatalf("wrapped callbacks=%v", counts)
	}
}

func TestCodeEditorHighlightLineSanitizesRunsAndSkipsOversizedLine(t *testing.T) {
	value := "key: 中文\n" + strings.Repeat("x", CodeEditorMaxHighlightedLine+1)
	fixture := newCodeFixture(value)
	fixture.bounds.Height = 600
	color := ui.RGB(195, 40, 45)
	var indexes []int
	fixture.snapshot.HighlightLine = func(_ string, lineIndex int, line string, runs []CodeEditorRun) []CodeEditorRun {
		indexes = append(indexes, lineIndex)
		if lineIndex != 0 {
			t.Fatalf("oversized line %d was highlighted", lineIndex)
		}
		return append(runs,
			CodeEditorRun{Start: 0, End: 3, Color: color},
			CodeEditorRun{Start: 2, End: 4, Color: ui.RGB(1, 2, 3)},
			CodeEditorRun{Start: len(line) + 1, End: len(line) + 2, Color: color},
		)
	}
	fixture.paint()
	if !slices.Equal(indexes, []int{0}) || fixture.snapshot.Value != value {
		t.Fatalf("highlight callback indexes=%v, source changed=%v", indexes, fixture.snapshot.Value != value)
	}
	for _, drawn := range fixture.canvas.Texts() {
		if drawn.Value == "key: 中文" {
			runs := fixture.canvas.TextRuns(drawn)
			if len(runs) < 2 || runs[0].Color != color || runs[0].Bytes != 3 {
				t.Fatalf("invalid runs were not cleaned: %+v", runs)
			}
			return
		}
	}
	t.Fatal("highlighted line was not painted")
}

func TestCodeEditorIgnoresBadHighlightRuns(t *testing.T) {
	f := newCodeFixture("key: 中文")
	f.snapshot.Highlight = func(_, line string, dst []CodeEditorRun) []CodeEditorRun {
		return append(dst,
			CodeEditorRun{Start: -2, End: 3, Color: ui.RGB(1, 1, 1)},
			CodeEditorRun{Start: 2, End: 4, Color: ui.RGB(2, 2, 2)}, // overlaps the one before
			CodeEditorRun{Start: 5, End: 5, Color: ui.RGB(3, 3, 3)}, // empty
			CodeEditorRun{Start: 7, End: 8, Color: ui.RGB(4, 4, 4)}, // inside 中
			CodeEditorRun{Start: len(line) + 10, End: len(line) + 20, Color: ui.RGB(5, 5, 5)},
			CodeEditorRun{Start: 4, End: 5, Color: ui.Color{}}, // invisible
		)
	}
	f.paint()
	if text := f.canvas.Texts(); len(text) == 0 {
		t.Fatal("nothing painted")
	}
	value := f.canvas.Hits()[0].Value
	if value != "key: 中文" {
		t.Fatalf("highlight changed the source: %q", value)
	}
}

func TestCodeEditorSelectionAndLineBreakCue(t *testing.T) {
	value := "abc\ndef\nghi"
	f := newCodeFixture(value)
	f.snapshot.SelectionStart, f.snapshot.SelectionEnd, f.snapshot.Caret = 1, 9, 9
	f.paint()
	cell, _, line := f.cell()
	got := f.fills(f.theme.Selection)
	if len(got) != 3 {
		t.Fatalf("%d selection fills for a selection over three rows: %+v", len(got), got)
	}
	content := f.editor.content
	if want := content.X + cell; !near(got[0].X, want) || !near(got[0].Width, 2*cell+cell/2) {
		t.Errorf("first row fill %+v: the rest of the line and half a cell for the break", got[0])
	}
	if !near(got[1].Width, 3*cell+cell/2) || !near(got[1].X, content.X) {
		t.Errorf("middle row fill %+v", got[1])
	}
	if !near(got[2].Width, cell) || !near(got[2].Y, content.Y+2*line) {
		t.Errorf("last row fill %+v: it ends inside the line, so no cue", got[2])
	}
	// The selected text is drawn in the selection's foreground.
	foreground := 0
	for _, text := range f.canvas.Texts() {
		for _, run := range f.canvas.TextRuns(text) {
			if run.Color == f.theme.SelectionForeground {
				foreground++
			}
		}
	}
	if foreground == 0 {
		t.Error("no run in the selection foreground")
	}
}

func TestCodeEditorMarksAndBrackets(t *testing.T) {
	value := "foo(bar) foo\nfoo"
	f := newCodeFixture(value)
	f.snapshot.Marks = []text.Span{{Start: 0, End: 3}, {Start: 9, End: 12}, {Start: 13, End: 16}}
	f.snapshot.CurrentMark = text.Span{Start: 0, End: 3}
	f.snapshot.MatchBrackets = true
	f.snapshot.Caret = 4
	f.paint()
	cell, _, line := f.cell()
	content := f.editor.content
	current := f.fills(codeCurrentColor)
	if len(current) != 1 || current[0].X != content.X || current[0].Width != 3*cell {
		t.Errorf("current match fills = %+v", current)
	}
	matches := f.fills(codeMatchColor)
	if len(matches) != 2 || matches[0].X != content.X+9*cell || matches[1].Y != content.Y+line {
		t.Errorf("match fills = %+v", matches)
	}
	outline := f.theme.Foreground
	outline.Alpha *= 0.22
	brackets := f.fills(outline)
	if len(brackets) != 2 || brackets[0].X != content.X+3*cell || brackets[1].X != content.X+7*cell {
		t.Errorf("bracket fills = %+v, want the pair at columns 3 and 7", brackets)
	}
	f.snapshot.Caret = 10
	f.paint()
	if got := f.fills(outline); len(got) != 0 {
		t.Errorf("brackets outlined away from any: %+v", got)
	}
	// Marks that start above the first visible row do not stop the walk.
	f.snapshot.Value = strings.Repeat("foo\n", 200)
	f.snapshot.Marks = nil
	for i := 0; i < 200; i++ {
		f.snapshot.Marks = append(f.snapshot.Marks, text.Span{Start: 4 * i, End: 4*i + 3})
	}
	f.snapshot.Caret = 4 * 150
	f.paint()
	if got := f.fills(codeMatchColor); len(got) == 0 || len(got) > 20 {
		t.Errorf("%d match fills painted for the rows in view", len(got))
	}
}

func TestCodeEditorGutterNumbersLinesNotRows(t *testing.T) {
	value := "short\naaaa bbbb cccc\nz"
	f := newCodeFixture(value)
	f.snapshot.LineNumbers = true
	f.snapshot.Wrap = true
	f.bounds.Width = f.widthFor(5) + 4*func() float32 { c, _, _ := f.cell(); return c }()
	f.paint()
	var numbers []string
	for _, text := range f.canvas.Texts() {
		if strings.TrimSpace(text.Value) != "" && strings.Trim(text.Value, "0123456789") == "" {
			numbers = append(numbers, text.Value)
		}
	}
	if strings.Join(numbers, ",") != "1,2,3" {
		t.Fatalf("gutter numbers = %v, want one per line", numbers)
	}
	cell, _, _ := f.cell()
	if got, want := f.editor.content.X, f.bounds.X+f.theme.Metrics.TextInsetX()+4*cell; got != want {
		t.Fatalf("text starts at %v, want after a 2-digit gutter of 4 cells (%v)", got, want)
	}
	f.snapshot.Value = strings.Repeat("x\n", 1000)
	f.paint()
	if got, want := f.editor.content.X, f.bounds.X+f.theme.Metrics.TextInsetX()+6*cell; got != want {
		t.Fatalf("a 4-digit count gives text at %v, want %v", got, want)
	}
}

func TestCodeEditorCurrentLineTint(t *testing.T) {
	f := newCodeFixture("a\nb\nc")
	f.snapshot.CurrentLine = true
	f.snapshot.Caret = 2
	f.paint()
	tint := f.theme.Foreground
	tint.Alpha *= 0.05
	got := f.fills(tint)
	_, _, line := f.cell()
	if len(got) != 1 || got[0].Y != f.editor.content.Y+line {
		t.Fatalf("tint = %+v, want the second row", got)
	}
	f.snapshot.SelectionStart, f.snapshot.SelectionEnd = 0, 3
	f.paint()
	if got := f.fills(tint); len(got) != 0 {
		t.Fatalf("a selection leaves the current line untinted, got %+v", got)
	}
}

func TestCodeEditorCompositionIsUnderlined(t *testing.T) {
	f := newCodeFixture("ab你好c")
	f.snapshot.CompositionStart, f.snapshot.CompositionEnd, f.snapshot.Caret = 2, 8, 8
	f.paint()
	cell, _, line := f.cell()
	var underline []ui.Rect
	for _, rectangle := range f.canvas.Rectangles() {
		if rectangle.Bounds.Height == 1 && rectangle.Bounds.Width == 4*cell && rectangle.Color == f.theme.Foreground {
			underline = append(underline, rectangle.Bounds)
		}
	}
	if len(underline) != 1 || underline[0].X != f.editor.content.X+2*cell || underline[0].Y != f.editor.content.Y+line-3 {
		t.Fatalf("underline = %+v", underline)
	}
}

func TestCodeEditorHitDescribesANativeEditorSurface(t *testing.T) {
	f := newCodeFixture("first\nsecond\n")
	f.snapshot.Hint = "hint"
	f.paint()
	hits := f.canvas.Hits()
	if len(hits) != 1 {
		t.Fatalf("hits = %+v", hits)
	}
	hit := hits[0]
	if !hit.HideNativeOverlays || !hit.Multiline || hit.Role != ui.RoleTextField || hit.Value != "first\nsecond\n" || hit.Hint != "hint" {
		t.Fatalf("hit = %+v", hit)
	}
	if hit.Placement == nil || hit.TextBounds != f.editor.content {
		t.Fatalf("placement %v, text bounds %v, want the painted content %v", hit.Placement, hit.TextBounds, f.editor.content)
	}
	if hit.TextColor.Alpha != 0 {
		t.Fatalf("the platform's own text is drawn: %+v", hit.TextColor)
	}
}

func TestCodeEditorPlaceholder(t *testing.T) {
	f := newCodeFixture("")
	f.snapshot.Placeholder = "Choose an encoding"
	f.paint()
	found := false
	for _, text := range f.canvas.Texts() {
		found = found || text.Value == "Choose an encoding"
	}
	if !found {
		t.Fatal("placeholder not painted")
	}
	if _, ok := f.editor.CaretBounds(); !ok {
		t.Fatal("an empty editor has a caret")
	}
}

func TestCodeEditorCaretDefaultsToReadableWidthWithoutOverridingHairline(t *testing.T) {
	fixture := newCodeFixture("hello")
	fixture.paint()
	defaultCaret, ok := fixture.editor.CaretBounds()
	if !ok || defaultCaret.Width <= 1 {
		t.Fatalf("default caret should not be a hairline: %+v, shown=%t", defaultCaret, ok)
	}
	fixture.snapshot.CaretWidth = 1
	fixture.paint()
	hairline, ok := fixture.editor.CaretBounds()
	if !ok || hairline.Width != 1 {
		t.Fatalf("explicit hairline should remain available: %+v, shown=%t", hairline, ok)
	}
}

// A caller that leaves CaretColor unset gets a caret in the text color. A zero
// color is fully transparent, so a caret painted with it would be invisible:
// every application that does not choose a caret color would lose the caret
// while still reporting one through CaretBounds.
func TestCodeEditorCaretIsPaintedInTheTextColorByDefault(t *testing.T) {
	f := newCodeFixture("package main\n")
	f.paint()
	bounds, ok := f.editor.CaretBounds()
	if !ok {
		t.Fatal("focused editor has no caret")
	}
	// The fill is snapped to the pixel grid, so it is found by where it is and
	// how thick it is rather than by an exact bounds match.
	caretFills := func() []ui.Rectangle {
		var fills []ui.Rectangle
		for _, rect := range f.canvas.Rectangles() {
			if rect.Bounds.Width == bounds.Width && rect.Bounds.X == bounds.X && rect.Bounds.Height <= bounds.Height+1 {
				fills = append(fills, rect)
			}
		}
		return fills
	}
	fills := caretFills()
	if len(fills) != 1 {
		t.Fatalf("found %d fills at the caret's x %v, want one", len(fills), bounds.X)
	}
	if fills[0].Color.Alpha == 0 {
		t.Fatalf("caret painted fully transparent at %+v", fills[0].Bounds)
	}
	if want := f.theme.Foreground; fills[0].Color != want {
		t.Fatalf("caret color = %+v, want the text color %+v", fills[0].Color, want)
	}

	// An application that does choose a color keeps it.
	own := ui.RGB(0, 128, 255)
	f.snapshot.CaretColor = own
	f.paint()
	fills = caretFills()
	if len(fills) != 1 || fills[0].Color != own {
		t.Fatalf("caret fills = %+v, want one in %+v", fills, own)
	}
}

func TestCodeEditorScrollFollowsTheCaretAndTheWheelIsKept(t *testing.T) {
	value := strings.Repeat("line\n", 200)
	f := newCodeFixture(value)
	f.paint()
	_, _, line := f.cell()
	f.snapshot.Caret = len(value) - 1
	f.paint()
	caret, ok := f.editor.CaretBounds()
	if f.editor.scrollY == 0 || !ok || caret.Y < f.editor.content.Y || caret.Y+line > f.editor.content.Y+f.editor.content.Height+0.5 {
		t.Fatalf("caret at the end did not scroll to it: scroll %v, caret %+v, %v", f.editor.scrollY, caret, ok)
	}
	f.editor.ScrollBy(0, -10*line)
	moved := f.editor.scrollY
	f.paint()
	if f.editor.scrollY != moved {
		t.Fatalf("paint undid the wheel: %v -> %v", moved, f.editor.scrollY)
	}
	result := f.editor.Scroll(ui.ScrollEvent{DeltaY: -3 * line, Precise: true})
	if !result.Handled || f.editor.scrollY != moved-3*line {
		t.Fatalf("Scroll moved %v -> %v", moved, f.editor.scrollY)
	}
	f.editor.Reveal(0)
	f.snapshot.Caret = 0
	f.paint()
	if f.editor.scrollY != 0 {
		t.Fatalf("Reveal did not bring the caret into view: %v", f.editor.scrollY)
	}
	if first, last := f.editor.VisibleLines(); first != 0 || last < 5 || last > 40 {
		t.Fatalf("VisibleLines = %d, %d", first, last)
	}
}

func TestCodeEditorWithoutWrapScrollsSidewaysAndShowsOnlyWhatIsInView(t *testing.T) {
	value := strings.Repeat("0123456789", 20000) // one 200,000 column line
	f := newCodeFixture(value)
	f.snapshot.HorizontalScrollbarID = ui.Item(3)
	f.snapshot.Caret = 100000
	f.paint()
	cell, _, _ := f.cell()
	var label ui.Text
	for _, text := range f.canvas.Texts() {
		if text.RunCount > 0 {
			label = text
		}
	}
	cols := int(f.editor.content.Width / cell)
	if len(label.Value) > cols+20 || len(label.Value) == 0 {
		t.Fatalf("a %d byte line painted as a %d byte label for %d columns", len(value), len(label.Value), cols)
	}
	if label.Bounds.Width > float32(cols+20)*cell {
		t.Fatalf("label is %v wide for a %v wide view", label.Bounds.Width, f.editor.content.Width)
	}
	caret, ok := f.editor.CaretBounds()
	if !ok || caret.X < f.editor.content.X || caret.X > f.editor.content.X+f.editor.content.Width {
		t.Fatalf("caret %+v outside the view %+v", caret, f.editor.content)
	}
	if f.editor.barH.MaxOffsetX <= 0 {
		t.Fatal("no horizontal scrollbar")
	}
	// The label sits where its first column belongs, so the text lines up with the caret.
	columnOfCaret := 100000
	wantX := f.editor.content.X - f.editor.scrollX + float32(columnOfCaret)*cell
	if caret.X != wantX {
		t.Fatalf("caret x %v, want %v", caret.X, wantX)
	}
	// The digit under the caret is the one the model has there.
	index := int((caret.X-label.Bounds.X)/cell + 0.5)
	if label.Value[index] != value[100000] {
		t.Fatalf("label has %q under the caret, text has %q", label.Value[index], value[100000])
	}
}

func TestCodeEditorScrollbarDragKeepsSelectionAndAllocatesNothingWhenSteady(t *testing.T) {
	f := newCodeFixture(strings.Repeat("key: 中文\n", 100))
	f.bounds = ui.Rect{Width: 300, Height: 200}
	f.snapshot.ScrollbarID = ui.Item(2)
	f.snapshot.LineNumbers = true
	f.snapshot.CurrentLine = true
	f.snapshot.MatchBrackets = true
	f.snapshot.Highlight = func(_, line string, dst []CodeEditorRun) []CodeEditorRun {
		return append(dst, CodeEditorRun{End: 3, Color: ui.RGB(200, 20, 20)})
	}
	f.snapshot.Marks = []text.Span{{Start: 0, End: 3}, {Start: 14, End: 17}}
	f.paint()
	if f.editor.barV.MaxOffsetY <= 0 {
		t.Fatal("missing scrollbar")
	}
	if allocations := testing.AllocsPerRun(100, f.paint); allocations != 0 {
		t.Fatalf("steady editor allocations: %g", allocations)
	}
	result := f.editor.Pointer(ui.PointerEvent{Kind: ui.PointerDown, Target: ui.Item(2), Y: f.editor.barV.VerticalTrack.Y + f.editor.barV.VerticalTrack.Height + 100})
	if !result.Handled || f.editor.scrollY != f.editor.barV.MaxOffsetY {
		t.Fatalf("scrollbar did not reach the bottom: %v of %v", f.editor.scrollY, f.editor.barV.MaxOffsetY)
	}
	f.paint()
	if f.editor.scrollY == 0 {
		t.Fatal("paint undid the user's scroll")
	}
	f.editor.Pointer(ui.PointerEvent{Kind: ui.PointerUp})
	if f.editor.drag != codeDragNone {
		t.Fatal("scrollbar retained the pointer")
	}
	if got := f.editor.Pointer(ui.PointerEvent{Kind: ui.PointerDown, Target: ui.Item(9)}); got.Handled {
		t.Fatal("a press elsewhere is not the scrollbar's")
	}
}

func TestCodeEditorHorizontalScrollbarDrag(t *testing.T) {
	f := newCodeFixture(strings.Repeat("x", 5000))
	f.snapshot.HorizontalScrollbarID = ui.Item(3)
	f.paint()
	if f.editor.barH.MaxOffsetX <= 0 {
		t.Fatal("missing horizontal scrollbar")
	}
	thumb := f.editor.barH.HorizontalThumb
	f.editor.Pointer(ui.PointerEvent{Kind: ui.PointerDown, Target: ui.Item(3), X: thumb.X + thumb.Width/2})
	f.editor.Pointer(ui.PointerEvent{Kind: ui.PointerMove, X: thumb.X + thumb.Width/2 + 40})
	if f.editor.scrollX <= 0 {
		t.Fatal("dragging the thumb did not scroll")
	}
	f.editor.Pointer(ui.PointerEvent{Kind: ui.PointerUp})
}

func TestCodeEditorFollowsTheCaretAcrossAWrapChange(t *testing.T) {
	value := strings.Repeat("word ", 400)
	f := newCodeFixture(value)
	f.snapshot.Wrap = true
	f.snapshot.Caret = len(value)
	f.paint()
	before := f.editor.scrollY
	f.bounds.Width = f.widthFor(20)
	f.paint()
	if f.editor.scrollY == before {
		t.Fatalf("narrowing the box left the caret's row where it was (%v)", before)
	}
	_, _, line := f.cell()
	caret, ok := f.editor.CaretBounds()
	if !ok || caret.Y < f.editor.content.Y || caret.Y+line > f.editor.content.Y+f.editor.content.Height+1 {
		t.Fatalf("caret %+v is not in view %+v", caret, f.editor.content)
	}
}

func TestCodeEditorCommandsForKeys(t *testing.T) {
	primary := ui.ModifierControl
	cases := []struct {
		event ui.KeyEvent
		want  CodeEditorCommand
		ok    bool
	}{
		{ui.KeyEvent{Key: ui.KeyTab}, CodeEditorInsertTab, true},
		{ui.KeyEvent{Key: ui.KeyTab, Modifiers: ui.ModifierShift}, CodeEditorOutdent, true},
		{ui.KeyEvent{Key: ui.KeyTab, Modifiers: primary}, 0, false}, // Ctrl+Tab switches tabs
		{ui.KeyEvent{Key: ui.KeyUnknown, Modifiers: primary, Text: "/"}, CodeEditorToggleComment, true},
		{ui.KeyEvent{Key: ui.KeyUnknown, Modifiers: ui.ModifierMeta, Text: "/"}, CodeEditorToggleComment, true},
		{ui.KeyEvent{Modifiers: primary, Text: "]"}, CodeEditorIndent, true},
		{ui.KeyEvent{Modifiers: primary, Text: "["}, CodeEditorOutdent, true},
		{ui.KeyEvent{Key: ui.KeyUp, Modifiers: ui.ModifierAlt}, CodeEditorMoveLinesUp, true},
		{ui.KeyEvent{Key: ui.KeyDown, Modifiers: ui.ModifierAlt}, CodeEditorMoveLinesDown, true},
		{ui.KeyEvent{Key: ui.KeyUp, Modifiers: ui.ModifierAlt | ui.ModifierShift}, CodeEditorDuplicateLinesUp, true},
		{ui.KeyEvent{Key: ui.KeyDown, Modifiers: ui.ModifierAlt | ui.ModifierShift}, CodeEditorDuplicateLinesDown, true},
		{ui.KeyEvent{Modifiers: primary | ui.ModifierShift, Text: "K"}, CodeEditorDeleteLines, true},
		{ui.KeyEvent{Modifiers: primary | ui.ModifierShift, Text: "k"}, CodeEditorDeleteLines, true},
		{ui.KeyEvent{Key: ui.KeyUp}, 0, false},
		{ui.KeyEvent{Modifiers: primary, Text: "s"}, 0, false},
		{ui.KeyEvent{Key: ui.KeyTab, Release: true}, 0, false},
	}
	for _, c := range cases {
		got, ok := CodeEditorCommandForKey(c.event)
		if ok != c.ok || ok && got != c.want {
			t.Errorf("%+v = %v, %v; want %v, %v", c.event, got, ok, c.want, c.ok)
		}
	}
}

func TestCodeEditorCommandsEditText(t *testing.T) {
	editor := text.NewEditor("a\nb")
	editor.SetCode(text.CodeOptions{TabWidth: 2, LineComment: "#"})
	editor.SetSelection(text.Range{Start: 0, End: 3})
	for _, step := range []struct {
		command CodeEditorCommand
		want    string
	}{
		{CodeEditorInsertTab, "  a\n  b"},
		{CodeEditorOutdent, "a\nb"},
		{CodeEditorIndent, "  a\n  b"},
		{CodeEditorToggleComment, "  # a\n  # b"},
		{CodeEditorToggleComment, "  a\n  b"},
		{CodeEditorDuplicateLinesDown, "  a\n  b\n  a\n  b"},
		{CodeEditorDeleteLines, "  a\n  b"},
		{CodeEditorMoveLinesDown, "  a\n  b"},
	} {
		if !step.command.Apply(editor) || editor.String() != step.want {
			t.Fatalf("command %d gave %q, want %q", step.command, editor.String(), step.want)
		}
	}
	if CodeEditorNoCommand.Apply(editor) {
		t.Fatal("no command is not a command")
	}
}

func BenchmarkCodeEditorPaintSteady(b *testing.B) {
	f := newCodeFixture(strings.Repeat("key: value # comment with some 中文\n", 30000))
	f.bounds = ui.Rect{Width: 900, Height: 600}
	f.snapshot.LineNumbers = true
	f.snapshot.ScrollbarID = ui.Item(2)
	f.snapshot.Highlight = func(_, line string, dst []CodeEditorRun) []CodeEditorRun {
		return append(dst, CodeEditorRun{End: 3, Color: ui.RGB(200, 20, 20)}, CodeEditorRun{Start: 11, End: len(line), Color: ui.RGB(20, 120, 20)})
	}
	f.paint()
	b.ReportAllocs()
	for b.Loop() {
		f.paint()
	}
}

// BenchmarkCodeEditorRelayoutOnEdit is the cost of a keystroke in a megabyte of
// text: the rows are laid out again for the new text.
func BenchmarkCodeEditorRelayoutOnEdit(b *testing.B) {
	base := strings.Repeat("key: value # comment with some 中文 and a long tail to wrap around the edge of the box\n", 12000)
	b.SetBytes(int64(len(base)))
	for _, wrap := range []bool{false, true} {
		name := "no wrap"
		if wrap {
			name = "wrap"
		}
		b.Run(name, func(b *testing.B) {
			f := newCodeFixture(base)
			f.bounds = ui.Rect{Width: 900, Height: 600}
			f.snapshot.Wrap = wrap
			f.paint()
			b.ReportAllocs()
			toggle := false
			for b.Loop() {
				toggle = !toggle
				if toggle {
					f.snapshot.Value = base + "x"
				} else {
					f.snapshot.Value = base
				}
				f.paint()
			}
		})
	}
}

// Blank lines draw nothing, and must not cost the runs buffer the next line needs.
func TestCodeEditorSteadyPaintWithBlankLinesAllocatesNothing(t *testing.T) {
	f := newCodeFixture("first: 1\n\n\nsecond: 2\n\nthird: 3\n")
	f.snapshot.Highlight = func(_, line string, dst []CodeEditorRun) []CodeEditorRun {
		return append(dst, CodeEditorRun{End: 3, Color: ui.RGB(200, 20, 20)})
	}
	f.paint()
	if allocations := testing.AllocsPerRun(100, f.paint); allocations != 0 {
		t.Fatalf("steady editor allocations with blank lines: %g", allocations)
	}
}

func numberedLines(n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteString("line ")
		b.WriteString(strings.Repeat("x", i%7))
		b.WriteByte('\n')
	}
	return b.String()
}

func TestCodeEditorScrollYIsSetAndClamped(t *testing.T) {
	f := newCodeFixture(numberedLines(100))
	f.snapshot.Wrap = true
	f.editor.SetScrollY(50) // before the first paint: ignored
	f.paint()
	if f.editor.ScrollY() != 0 {
		t.Fatalf("scroll before any move = %v", f.editor.ScrollY())
	}
	most := f.editor.MaxScrollY()
	if most <= 0 {
		t.Fatalf("a hundred lines in a 200px box must scroll, max=%v", most)
	}
	f.editor.SetScrollY(most / 2)
	f.paint()
	if !near(f.editor.ScrollY(), most/2) {
		t.Errorf("a paint with the caret unmoved changed the scroll: %v, want %v", f.editor.ScrollY(), most/2)
	}
	f.editor.SetScrollY(most * 10)
	if f.editor.ScrollY() != most {
		t.Errorf("scroll past the end = %v, want %v", f.editor.ScrollY(), most)
	}
	f.editor.SetScrollY(-5)
	if f.editor.ScrollY() != 0 {
		t.Errorf("scroll above the start = %v", f.editor.ScrollY())
	}
}

func TestCodeEditorScrollXIsSetAndClamped(t *testing.T) {
	f := newCodeFixture(strings.Repeat("wide text ", 300))
	f.snapshot.HorizontalScrollbarID = ui.Item(3)
	f.editor.SetScrollX(50)
	f.paint()
	if f.editor.ScrollX() != 0 {
		t.Fatalf("scroll before first paint = %v", f.editor.ScrollX())
	}
	maximum := f.editor.MaxScrollX()
	if maximum <= 0 {
		t.Fatalf("long line must scroll, max=%v", maximum)
	}
	f.editor.SetScrollX(maximum / 2)
	f.paint()
	if !near(f.editor.ScrollX(), maximum/2) {
		t.Fatalf("paint changed horizontal scroll: %v", f.editor.ScrollX())
	}
	f.editor.SetScrollX(maximum * 10)
	if f.editor.ScrollX() != maximum {
		t.Fatalf("scroll past end = %v", f.editor.ScrollX())
	}
	f.editor.SetScrollX(-5)
	if f.editor.ScrollX() != 0 {
		t.Fatalf("scroll before start = %v", f.editor.ScrollX())
	}
	f.snapshot.Wrap = true
	f.paint()
	if f.editor.ScrollX() != 0 || f.editor.MaxScrollX() != 0 {
		t.Fatal("wrapped text retained horizontal scrolling")
	}
}

// The caret is only pulled back into view when it moves; a scroll set by the
// application must survive repaints and then yield to the next caret move.
func TestCodeEditorSetScrollYYieldsToCaretMoves(t *testing.T) {
	f := newCodeFixture(numberedLines(100))
	f.paint()
	f.editor.SetScrollY(f.editor.MaxScrollY())
	f.paint()
	f.paint()
	if f.editor.ScrollY() != f.editor.MaxScrollY() {
		t.Fatal("repainting moved a scroll the application set")
	}
	f.snapshot.Caret = 1 // the caret leaves the view, as typing at the top would
	f.paint()
	if f.editor.ScrollY() != 0 {
		t.Errorf("a caret move did not bring the caret back: scroll %v", f.editor.ScrollY())
	}
}

func TestCodeEditorOffsetAndYAreInverse(t *testing.T) {
	value := numberedLines(30) + "中文行,带有汉字\nlast"
	f := newCodeFixture(value)
	f.snapshot.Wrap = true
	if _, ok := f.editor.YAtOffset(0); ok {
		t.Fatal("a position before the first paint")
	}
	f.paint()
	_, _, line := f.cell()
	previous := float32(-1)
	for offset := 0; offset <= len(value); offset++ {
		if offset < len(value) && !utf8.RuneStart(value[offset]) {
			continue
		}
		y, ok := f.editor.YAtOffset(offset)
		if !ok || y < previous {
			t.Fatalf("offset %d: y=%v after %v", offset, y, previous)
		}
		previous = y
		back, _ := f.editor.OffsetAtY(y)
		// The end of a row is the bottom of it, which is also the top of the next
		// row; every other offset comes back to itself or to an earlier cell.
		atRowEnd := near(y, float32(int(y/line+0.5))*line)
		if back > offset && !atRowEnd {
			t.Fatalf("offset %d maps to y=%v which maps back to %d", offset, y, back)
		}
		if again, _ := f.editor.YAtOffset(back); !atRowEnd && (again > y+0.01 || y-again >= line) {
			t.Fatalf("offset %d: round trip left the row (%v -> %d -> %v)", offset, y, back, again)
		}
	}
	if offset, _ := f.editor.OffsetAtY(-3); offset != 0 {
		t.Errorf("above the text: %d", offset)
	}
	if offset, _ := f.editor.OffsetAtY(1e6); offset != len(value) {
		t.Errorf("below the text: %d", offset)
	}
}

// A logical line that soft-wraps takes several rows, and a later line sits
// below all of them: this is what makes row positions differ from line numbers.
func TestCodeEditorYAtOffsetCountsWrappedRows(t *testing.T) {
	long := strings.Repeat("abcdefghij", 20)
	value := long + "\nnext"
	f := newCodeFixture(value)
	f.bounds.Width = f.widthFor(30)
	f.snapshot.Wrap = true
	f.paint()
	_, _, line := f.cell()
	rows := len(f.editor.rows)
	if rows < 4 {
		t.Fatalf("expected the long line to wrap, got %d rows", rows)
	}
	y, _ := f.editor.YAtOffset(strings.Index(value, "next"))
	if !near(y, float32(rows-1)*line) {
		t.Errorf("the line after a wrapped one is at %v, want row %d (%v)", y, rows-1, float32(rows-1)*line)
	}
}

func BenchmarkCodeEditorYAtOffset(b *testing.B) {
	value := numberedLines(5000)
	f := newCodeFixture(value)
	f.paint()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		y, _ := f.editor.YAtOffset(len(value) / 2)
		f.editor.OffsetAtY(y)
		f.editor.SetScrollY(y)
	}
}
