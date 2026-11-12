package components

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

func TestCodeEditorMarkLayersUseIndependentColorsBelowFindAndSelection(t *testing.T) {
	fixture := newCodeFixture("cat cat\ncat")
	cyan, orange := gfx.RGB(0, 255, 255), gfx.RGB(255, 128, 0)
	fixture.snapshot.MarkLayers = []CodeEditorMarkLayer{
		{Spans: []text.Span{{Start: 0, End: 3}, {Start: 8, End: 11}}, Color: cyan},
		{Spans: []text.Span{{Start: 1, End: 2}, {Start: 4, End: 7}}, Color: orange},
	}
	fixture.snapshot.Marks = []text.Span{{Start: 0, End: 3}}
	fixture.snapshot.SelectionStart, fixture.snapshot.SelectionEnd = 0, 2
	fixture.paint()
	cell, _, line := fixture.cell()
	content := fixture.editor.content
	if fills := fixture.fills(cyan); len(fills) != 2 || !near(fills[0].X, content.X) || !near(fills[0].Width, 3*cell) || !near(fills[1].Y, content.Y+line) {
		t.Fatalf("cyan layer fills = %+v", fills)
	}
	if fills := fixture.fills(orange); len(fills) != 2 || !near(fills[0].X, content.X+cell) || !near(fills[0].Width, cell) {
		t.Fatalf("orange layer fills = %+v", fills)
	}
	firstCyan, firstOrange, firstFind, firstSelection := -1, -1, -1, -1
	for index, rectangle := range fixture.canvas.Rectangles() {
		switch rectangle.Color {
		case cyan:
			if firstCyan < 0 {
				firstCyan = index
			}
		case orange:
			if firstOrange < 0 {
				firstOrange = index
			}
		case codeMatchColor:
			firstFind = index
		case fixture.theme.Selection:
			firstSelection = index
		}
	}
	if firstCyan < 0 || firstOrange <= firstCyan || firstFind <= firstOrange || firstSelection <= firstFind {
		t.Fatalf("paint order cyan/orange/find/selection = %d/%d/%d/%d", firstCyan, firstOrange, firstFind, firstSelection)
	}
}

func TestCodeEditorMarkLayersShareWrappedUnicodeAndTabGeometry(t *testing.T) {
	value := "\t猫ab 猫\nlast"
	fixture := newCodeFixture(value)
	fixture.snapshot.Wrap = true
	fixture.bounds.Width = fixture.widthFor(5)
	color := gfx.RGB(100, 200, 70)
	fixture.snapshot.MarkLayers = []CodeEditorMarkLayer{{Spans: []text.Span{{Start: 0, End: len(value)}}, Color: color}}
	fixture.paint()
	fills := fixture.fills(color)
	if len(fills) != len(fixture.editor.rows) || len(fills) < 3 {
		t.Fatalf("wrapped layer fills = %+v for %d rows", fills, len(fixture.editor.rows))
	}
	for _, fill := range fills {
		if fill.X < fixture.editor.content.X || fill.X+fill.Width > fixture.editor.content.X+fixture.editor.content.Width+0.01 {
			t.Fatalf("mark escaped text clip: %+v in %+v", fill, fixture.editor.content)
		}
	}
	if fixture.snapshot.Value != value {
		t.Fatal("mark layer changed source text")
	}
}

func TestCodeEditorMarkLayersOnlyPaintVisibleRanges(t *testing.T) {
	fixture := newCodeFixture(strings.Repeat("abc\n", 5000))
	color := gfx.RGB(80, 130, 240)
	spans := make([]text.Span, 5000)
	for index := range spans {
		spans[index] = text.Span{Start: index * 4, End: index*4 + 3}
	}
	fixture.snapshot.MarkLayers = []CodeEditorMarkLayer{{Spans: spans, Color: color}}
	fixture.snapshot.Caret = 4 * 4000
	fixture.paint()
	if fills := fixture.fills(color); len(fills) == 0 || len(fills) > 20 {
		t.Fatalf("painted %d colored marks for a small scrolled viewport", len(fills))
	}
}
