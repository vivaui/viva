package components

import (
	"fmt"
	"strings"
	"testing"

	"github.com/vivaui/viva/ui"
)

func TestDiffCodeLinesPreservesAlignmentAndTrailingLines(t *testing.T) {
	tests := []struct {
		name  string
		left  string
		right string
		want  []string
		stats CodeDiffStats
	}{
		{
			name: "unchanged", left: "one\ntwo", right: "one\ntwo",
			want: []string{"=0:0", "=1:1"},
		},
		{
			name: "insert", left: "one\nthree", right: "one\ntwo\nthree",
			want:  []string{"=0:0", "+-1:1", "=1:2"},
			stats: CodeDiffStats{Added: 1},
		},
		{
			name: "remove", left: "one\ntwo\nthree", right: "one\nthree",
			want:  []string{"=0:0", "-1:-1", "=2:1"},
			stats: CodeDiffStats{Removed: 1},
		},
		{
			name: "change", left: "one\nold\nthree", right: "one\nnew\nthree",
			want:  []string{"=0:0", "~1:1", "=2:2"},
			stats: CodeDiffStats{Added: 1, Removed: 1},
		},
		{
			name: "repeated", left: "a\nsame\nb\nsame\nc", right: "a\nsame\nc",
			want:  []string{"=0:0", "=1:1", "-2:-1", "-3:-1", "=4:2"},
			stats: CodeDiffStats{Removed: 2},
		},
		{
			name: "trailing newline", left: "one\n", right: "one",
			want:  []string{"=0:0", "-1:-1"},
			stats: CodeDiffStats{Removed: 1},
		},
		{
			name: "empty", left: "", right: "",
			want: []string{"=0:0"},
		},
		{
			name: "unicode", left: "标题\n说明", right: "标题\n新说明",
			want:  []string{"=0:0", "~1:1"},
			stats: CodeDiffStats{Added: 1, Removed: 1},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := DiffCodeLines(test.left, test.right)
			if got := summarizeDiffRows(result.Rows); !slicesEqual(got, test.want) {
				t.Fatalf("rows = %v, want %v", got, test.want)
			}
			if result.Stats != test.stats {
				t.Fatalf("stats = %+v, want %+v", result.Stats, test.stats)
			}
			for index := 0; index < len(result.Rows); index++ {
				row := result.Rows[index]
				if row.LeftLine < -1 || row.LeftLine >= len(result.Left) || row.RightLine < -1 || row.RightLine >= len(result.Right) {
					t.Fatalf("row %d is outside its source: %+v", index, row)
				}
			}
		})
	}
}

func TestDiffCodeLinesCoarsensAtWorkBound(t *testing.T) {
	result := diffCodeLines(strings.Repeat("left\n", 20), strings.Repeat("right\n", 20), 12)
	if !result.Stats.Coarse {
		t.Fatal("bounded comparison did not report coarse output")
	}
	if len(result.Rows) == 0 {
		t.Fatal("bounded comparison produced no rows")
	}
	for _, row := range result.Rows {
		if row.Kind == CodeDiffRemoved || row.Kind == CodeDiffAdded || row.Kind == CodeDiffChanged {
			return
		}
	}
	t.Fatal("bounded comparison hid every change")
}

func TestDiffCodeLinesNormalizesIdenticalLinesInsideAnUnanchoredHunk(t *testing.T) {
	left := "head\nthe same\nold\nthe same\nolder\ntail\n"
	right := "head\nthe same\nnew\nthe same\nnewer\ntail\n"
	result := DiffCodeLines(left, right)
	want := []string{"=0:0", "=1:1", "~2:2", "=3:3", "~4:4", "=5:5", "=6:6"}
	if got := summarizeDiffRows(result.Rows); !slicesEqual(got, want) {
		t.Fatalf("rows = %v, want %v", got, want)
	}
	if result.Stats != (CodeDiffStats{Added: 2, Removed: 2}) {
		t.Fatalf("stats = %+v", result.Stats)
	}
}

func TestCodeDiffPaintsEditableRightSourceWithSyntaxAndPlacement(t *testing.T) {
	theme := ui.DarkTheme()
	left := "key: old\n# keep\nend\n"
	right := "key: 新值\n# keep\nadded\nend\n"
	result := DiffCodeLines(left, right)
	var canvas ui.Canvas
	var diff CodeDiff
	snapshot := CodeDiffSnapshot{
		Result: result, ID: ui.Item(10), Label: "Edited file", Value: right,
		Language: "yaml", Focused: true, ShowCaret: true,
		Caret: len("key: 新值"), SelectionStart: 0, SelectionEnd: 3,
		CompositionStart: len("key: "), CompositionEnd: len("key: 新值"),
		VerticalScrollbarID: ui.Item(11), HorizontalScrollbarID: ui.Item(12),
		Highlight: func(_, line string, dst []CodeEditorRun) []CodeEditorRun {
			if strings.HasPrefix(line, "key:") {
				return append(dst, CodeEditorRun{Start: 0, End: 3, Color: theme.Chart[0]})
			}
			return dst
		},
	}
	paint := func() {
		canvas.Reset()
		diff.Paint(&canvas, theme, ui.Rect{X: 4, Y: 5, Width: 640, Height: 220}, snapshot)
	}
	paint()
	for _, hit := range canvas.Hits() {
		if hit.ID != snapshot.ID {
			continue
		}
		if hit.Value != right || !hit.Multiline || hit.ReadOnly || hit.TextColor.Alpha != 0 || !hit.HideNativeOverlays || hit.Placement != &diff {
			t.Fatalf("editable diff hit = %+v", hit)
		}
		offset, ok := diff.OffsetAt(ui.Point{X: hit.Bounds.X + 12, Y: hit.Bounds.Y + 4})
		if !ok || offset < 0 || offset > len(right) {
			t.Fatalf("placement = %d, %t", offset, ok)
		}
		if caret, shown := diff.CaretBounds(); !shown || caret.Width <= 1 {
			t.Fatalf("caret = %+v, %t", caret, shown)
		}
		goto found
	}
	t.Fatal("editable right hit not painted")
found:
	if len(canvas.Texts()) == 0 {
		t.Fatal("diff painted no source lines")
	}
	foundRemoved, foundAdded := false, false
	for _, rectangle := range canvas.Rectangles() {
		if rectangle.Color.Alpha > 0 && rectangle.Color.Alpha < theme.Danger.Alpha {
			if rectangle.Color.Red == theme.Danger.Red && rectangle.Color.Green == theme.Danger.Green && rectangle.Color.Blue == theme.Danger.Blue {
				foundRemoved = true
			}
			if rectangle.Color.Red == theme.Success.Red && rectangle.Color.Green == theme.Success.Green && rectangle.Color.Blue == theme.Success.Blue {
				foundAdded = true
			}
		}
	}
	if !foundRemoved || !foundAdded {
		t.Fatalf("missing changed backgrounds: removed=%t added=%t", foundRemoved, foundAdded)
	}

	paint()
	if allocations := testing.AllocsPerRun(100, paint); allocations > 8 {
		t.Fatalf("steady code diff allocations = %v", allocations)
	}
}

func TestCodeDiffPaintsOneSelectionPerLineAndRebuildsSelectedText(t *testing.T) {
	theme := ui.DarkTheme()
	value := "alpha\nmiddle\nomega\n"
	start := len("al")
	end := strings.LastIndex(value, "omega") + len("om")
	var canvas ui.Canvas
	var diff CodeDiff
	snapshot := CodeDiffSnapshot{
		Result: DiffCodeLines(value, value), ID: ui.Item(41), Label: "Selected file", Value: value,
		Focused: true, SelectionStart: start, SelectionEnd: end,
	}
	diff.Paint(&canvas, theme, ui.Rect{Width: 620, Height: 180}, snapshot)

	selectionBackground, selectionForeground := theme.Selection, theme.SelectionForeground
	var selected []ui.Rect
	for _, rectangle := range canvas.Rectangles() {
		if rectangle.Color == selectionBackground {
			selected = append(selected, rectangle.Bounds)
		}
	}
	if len(selected) != 3 {
		t.Fatalf("selection rectangles = %+v, want one per selected line", selected)
	}
	wantCells := []int{len("pha"), len("middle"), len("om")}
	cell, _ := canvas.MonoCellSize(theme.Metrics.FontSizeM, ui.MonoSystem)
	for index, rectangle := range selected {
		if rectangle.Width <= 0 || rectangle.Width >= diff.rightCode.Width {
			t.Errorf("line %d selection = %+v, right code width = %v", index, rectangle, diff.rightCode.Width)
		}
		// A selection that runs on to the next line also covers half a cell for the line break.
		want := float32(wantCells[index]) * cell
		if index < 2 {
			want += cell / 2
		}
		if delta := rectangle.Width - want; delta < -.01 || delta > .01 {
			t.Errorf("line %d selection width = %v, want %v", index, rectangle.Width, want)
		}
		if rectangle.Height != diff.rowHeight-2 || rectangle.Y != diff.rightCode.Y+float32(index)*diff.rowHeight+1 {
			t.Errorf("line %d selection bounds = %+v, row height = %v", index, rectangle, diff.rowHeight)
		}
	}

	// The selected characters, and only those, are drawn in the selection's foreground.
	selectedCells := 0
	for _, text := range canvas.Texts() {
		for _, run := range canvas.TextRuns(text) {
			if run.Color == selectionForeground {
				selectedCells += int(run.Cells)
			}
		}
	}
	if want := wantCells[0] + wantCells[1] + wantCells[2]; selectedCells != want {
		t.Errorf("%d cells drawn in SelectionForeground, want %d", selectedCells, want)
	}
}

func TestCodeDiffDoesNotSyntaxColourUnchangedRows(t *testing.T) {
	theme := ui.DarkTheme()
	left := "keep: yes\nold: 1\n"
	right := "keep: yes\nnew: 1\n"
	var highlightedUnchanged bool
	snapshot := CodeDiffSnapshot{
		Result: DiffCodeLines(left, right), Value: right,
		Highlight: func(_, line string, dst []CodeEditorRun) []CodeEditorRun {
			if strings.HasPrefix(line, "keep:") {
				highlightedUnchanged = true
			}
			return append(dst, CodeEditorRun{Start: 0, End: len("keep:"), Color: ui.RGB(255, 191, 26)})
		},
	}
	var canvas ui.Canvas
	var diff CodeDiff
	diff.Paint(&canvas, theme, ui.Rect{Width: 640, Height: 180}, snapshot)
	if highlightedUnchanged {
		t.Fatal("an unchanged row was passed through the syntax highlighter")
	}
}

func TestCodeDiffPaintsEachRowAsOneGridLabelWhateverTheHighlight(t *testing.T) {
	theme := ui.DarkTheme()
	left := "<revision>41</revision>\n<log-level>info</log-level>\n"
	right := "<revision>42</revision>\n<log-level>warning</log-level>\n"
	snapshot := CodeDiffSnapshot{
		Result: DiffCodeLines(left, right), Value: right,
		Highlight: func(_, line string, dst []CodeEditorRun) []CodeEditorRun {
			for start := 0; start < len(line); {
				open := strings.IndexByte(line[start:], '<')
				if open < 0 {
					break
				}
				open += start
				close := strings.IndexByte(line[open:], '>')
				if close < 0 {
					break
				}
				close += open + 1
				dst = append(dst, CodeEditorRun{Start: open, End: close, Color: theme.Chart[2]})
				start = close
			}
			return dst
		},
	}
	var canvas ui.Canvas
	var diff CodeDiff
	diff.Paint(&canvas, theme, ui.Rect{Width: 1000, Height: 300}, snapshot)
	cell, _ := canvas.MonoCellSize(theme.Metrics.FontSizeM, ui.MonoSystem)
	labels := 0
	for _, text := range canvas.Texts() {
		if !strings.Contains(text.Value, "<") {
			continue
		}
		labels++
		if !text.Style.Mono || text.CellWidth != cell || text.RunCount == 0 {
			t.Fatalf("source label is not a grid line: %+v", text)
		}
		bytes := 0
		for _, run := range canvas.TextRuns(text) {
			bytes += int(run.Bytes)
		}
		if bytes != len(text.Value) {
			t.Fatalf("runs cover %d of %d bytes of %q", bytes, len(text.Value), text.Value)
		}
	}
	if labels != 4 {
		t.Fatalf("%d source labels, want one per row and side (4)", labels)
	}
	var tagged int
	for _, text := range canvas.Texts() {
		for _, run := range canvas.TextRuns(text) {
			if run.Color == theme.Chart[2] {
				tagged++
			}
		}
	}
	if tagged == 0 {
		t.Fatal("no run took the highlight color")
	}
}

func TestCodeDiffKeepsThePreviousAlignmentWhileTheEditableTextChanges(t *testing.T) {
	theme := ui.DarkTheme()
	left := "old value\nstable\n"
	right := "new value\nstable\n"
	snapshot := CodeDiffSnapshot{
		Result: DiffCodeLines(left, right),
		Value:  "brand new value\nstable\n",
	}
	var canvas ui.Canvas
	var diff CodeDiff
	diff.Paint(&canvas, theme, ui.Rect{Width: 620, Height: 180}, snapshot)
	var removed, added bool
	for _, rectangle := range canvas.Rectangles() {
		if rectangle.Color.Alpha <= 0 || rectangle.Color.Alpha >= theme.Danger.Alpha {
			continue
		}
		removed = removed || rectangle.Color.Red == theme.Danger.Red &&
			rectangle.Color.Green == theme.Danger.Green && rectangle.Color.Blue == theme.Danger.Blue
		added = added || rectangle.Color.Red == theme.Success.Red &&
			rectangle.Color.Green == theme.Success.Green && rectangle.Color.Blue == theme.Success.Blue
	}
	if !removed || !added {
		t.Fatalf("editing current text discarded the previous alignment: removed=%t added=%t", removed, added)
	}
}

func TestCodeDiffScrollsBothColumns(t *testing.T) {
	theme := ui.LightTheme()
	left := strings.Repeat("old line"+strings.Repeat("x", 500)+"\n", 100)
	right := strings.Repeat("new line"+strings.Repeat("y", 500)+"\n", 100)
	result := DiffCodeLines(left, right)
	var canvas ui.Canvas
	var diff CodeDiff
	snapshot := CodeDiffSnapshot{
		Result: result, ID: ui.Item(20), Label: "Changed file", Value: right,
		Focused: true, Caret: 0,
		VerticalScrollbarID: ui.Item(21), HorizontalScrollbarID: ui.Item(22),
	}
	diff.Paint(&canvas, theme, ui.Rect{Width: 500, Height: 120}, snapshot)
	if diff.Geometry.MaxOffsetY <= 0 {
		t.Fatal("vertical scrollbar missing")
	}
	resultScroll := diff.Scroll(ui.ScrollEvent{DeltaY: 240})
	if !resultScroll.Handled || diff.scrollY <= 0 {
		t.Fatalf("vertical scroll = %+v offset=%v", resultScroll, diff.scrollY)
	}
	if _, handled := diff.ScrollForKey(ui.KeyRight, false); !handled || diff.scrollX <= 0 {
		t.Fatalf("horizontal key scroll = %t offset=%v", handled, diff.scrollX)
	}
	canvas.Reset()
	diff.Paint(&canvas, theme, ui.Rect{Width: 500, Height: 120}, snapshot)
	if diff.Geometry.OffsetY <= 0 {
		t.Fatal("paint discarded scroll position")
	}
	diff.ScrollTo(0, 0)
	diff.RevealCaret(right, len(right)-1)
	if diff.scrollY <= 0 {
		t.Fatal("keyboard caret reveal did not scroll a distant line into view")
	}
}

func TestCodeDiffLongLinesRemainReachableAndScrollbarDragsTrackThePointer(t *testing.T) {
	theme := ui.LightTheme()
	line := strings.Repeat("long-value-", 90)
	left := strings.Repeat("stable\n", 30) + line + "\n"
	right := strings.Repeat("stable\n", 30) + line + "\n"
	snapshot := CodeDiffSnapshot{
		Result: DiffCodeLines(left, right), ID: ui.Item(30), Label: "Long file", Value: right,
		VerticalScrollbarID: ui.Item(31), HorizontalScrollbarID: ui.Item(32),
	}
	var canvas ui.Canvas
	var diff CodeDiff
	bounds := ui.Rect{X: 10, Y: 20, Width: 500, Height: 180}
	diff.Paint(&canvas, theme, bounds, snapshot)
	if diff.Geometry.MaxOffsetX <= 0 || diff.Geometry.MaxOffsetY <= 0 {
		t.Fatalf("missing scroll extents: %+v", diff.Geometry)
	}
	wantY := float32(len(snapshot.Result.Rows))*diff.rowHeight - diff.Geometry.Viewport.Height
	if diff.Geometry.MaxOffsetY != wantY {
		t.Fatalf("vertical max = %v, want %v", diff.Geometry.MaxOffsetY, wantY)
	}

	vertical := diff.Geometry.VerticalThumb
	down := diff.Pointer(ui.PointerEvent{
		Kind: ui.PointerDown, Target: snapshot.VerticalScrollbarID,
		X: vertical.X + vertical.Width/2, Y: vertical.Y + vertical.Height/2,
	})
	if !down.Handled || diff.scrollY != 0 {
		t.Fatalf("vertical thumb press = %+v offset=%v", down, diff.scrollY)
	}
	before := diff.scrollY
	first := diff.Pointer(ui.PointerEvent{
		Kind: ui.PointerMove, Captured: true, Target: snapshot.VerticalScrollbarID,
		X: vertical.X, Y: vertical.Y + vertical.Height/2 + 24,
	})
	second := diff.Pointer(ui.PointerEvent{
		Kind: ui.PointerMove, Captured: true, Target: snapshot.VerticalScrollbarID,
		X: vertical.X, Y: vertical.Y + vertical.Height/2 + 48,
	})
	if !first.Handled || !second.Handled || diff.scrollY <= before {
		t.Fatalf("captured vertical drag did not follow downward motion: before=%v after=%v", before, diff.scrollY)
	}
	if up := diff.Pointer(ui.PointerEvent{Kind: ui.PointerUp, Captured: true, Target: snapshot.VerticalScrollbarID}); !up.Handled || diff.verticalDrag {
		t.Fatalf("vertical drag did not release: %+v dragging=%t", up, diff.verticalDrag)
	}

	diff.ScrollTo(0, 0)
	horizontal := diff.Geometry.HorizontalThumb
	if result := diff.Pointer(ui.PointerEvent{
		Kind: ui.PointerDown, Target: snapshot.HorizontalScrollbarID,
		X: horizontal.X + horizontal.Width/2, Y: horizontal.Y + horizontal.Height/2,
	}); !result.Handled || diff.scrollX != 0 {
		t.Fatalf("horizontal thumb press = %+v offset=%v", result, diff.scrollX)
	}
	beforeX := diff.scrollX
	if result := diff.Pointer(ui.PointerEvent{
		Kind: ui.PointerMove, Captured: true, Target: snapshot.HorizontalScrollbarID,
		X: horizontal.X + horizontal.Width/2 + 40, Y: horizontal.Y + horizontal.Height/2,
	}); !result.Handled || diff.scrollX <= beforeX {
		t.Fatalf("captured horizontal drag did not follow motion: before=%v after=%v", beforeX, diff.scrollX)
	}

	diff.ScrollTo(diff.Geometry.MaxOffsetX, diff.Geometry.MaxOffsetY)
	canvas.Reset()
	diff.Paint(&canvas, theme, bounds, snapshot)
	foundRightEnd := false
	cell, _ := canvas.MonoCellSize(theme.Metrics.FontSizeM, ui.MonoSystem)
	for _, text := range canvas.Texts() {
		// The label of the right pane, which may start a few cells left of the view.
		if text.RunCount == 0 || !strings.HasSuffix(line, text.Value) || text.Bounds.X+text.Bounds.Width <= diff.rightCode.X {
			continue
		}
		// The label holds only the columns in view, so its last cell is the last of the line.
		rightEdge := text.Bounds.X + float32(len(text.Value))*cell
		visibleRight := diff.rightCode.X + diff.rightCode.Width - codeDiffPadX
		if rightEdge >= visibleRight-.5 && rightEdge <= visibleRight+cell {
			foundRightEnd = true
		}
		if len(text.Value) > int(diff.rightCode.Width/cell)+8 {
			t.Fatalf("a %d column label for a %v wide view", len(text.Value), diff.rightCode.Width)
		}
	}
	if !foundRightEnd {
		t.Fatal("a long line could not be shown through to its trailing edge")
	}
}

func BenchmarkDiffCodeLines(b *testing.B) {
	left := strings.Repeat("key: old value\n# comment\nnext: true\n", 15000)
	right := strings.Repeat("key: new value\n# comment\nnext: true\n", 15000)
	b.ReportAllocs()
	for b.Loop() {
		_ = DiffCodeLines(left, right)
	}
}

func BenchmarkDiffCodeLinesOneMiBShortLines(b *testing.B) {
	line := strings.Repeat("a", 19) + "\n"
	left := strings.Repeat(line, (1<<20)/len(line))
	right := strings.Repeat(strings.Repeat("b", 19)+"\n", (1<<20)/len(line))
	b.SetBytes(int64(len(left) + len(right)))
	b.ReportAllocs()
	for b.Loop() {
		_ = DiffCodeLines(left, right)
	}
}

func BenchmarkDiffCodeLinesAtCompareLimits(b *testing.B) {
	left := strings.Repeat("service: old value: 000\n", 20_000)
	right := strings.ReplaceAll(left, "old", "new")
	b.SetBytes(int64(len(left) + len(right)))
	b.ReportAllocs()
	for b.Loop() {
		_ = DiffCodeLines(left, right)
	}
}

func BenchmarkDiffCodeLinesUniqueLinesAtCompareLimits(b *testing.B) {
	var left strings.Builder
	for index := range 20_000 {
		fmt.Fprintf(&left, "service-%05d: old\n", index)
	}
	leftValue := left.String()
	rightValue := strings.ReplaceAll(leftValue, ": old", ": new")
	b.SetBytes(int64(len(leftValue) + len(rightValue)))
	b.ReportAllocs()
	for b.Loop() {
		_ = DiffCodeLines(leftValue, rightValue)
	}
}

func BenchmarkDiffCodeLinesSingleEditAtLineLimit(b *testing.B) {
	var left strings.Builder
	for index := range 20_000 {
		fmt.Fprintf(&left, "service-%05d: old\n", index)
	}
	leftValue := left.String()
	rightValue := strings.Replace(leftValue, "service-10000: old", "service-10000: new", 1)
	b.SetBytes(int64(len(leftValue) + len(rightValue)))
	b.ReportAllocs()
	for b.Loop() {
		_ = DiffCodeLines(leftValue, rightValue)
	}
}

func BenchmarkDiffCodeLinesSingleEditInThreeHundred(b *testing.B) {
	const line = "service.port: 8080\n"
	left := strings.Repeat(line, 300)
	right := strings.Replace(left, line, "service.port: 9090\n", 1)
	b.SetBytes(int64(len(left) + len(right)))
	b.ReportAllocs()
	for b.Loop() {
		_ = DiffCodeLines(left, right)
	}
}

func BenchmarkDiffCodeLinesLongLine(b *testing.B) {
	left := strings.Repeat("a", 1<<20)
	right := strings.Repeat("b", 1<<20)
	b.SetBytes(int64(len(left) + len(right)))
	b.ReportAllocs()
	for b.Loop() {
		_ = DiffCodeLines(left, right)
	}
}

func BenchmarkCodeDiffPaintHalfMiBLine(b *testing.B) {
	theme := ui.LightTheme()
	left := strings.Repeat("a", 512<<10)
	right := strings.Repeat("b", 512<<10)
	snapshot := CodeDiffSnapshot{Result: DiffCodeLines(left, right), Value: right}
	bounds := ui.Rect{Width: 1000, Height: 300}
	var canvas ui.Canvas
	var diff CodeDiff
	b.ReportAllocs()
	for b.Loop() {
		canvas.Reset()
		diff.Paint(&canvas, theme, bounds, snapshot)
	}
}

func summarizeDiffRows(rows []CodeDiffRow) []string {
	result := make([]string, len(rows))
	for index, row := range rows {
		sign := "="
		switch row.Kind {
		case CodeDiffAdded:
			sign = "+"
		case CodeDiffRemoved:
			sign = "-"
		case CodeDiffChanged:
			sign = "~"
		}
		result[index] = fmt.Sprintf("%s%d:%d", sign, row.LeftLine, row.RightLine)
	}
	return result
}

func slicesEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

// The horizontal range follows the widest line of the file, wherever the view is, and a file
// whose lines fit gets no horizontal bar at all.
func TestCodeDiffHorizontalRangeCoversTheWholeFile(t *testing.T) {
	theme := ui.LightTheme()
	bounds := ui.Rect{X: 10, Y: 20, Width: 500, Height: 180}
	paint := func(diff *CodeDiff, text string) {
		var canvas ui.Canvas
		diff.Paint(&canvas, theme, bounds, CodeDiffSnapshot{
			Result: DiffCodeLines(text, text), ID: ui.Item(30), Label: "File", Value: text,
			VerticalScrollbarID: ui.Item(31), HorizontalScrollbarID: ui.Item(32),
		})
	}

	long := strings.Repeat("stable\n", 60) + strings.Repeat("long-value-", 90) + "\n" + strings.Repeat("stable\n", 60)
	var diff CodeDiff
	paint(&diff, long)
	atTop := diff.Geometry.MaxOffsetX
	if atTop <= 0 {
		t.Fatalf("a line far below the view is not reachable: %+v", diff.Geometry)
	}
	diff.ScrollForKey(ui.KeyEnd, false)
	diff.scrollX = atTop
	paint(&diff, long)
	if diff.Geometry.MaxOffsetX != atTop || diff.Geometry.OffsetX != atTop {
		t.Fatalf("scrolling down changed the horizontal range: %v then %v, offset %v", atTop, diff.Geometry.MaxOffsetX, diff.Geometry.OffsetX)
	}

	var short CodeDiff
	paint(&short, strings.Repeat("short\n", 100))
	if short.Geometry.MaxOffsetX != 0 || short.Geometry.HorizontalTrack != (ui.Rect{}) {
		t.Fatalf("lines that fit got a horizontal bar: %+v", short.Geometry)
	}
}

// A file whose lines all repeat has nothing unique to anchor on; a line added at the top and
// one removed at the bottom are still two changes, not a rewrite of every row.
func TestDiffCodeLinesAlignsRepeatedLinesByShortestEdits(t *testing.T) {
	lines := make([]string, 10000)
	for index := 0; index < len(lines); index += 2 {
		lines[index] = "}"
	}
	left := strings.Join(lines, "\n")
	right := strings.Join(append([]string{"}"}, lines[:len(lines)-1]...), "\n")
	result := DiffCodeLines(left, right)
	if result.Stats != (CodeDiffStats{Added: 1, Removed: 1}) {
		t.Fatalf("stats = %+v, want one line added and one removed", result.Stats)
	}
	changed := 0
	for _, row := range result.Rows {
		if row.Kind != CodeDiffUnchanged {
			changed++
		}
	}
	if changed != 2 {
		t.Fatalf("%d rows changed, want 2", changed)
	}
}

// Whatever the repetition, every line of both sources appears in order in the rows, and a row
// marked unchanged pairs equal lines.
func TestDiffCodeLinesRowsReplayBothSources(t *testing.T) {
	seed := uint32(7)
	next := func(n uint32) uint32 {
		seed = seed*1664525 + 1013904223
		return (seed >> 16) % n
	}
	build := func() string {
		var lines []string
		for range next(40) {
			lines = append(lines, string(rune('a'+next(3))))
		}
		return strings.Join(lines, "\n")
	}
	for range 300 {
		left, right := build(), build()
		result := DiffCodeLines(left, right)
		wantLeft, wantRight := 0, 0
		for _, row := range result.Rows {
			if row.LeftLine >= 0 {
				if row.LeftLine != wantLeft {
					t.Fatalf("%q vs %q: left line %d out of order", left, right, row.LeftLine)
				}
				wantLeft++
			}
			if row.RightLine >= 0 {
				if row.RightLine != wantRight {
					t.Fatalf("%q vs %q: right line %d out of order", left, right, row.RightLine)
				}
				wantRight++
			}
			if row.Kind == CodeDiffUnchanged && result.Left[row.LeftLine] != result.Right[row.RightLine] {
				t.Fatalf("%q vs %q: unchanged row pairs different lines", left, right)
			}
		}
		if wantLeft != len(result.Left) || wantRight != len(result.Right) {
			t.Fatalf("%q vs %q: rows hold %d and %d of %d and %d lines", left, right, wantLeft, wantRight, len(result.Left), len(result.Right))
		}
	}
}

func BenchmarkDiffCodeLinesRepeatedLinesAtLineLimit(b *testing.B) {
	lines := make([]string, 20000)
	for index := range lines {
		if index%2 == 0 {
			lines[index] = "}"
		}
	}
	left := strings.Join(lines, "\n")
	right := strings.Join(append([]string{"}"}, lines[:len(lines)-1]...), "\n")
	b.ReportAllocs()
	for b.Loop() {
		DiffCodeLines(left, right)
	}
}

func BenchmarkCodeDiffPaintLongFileWidestLineCached(b *testing.B) {
	text := strings.Repeat("stable line of ordinary length\n", 19999) + strings.Repeat("long-value-", 90)
	snapshot := CodeDiffSnapshot{Result: DiffCodeLines(text, text), ID: ui.Item(30), Label: "File", Value: text}
	var diff CodeDiff
	var canvas ui.Canvas
	theme := ui.LightTheme()
	b.ReportAllocs()
	for b.Loop() {
		canvas.Reset()
		diff.Paint(&canvas, theme, ui.Rect{Width: 500, Height: 180}, snapshot)
	}
}

func codeDiffFixture(left, right string) (*CodeDiff, *ui.Canvas, CodeDiffSnapshot) {
	var diff CodeDiff
	var canvas ui.Canvas
	snapshot := CodeDiffSnapshot{
		Result: DiffCodeLines(left, right), ID: ui.Item(50), Label: "Edited", Value: right,
		Focused: true, ShowCaret: true,
	}
	return &diff, &canvas, snapshot
}

func TestCodeDiffCaretDefaultsToEditorWidthWithoutOverridingHairline(t *testing.T) {
	diff, canvas, snapshot := codeDiffFixture("old\n", "new\n")
	paint := func() (ui.Rect, bool) {
		canvas.Reset()
		diff.Paint(canvas, ui.LightTheme(), ui.Rect{Width: 500, Height: 180}, snapshot)
		return diff.CaretBounds()
	}
	defaultCaret, ok := paint()
	if !ok || defaultCaret.Width <= 1 {
		t.Fatalf("default diff caret should not be a hairline: %+v, shown=%t", defaultCaret, ok)
	}
	snapshot.CaretWidth = 1
	hairline, ok := paint()
	if !ok || hairline.Width != 1 {
		t.Fatalf("explicit diff hairline should remain available: %+v, shown=%t", hairline, ok)
	}
}

func TestCodeDiffPlacesTheCaretAndPressesOnTheCellGrid(t *testing.T) {
	theme := ui.LightTheme()
	right := "ab\tc中d\nxyz\n"
	diff, canvas, snapshot := codeDiffFixture("old\nxyz\n", right)
	bounds := ui.Rect{X: 0, Y: 0, Width: 700, Height: 200}
	cell, _ := canvas.MonoCellSize(theme.Metrics.FontSizeM, ui.MonoSystem)
	for _, c := range []struct{ offset, column, line int }{{0, 0, 0}, {1, 1, 0}, {3, 4, 0}, {4, 5, 0}, {7, 7, 0}, {8, 8, 0}, {9, 0, 1}, {12, 3, 1}} {
		snapshot.Caret = c.offset
		canvas.Reset()
		diff.Paint(canvas, theme, bounds, snapshot)
		caret, ok := diff.CaretBounds()
		if !ok {
			t.Fatalf("offset %d: no caret", c.offset)
		}
		origin := diff.rightCode.X + codeDiffPadX - diff.Geometry.OffsetX
		if want := origin + float32(c.column)*cell; caret.X < want-.01 || caret.X > want+.01 {
			t.Errorf("offset %d: caret x %v, want %v", c.offset, caret.X, want)
		}
		if want := diff.rightCode.Y + float32(c.line)*diff.rowHeight; caret.Y != want {
			t.Errorf("offset %d: caret y %v, want %v", c.offset, caret.Y, want)
		}
		got, ok := diff.OffsetAt(ui.Point{X: caret.X + .01, Y: caret.Y + diff.rowHeight/2})
		if !ok || got != c.offset {
			t.Errorf("offset %d: a press on its caret gave %d, %v", c.offset, got, ok)
		}
	}
}

func TestCodeDiffMoveKeepsTheColumnAcrossShortLines(t *testing.T) {
	right := "abcdef\nxy\nabcdefgh\n"
	diff, canvas, snapshot := codeDiffFixture(right, right)
	diff.Paint(canvas, ui.LightTheme(), ui.Rect{Width: 700, Height: 200}, snapshot)
	caret := 5
	for _, want := range []int{9, 15, 9, 5} {
		down := want == 9 && caret == 5 || want == 15
		movement := TextareaMoveUp
		if down {
			movement = TextareaMoveDown
		}
		caret = diff.Move(right, caret, movement)
		if caret != want {
			t.Fatalf("caret %d, want %d", caret, want)
		}
	}
	// Home goes to the first character first, then to the start of the line.
	indented := "x\n    text here\n"
	diff2, canvas2, snapshot2 := codeDiffFixture(indented, indented)
	diff2.Paint(canvas2, ui.LightTheme(), ui.Rect{Width: 700, Height: 200}, snapshot2)
	caret = 12
	for _, want := range []int{6, 2, 6} {
		caret = diff2.Move(indented, caret, TextareaMoveLineStart)
		if caret != want {
			t.Fatalf("Home gave %d, want %d", caret, want)
		}
	}
}

func TestCodeDiffExpandsTabsAndCountsWideCharactersAsTwoCells(t *testing.T) {
	theme := ui.LightTheme()
	diff, canvas, snapshot := codeDiffFixture("a\tb\n", "a\tb 中\n")
	snapshot.TabWidth = 4
	diff.Paint(canvas, theme, ui.Rect{Width: 700, Height: 120}, snapshot)
	var shown []string
	for _, text := range canvas.Texts() {
		if text.RunCount > 0 && strings.Contains(text.Value, "b") {
			shown = append(shown, text.Value)
		}
	}
	if len(shown) != 2 || shown[0] != "a   b" || shown[1] != "a   b 中" {
		t.Fatalf("labels = %q, want the tab expanded to the next stop of 4", shown)
	}
	// a, the tab out to column 4, b, a space and a wide character of two cells.
	if want := float32(1 + 3 + 1 + 1 + 2); diff.lineWidth != want*diff.cellWidth {
		t.Fatalf("widest line = %v cells, want %v", diff.lineWidth/diff.cellWidth, want)
	}
}

func TestCodeDiffGutterGrowsWithTheLineNumbers(t *testing.T) {
	theme := ui.LightTheme()
	small, canvasA, snapshotA := codeDiffFixture("a\n", "b\n")
	small.Paint(canvasA, theme, ui.Rect{Width: 700, Height: 200}, snapshotA)
	text := strings.Repeat("line\n", 123456)
	large, canvasB, snapshotB := codeDiffFixture(text, text+"x")
	large.Paint(canvasB, theme, ui.Rect{Width: 700, Height: 200}, snapshotB)
	if small.gutter != codeDiffGutterWidth || large.gutter <= small.gutter {
		t.Fatalf("gutters %v and %v: a 6 digit count needs more room than the minimum", small.gutter, large.gutter)
	}
	cell, _ := canvasB.MonoCellSize(theme.Metrics.FontSizeS, ui.MonoSystem)
	if need := 17 + 6*cell + 6; large.gutter < need-.01 {
		t.Fatalf("gutter %v cannot hold six digits (%v)", large.gutter, need)
	}
}

func TestCodeDiffSteadyPaintAllocatesNothing(t *testing.T) {
	theme := ui.DarkTheme()
	left := strings.Repeat("key: old\n\n\tindented 中文\n", 300)
	right := strings.Repeat("key: new\n\n\tindented 中文\n", 300)
	diff, canvas, snapshot := codeDiffFixture(left, right)
	snapshot.VerticalScrollbarID, snapshot.HorizontalScrollbarID = ui.Item(51), ui.Item(52)
	snapshot.SelectionStart, snapshot.SelectionEnd, snapshot.Caret = 3, 40, 40
	snapshot.Highlight = func(_, line string, dst []CodeEditorRun) []CodeEditorRun {
		return append(dst, CodeEditorRun{End: 3, Color: ui.RGB(200, 20, 20)})
	}
	paint := func() {
		canvas.Reset()
		diff.Paint(canvas, theme, ui.Rect{Width: 800, Height: 300}, snapshot)
	}
	paint()
	if allocations := testing.AllocsPerRun(100, paint); allocations != 0 {
		t.Fatalf("steady code diff allocations = %v", allocations)
	}
}

// BenchmarkCodeDiffRelayoutOnEdit is the cost of a keystroke in a comparison of
// half a megabyte: the right source is split again, its lines measured for the
// scroll range and the next frame painted.
func BenchmarkCodeDiffRelayoutOnEdit(b *testing.B) {
	base := strings.Repeat("key: value # comment with some 中文 and a long tail to wrap around the edge of the box\n", 6000)
	changed := strings.Replace(base, "key", "kex", 1)
	theme := ui.LightTheme()
	diff, canvas, snapshot := codeDiffFixture(base, base)
	b.SetBytes(int64(len(base)))
	b.ReportAllocs()
	toggle := false
	for b.Loop() {
		toggle = !toggle
		if toggle {
			snapshot.Value = changed
		} else {
			snapshot.Value = base
		}
		canvas.Reset()
		diff.Paint(canvas, theme, ui.Rect{Width: 900, Height: 600}, snapshot)
	}
}
