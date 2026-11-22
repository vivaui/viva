//go:build (darwin && !ios) || (linux && !android)

package app

import (
	"math"
	"testing"
	"unicode/utf8"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/text"
)

func TestPainterPublishesBothImeAnchorsOfOneBidiOffset(t *testing.T) {
	const prefix = "a\u0301\U00010437 "
	const value = prefix + "\u0633\u0644\u0627\u0645 z"
	engine := PlatformTextEngine()
	if _, ok := engine.(gfx.FittedMonoCaretEngine); !ok {
		t.Skip("native fitted text engine unavailable")
	}
	doc := text.NewEditor(value)
	var editor components.CodeEditor
	var canvas gfx.Canvas
	canvas.SetTextEngine(engine)
	painter := newTestPainter(t, &canvas, Config{Title: "Shaped text placement", Width: 500, Height: 180,
		Paint: func(canvas *gfx.Canvas, size gfx.Size) {
			selection := doc.Selection()
			editor.Paint(canvas, gfx.Theme{}, gfx.Rect{Width: size.Width, Height: size.Height}, components.CodeEditorSnapshot{
				ID: gfx.Item(7), Label: "Text", Value: doc.String(), FontSize: 18,
				Caret: doc.ByteOffset(doc.Caret()), SelectionStart: doc.ByteOffset(selection.Start), SelectionEnd: doc.ByteOffset(selection.End),
				ShapedBidi: true,
			})
		},
		TextInput: func(context TextInputContext) *TextInput {
			if context.ID == gfx.Item(7) {
				return &TextInput{State: TextStateOf(doc), OnEvent: func(event TextEvent) (TextState, error) {
					event.Apply(doc)
					return TextStateOf(doc), nil
				}}
			}
			return nil
		},
	})
	labels := canvas.Texts()
	if len(labels) != 1 || labels[0].Value != value {
		t.Fatalf("expected an intact fitted run, got %+v", labels)
	}
	label := labels[0]
	carets, ok := canvas.FittedBidiMonoCarets(nil, label.Value, label.Style, label.Bounds.Width)
	if !ok || len(carets) != 10 || carets[3].Offset != len(prefix) || carets[3].Leading-carets[3].Trailing < 1 {
		t.Fatalf("missing distinct edges at the Arabic boundary: %+v", carets)
	}
	y := label.Bounds.Y + label.Bounds.Height/2
	for _, edge := range []float32{carets[3].Trailing, carets[3].Leading} {
		want := label.Bounds.X + edge
		frame, err := painter.pointerEvent(platform.PointerEvent{Kind: platform.PointerDown, X: want - 0.25, Y: y,
			Button: gfx.PointerButtonPrimary, ClickCount: 1})
		if err != nil {
			t.Fatal(err)
		}
		update := frame.TextInput
		if !update.Changed || update.Input == nil || !update.Input.Placed || math.Abs(float64(update.Input.CaretX-want)) > 0.01 {
			t.Fatalf("callback did not publish the clicked IME anchor at %g: %+v", want, painter.nativeText)
		}
		if got := doc.Caret(); got != utf8.RuneCountInString(prefix) {
			t.Errorf("clicked document caret = %d; expected the same rune boundary", got)
		}
		if _, err := painter.pointerEvent(platform.PointerEvent{Kind: platform.PointerUp, X: want - 0.25, Y: y, Button: gfx.PointerButtonPrimary}); err != nil {
			t.Fatal(err)
		}
	}
	if doc.String() != value || doc.CanUndo() {
		t.Fatal("pointer placement changed the document or edit history")
	}
}
