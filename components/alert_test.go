package components

import (
	"slices"
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestAlertDestructiveMatchesVegaTreatment(t *testing.T) {
	theme := gfx.LightTheme()
	var canvas gfx.Canvas
	Alert(&canvas, theme, gfx.Rect{X: 4, Y: 6, Width: 280, Height: 64}, AlertData{Variant: AlertDestructive, Title: "Unable to save", Description: "Check the destination and try again."})
	if len(canvas.Rectangles()) != 2 || len(canvas.Texts()) != 2 {
		t.Fatalf("danger alert commands = %d rectangles, %d texts", len(canvas.Rectangles()), len(canvas.Texts()))
	}
	if canvas.Rectangles()[0].Color != theme.Border || canvas.Rectangles()[1].Color != theme.Card || canvas.Rectangles()[0].Radius != theme.Metrics.RadiusSurface {
		t.Fatalf("danger alert colors = %+v", canvas.Rectangles())
	}
	if canvas.Texts()[0].Style.Color != theme.Danger || canvas.Texts()[1].Style.Color.Red != theme.Danger.Red || canvas.Texts()[1].Style.Color.Alpha != theme.Danger.Alpha*0.90 {
		t.Fatalf("danger alert text styles = %+v", canvas.Texts())
	}
	if canvas.Texts()[0].Style.Weight != gfx.WeightMedium || canvas.Texts()[1].Style.Size != theme.Metrics.FontSizeM {
		t.Fatalf("danger alert typography = %+v", canvas.Texts())
	}
}

func TestAlertDefaultUsesNeutralCardTokens(t *testing.T) {
	theme := gfx.LightTheme()
	var canvas gfx.Canvas
	Alert(&canvas, theme, gfx.Rect{Width: 240}, AlertData{Title: "Saved"})
	if canvas.Rectangles()[0].Color != theme.Border || canvas.Rectangles()[1].Color != theme.Card {
		t.Fatalf("default alert surface = %+v", canvas.Rectangles())
	}
	if canvas.Texts()[0].Style.Color != theme.CardForeground {
		t.Fatalf("default alert title = %+v", canvas.Texts()[0].Style)
	}
}

func TestAlertRepeatedPaintIsStableAndAllocationFree(t *testing.T) {
	var canvas gfx.Canvas
	paint := func() {
		canvas.Reset()
		Alert(&canvas, gfx.DarkTheme(), gfx.Rect{Width: 260, Height: 64}, AlertData{Title: "Update available", Description: "Restart when convenient.", Icon: checkIcon})
	}
	paint()
	rectangles := slices.Clone(canvas.Rectangles())
	texts := slices.Clone(canvas.Texts())
	paint()
	if !slices.Equal(rectangles, canvas.Rectangles()) || !slices.Equal(texts, canvas.Texts()) {
		t.Fatal("identical alerts produced different command streams")
	}
	if allocations := testing.AllocsPerRun(100, paint); allocations != 0 {
		t.Fatalf("alert paint allocations = %v, want 0 after warm-up", allocations)
	}
}

func TestAlertIconShiftsTextAndLongDescriptionWrapsAndGrows(t *testing.T) {
	theme := gfx.LightTheme()
	var canvas gfx.Canvas
	data := AlertData{Title: "Heads up", Description: "The remote host closed the connection while the transfer was still running, so the file may be incomplete."}
	narrow := gfx.Rect{Width: 220}
	plainHeight := AlertHeight(&canvas, theme, narrow.Width, AlertData{Title: "Heads up"})
	height := AlertHeight(&canvas, theme, narrow.Width, data)
	if height <= plainHeight+alertLineHeight(theme.Metrics) {
		t.Fatalf("wrapped alert height %v should exceed title-only %v by more than one line", height, plainHeight)
	}
	Alert(&canvas, theme, narrow, data)
	withoutIcon := canvas.Texts()[0].Bounds.X
	if canvas.Rectangles()[0].Bounds.Height != height {
		t.Fatalf("auto height = %v, want %v", canvas.Rectangles()[0].Bounds.Height, height)
	}
	if len(canvas.Texts()) < 3 {
		t.Fatalf("description did not wrap: %d texts", len(canvas.Texts()))
	}
	canvas.Reset()
	data.Icon = checkIcon
	Alert(&canvas, theme, narrow, data)
	if canvas.Texts()[0].Bounds.X <= withoutIcon {
		t.Fatalf("icon did not move the text: %v <= %v", canvas.Texts()[0].Bounds.X, withoutIcon)
	}
	if len(canvas.Triangles()) == 0 {
		t.Fatal("icon was not painted")
	}
}
