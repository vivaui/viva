//go:build darwin || windows || linux

package headless

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

// A screenshot must draw fixed-pitch text in the face the application asked for,
// the named families included.
func TestSceneryKeepsTheFixedPitchFaceOfALabel(t *testing.T) {
	face := ui.MonoFamily("Convert Test Mono")
	_, labels, _ := scenery(app.Snapshot{Labels: []app.Label{
		{Value: "a", Mono: true, MonoFace: ui.MonoClassic},
		{Value: "b", Mono: true, MonoFace: face},
	}})
	if len(labels) != 2 || labels[0].MonoFace != ui.MonoClassic || labels[1].MonoFace != face {
		t.Fatalf("labels = %+v", labels)
	}
}

// A screenshot must keep the weight the application declared for its native
// editor, or the offscreen rendering shows the very bug the weight fixes.
func TestSceneryKeepsTheEditorsDeclaredWeight(t *testing.T) {
	_, _, input := scenery(app.Snapshot{Editor: &app.Editor{
		Bounds: ui.Rect{Width: 80, Height: 20}, FontSize: 13, Weight: 600,
	}})
	if input == nil || input.Weight != 600 {
		t.Fatalf("input = %+v, want the declared weight 600", input)
	}
}

func TestSceneryKeepsTheEditorsDeclaredAlignment(t *testing.T) {
	for _, alignment := range []ui.Alignment{ui.AlignCenter, ui.AlignRight} {
		_, _, input := scenery(app.Snapshot{Editor: &app.Editor{
			Bounds: ui.Rect{Width: 120, Height: 30}, FontSize: 16, Alignment: alignment,
		}})
		if input == nil || input.Alignment != alignment {
			t.Fatalf("headless alignment = %+v, want %v", input, alignment)
		}
	}
}

func TestSceneryKeepsProportionalItalicLabel(t *testing.T) {
	_, labels, _ := scenery(app.Snapshot{Labels: []app.Label{{Value: "Italic", FontSize: 16, Italic: true}}})
	if len(labels) != 1 || !labels[0].Italic {
		t.Fatalf("italic label = %+v", labels)
	}
}
