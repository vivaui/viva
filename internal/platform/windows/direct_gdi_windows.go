package windows

import (
	"math"
	"syscall"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

// HintedGrid is whether the fixed-pitch text of a terminal is rasterized by GDI's
// ClearType rather than by Direct2D's natural rendering. GDI fits the stems of a glyph
// to the pixel grid and blends the subpixel coverage with the gamma of the display, so a
// small stem lands on whole pixels and reads as a solid stroke; Direct2D's natural mode
// keeps the outline where the font puts it and spreads every stem over two columns of
// colour, which at terminal sizes looks thin and fringed. The two renderings are the same
// glyphs at the same size: only the rasterization differs.
//
// GDI has no shaper, so a line with ligatures or with characters a fixed-pitch font may
// lack is left to Direct2D, as are colour glyphs. Everything else — the coverage of a
// run read per channel, one texture for a whole line and a colour per run — stays as it
// is: GDI is only the source of the coverage.
var HintedGrid = true

var (
	gdiCreateFont   = graphics.NewProc("CreateFontIndirectW")
	gdiExtTextOut   = graphics.NewProc("ExtTextOutW")
	gdiSetTextColor = graphics.NewProc("SetTextColor")
	gdiSetBkMode    = graphics.NewProc("SetBkMode")
	gdiSetTextAlign = graphics.NewProc("SetTextAlign")
)

// gdiLogFont is LOGFONTW.
type gdiLogFont struct {
	Height, Width, Escapement, Orientation, Weight int32
	Italic, Underline, StrikeOut, CharSet          byte
	OutPrecision, ClipPrecision, Quality, Pitch    byte
	FaceName                                       [32]uint16
}

// gdiFontKey names one font of the cache: the family at a device height, weight and slope.
type gdiFontKey struct {
	family string
	height int32
	weight int32
	italic bool
}

// gdiGridRasterizer draws fixed-pitch lines with GDI into a bitmap of its own.
type gdiGridRasterizer struct {
	bitmap *textBitmap
	fonts  map[gdiFontKey]uintptr
}

func (rasterizer *gdiGridRasterizer) close() {
	for key, font := range rasterizer.fonts {
		deleteObject.Call(font)
		delete(rasterizer.fonts, key)
	}
	if rasterizer.bitmap != nil {
		rasterizer.bitmap.close()
		rasterizer.bitmap = nil
	}
}

// bind returns the scratch bitmap the line is drawn into, at least width by height.
func (rasterizer *gdiGridRasterizer) bind(width, height int) (*textBitmap, error) {
	if rasterizer.bitmap != nil && width <= rasterizer.bitmap.Width && height <= rasterizer.bitmap.Height {
		return rasterizer.bitmap, nil
	}
	bitmap, err := newTextBitmap(max(width, 256), max(height, 32))
	if err != nil {
		return nil, err
	}
	if rasterizer.bitmap != nil {
		rasterizer.bitmap.close()
	}
	rasterizer.bitmap = bitmap
	return bitmap, nil
}

// font returns the GDI font of the family at a device height, weight and slope.
func (rasterizer *gdiGridRasterizer) font(family string, height, weight int32, italic bool) uintptr {
	key := gdiFontKey{family: family, height: height, weight: weight, italic: italic}
	if font, ok := rasterizer.fonts[key]; ok {
		return font
	}
	if rasterizer.fonts == nil {
		rasterizer.fonts = make(map[gdiFontKey]uintptr)
	}
	logFont := gdiLogFont{
		Height: height, Weight: weight,
		// ANSI_CHARSET, CLEARTYPE_QUALITY, FIXED_PITCH | FF_MODERN.
		CharSet: 0, Quality: 5, Pitch: 0x31,
	}
	if italic {
		logFont.Italic = 1
	}
	name, err := syscall.UTF16FromString(family)
	if err != nil || len(name) > len(logFont.FaceName) {
		return 0
	}
	copy(logFont.FaceName[:], name)
	font, _, _ := gdiCreateFont.Call(uintptr(unsafe.Pointer(&logFont)))
	rasterizer.fonts[key] = font
	return font
}

// gdiGrid draws a fixed-pitch label's runs with GDI ClearType into pixels, as the
// per-channel coverage the text shader blends with each run's colour, and appends the
// columns of every run to spans. ok is false when the label is not one GDI can draw
// faithfully, and the caller falls back to Direct2D.
func (rasterizer *directRasterizer) gdiGrid(pixels []byte, label platform.TextLabel, size float32, width, height int) (ok bool, err error) {
	if !HintedGrid || label.Value == "" || width <= 0 || height <= 0 {
		return false, nil
	}
	if !printableASCII(label.Value) {
		return false, nil
	}
	pitch := label.CellWidth * float32(size/label.FontSize)
	pieces := rasterizer.gridPieces(label, pitch)
	for _, piece := range pieces {
		// GDI shapes nothing: a run of ligatures is Direct2D's.
		if piece.run.Ligatures {
			return false, nil
		}
	}
	if rasterizer.gdi == nil {
		rasterizer.gdi = &gdiGridRasterizer{}
	}
	bitmap, err := rasterizer.gdi.bind(width, height)
	if err != nil {
		return false, err
	}
	fill(bitmap, width, height, d2dColor{A: 1})
	previous, _, _ := selectObject.Call(bitmap.dc, bitmap.bitmap)
	gdiSetBkMode.Call(bitmap.dc, 1)             // TRANSPARENT
	gdiSetTextColor.Call(bitmap.dc, 0x00ffffff) // white: the pixel is its coverage
	gdiSetTextAlign.Call(bitmap.dc, 24)         // TA_LEFT | TA_BASELINE
	baseline := int32(platform.LabelBaseline(height, float64(size)))
	family := monoFamily(label.MonoFace)
	drawn := false
	for _, piece := range pieces {
		if piece.text == "" {
			continue
		}
		font := rasterizer.gdi.font(family, -int32(math.Round(float64(size))), int32(nativeFontWeight(piece.weight)), piece.run.Italic)
		if font == 0 {
			selectObject.Call(bitmap.dc, previous)
			return false, nil
		}
		selectObject.Call(bitmap.dc, font)
		units, err := syscall.UTF16FromString(piece.text)
		if err != nil {
			continue
		}
		count := len(units) - 1
		if count <= 0 {
			continue
		}
		var advance uintptr
		if pitch > 0 && int(math.Round(float64(pitch))) > 0 {
			// Every character stands on its cell, as the grid does: the advances are the
			// cells, not the font's own.
			if cap(rasterizer.gdiSpacing) < count {
				rasterizer.gdiSpacing = make([]int32, count)
			}
			spacing := rasterizer.gdiSpacing[:count]
			for index := range spacing {
				spacing[index] = int32(math.Round(float64(pitch)))
			}
			advance = uintptr(unsafe.Pointer(&spacing[0]))
		}
		x := int32(math.Round(float64(piece.left)))
		gdiExtTextOut.Call(bitmap.dc, uintptr(x), uintptr(baseline), 0, 0,
			uintptr(unsafe.Pointer(&units[0])), uintptr(count), advance)
		drawn = true
	}
	selectObject.Call(bitmap.dc, previous)
	if !drawn {
		return false, nil
	}
	// The drawing is white text on black: its pixels are the coverage of the ClearType
	// filter, channel by channel, and their luminance is the alpha of the texture.
	for row := 0; row < height; row++ {
		source := bitmap.Pixels[row*bitmap.Width*4:]
		out := pixels[row*width*4 : (row+1)*width*4]
		for index := 0; index+3 < len(out); index += 4 {
			blue, green, red := source[index], source[index+1], source[index+2]
			out[index], out[index+1], out[index+2], out[index+3] = blue, green, red, luminance(red, green, blue)
		}
	}
	// A line has a colour per run: its coverage is drawn once for each, cut to the columns
	// of that run. GDI's coverage does not depend on the colour, so one drawing serves them
	// all, unlike Direct2D's.
	rasterizer.spans = rasterizer.spans[:0]
	for _, piece := range pieces {
		left, right := columnsOf(piece, width, len(pieces) == 1)
		if left == right {
			continue
		}
		if n := len(rasterizer.spans); n > 0 && rasterizer.spans[n-1].color == piece.color && rasterizer.spans[n-1].right >= left {
			rasterizer.spans[n-1].right = max(rasterizer.spans[n-1].right, right)
			continue
		}
		rasterizer.spans = append(rasterizer.spans, colorSpan{left: left, right: right, color: piece.color})
	}
	return true, nil
}
