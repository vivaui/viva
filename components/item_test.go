package components

import (
	"slices"
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestItemOutlineLaysOutContentAndDeclaresHit(t *testing.T) {
	theme := gfx.DarkTheme()
	var canvas gfx.Canvas
	BeginItem(&canvas, theme, gfx.Item(42), gfx.Rect{X: 8, Y: 10, Width: 320}).Variant(ItemOutline).LeadingSpace(24).Paint("Document", "Updated moments ago", "12 KB")
	if len(canvas.Rectangles()) != 0 || len(canvas.Triangles()) == 0 || len(canvas.Texts()) != 3 {
		t.Fatalf("item commands = %d rectangles, %d triangles, %d texts", len(canvas.Rectangles()), len(canvas.Triangles()), len(canvas.Texts()))
	}
	if len(canvas.Hits()) != 1 || canvas.Hits()[0].ID != gfx.Item(42) || canvas.Hits()[0].Label != "Document" {
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
	var canvas gfx.Canvas
	canvas.SetInteraction(gfx.Item(9), gfx.Item(9), gfx.Item(9), true)
	BeginItem(&canvas, gfx.LightTheme(), gfx.Item(9), gfx.Rect{Width: 240}).Disabled().Paint("Locked", "Read only", "")
	if len(canvas.Hits()) != 0 || len(canvas.Rectangles()) != 0 {
		t.Fatalf("disabled item commands = %d hits, %d rectangles", len(canvas.Hits()), len(canvas.Rectangles()))
	}
	want := gfx.LightTheme().Foreground
	want.Alpha *= 0.50
	if canvas.Texts()[0].Style.Color != want {
		t.Fatalf("disabled title color = %+v", canvas.Texts()[0].Style.Color)
	}
}

func TestItemDefaultIsTransparentAndMutedUsesHalfToken(t *testing.T) {
	theme := gfx.LightTheme()
	var defaultCanvas, mutedCanvas gfx.Canvas
	BeginItem(&defaultCanvas, theme, gfx.ID{}, gfx.Rect{Width: 220}).Paint("Default", "", "")
	BeginItem(&mutedCanvas, theme, gfx.ID{}, gfx.Rect{Width: 220}).Variant(ItemMuted).Paint("Muted", "", "")
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
	theme := gfx.LightTheme()
	var defaultCanvas, smallCanvas, extraSmallCanvas gfx.Canvas
	BeginItem(&defaultCanvas, theme, gfx.ID{}, gfx.Rect{Width: 260}).Paint("Title", "Description", "")
	BeginItem(&smallCanvas, theme, gfx.ID{}, gfx.Rect{Width: 260}).Size(ItemSizeSmall).Paint("Title", "Description", "")
	BeginItem(&extraSmallCanvas, theme, gfx.ID{}, gfx.Rect{Width: 260}).Size(ItemSizeExtraSmall).Paint("Title", "Description", "")
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
	var canvas gfx.Canvas
	paint := func() {
		canvas.Reset()
		BeginItem(&canvas, gfx.DarkTheme(), gfx.Item(7), gfx.Rect{Width: 300}).Variant(ItemMuted).Paint("Settings", "Manage application preferences", "⌘,")
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
	theme := gfx.LightTheme()
	var canvas gfx.Canvas
	bounds := gfx.Rect{X: 5, Y: 7, Width: 300}
	large := BeginItem(&canvas, theme, gfx.ID{}, bounds).Media(ItemMediaImage).Paint("Photo", "Taken today", "")
	small := BeginItem(&canvas, theme, gfx.ID{}, bounds).Size(ItemSizeSmall).Media(ItemMediaImage).Paint("Photo", "Taken today", "")
	extra := BeginItem(&canvas, theme, gfx.ID{}, bounds).Size(ItemSizeExtraSmall).Media(ItemMediaImage).Paint("Photo", "Taken today", "")
	if !(large.Media.Width > small.Media.Width && small.Media.Width > extra.Media.Width) {
		t.Fatalf("image media sizes %v / %v / %v should shrink with the item", large.Media.Width, small.Media.Width, extra.Media.Width)
	}
	if large.Content.X < large.Media.X+large.Media.Width {
		t.Fatalf("text column %+v overlaps the media %+v", large.Content, large.Media)
	}
	if none := BeginItem(&canvas, theme, gfx.ID{}, bounds).Paint("Plain", "", ""); none.Media != (gfx.Rect{}) {
		t.Fatalf("an item without media returned %+v", none.Media)
	}
}
