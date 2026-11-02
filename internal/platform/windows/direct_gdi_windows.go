package windows

import (
	"fmt"
	"math"
	"strings"
	"syscall"
	"unsafe"
)

// A fixed-pitch line is drawn by GDI's ClearType wherever GDI can draw a run faithfully.
// GDI fits the stems of a glyph to the pixel grid and blends the subpixel coverage with
// the gamma of the display, so a small stem lands on whole pixels and reads as a solid
// stroke; Direct2D's natural mode keeps the outline where the font puts it and spreads
// every stem over two columns of colour, which at terminal sizes looks thin and fringed.
// The two renderings are the same glyphs at the same size: only the rasterization differs.
//
// GDI has no shaper and no font fallback, so a run with ligatures, a run with characters
// outside printable ASCII (the grid gives each of those a run of its own), a font GDI
// does not have, and colour glyphs are Direct2D's. A line may hold both: every run is
// drawn by one of the two, and its columns are read from that drawing, so a prompt in
// ASCII next to a wide character is hinted and the wide character is not.
//
// GDI is only the source of the coverage. It is read per channel, one texture for a whole
// line and a colour per run, exactly as Direct2D's is.

var (
	gdiCreateFont   = graphics.NewProc("CreateFontIndirectW")
	gdiGetTextFace  = graphics.NewProc("GetTextFaceW")
	gdiExtTextOut   = graphics.NewProc("ExtTextOutW")
	gdiSetTextColor = graphics.NewProc("SetTextColor")
	gdiSetBkMode    = graphics.NewProc("SetBkMode")
	gdiSetTextAlign = graphics.NewProc("SetTextAlign")
	gdiFlush        = graphics.NewProc("GdiFlush")
)

// gdiLogFont is LOGFONTW.
type gdiLogFont struct {
	Height, Width, Escapement, Orientation, Weight int32
	Italic, Underline, StrikeOut, CharSet          byte
	OutPrecision, ClipPrecision, Quality, Pitch    byte
	FaceName                                       [32]uint16
}

// gdiFontKey names one font of the cache: the family at a device height, weight, slope
// and decoration. GDI draws the underline and strikethrough of a run with its font.
type gdiFontKey struct {
	family            string
	height, weight    int32
	italic, underline bool
	strike            bool
}

// gdiGridRasterizer draws fixed-pitch runs with GDI into a bitmap of its own.
type gdiGridRasterizer struct {
	bitmap *textBitmap
	// fonts are the fonts made so far; 0 is a family GDI does not have, which is
	// remembered so it is asked once.
	fonts   map[gdiFontKey]uintptr
	text    []uint16 // the characters of the run being drawn
	spacing []int32  // the advance of each of them
}

func (rasterizer *gdiGridRasterizer) close() {
	// The device context holds no font of ours (draw puts its own back), so they can go
	// before the bitmap and its context.
	for key, font := range rasterizer.fonts {
		if font != 0 {
			deleteObject.Call(font)
		}
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

// font returns the GDI font for key, or 0 when GDI has no such family. CreateFontIndirect
// never fails for a missing family: it hands back whichever font matches best, and
// terminal text in a different face than the one asked for is worse than no hinting. So
// the face the context ends up with is read back, and the font is kept only if it is the
// family. The bitmap must be bound: its context does the asking.
func (rasterizer *gdiGridRasterizer) font(key gdiFontKey) uintptr {
	if font, ok := rasterizer.fonts[key]; ok {
		return font
	}
	if rasterizer.fonts == nil {
		rasterizer.fonts = make(map[gdiFontKey]uintptr)
	}
	rasterizer.fonts[key] = 0
	name, err := syscall.UTF16FromString(key.family)
	var logFont gdiLogFont
	if err != nil || len(name) > len(logFont.FaceName) {
		return 0
	}
	// ANSI_CHARSET, DEFAULT_QUALITY (the smoothing the user has set: ClearType, or the
	// grayscale or aliased text they chose instead), FIXED_PITCH | FF_MODERN.
	logFont = gdiLogFont{Height: key.height, Weight: key.weight, CharSet: 0, Quality: 0, Pitch: 0x31}
	copy(logFont.FaceName[:], name)
	if key.italic {
		logFont.Italic = 1
	}
	if key.underline {
		logFont.Underline = 1
	}
	if key.strike {
		logFont.StrikeOut = 1
	}
	font, _, _ := gdiCreateFont.Call(uintptr(unsafe.Pointer(&logFont)))
	if font == 0 {
		return 0
	}
	dc := rasterizer.bitmap.dc
	previous, _, _ := selectObject.Call(dc, font)
	var face [len(logFont.FaceName)]uint16
	length, _, _ := gdiGetTextFace.Call(dc, uintptr(len(face)), uintptr(unsafe.Pointer(&face[0])))
	if previous != 0 {
		selectObject.Call(dc, previous)
	}
	if previous == 0 || length == 0 || !strings.EqualFold(syscall.UTF16ToString(face[:]), key.family) {
		deleteObject.Call(font)
		return 0
	}
	rasterizer.fonts[key] = font
	return font
}

// draw draws the pieces of a line that GDI has a font for and whose colour has the brightness
// of light, as ClearType coverage: white text on black for light colours and black on white
// for dark ones, because GDI, like Direct2D, thins light text and thickens dark text, so
// the coverage of a glyph depends on which of the two it is. The returned bitmap is
// valid until the next draw. Each piece stands on the cells of the grid at pitch, with the
// baseline of the line on row baseline.
func (rasterizer *gdiGridRasterizer) draw(pieces []gridPiece, light bool, pitch float32, baseline, width, height int) (*textBitmap, error) {
	bitmap, err := rasterizer.bind(width, height)
	if err != nil {
		return nil, err
	}
	background, color := d2dColor{A: 1}, uintptr(0x00ffffff)
	if !light {
		background, color = d2dColor{R: 1, G: 1, B: 1, A: 1}, 0
	}
	fill(bitmap, width, height, background)
	gdiSetBkMode.Call(bitmap.dc, 1) // TRANSPARENT
	gdiSetTextColor.Call(bitmap.dc, color)
	gdiSetTextAlign.Call(bitmap.dc, 24) // TA_LEFT | TA_BASELINE
	var stock uintptr
	var drawErr error
	for index := range pieces {
		piece := &pieces[index]
		if piece.font == 0 || isLight(piece.color) != light || piece.text == "" {
			continue
		}
		previous, _, _ := selectObject.Call(bitmap.dc, piece.font)
		if stock == 0 {
			// The context's own font, which it gets back: a font that is still selected
			// when the context goes cannot be deleted.
			stock = previous
		}
		if err := rasterizer.drawPiece(bitmap, piece, pitch, baseline); err != nil && drawErr == nil {
			drawErr = err
		}
	}
	if stock != 0 {
		selectObject.Call(bitmap.dc, stock)
	}
	// The bitmap is a DIB section, which GDI draws into in batches: what is read from its
	// memory is what GDI has finished.
	gdiFlush.Call()
	return bitmap, drawErr
}

// drawPiece draws a piece of printable ASCII, with its font selected, so that character k
// stands on cell column+k of the grid: the advances are the differences of the rounded
// cell edges, which keeps a fractional pitch (125% scaling) from drifting along the line.
func (rasterizer *gdiGridRasterizer) drawPiece(bitmap *textBitmap, piece *gridPiece, pitch float32, baseline int) error {
	count := len(piece.text)
	if cap(rasterizer.text) < count {
		rasterizer.text = make([]uint16, count)
	}
	text := rasterizer.text[:count]
	for index := range text {
		text[index] = uint16(piece.text[index])
	}
	left, step := float64(piece.left), float64(pitch)
	var advances *int32
	if step > 0 && math.Floor(step+0.5) >= 1 {
		if cap(rasterizer.spacing) < count {
			rasterizer.spacing = make([]int32, count)
		}
		spacing := rasterizer.spacing[:count]
		edge := math.Floor(left + 0.5)
		for index := range spacing {
			next := math.Floor(left + float64(index+1)*step + 0.5)
			spacing[index] = int32(next - edge)
			edge = next
		}
		advances = &spacing[0]
	}
	// The pointers are converted in the call, as the rules of unsafe.Pointer require.
	drawn, _, callErr := gdiExtTextOut.Call(bitmap.dc, uintptr(int32(math.Floor(left+0.5))), uintptr(int32(baseline)), 0, 0,
		uintptr(unsafe.Pointer(&text[0])), uintptr(count), uintptr(unsafe.Pointer(advances)))
	if drawn == 0 {
		return fmt.Errorf("windows: draw grid text with GDI: %w", callErr)
	}
	return nil
}

// useGDI sets the font of every piece that GDI draws, and clears it for the others. size is
// the device size of the line.
func (rasterizer *directRasterizer) useGDI(pieces []gridPiece, family string, size float32, width, height int) error {
	emHeight := int32(math.Floor(float64(size) + 0.5))
	if rasterizer.directOnly || emHeight < 1 {
		return nil
	}
	for index := range pieces {
		piece := &pieces[index]
		// GDI shapes nothing, and has no font to fall back to for what the font lacks.
		if piece.run.Ligatures || !printableASCII(piece.text) {
			continue
		}
		if rasterizer.gdi == nil {
			rasterizer.gdi = &gdiGridRasterizer{}
		}
		if _, err := rasterizer.gdi.bind(width, height); err != nil {
			return err
		}
		piece.font = rasterizer.gdi.font(gdiFontKey{
			family: family, height: -emHeight, weight: int32(nativeFontWeight(piece.weight)),
			italic: piece.run.Italic, underline: piece.run.Underline, strike: piece.run.Strike,
		})
	}
	return nil
}
