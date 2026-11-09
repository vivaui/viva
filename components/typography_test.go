package components

import (
	"slices"
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestTypographyVariantsUseThemeScale(t *testing.T) {
	theme := gfx.DarkTheme()
	variants := []TypographyVariant{
		TypographyH1, TypographyH2, TypographyH3, TypographyH4,
		TypographyParagraph, TypographyLead, TypographyLarge,
		TypographySmall, TypographyMuted,
	}
	var canvas gfx.Canvas
	for _, variant := range variants {
		Typography(&canvas, theme, gfx.Rect{Width: 300, Height: 40}, variant, "Text")
	}
	texts := canvas.Texts()
	if texts[0].Style.Size != theme.Metrics.FontSizeL*1.5 || texts[0].Style.Weight != gfx.WeightBold {
		t.Fatalf("h1 style = %+v", texts[0].Style)
	}
	if texts[1].Style.Size != theme.Metrics.FontSizeL+6 || texts[2].Style.Size != theme.Metrics.FontSizeL || texts[3].Style.Size != theme.Metrics.FontSizeM+7 {
		t.Fatalf("heading scale = %+v / %+v / %+v", texts[1].Style, texts[2].Style, texts[3].Style)
	}
	if texts[4].Style.Weight != gfx.WeightRegular || texts[5].Style.Color != theme.MutedForeground || texts[6].Style.Weight != gfx.WeightSemibold || texts[8].Style.Color != theme.MutedForeground {
		t.Fatalf("body typography styles = %+v", texts)
	}
}

func TestTypographyRepeatedPaintIsStableAndAllocationFree(t *testing.T) {
	var canvas gfx.Canvas
	paint := func() {
		canvas.Reset()
		Typography(&canvas, gfx.LightTheme(), gfx.Rect{Width: 240, Height: 32}, TypographyH2, "Overview")
	}
	paint()
	texts := slices.Clone(canvas.Texts())
	paint()
	if !slices.Equal(texts, canvas.Texts()) {
		t.Fatal("identical typography produced different command streams")
	}
	if allocations := testing.AllocsPerRun(100, paint); allocations != 0 {
		t.Fatalf("typography paint allocations = %v, want 0 after warm-up", allocations)
	}
}
