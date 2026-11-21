package gfx

import (
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestLabelHighlightKeepsRTLAlignmentAndPaintsBeforeText(t *testing.T) {
	var canvas Canvas
	tint := RGB(220, 190, 80)
	canvas.LabelHighlighted("שלום design", Rect{Width: 350, Height: 24}, TextStyle{Size: 14}, AlignEnd, "DESIGN", tint)
	label := canvas.Texts()[0]
	start := strings.Index(label.Value, "design")
	spans := canvas.TextSelectionStyle(nil, label.Value, label.Style, start, start+len("design"))
	for _, rect := range canvas.Rectangles() {
		if rect.Color == tint {
			if rect.Order >= label.Order || math.Abs(float64(rect.Bounds.X-label.Bounds.X-spans[0].X)) > .01 {
				t.Fatalf("highlight lost text direction or paint order: %+v / %+v", rect, label)
			}
			return
		}
	}
	t.Fatal("missing RTL highlight")
}

func TestLabelHighlightUnicodeMatchesAndPreservesTheLabel(t *testing.T) {
	tint := RGB(220, 190, 80)
	for _, tc := range []struct {
		name, value, query string
		want               int
	}{
		{"chinese", "项目设计与设计资料", "设计", 2},
		{"different byte lengths", "K kelvin K", "k", 3},
		{"greek fold", "Σ σ ς", "σ", 3},
		{"emoji", "a👩‍💻b👩‍💻", "👩‍💻", 2},
		{"non overlapping", "aaaaa", "aa", 2},
		{"too long", "短", "很长的查询", 0},
		{"empty", "Design", "", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var canvas, plain Canvas
			bounds := Rect{Width: 400, Height: 24}
			style := TextStyle{Size: 14}
			canvas.LabelHighlighted(tc.value, bounds, style, AlignCenter, tc.query, tint)
			plain.LabelAligned(tc.value, bounds, style, AlignCenter)
			if len(canvas.Texts()) != 1 || canvas.Texts()[0].Value != plain.Texts()[0].Value || canvas.Texts()[0].Bounds != plain.Texts()[0].Bounds {
				t.Fatal("highlight changed the original label or its alignment")
			}
			matches := 0
			for _, rect := range canvas.Rectangles() {
				if rect.Color == tint {
					matches++
				}
			}
			if matches != tc.want {
				t.Fatalf("highlights = %d, want %d", matches, tc.want)
			}
		})
	}
}

func TestLabelHighlightDisabledUsesTheUnchangedLabelPath(t *testing.T) {
	for _, color := range []Color{{}, {Alpha: -1}} {
		var highlighted, plain Canvas
		bounds := Rect{Width: 200, Height: 24}
		style := TextStyle{Size: 14, Direction: DirectionRTL}
		highlighted.LabelHighlighted("Design", bounds, style, AlignEnd, "design", color)
		plain.LabelAligned("Design", bounds, style, AlignEnd)
		if !reflect.DeepEqual(highlighted.Texts(), plain.Texts()) || len(highlighted.Rectangles()) != 0 {
			t.Fatal("disabled highlight changed the canvas output")
		}
	}
}
