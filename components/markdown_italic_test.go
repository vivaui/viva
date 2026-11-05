package components

import (
	"testing"

	"github.com/vivaui/viva/markdown"
	"github.com/vivaui/viva/ui"
)

func TestMarkdownEmphasisCarriesItalicStyleWithoutChangingBold(t *testing.T) {
	var document markdown.Document
	document.Parse("plain *italic* **bold**")
	var canvas ui.Canvas
	var view MarkdownView
	style := MarkdownStyle{Size: 16, LineHeight: 24}
	view.Layout(&canvas, style, &document, 500)
	view.Paint(&canvas, MarkdownPaint{Look: MarkdownLookFor(ui.LightTheme()), Clip: ui.Rect{Width: 500, Height: 100}})
	italic, bold := false, false
	for _, label := range canvas.Texts() {
		if label.Value == "italic" {
			italic = label.Style.Italic && label.Style.Weight == ui.WeightRegular
		}
		if label.Value == "bold" {
			bold = !label.Style.Italic && label.Style.Weight >= ui.WeightSemibold
		}
	}
	if !italic || !bold {
		t.Fatalf("emphasis/bold styles = %+v", canvas.Texts())
	}
}
