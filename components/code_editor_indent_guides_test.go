package components

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/ui"
)

func TestCodeEditorIndentGuidesFollowExpandedLeadingCellsWithoutMovingText(t *testing.T) {
	fixture := newCodeFixture("    one\n\t\ttwo\n  three")
	fixture.snapshot.TabWidth = 4
	fixture.snapshot.InvisibleColor = ui.RGB(48, 96, 144)
	fixture.paint()
	startX, startY := fixture.editor.content.X, fixture.editor.content.Y
	rows := strings.Join(fixture.rowTexts(), "|")
	fixture.snapshot.ShowIndentGuides = true
	fixture.paint()
	guide := fixture.snapshot.InvisibleColor
	guide.Alpha *= 0.5
	marks := fixture.fills(guide)
	if len(marks) == 0 || fixture.editor.content.X != startX || fixture.editor.content.Y != startY ||
		strings.Join(fixture.rowTexts(), "|") != rows || fixture.snapshot.Value != "    one\n\t\ttwo\n  three" {
		t.Fatalf("indent guides changed geometry or did not paint: %+v", marks)
	}
	seen := map[int]int{}
	for _, mark := range marks {
		line := int((mark.Y - fixture.editor.content.Y) / fixture.editor.lineHeight)
		seen[line]++
		if mark.X < fixture.editor.content.X || mark.X+mark.Width > fixture.editor.content.X+fixture.editor.content.Width || !near(mark.Width, 1) {
			t.Fatalf("guide outside content clip: %+v", mark)
		}
	}
	if seen[0] == 0 || seen[1] <= seen[0] || seen[2] != 0 {
		t.Fatalf("guides for 4, 8 and 2 leading cells = %v", seen)
	}
	fixture.snapshot.ShowIndentGuides = false
	fixture.paint()
	if len(fixture.fills(guide)) != 0 {
		t.Fatal("disabled indent guides still painted")
	}
}

func TestCodeEditorBlankIndentLooksBothWaysAndWrappedRowsDoNotRepeatGuides(t *testing.T) {
	fixture := newCodeFixture("    parent\n\n        child whose text wraps across rows\nnext")
	fixture.bounds.Width = fixture.widthFor(12)
	fixture.bounds.Height = 520
	fixture.snapshot.TabWidth = 4
	fixture.snapshot.Wrap = true
	fixture.snapshot.InvisibleColor = ui.RGB(48, 96, 144)
	fixture.snapshot.ShowIndentGuides = true
	fixture.paint()
	guide := fixture.snapshot.InvisibleColor
	guide.Alpha *= 0.5
	marks := fixture.fills(guide)
	if len(fixture.editor.rows) < 5 {
		t.Fatalf("fixture did not soft-wrap: %d rows", len(fixture.editor.rows))
	}
	counts := map[int]int{}
	for _, mark := range marks {
		row := int((mark.Y - fixture.editor.content.Y) / fixture.editor.lineHeight)
		counts[row]++
	}
	if counts[1] == 0 || counts[2] <= counts[1] {
		t.Fatalf("blank line did not inherit minimum adjacent indentation: %v", counts)
	}
	for index, row := range fixture.editor.rows {
		if row.line == 2 && !fixture.editor.firstOfLine(index) && counts[index] != 0 {
			t.Fatalf("soft-wrap continuation has invented indent guides: row %d, %v", index, counts)
		}
	}
	fixture.snapshot.FoldedLines = []CodeEditorFoldRange{{Header: 0, LastLine: 2}}
	fixture.paint()
	if len(fixture.editor.rows) != 2 {
		t.Fatalf("folded guide rows = %d, want header and next line", len(fixture.editor.rows))
	}
	for _, mark := range fixture.fills(guide) {
		if mark.Y >= fixture.editor.content.Y+fixture.editor.lineHeight {
			t.Fatalf("folded indentation leaked into next visible row: %+v", mark)
		}
	}
}

func TestCodeEditorIndentGuidesBoundWorkToScrolledViewport(t *testing.T) {
	fixture := newCodeFixture(strings.Repeat(" ", 50000) + "x")
	fixture.bounds.Width = fixture.widthFor(6)
	fixture.snapshot.TabWidth = 4
	fixture.snapshot.Caret = len(fixture.snapshot.Value)
	fixture.snapshot.InvisibleColor = ui.RGB(48, 96, 144)
	fixture.snapshot.ShowIndentGuides = true
	fixture.paint()
	guide := fixture.snapshot.InvisibleColor
	guide.Alpha *= 0.5
	marks := fixture.fills(guide)
	if fixture.editor.scrollX <= 0 || len(marks) == 0 || len(marks) > 30 {
		t.Fatalf("unbounded or absent scrolled guides: scroll=%v marks=%d", fixture.editor.scrollX, len(marks))
	}
	for _, rectangle := range fixture.canvas.Rectangles() {
		if rectangle.Color == guide && (!rectangle.HasClip || rectangle.ClipOf.X < fixture.editor.content.X) {
			t.Fatalf("guide escaped text clip: %+v", rectangle)
		}
	}
}

func TestCodeEditorIndentGuidesFollowTabWidthAndVerticalScroll(t *testing.T) {
	fixture := newCodeFixture(strings.Repeat("\tchild\n", 48) + "\tlast")
	fixture.bounds.Height = 90
	fixture.snapshot.Caret = len(fixture.snapshot.Value)
	fixture.snapshot.ShowIndentGuides = true
	fixture.snapshot.InvisibleColor = ui.RGB(48, 96, 144)
	guide := fixture.snapshot.InvisibleColor
	guide.Alpha *= 0.5
	fixture.snapshot.TabWidth = 4
	fixture.paint()
	first := fixture.fills(guide)
	if fixture.editor.scrollY <= 0 || len(first) == 0 {
		t.Fatal("vertical scrolling did not show indented rows")
	}
	for _, rectangle := range fixture.canvas.Rectangles() {
		if rectangle.Color == guide && (!rectangle.HasClip || rectangle.ClipOf.Y < fixture.editor.content.Y || rectangle.ClipOf.Y+rectangle.ClipOf.Height > fixture.editor.content.Y+fixture.editor.content.Height) {
			t.Fatalf("scrolled guide escaped content: %+v", rectangle)
		}
	}
	x := first[0].X
	fixture.snapshot.TabWidth = 8
	fixture.paint()
	second := fixture.fills(guide)
	if len(second) == 0 || second[0].X <= x+3*fixture.editor.cellWidth {
		t.Fatalf("tab-width change kept stale guide geometry: before=%v after=%+v", x, second)
	}
}
