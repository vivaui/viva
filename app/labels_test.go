package app

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

func accessibilityLabel(t *testing.T, painter *painter, id gfx.ID) string {
	t.Helper()
	for _, node := range painter.accessibility {
		if node.ID == id {
			return node.Label
		}
	}
	t.Fatalf("no accessibility node for %v", id)
	return ""
}

func labelledPainter(t *testing.T, paint func(*gfx.Canvas)) *painter {
	t.Helper()
	var canvas gfx.Canvas
	config := Config{Title: "labels", Width: 200, Height: 100}
	config.Paint = func(canvas *gfx.Canvas, _ gfx.Size) { paint(canvas) }
	return newTestPainter(t, &canvas, config)
}

func TestLabelHitsNameTheirControlInEitherDeclarationOrder(t *testing.T) {
	bounds := gfx.Rect{Width: 80, Height: 20}
	for _, test := range []struct {
		name  string
		paint func(*gfx.Canvas)
	}{
		{"label after its control", func(canvas *gfx.Canvas) {
			canvas.HitWithLabel(gfx.Item(2), bounds, "Button")
			canvas.HitLabel(gfx.Item(2), bounds, "Real label", false)
			canvas.HitWithLabel(gfx.Item(1), bounds, "Other")
		}},
		{"label before its control", func(canvas *gfx.Canvas) {
			canvas.HitLabel(gfx.Item(2), bounds, "Real label", false)
			canvas.HitWithLabel(gfx.Item(2), bounds, "Button")
			canvas.HitWithLabel(gfx.Item(1), bounds, "Other")
		}},
		{"label before its control with another control between", func(canvas *gfx.Canvas) {
			canvas.HitLabel(gfx.Item(2), bounds, "Real label", false)
			canvas.HitWithLabel(gfx.Item(1), bounds, "Other")
			canvas.HitWithLabel(gfx.Item(2), bounds, "Button")
		}},
		{"label after its control with another control between", func(canvas *gfx.Canvas) {
			canvas.HitWithLabel(gfx.Item(2), bounds, "Button")
			canvas.HitWithLabel(gfx.Item(1), bounds, "Other")
			canvas.HitLabel(gfx.Item(2), bounds, "Real label", false)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			painter := labelledPainter(t, test.paint)
			if got := accessibilityLabel(t, painter, gfx.Item(2)); got != "Real label" {
				t.Fatalf("control 2 label = %q", got)
			}
			if got := accessibilityLabel(t, painter, gfx.Item(1)); got != "Other" {
				t.Fatalf("control 1 label = %q", got)
			}
		})
	}
}

func TestTheFirstOfTwoLabelsForOneControlNamesIt(t *testing.T) {
	bounds := gfx.Rect{Width: 80, Height: 20}
	painter := labelledPainter(t, func(canvas *gfx.Canvas) {
		canvas.HitWithLabel(gfx.Item(1), bounds, "Button")
		canvas.HitLabel(gfx.Item(1), bounds, "first", false)
		canvas.HitLabel(gfx.Item(1), bounds, "second", false)
	})
	if got := accessibilityLabel(t, painter, gfx.Item(1)); got != "first" {
		t.Fatalf("label = %q, want the first label", got)
	}
}
