package components

import (
	"reflect"
	"testing"

	"github.com/vivaui/viva/ui"
)

func TestCodeEditorFoldedRowsKeepSourceOffsetsAndLogicalNumbers(t *testing.T) {
	value := "root {\n  alpha\n  beta\n}\ntail"
	fixture := newCodeFixture(value)
	fixture.snapshot.FoldedLines = []CodeEditorFoldRange{{Header: 0, LastLine: 2}}
	fixture.snapshot.LineNumbers = true
	fixture.paint()
	if got := fixture.rowTexts(); !reflect.DeepEqual(got, []string{"root {", "}", "tail"}) {
		t.Fatalf("folded rows = %q", got)
	}
	if got := []int32{fixture.editor.rows[0].line, fixture.editor.rows[1].line, fixture.editor.rows[2].line}; !reflect.DeepEqual(got, []int32{0, 3, 4}) {
		t.Fatalf("logical line indexes = %v", got)
	}
	second := fixture.editor.rows[1]
	if got := fixture.editor.Move(value, int(fixture.editor.rows[0].end), TextareaMoveDown); got != int(second.end) {
		t.Fatalf("down across fold = %d, want %d", got, second.end)
	}
	_, _, line := fixture.cell()
	point := ui.Point{X: fixture.editor.content.X + 1, Y: fixture.editor.content.Y + line + 2}
	if got, ok := fixture.editor.OffsetAt(point); !ok || got != int(second.start) {
		t.Fatalf("folded pointer placement = %d, %v", got, ok)
	}
	if y, ok := fixture.editor.YAtOffset(int(fixture.editor.rows[2].start)); !ok || y != 2*line {
		t.Fatalf("tail offset Y = %v, %v", y, ok)
	}
	if got, ok := fixture.editor.OffsetAtY(2 * line); !ok || got != int(fixture.editor.rows[2].start) {
		t.Fatalf("tail Y offset = %d, %v", got, ok)
	}
	if fixture.snapshot.Value != value {
		t.Fatal("folding changed caller text")
	}
}

func TestCodeEditorFoldedWrapAndScrollbarUseOnlyVisibleRows(t *testing.T) {
	value := "abcdefghijabcdefghij\ninside\ninside too\nafter\nlast"
	fixture := newCodeFixture(value)
	fixture.bounds.Width = fixture.widthFor(10) + codeEditorBarSize
	fixture.bounds.Height = 60
	fixture.snapshot.Wrap = true
	fixture.snapshot.ScrollbarID = ui.Item(7)
	fixture.snapshot.FoldedLines = []CodeEditorFoldRange{{Header: 0, LastLine: 2}}
	fixture.paint()
	if got := fixture.rowTexts(); !reflect.DeepEqual(got, []string{"abcdefghij", "abcdefghij", "after", "last"}) {
		t.Fatalf("wrapped fold rows = %q", got)
	}
	if fixture.editor.barV.MaxOffsetY <= 0 || fixture.editor.barV.MaxOffsetY >= 4*fixture.editor.lineHeight {
		t.Fatalf("folded scrollbar extent = %v", fixture.editor.barV.MaxOffsetY)
	}
	fixture.snapshot.FoldedLines = nil
	fixture.paint()
	if len(fixture.editor.rows) != 6 {
		t.Fatalf("expanding fold retained hidden rows: %d", len(fixture.editor.rows))
	}
}

func TestCodeEditorFoldMarkersExposeOnlyVisibleGutterButtons(t *testing.T) {
	fixture := newCodeFixture("head\nhidden\nvisible")
	marker := ui.RGB(32, 88, 161)
	fixture.snapshot.MarkerGutter = true
	fixture.snapshot.GutterMarkers = []CodeEditorGutterMarker{{Line: 0, Color: marker}}
	fixture.snapshot.FoldGutter = true
	fixture.snapshot.FoldedLines = []CodeEditorFoldRange{{Header: 0, LastLine: 1}}
	fixture.snapshot.FoldMarkers = []CodeEditorFoldMarker{
		{Line: 0, ID: ui.Item(20), Collapsed: true},
		{Line: 1, ID: ui.Item(21)},
		{Line: 2, ID: ui.Item(22)},
	}
	fixture.paint()
	hits := map[ui.ID]ui.Hit{}
	for _, hit := range fixture.canvas.Hits() {
		hits[hit.ID] = hit
	}
	if _, hidden := hits[ui.Item(21)]; hidden {
		t.Fatal("fold marker for a hidden logical line was hit-testable")
	}
	for _, id := range []ui.ID{ui.Item(20), ui.Item(22)} {
		hit, ok := hits[id]
		if !ok || hit.Role != ui.RoleButton || hit.Bounds.Width < 12 || hit.Label == "" {
			t.Fatalf("fold marker %v = %+v, %v", id, hit, ok)
		}
	}
	if markers := fixture.fills(marker); len(markers) != 1 || hits[ui.Item(20)].Bounds.X >= markers[0].X {
		t.Fatalf("fold and bookmark gutter lanes overlap: fold=%+v bookmark=%+v", hits[ui.Item(20)].Bounds, markers)
	}
}

func TestCodeEditorFoldedSteadyPaintKeepsItsRowCache(t *testing.T) {
	fixture := newCodeFixture("head\ninside\nafter")
	fixture.snapshot.FoldedLines = []CodeEditorFoldRange{{Header: 0, LastLine: 1}}
	fixture.paint()
	if allocations := testing.AllocsPerRun(100, fixture.paint); allocations != 0 {
		t.Fatalf("steady folded paint allocated %g times", allocations)
	}
}

func TestCodeEditorFoldRangesIgnoreInvalidAndOverlappingEntries(t *testing.T) {
	fixture := newCodeFixture("head\ninside\nafter\nlast")
	fixture.snapshot.FoldedLines = []CodeEditorFoldRange{
		{Header: -1, LastLine: 2},
		{Header: 0, LastLine: 1},
		{Header: 0, LastLine: 2},
		{Header: 2, LastLine: 99},
	}
	fixture.paint()
	if got := fixture.rowTexts(); !reflect.DeepEqual(got, []string{"head", "after", "last"}) {
		t.Fatalf("invalid ranges hid the wrong lines: %q", got)
	}
	if allocations := testing.AllocsPerRun(50, fixture.paint); allocations != 0 {
		t.Fatalf("unchanged invalid fold input allocated %g times", allocations)
	}
}

func TestCodeEditorFoldMarkerSteadyPaintDoesNotAllocate(t *testing.T) {
	fixture := newCodeFixture("head\ninside\nafter")
	fixture.snapshot.FoldMarkers = []CodeEditorFoldMarker{{Line: 0, ID: ui.Item(20), Collapsed: true}}
	fixture.paint()
	if allocations := testing.AllocsPerRun(50, fixture.paint); allocations != 0 {
		t.Fatalf("steady fold-marker paint allocated %g times", allocations)
	}
}

func TestCodeEditorHiddenOffsetTakesTheFoldHeaderRow(t *testing.T) {
	value := "head\naa\nbb\ncc\ntail"
	fixture := newCodeFixture(value)
	fixture.snapshot.FoldedLines = []CodeEditorFoldRange{{Header: 0, LastLine: 3}}
	fixture.paint()
	// Offset 6 is inside "bb", which the fold hides. It stands for the header row,
	// so it reports that row's top instead of sliding onto the row below it.
	if y, ok := fixture.editor.YAtOffset(6); !ok || y != 0 {
		t.Fatalf("hidden offset y = %v, ok = %v, want the header row top 0", y, ok)
	}
	// A visible offset still reports its own row.
	want := fixture.editor.lineHeight
	if got, ok := fixture.editor.YAtOffset(int(fixture.editor.rows[1].start)); !ok || got != want {
		t.Fatalf("visible offset y = %v, ok = %v, want %v", got, ok, want)
	}
}
