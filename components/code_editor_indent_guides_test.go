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
	// One guide per complete level: 4 cells hold one, 8 cells two, and an
	// incomplete level (2 cells) none.
	if seen[0] != 1 || seen[1] != 2 || seen[2] != 0 {
		t.Fatalf("guides for 4, 8 and 2 leading cells = %v", seen)
	}
	for _, mark := range marks {
		columns := (mark.X - fixture.editor.content.X) / fixture.editor.cellWidth
		if remainder := columns - float32(int(columns/4+0.5))*4; !near(remainder, 0) {
			t.Fatalf("guide not on a tab stop: %+v", mark)
		}
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
	visibleLevels := int(fixture.editor.content.Width/fixture.editor.cellWidth)/4 + 2
	if fixture.editor.scrollX <= 0 || len(marks) == 0 || len(marks) > visibleLevels*len(fixture.editor.rows) {
		t.Fatalf("unbounded or absent scrolled guides: scroll=%v marks=%d", fixture.editor.scrollX, len(marks))
	}
	for _, rectangle := range fixture.canvas.Rectangles() {
		if rectangle.Color == guide && (!rectangle.HasClip || rectangle.ClipOf.X < fixture.editor.content.X) {
			t.Fatalf("guide escaped text clip: %+v", rectangle)
		}
	}
}

func TestCodeEditorIndentGuidesFollowTabWidthAndVerticalScroll(t *testing.T) {
	fixture := newCodeFixture(strings.Repeat("\t\tchild\n", 48) + "\t\tlast")
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
	spacing := func(marks []ui.Rect) float32 {
		for _, mark := range marks[1:] {
			if mark.Y == marks[0].Y {
				return mark.X - marks[0].X
			}
		}
		return 0
	}
	if got, want := spacing(first), 4*fixture.editor.cellWidth; !near(got, want) {
		t.Fatalf("guide spacing at tab width 4 = %v, want %v", got, want)
	}
	fixture.snapshot.TabWidth = 8
	fixture.paint()
	second := fixture.fills(guide)
	if len(second) == 0 {
		t.Fatal("no guides after the tab width changed")
	}
	if got, want := spacing(second), 8*fixture.editor.cellWidth; !near(got, want) {
		t.Fatalf("tab-width change kept stale guide geometry: spacing %v, want %v", got, want)
	}
}

func TestCodeEditorIndentGuidesNeverReachTheFirstTextColumn(t *testing.T) {
	fixture := newCodeFixture("a\n    b\n        c\n\t\td\n   e")
	fixture.snapshot.TabWidth = 4
	fixture.snapshot.InvisibleColor = ui.RGB(48, 96, 144)
	fixture.snapshot.ShowIndentGuides = true
	fixture.paint()
	guide := fixture.snapshot.InvisibleColor
	guide.Alpha *= 0.5
	indents := []int{0, 4, 8, 8, 3}
	for _, mark := range fixture.fills(guide) {
		line := int((mark.Y - fixture.editor.content.Y) / fixture.editor.lineHeight)
		column := (mark.X - fixture.editor.content.X) / fixture.editor.cellWidth
		if column+0.01 >= float32(indents[line]) {
			t.Fatalf("guide at column %v reaches the text of line %d, indented %d", column, line, indents[line])
		}
	}
}

func BenchmarkCodeEditorPaintIndentGuides(b *testing.B) {
	source := strings.Repeat("func f() {\n\tif x {\n\t\tcall()\n\n\t}\n}\n", 5000)
	for _, guides := range []bool{false, true} {
		name := "off"
		if guides {
			name = "on"
		}
		b.Run(name, func(b *testing.B) {
			f := newCodeFixture(source)
			f.bounds = ui.Rect{Width: 900, Height: 600}
			f.snapshot.TabWidth = 4
			f.snapshot.ShowIndentGuides = guides
			f.snapshot.InvisibleColor = ui.RGB(48, 96, 144)
			f.paint()
			b.ReportAllocs()
			for b.Loop() {
				f.paint()
			}
		})
	}
}
