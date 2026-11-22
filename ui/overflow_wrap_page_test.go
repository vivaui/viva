package ui_test

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/ui"
)

// A page paragraph given OverflowWrapAnywhere asks for the room of the column it is
// centred in rather than for its longest line, so a word no line is wide enough for is
// broken and every line stays inside the column. Without it the same paragraph is one
// long line that reaches past both edges, which is the behaviour it has always had.
func TestPageParagraphBreaksAnywhereInsideItsColumn(t *testing.T) {
	const value = "https://example.com/a/b/c/d/e/f/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const width = 120
	draw := func(wrap func(b ui.Box) ui.Box) (lines []string, widest float32, start float32) {
		view := func(c ui.C) {
			ui.Column(c).Width(width).Items(ui.Center).Children(func() {
				wrap(ui.Wrap(c, value).FontSize(14))
			})
		}
		page := ui.NewPage(ui.PageConfig{View: view})
		canvas := benchCanvas()
		drawPage(page, canvas, view)
		start = width // the left edge of the leftmost line
		for _, text := range canvas.Texts() {
			lines = append(lines, text.Value)
			widest = max(widest, text.Bounds.Width)
			start = min(start, text.Bounds.X)
		}
		if strings.Join(lines, "") != value {
			t.Errorf("the paragraph painted %q, want the URL back whole", lines)
		}
		return lines, widest, start
	}

	plain, plainWidest, plainStart := draw(func(b ui.Box) ui.Box { return b })
	if len(plain) != 1 || plainWidest <= width || plainStart >= 0 {
		t.Errorf("without OverflowWrap the URL painted in %d lines up to %v wide from %v, want one line that runs out over the column", len(plain), plainWidest, plainStart)
	}

	anywhere, anywhereWidest, anywhereStart := draw(func(b ui.Box) ui.Box {
		return b.OverflowWrap(ui.OverflowWrapAnywhere)
	})
	if len(anywhere) < 2 {
		t.Errorf("with OverflowWrapAnywhere the URL painted in %d lines, want it broken into the column", len(anywhere))
	}
	if anywhereWidest > width || anywhereStart < 0 {
		t.Errorf("with OverflowWrapAnywhere the widest line is %v wide from %v, want every line inside the %v wide column", anywhereWidest, anywhereStart, float32(width))
	}
}
