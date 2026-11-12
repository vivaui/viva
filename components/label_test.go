package components

import (
	"slices"
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestLabelRequiredAndDisabledStates(t *testing.T) {
	theme := gfx.LightTheme()
	var required gfx.Canvas
	BeginLabel(&required, theme, gfx.Rect{Width: 180}).Required().Paint("Email")
	if len(required.Texts()) != 2 || required.Texts()[1].Value != "*" || required.Texts()[1].Style.Color != theme.Danger {
		t.Fatalf("required label commands = %+v", required.Texts())
	}

	var disabled gfx.Canvas
	BeginLabel(&disabled, theme, gfx.Rect{Width: 180}).Required().Disabled().Paint("Email")
	wantText := theme.Foreground
	wantText.Alpha *= 0.50
	wantMarker := theme.Danger
	wantMarker.Alpha *= 0.50
	if disabled.Texts()[0].Style.Color != wantText || disabled.Texts()[1].Style.Color != wantMarker {
		t.Fatalf("disabled label colors = %+v", disabled.Texts())
	}
	if disabled.Texts()[0].Bounds.Height != theme.Metrics.FontSizeM+3 || disabled.Texts()[0].Style.Weight != gfx.WeightMedium {
		t.Fatalf("label intrinsic typography = %+v", disabled.Texts()[0])
	}
}

func TestLabelRepeatedPaintIsStableAndAllocationFree(t *testing.T) {
	var canvas gfx.Canvas
	paint := func() {
		canvas.Reset()
		BeginLabel(&canvas, gfx.DarkTheme(), gfx.Rect{Width: 160}).Required().Paint("Workspace")
	}
	paint()
	texts := slices.Clone(canvas.Texts())
	paint()
	if !slices.Equal(texts, canvas.Texts()) {
		t.Fatal("identical labels produced different command streams")
	}
	if allocations := testing.AllocsPerRun(100, paint); allocations != 0 {
		t.Fatalf("label paint allocations = %v, want 0 after warm-up", allocations)
	}
}
