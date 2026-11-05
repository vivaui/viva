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

func TestMarkdownItalicHitUsesStyledCoreTextAdvance(t *testing.T) {
	var document markdown.Document
	document.Parse("plain *Italic affine* end")
	var canvas ui.Canvas
	canvas.SetTextEngine(darwin.SharedTextEngine())
	var view MarkdownView
	view.Layout(&canvas, MarkdownStyle{Size: 20, LineHeight: 28}, &document, 500)
	view.Paint(&canvas, MarkdownPaint{Look: MarkdownLookFor(ui.LightTheme()), Clip: ui.Rect{Width: 500, Height: 80}})
	for _, piece := range view.pieces {
		if piece.text != "Italic affine" {
			continue
		}
		style := ui.TextStyle{Size: piece.size, Weight: piece.weight, Italic: true}
		for localX := float32(0); localX < piece.w; localX += 0.25 {
			styled := canvas.TextOffsetAtStyle(piece.text, style, localX)
			regular := canvas.TextOffsetAt(piece.text, piece.size, piece.weight, localX)
			if styled == regular {
				continue
			}
			actual, ok := view.OffsetAt(ui.Point{X: piece.x + localX, Y: piece.y + piece.h/2})
			want := int(piece.plainLo) + styled
			if !ok || actual != want {
				t.Fatalf("italic hit offset=%d, want=%d (regular=%d)", actual, want, int(piece.plainLo)+regular)
			}
			return
		}
		t.Fatal("fixture produced no italic/regular hit-test difference")
	}
	t.Fatal("italic piece missing")
}
