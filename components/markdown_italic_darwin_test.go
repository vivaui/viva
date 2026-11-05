//go:build darwin

package components

import (
	"math"
	"testing"

	"github.com/vivaui/viva/internal/platform/darwin"
	"github.com/vivaui/viva/markdown"
	"github.com/vivaui/viva/ui"
)

func TestMarkdownItalicPieceUsesCoreTextItalicAdvance(t *testing.T) {
	var document markdown.Document
	document.Parse("plain *Italic affine* end")
	var canvas ui.Canvas
	canvas.SetTextEngine(darwin.SharedTextEngine())
	var view MarkdownView
	view.Layout(&canvas, MarkdownStyle{Size: 16, LineHeight: 24}, &document, 500)
	for _, piece := range view.pieces {
		if piece.text == "Italic affine" {
			want := canvas.TextWidthStyle(piece.text, ui.TextStyle{Size: piece.size, Weight: ui.WeightRegular, Italic: true})
			if math.Abs(float64(piece.w-want)) > 0.5 {
				t.Fatalf("italic piece width=%v, CoreText=%v", piece.w, want)
			}
			return
		}
	}
	t.Fatal("italic piece missing")
}
