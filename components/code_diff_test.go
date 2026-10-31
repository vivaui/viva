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
		if caret, shown := diff.CaretBounds(); !shown || caret.Width != 1 {
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
	wantText := []string{"pha", "middle", "om"}
	for index, rectangle := range selected {
		if rectangle.Width <= 0 || rectangle.Width >= diff.rightCode.Width {
			t.Errorf("line %d selection = %+v, right code width = %v", index, rectangle, diff.rightCode.Width)
		}
		wantWidth := canvas.TextWidthWeight(wantText[index], theme.Metrics.FontSizeM, ui.WeightRegular)
		if delta := rectangle.Width - wantWidth; delta < -.5 || delta > .5 {
			t.Errorf("line %d selection width = %v, want %v", index, rectangle.Width, wantWidth)
		}
		if rectangle.Height != diff.rowHeight-2 || rectangle.Y != diff.rightCode.Y+float32(index)*diff.rowHeight+1 {
			t.Errorf("line %d selection bounds = %+v, row height = %v", index, rectangle, diff.rowHeight)
		}
	}

	selectedForeground := make(map[string]bool)
	for _, text := range canvas.Texts() {
		if text.Style.Color == selectionForeground {
			selectedForeground[text.Value] = true
		}
	}
	for _, line := range []string{"alpha", "middle", "omega"} {
		if !selectedForeground[line] {
			t.Errorf("selected line %q was not repainted with SelectionForeground", line)
		}
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

func TestCodeDiffSyntaxLabelsKeepTheShapedEditorFont(t *testing.T) {
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
	for _, text := range canvas.Texts() {
		if text.Bounds.X < diff.rightCode.X || !strings.Contains(text.Value, "<") {
			continue
		}
		// CodeEditor and CodeDiff both split highlighted lines into clipped
		// labels. Those clips use shaped-font advances; requesting the grid
		// path without a cell width makes the last glyph disappear on Windows.
		if text.Style.Mono || text.CellWidth != 0 {
			t.Fatalf("syntax label uses an unconfigured fixed-pitch grid: %+v", text)
		}
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
	for _, text := range canvas.Texts() {
		if text.Value != line {
			continue
		}
		rightEdge := text.Bounds.X + canvas.TextWidthWeight(line, theme.Metrics.FontSizeM, ui.WeightRegular)
		visibleRight := diff.rightCode.X + diff.rightCode.Width - codeDiffPadX
		if text.ClipOf.X >= diff.rightCode.X && rightEdge >= visibleRight-.5 {
			foundRightEnd = true
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
