package components

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
)

func wrapSymbols(fixture *codeFixture) []gfx.Text {
	var marks []gfx.Text
	for _, label := range fixture.canvas.Texts() {
		if label.Value == "↪" {
			marks = append(marks, label)
		}
	}
	return marks
}

func TestCodeEditorWrapSymbolMarksOnlySoftRowEndsAndReservesSpace(t *testing.T) {
	fixture := newCodeFixture("abcdefghij\nlast")
	fixture.bounds.Width = fixture.widthFor(5)
	fixture.snapshot.Wrap = true
	fixture.snapshot.InvisibleColor = gfx.RGB(48, 96, 144)
	fixture.paint()
	without := len(fixture.editor.rows)
	if len(wrapSymbols(fixture)) != 0 {
		t.Fatal("wrap marks painted while disabled")
	}
	fixture.snapshot.ShowWrapSymbol = true
	fixture.paint()
	want := 0
	for row := range fixture.editor.rows {
		if fixture.editor.softWrapped(row) {
			want++
		}
	}
	marks := wrapSymbols(fixture)
	if want < 2 || len(marks) != want || len(fixture.editor.rows) <= without || fixture.snapshot.Value != "abcdefghij\nlast" {
		t.Fatalf("wrap marks=%d want=%d rows=%d before=%d", len(marks), want, len(fixture.editor.rows), without)
	}
	for _, mark := range marks {
		if mark.Bounds.X < fixture.editor.content.X+fixture.editor.content.Width-fixture.editor.cellWidth ||
			mark.Bounds.X+mark.Bounds.Width > fixture.editor.content.X+fixture.editor.content.Width ||
			!mark.HasClip || mark.ClipOf.X < fixture.editor.content.X {
			t.Fatalf("wrap mark overlaps text or leaves clip: %+v", mark)
		}
		for _, label := range fixture.canvas.Texts() {
			if label.Value != "↪" && label.Bounds.Y == mark.Bounds.Y && label.Bounds.X+label.Bounds.Width > mark.Bounds.X {
				t.Fatalf("row text overlaps wrap mark: text=%+v mark=%+v", label.Bounds, mark.Bounds)
			}
		}
	}
	fixture.snapshot.ShowWrapSymbol = false
	fixture.paint()
	if len(wrapSymbols(fixture)) != 0 || len(fixture.editor.rows) != without {
		t.Fatal("turning wrap symbols off did not restore the normal layout")
	}
}

func TestCodeEditorWrapSymbolIgnoresHardBreaksAndUnwrappedText(t *testing.T) {
	fixture := newCodeFixture("short\nlast\n")
	fixture.snapshot.Wrap, fixture.snapshot.ShowWrapSymbol = true, true
	fixture.paint()
	if len(wrapSymbols(fixture)) != 0 {
		t.Fatal("hard EOL was marked as a soft wrap")
	}
	fixture.snapshot.Value = "a long line that wraps at a narrow width"
	fixture.bounds.Width = fixture.widthFor(6)
	fixture.snapshot.Wrap = false
	fixture.snapshot.Caret = len(fixture.snapshot.Value)
	fixture.paint()
	if len(wrapSymbols(fixture)) != 0 || fixture.editor.scrollX <= 0 {
		t.Fatal("disabled word wrap showed a wrap symbol or lost horizontal scrolling")
	}
	fixture.snapshot.Wrap = true
	fixture.paint()
	if len(wrapSymbols(fixture)) == 0 {
		t.Fatal("re-enabling word wrap did not restore wrap symbols")
	}
}

func TestCodeEditorWrapSymbolSkipsFoldedRowsAndClipsVerticalScroll(t *testing.T) {
	fixture := newCodeFixture("header\n" + strings.Repeat("hidden child text\n", 10) + strings.Repeat("visible wrapped text\n", 40))
	fixture.bounds.Width = fixture.widthFor(8)
	fixture.bounds.Height = 96
	fixture.snapshot.Wrap, fixture.snapshot.ShowWrapSymbol = true, true
	fixture.snapshot.FoldedLines = []CodeEditorFoldRange{{Header: 0, LastLine: 10}}
	fixture.snapshot.Caret = len(fixture.snapshot.Value)
	fixture.paint()
	if fixture.editor.scrollY <= 0 || len(wrapSymbols(fixture)) == 0 {
		t.Fatal("the fixture did not show scrolled wrapped rows")
	}
	for _, row := range fixture.editor.rows {
		if row.line > 0 && row.line <= 10 {
			t.Fatalf("folded row %d remained visible", row.line)
		}
	}
	for _, mark := range wrapSymbols(fixture) {
		if !mark.HasClip || mark.ClipOf.Y < fixture.editor.content.Y || mark.ClipOf.Y+mark.ClipOf.Height > fixture.editor.content.Y+fixture.editor.content.Height {
			t.Fatalf("wrap marker escaped scrolled content: %+v", mark)
		}
	}
}
