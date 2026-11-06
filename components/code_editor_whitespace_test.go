package components

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/ui"
)

func TestCodeEditorVisibleSpaceTabAndEOLDoNotChangeGeometry(t *testing.T) {
	fixture := newCodeFixture("a \tb\nc")
	marker := ui.RGB(33, 104, 171)
	fixture.snapshot.InvisibleColor = marker
	fixture.paint()
	startX, startY := fixture.editor.content.X, fixture.editor.content.Y
	rows := strings.Join(fixture.rowTexts(), "|")
	if len(fixture.fills(marker)) != 0 {
		t.Fatal("markers painted before visibility was enabled")
	}
	fixture.snapshot.ShowWhitespace, fixture.snapshot.ShowEOL, fixture.snapshot.EOLLabel = true, true, "CRLF"
	fixture.paint()
	if len(fixture.fills(marker)) < 2 || fixture.editor.content.X != startX || fixture.editor.content.Y != startY ||
		strings.Join(fixture.rowTexts(), "|") != rows || fixture.snapshot.Value != "a \tb\nc" {
		t.Fatalf("invisible marks changed text geometry or were absent: %+v", fixture.fills(marker))
	}
	count := 0
	for _, label := range fixture.canvas.Texts() {
		if label.Value == "CRLF" {
			count++
			if label.Bounds.X < fixture.editor.content.X+4*fixture.editor.cellWidth {
				t.Fatalf("EOL marker overlaps visible code: %+v", label.Bounds)
			}
		}
	}
	if count != 1 {
		t.Fatalf("EOL marker count = %d, want only the hard line break", count)
	}
}

func TestCodeEditorEOLMarkersOnlyAppearOnHardWraps(t *testing.T) {
	fixture := newCodeFixture("one two three\nlast")
	fixture.bounds.Width = fixture.widthFor(5)
	fixture.snapshot.Wrap, fixture.snapshot.ShowEOL, fixture.snapshot.EOLLabel = true, true, "LF"
	fixture.paint()
	if len(fixture.editor.rows) < 3 {
		t.Fatal("fixture did not soft-wrap")
	}
	count := 0
	for _, label := range fixture.canvas.Texts() {
		if label.Value == "LF" {
			count++
			if label.Bounds.Y < fixture.editor.content.Y+fixture.editor.lineHeight {
				t.Fatalf("soft-wrap row received EOL marker: %+v", label.Bounds)
			}
		}
	}
	if count != 1 {
		t.Fatalf("hard EOL count = %d", count)
	}
}

func TestCodeEditorWhitespaceMarkersStayClippedWhenHorizontallyScrolled(t *testing.T) {
	fixture := newCodeFixture("a b c d e f g h")
	fixture.bounds.Width = fixture.widthFor(4)
	fixture.snapshot.InvisibleColor = ui.RGB(33, 104, 171)
	fixture.snapshot.ShowWhitespace = true
	fixture.snapshot.Caret = len(fixture.snapshot.Value)
	fixture.paint()
	if fixture.editor.scrollX <= 0 || len(fixture.fills(fixture.snapshot.InvisibleColor)) == 0 {
		t.Fatal("fixture did not scroll to visible whitespace")
	}
	for _, rectangle := range fixture.canvas.Rectangles() {
		if rectangle.Color == fixture.snapshot.InvisibleColor && (!rectangle.HasClip || rectangle.ClipOf.X < fixture.editor.content.X) {
			t.Fatalf("whitespace marker escaped code clip: %+v", rectangle)
		}
	}
}

func TestCodeEditorEOLLabelsCanVaryPerLogicalLine(t *testing.T) {
	fixture := newCodeFixture("a\nb\nc\nd")
	fixture.snapshot.ShowEOL = true
	fixture.snapshot.EOLLabel = "CRLF"
	fixture.snapshot.EOLLabels = []string{"CRLF", "LF", "CR"}
	fixture.paint()
	got := make([]string, 0, 3)
	for _, label := range fixture.canvas.Texts() {
		if label.Value == "CRLF" || label.Value == "LF" || label.Value == "CR" {
			got = append(got, label.Value)
		}
	}
	if strings.Join(got, ",") != "CRLF,LF,CR" {
		t.Fatalf("logical EOL markers = %v", got)
	}
}

func TestCodeEditorWhitespaceMarkersStopAtTheVisibleColumns(t *testing.T) {
	// One minified line: walking it end to end would be megabytes per paint,
	// for markers that are all off screen.
	fixture := newCodeFixture(strings.Repeat("a ", 20000))
	marker := ui.RGB(33, 104, 171)
	fixture.snapshot.InvisibleColor = marker
	fixture.snapshot.ShowWhitespace = true
	fixture.paint()
	visible := len(fixture.fills(marker))
	if visible == 0 {
		t.Fatal("no whitespace marker inside the viewport")
	}
	// The viewport holds a few dozen cells; a marker count anywhere near the
	// line's length means the scan did not stop at the visible columns.
	if visible > 200 {
		t.Fatalf("painted %d markers on a line of %d spaces", visible, 20000)
	}
}
