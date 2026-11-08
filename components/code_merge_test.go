package components

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/ui"
)

func TestCodeMergePaintsThreeAlignedSourcesAndEditsResult(t *testing.T) {
	var canvas ui.Canvas
	var merge CodeMerge
	result := "header\nchoice\ntail"
	snapshot := CodeMergeSnapshot{
		Current:  strings.Split("header\nlocal\ntail", "\n"),
		Incoming: strings.Split("header\nremote\ntail", "\n"),
		Rows: []CodeMergeRow{
			{CurrentLine: 0, ResultLine: 0, IncomingLine: 0},
			{CurrentLine: 1, ResultLine: 1, IncomingLine: 1, CurrentChanged: true, IncomingChanged: true},
			{CurrentLine: 2, ResultLine: 2, IncomingLine: 2},
		},
		Editor:           CodeDiffSnapshot{ID: ui.Item(15), Label: "Result", Value: result, Focused: true, ShowCaret: true, Caret: len("header\nchoice")},
		CurrentActionID:  ui.Item(16),
		IncomingActionID: ui.Item(17),
		Actions:          []CodeMergeAction{{Row: 1, Incoming: true, ID: ui.Item(18), Label: "Accept"}},
	}
	merge.Paint(&canvas, ui.DarkTheme(), ui.Rect{Width: 900, Height: 190}, snapshot)
	for _, word := range []string{"local", "choice", "remote"} {
		found := false
		for _, label := range canvas.Texts() {
			found = found || strings.Contains(label.Value, word)
		}
		if !found {
			t.Fatalf("source %q was not painted", word)
		}
	}
	if _, shown := merge.CaretBounds(); !shown {
		t.Fatal("result caret was not painted")
	}
	for _, action := range []struct {
		region   ui.Rect
		incoming bool
	}{
		{merge.currentAction, false}, {merge.incomingAction, true},
	} {
		row, incoming, ok := merge.ActionAt(ui.Point{X: action.region.X + 1, Y: action.region.Y + merge.rowHeight*1.5})
		if !ok || row != 1 || incoming != action.incoming {
			t.Fatalf("action at %+v: row=%d incoming=%t ok=%t", action.region, row, incoming, ok)
		}
	}
	for _, hit := range canvas.Hits() {
		if hit.ID == ui.Item(18) && (hit.Role != ui.RoleButton || hit.Bounds.Height > merge.rowHeight) {
			t.Fatalf("merge action is not row-sized: %+v", hit)
		}
		if hit.ID == snapshot.Editor.ID {
			if hit.Value != result || hit.Placement != &merge || !hit.Multiline {
				t.Fatalf("editable result = %+v", hit)
			}
			return
		}
	}
	t.Fatal("result had no native text input region")
}

func TestCodeMergeFollowsCallerAlignmentAfterResultAddsALine(t *testing.T) {
	var canvas ui.Canvas
	var merge CodeMerge
	lines := strings.Split("head\ntail", "\n")
	snapshot := CodeMergeSnapshot{
		Current: lines, Incoming: lines,
		Rows:             []CodeMergeRow{{CurrentLine: 0, ResultLine: 0, IncomingLine: 0}, {CurrentLine: 1, ResultLine: 1, IncomingLine: 1}},
		Editor:           CodeDiffSnapshot{ID: ui.Item(20), Value: "head\ntail", Focused: true, ShowCaret: true},
		IncomingActionID: ui.Item(21),
	}
	paint := func() {
		canvas.Reset()
		merge.Paint(&canvas, ui.DarkTheme(), ui.Rect{Width: 700, Height: 210}, snapshot)
	}
	paint()
	snapshot.Editor.Value = "head\ninserted\ntail"
	snapshot.Rows = []CodeMergeRow{
		{CurrentLine: 0, ResultLine: 0, IncomingLine: 0},
		{CurrentLine: -1, ResultLine: 1, IncomingLine: -1},
		{CurrentLine: 1, ResultLine: 2, IncomingLine: 1},
	}
	snapshot.Editor.Caret = len("head\ninserted")
	paint()
	point := ui.Point{X: merge.rightCode.X + 1, Y: merge.rightCode.Y + merge.rowHeight*1.5}
	if offset, ok := merge.OffsetAt(point); !ok || offset != len("head\n") {
		t.Fatalf("result input mapped to offset %d, ok=%t", offset, ok)
	}
	point.X = merge.incomingAction.X + 1
	if row, line, incoming, ok := merge.GutterAt(point); !ok || row != 1 || line != -1 || !incoming {
		t.Fatalf("empty incoming row = %d %d %t %t", row, line, incoming, ok)
	}
	if _, shown := merge.CaretBounds(); !shown {
		t.Fatal("new result line lost its caret")
	}
}

func TestCodeMergeKeepsAllThreePanesAlignedWhenScrolled(t *testing.T) {
	var current, result, incoming []string
	var rows []CodeMergeRow
	for index := range 80 {
		current = append(current, "current "+strings.Repeat("x", index+1))
		result = append(result, "result "+strings.Repeat("x", index+1))
		incoming = append(incoming, "incoming "+strings.Repeat("x", index+1))
		rows = append(rows, CodeMergeRow{CurrentLine: index, ResultLine: index, IncomingLine: index})
	}
	snapshot := CodeMergeSnapshot{Current: current, Incoming: incoming, Rows: rows,
		Editor: CodeDiffSnapshot{ID: ui.Item(20), Value: strings.Join(result, "\n")}}
	var canvas ui.Canvas
	var merge CodeMerge
	paint := func() {
		canvas.Reset()
		merge.Paint(&canvas, ui.DarkTheme(), ui.Rect{Width: 900, Height: 220}, snapshot)
	}
	paint()
	if merge.Geometry.MaxOffsetY <= 0 {
		t.Fatal("the comparison cannot scroll")
	}
	merge.ScrollTo(0, merge.rowHeight*20)
	paint()
	for _, pane := range []struct {
		point ui.Point
		want  int
	}{
		{ui.Point{X: merge.currentAction.X + 1, Y: merge.currentAction.Y + merge.rowHeight/2}, 20},
		{ui.Point{X: merge.incomingAction.X + 1, Y: merge.incomingAction.Y + merge.rowHeight/2}, 20},
	} {
		row, _, _, ok := merge.GutterAt(pane.point)
		if !ok || row != pane.want {
			t.Fatalf("source pane row = %d, ok=%t", row, ok)
		}
	}
	if offset, ok := merge.OffsetAt(ui.Point{X: merge.rightCode.X + 1, Y: merge.rightCode.Y + merge.rowHeight/2}); !ok || offset != len(strings.Join(result[:20], "\n"))+1 {
		t.Fatalf("scrolled result offset = %d, ok=%t", offset, ok)
	}
}

func BenchmarkCodeMergePaint(b *testing.B) {
	var canvas ui.Canvas
	var merge CodeMerge
	lines := strings.Split(strings.Repeat("unchanged line\n", 100), "\n")
	rows := make([]CodeMergeRow, len(lines))
	for index := range rows {
		rows[index] = CodeMergeRow{CurrentLine: index, ResultLine: index, IncomingLine: index}
	}
	snapshot := CodeMergeSnapshot{Current: lines, Incoming: lines, Rows: rows,
		Editor: CodeDiffSnapshot{ID: ui.Item(15), Value: strings.Join(lines, "\n")}}
	theme := ui.DarkTheme()
	bounds := ui.Rect{Width: 900, Height: 190}
	merge.Paint(&canvas, theme, bounds, snapshot)
	b.ReportAllocs()
	for range b.N {
		canvas.Reset()
		merge.Paint(&canvas, theme, bounds, snapshot)
	}
}
