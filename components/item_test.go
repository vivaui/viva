package components

import (
	"slices"
	"testing"

	"github.com/vivaui/viva/ui"
)

func TestItemOutlineLaysOutContentAndDeclaresHit(t *testing.T) {
	theme := ui.DarkTheme()
	var canvas ui.Canvas
	BeginItem(&canvas, theme, ui.Item(42), ui.Rect{X: 8, Y: 10, Width: 320}).Variant(ItemOutline).LeadingSpace(24).Paint("Document", "Updated moments ago", "12 KB")
	if len(canvas.Rectangles()) != 0 || len(canvas.Triangles()) == 0 || len(canvas.Texts()) != 3 {
		t.Fatalf("item commands = %d rectangles, %d triangles, %d texts", len(canvas.Rectangles()), len(canvas.Triangles()), len(canvas.Texts()))
	}
	if len(canvas.Hits()) != 1 || canvas.Hits()[0].ID != ui.Item(42) || canvas.Hits()[0].Label != "Document" {
		t.Fatalf("item hits = %+v", canvas.Hits())
	}
	if canvas.Texts()[1].Bounds.X < 8+theme.Metrics.PadSurface+24+theme.Metrics.PadSurface-2 {
		t.Fatalf("title did not reserve leading space: %+v", canvas.Texts()[1].Bounds)
	}
	if canvas.Texts()[0].Value != "12 KB" || canvas.Texts()[2].Style.Color != theme.MutedForeground || canvas.Texts()[2].Style.Size != theme.Metrics.FontSizeM {
		t.Fatalf("item text commands = %+v", canvas.Texts())
	}
}

func TestItemDisabledSuppressesInteraction(t *testing.T) {
	var canvas ui.Canvas
	canvas.SetInteraction(ui.Item(9), ui.Item(9), ui.Item(9), true)
	BeginItem(&canvas, ui.LightTheme(), ui.Item(9), ui.Rect{Width: 240}).Disabled().Paint("Locked", "Read only", "")
	if len(canvas.Hits()) != 0 || len(canvas.Rectangles()) != 0 {
		t.Fatalf("disabled item commands = %d hits, %d rectangles", len(canvas.Hits()), len(canvas.Rectangles()))
	}
	want := ui.LightTheme().Foreground
	want.Alpha *= 0.50
	if canvas.Texts()[0].Style.Color != want {
		t.Fatalf("disabled title color = %+v", canvas.Texts()[0].Style.Color)
	}
}

func TestItemDefaultIsTransparentAndMutedUsesHalfToken(t *testing.T) {
	theme := ui.LightTheme()
	var defaultCanvas, mutedCanvas ui.Canvas
	BeginItem(&defaultCanvas, theme, ui.ID{}, ui.Rect{Width: 220}).Paint("Default", "", "")
	BeginItem(&mutedCanvas, theme, ui.ID{}, ui.Rect{Width: 220}).Variant(ItemMuted).Paint("Muted", "", "")
	if len(defaultCanvas.Rectangles()) != 0 {
		t.Fatalf("default item painted a surface: %+v", defaultCanvas.Rectangles())
	}
	want := theme.Muted
	want.Alpha *= 0.50
	if len(mutedCanvas.Rectangles()) != 1 || mutedCanvas.Rectangles()[0].Color != want {
		t.Fatalf("muted item surface = %+v, want %+v", mutedCanvas.Rectangles(), want)
	}
}

func TestItemSizesApplyVegaSpacingAndExtraSmallType(t *testing.T) {
	theme := ui.LightTheme()
	var defaultCanvas, smallCanvas, extraSmallCanvas ui.Canvas
	BeginItem(&defaultCanvas, theme, ui.ID{}, ui.Rect{Width: 260}).Paint("Title", "Description", "")
	BeginItem(&smallCanvas, theme, ui.ID{}, ui.Rect{Width: 260}).Size(ItemSizeSmall).Paint("Title", "Description", "")
	BeginItem(&extraSmallCanvas, theme, ui.ID{}, ui.Rect{Width: 260}).Size(ItemSizeExtraSmall).Paint("Title", "Description", "")
	if defaultCanvas.Texts()[0].Bounds.X != theme.Metrics.PadSurface || smallCanvas.Texts()[0].Bounds.X != theme.Metrics.PadControlX || extraSmallCanvas.Texts()[0].Bounds.X != theme.Metrics.PadControlX-2 {
		t.Fatalf("item size insets = %v, %v, %v", defaultCanvas.Texts()[0].Bounds.X, smallCanvas.Texts()[0].Bounds.X, extraSmallCanvas.Texts()[0].Bounds.X)
	}
	if extraSmallCanvas.Texts()[1].Style.Size != theme.Metrics.FontSizeS {
		t.Fatalf("extra-small description style = %+v", extraSmallCanvas.Texts()[1].Style)
	}
	if !(defaultCanvas.Texts()[0].Bounds.Y > smallCanvas.Texts()[0].Bounds.Y && smallCanvas.Texts()[0].Bounds.Y > extraSmallCanvas.Texts()[0].Bounds.Y) {
		t.Fatalf("item size vertical insets = %v, %v, %v", defaultCanvas.Texts()[0].Bounds.Y, smallCanvas.Texts()[0].Bounds.Y, extraSmallCanvas.Texts()[0].Bounds.Y)
	}
}

func TestItemRepeatedPaintIsStableAndAllocationFree(t *testing.T) {
	var canvas ui.Canvas
	paint := func() {
		canvas.Reset()
		BeginItem(&canvas, ui.DarkTheme(), ui.Item(7), ui.Rect{Width: 300}).Variant(ItemMuted).Paint("Settings", "Manage application preferences", "⌘,")
	}
	paint()
	rectangles := slices.Clone(canvas.Rectangles())
	texts := slices.Clone(canvas.Texts())
	hits := slices.Clone(canvas.Hits())
	paint()
	if !slices.Equal(rectangles, canvas.Rectangles()) || !slices.Equal(texts, canvas.Texts()) || !slices.Equal(hits, canvas.Hits()) {
		t.Fatal("identical items produced different command streams")
	}
	if allocations := testing.AllocsPerRun(100, paint); allocations != 0 {
		t.Fatalf("item paint allocations = %v, want 0 after warm-up", allocations)
	}
}

func TestItemMediaBoxFollowsSizeAndIsReturnedForPainting(t *testing.T) {
	theme := ui.LightTheme()
	var canvas ui.Canvas
	bounds := ui.Rect{X: 5, Y: 7, Width: 300}
	large := BeginItem(&canvas, theme, ui.ID{}, bounds).Media(ItemMediaImage).Paint("Photo", "Taken today", "")
	small := BeginItem(&canvas, theme, ui.ID{}, bounds).Size(ItemSizeSmall).Media(ItemMediaImage).Paint("Photo", "Taken today", "")
	extra := BeginItem(&canvas, theme, ui.ID{}, bounds).Size(ItemSizeExtraSmall).Media(ItemMediaImage).Paint("Photo", "Taken today", "")
	if !(large.Media.Width > small.Media.Width && small.Media.Width > extra.Media.Width) {
		t.Fatalf("image media sizes %v / %v / %v should shrink with the item", large.Media.Width, small.Media.Width, extra.Media.Width)
	}
	if large.Content.X < large.Media.X+large.Media.Width {
		t.Fatalf("text column %+v overlaps the media %+v", large.Content, large.Media)
	}
	if none := BeginItem(&canvas, theme, ui.ID{}, bounds).Paint("Plain", "", ""); none.Media != (ui.Rect{}) {
		t.Fatalf("an item without media returned %+v", none.Media)
	}
}
