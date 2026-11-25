package components

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestTextareaPaintLineAlsoPaintsSelectedText(t *testing.T) {
	var canvas gfx.Canvas
	theme := gfx.LightTheme()
	calls := 0
	PaintTextarea(&canvas, theme, gfx.Rect{Width: 200, Height: 80}, TextareaSnapshot{
		Value: "hello", SelectionStart: 0, SelectionEnd: 2,
		PaintLine: func(c *gfx.Canvas, value string, bounds gfx.Rect, style gfx.TextStyle) {
			calls++
			c.Label("custom:"+value, bounds, style)
		},
	})
	if calls != 2 {
		t.Fatalf("custom painter called %d times, want normal and selected passes", calls)
	}
	labels := canvas.Texts()
	if len(labels) != 2 || labels[0].Value != "custom:hello" || labels[1].Value != "custom:hello" {
		t.Fatalf("selected text bypassed the custom painter: %+v", labels)
	}
	_, selectedColor := selectionColors(theme)
	if labels[1].Style.Color != selectedColor || !labels[1].HasClip {
		t.Fatalf("selected painter lost selection color or clip: %+v", labels[1])
	}
}
