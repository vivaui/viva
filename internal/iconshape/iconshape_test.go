package iconshape

import (
	"bytes"
	"image"
	"image/color"
	"testing"
)

func solid(size int, c color.NRGBA) *image.NRGBA {
	picture := image.NewNRGBA(image.Rect(0, 0, size, size))
	for index := 0; index < len(picture.Pix); index += 4 {
		picture.Pix[index], picture.Pix[index+1], picture.Pix[index+2], picture.Pix[index+3] = c.R, c.G, c.B, c.A
	}
	return picture
}

func alphaAt(picture *image.NRGBA, x, y int) uint8 { return picture.Pix[picture.PixOffset(x, y)+3] }

func TestAFullBleedSquareBecomesARoundedTile(t *testing.T) {
	red := color.NRGBA{R: 200, G: 30, B: 60, A: 255}
	tile := Tile(solid(256, red), 256, 1)
	if alphaAt(tile, 0, 0) != 0 {
		t.Error("the corner of the canvas stayed opaque: the square was not rounded")
	}
	if alphaAt(tile, 128, 128) != 255 || alphaAt(tile, 128, 2) != 255 {
		t.Error("the middle and the middle of an edge must stay opaque")
	}
	if got := tile.NRGBAAt(128, 128); got != red {
		t.Errorf("the color changed from %+v to %+v", red, got)
	}
}

func TestAnInsetLeavesTheTransparentMarginMacOSDrawsAround(t *testing.T) {
	tile := Tile(solid(512, color.NRGBA{A: 255}), 512, MacInset)
	if alphaAt(tile, 20, 256) != 0 || alphaAt(tile, 256, 20) != 0 {
		t.Error("the margin around the tile is not transparent")
	}
	if alphaAt(tile, 256, 256) != 255 {
		t.Error("the tile itself is not opaque")
	}
}

func TestAPictureThatAlreadyHasAShapeIsOnlyScaled(t *testing.T) {
	// A disc on a transparent canvas: its corners are empty, so it is taken as shaped.
	disc := image.NewNRGBA(image.Rect(0, 0, 128, 128))
	for y := 0; y < 128; y++ {
		for x := 0; x < 128; x++ {
			if dx, dy := x-64, y-64; dx*dx+dy*dy < 60*60 {
				disc.SetNRGBA(x, y, color.NRGBA{R: 9, A: 255})
			}
		}
	}
	if !Shaped(disc) || Shaped(solid(16, color.NRGBA{A: 255})) {
		t.Fatal("Shaped must tell a picture with empty corners from a full-bleed one")
	}
	tile := Tile(disc, 64, MacInset)
	if alphaAt(tile, 32, 32) != 255 {
		t.Error("the disc's middle is lost")
	}
	if alphaAt(tile, 32, 3) != 255 {
		t.Error("the disc was shrunk by an inset it should not get: its own artwork decides the margin")
	}
}

func TestAnAlphaEdgeDoesNotDarkenWhenScaled(t *testing.T) {
	picture := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 2; x++ {
			picture.SetNRGBA(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
		}
	}
	// Half opaque white, half empty: averaged together the color must stay white
	// at half the opacity, not drift toward the empty half's black.
	scaled := scale(picture, 1)
	if got := scaled.NRGBAAt(0, 0); got.R != 255 || got.G != 255 || got.A < 126 || got.A > 129 {
		t.Errorf("half white and half empty averaged to %+v, want white at half opacity", got)
	}
}

// BenchmarkTileTheDockIcon is the work a program does once at start-up on macOS:
// a 1024 px picture to the 512 px tile of the Dock.
func BenchmarkTileTheDockIcon(b *testing.B) {
	picture := solid(1024, color.NRGBA{R: 20, G: 40, B: 60, A: 255})
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		Tile(picture, 512, MacInset)
	}
}

func TestATileMarkFillsTheIconWithTheMark(t *testing.T) {
	// A quarter-wide dark mark on a white canvas, as an application draws its
	// mark for a desktop that gives the tile a margin.
	picture := solid(128, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
	for y := 48; y < 80; y++ {
		for x := 48; x < 80; x++ {
			picture.SetNRGBA(x, y, color.NRGBA{R: 16, G: 16, B: 16, A: 255})
		}
	}
	tight := TileMark(picture, 32)
	tile := Tile(picture, 32, 1)
	if got := alphaAt(tight, 0, 0); got != 0 {
		t.Error("the corner of the tight tile stayed opaque: the square was not rounded")
	}
	if got := tight.NRGBAAt(16, 1); got.A != 255 || got.R < 200 {
		t.Errorf("the background above the mark is %+v, want the picture's own background", got)
	}
	mark := opaqueDarkRow(tight, 16)
	if mark < 24 {
		t.Errorf("the mark spans %d of 32 px columns: the canvas margin was kept instead of spent", mark)
	}
	if grew := mark - opaqueDarkRow(tile, 16); grew < 8 {
		t.Errorf("the tight tile's mark spans %d px, the plain tile's %d: the status area must show the mark larger", mark, mark-grew)
	}
}

func TestTileMarkLeavesAPictureThatHasAShapeOfItsOwn(t *testing.T) {
	disc := image.NewNRGBA(image.Rect(0, 0, 128, 128))
	for y := 0; y < 128; y++ {
		for x := 0; x < 128; x++ {
			if dx, dy := x-64, y-64; dx*dx+dy*dy < 60*60 {
				disc.SetNRGBA(x, y, color.NRGBA{R: 9, A: 255})
			}
		}
	}
	if !bytes.Equal(TileMark(disc, 64).Pix, Tile(disc, 64, 1).Pix) {
		t.Error("a picture with its own shape must only be scaled, not cropped")
	}
}

func TestTileMarkLeavesAPictureWhoseMarkFillsTheCanvas(t *testing.T) {
	// The mark reaches both edges, so there is no margin to crop; a solid
	// picture has no mark at all. Both must come out as the plain tile.
	split := solid(128, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
	for y := 0; y < 128; y++ {
		for x := 64; x < 128; x++ {
			split.SetNRGBA(x, y, color.NRGBA{R: 16, G: 16, B: 16, A: 255})
		}
	}
	for name, picture := range map[string]*image.NRGBA{
		"mark to the edges": split,
		"no mark":           solid(128, color.NRGBA{R: 30, G: 140, B: 90, A: 255}),
	} {
		if !bytes.Equal(TileMark(picture, 32).Pix, Tile(picture, 32, 1).Pix) {
			t.Errorf("%s: the picture was cropped although there is no margin to spend", name)
		}
	}
}

// opaqueDarkRow is how many opaque, dark pixels row y holds: the mark's width
// where it crosses that row.
func opaqueDarkRow(picture *image.NRGBA, y int) int {
	count := 0
	for x := 0; x < picture.Bounds().Dx(); x++ {
		if pixel := picture.NRGBAAt(x, y); pixel.A > 128 && pixel.R < 64 {
			count++
		}
	}
	return count
}

// BenchmarkTileMarkTheStatusIcon is the work a program does once per status
// item: a 512 px picture cropped to its mark and shaped at the 16 px the
// desktop draws.
func BenchmarkTileMarkTheStatusIcon(b *testing.B) {
	picture := solid(512, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
	for y := 200; y < 340; y++ {
		for x := 180; x < 332; x++ {
			picture.SetNRGBA(x, y, color.NRGBA{R: 16, G: 16, B: 16, A: 255})
		}
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		TileMark(picture, 16)
	}
}

func TestSilhouetteIsTheMarkCroppedAndCenteredAsBlackAlpha(t *testing.T) {
	// A dark mark and a green dot on white, off-center in a large canvas.
	picture := solid(200, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
	for y := 20; y < 60; y++ {
		for x := 30; x < 110; x++ {
			picture.SetNRGBA(x, y, color.NRGBA{R: 17, G: 17, B: 17, A: 255})
		}
	}
	for y := 20; y < 30; y++ {
		for x := 100; x < 110; x++ {
			picture.SetNRGBA(x, y, color.NRGBA{R: 3, G: 185, B: 79, A: 255})
		}
	}
	shape := Silhouette(picture, 32)
	if shape.NRGBAAt(0, 0).A != 0 || shape.NRGBAAt(31, 31).A != 0 {
		t.Error("the canvas corners are not empty")
	}
	if got := shape.NRGBAAt(16, 16); got.A < 240 || got.R != 0 || got.G != 0 || got.B != 0 {
		t.Errorf("the middle of the mark is %+v, want opaque black", got)
	}
	// The mark was far off-center in the source; cropped, it fills the canvas
	// evenly, so the first and last opaque columns are about equally far from the edges.
	first, last := -1, -1
	for x := 0; x < 32; x++ {
		if shape.NRGBAAt(x, 16).A > 128 {
			if first < 0 {
				first = x
			}
			last = x
		}
	}
	if first < 0 || first-(31-last) > 1 || (31-last)-first > 1 {
		t.Errorf("the mark spans columns %d to %d of 32: not centered", first, last)
	}
	// The green dot, far from white, is part of the shape.
	if shape.NRGBAAt(28, 10).A < 200 {
		t.Errorf("the dot is missing from the shape: %+v", shape.NRGBAAt(28, 10))
	}
}
