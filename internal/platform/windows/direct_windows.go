package windows

import (
	"fmt"
	"math"
	"syscall"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/ui"
)

// Labels are drawn with DirectWrite through Direct2D, the engine that draws the text of
// Explorer, Edge and every modern Windows application: its glyph shapes, kerning, font
// fallback, color fonts, and the user's ClearType settings (rendering mode and grid
// fitting recommended per font and size, pixel geometry, level, gamma and enhanced
// contrast). Direct2D renders into a GDI bitmap; the label texture keeps what it drew.

var d2dFactoryCreate = systemDLL("d2d1.dll").NewProc("D2D1CreateFactory")
var d2dFactoryIID = guid{0x06152247, 0x6f50, 0x465a, [8]byte{0x92, 0x45, 0x11, 0x8b, 0xfd, 0x3b, 0x60, 0x07}}
var writeLayout1IID = guid{0x9064d822, 0x80a7, 0x465c, [8]byte{0xa9, 0x86, 0xdf, 0x65, 0xf7, 0x8b, 0x8f, 0xeb}}

// Method slots of ID2D1Factory, ID2D1RenderTarget and IDWriteTextLayout(1).
const (
	d2dFactoryCreateDCTarget = 16
	d2dCreateSolidColorBrush = 8
	d2dSetColor              = 8 // ID2D1SolidColorBrush
	d2dDrawTextLayout        = 28
	d2dSetTransform          = 30
	d2dSetTextAntialiasMode  = 34
	d2dBeginDraw             = 48
	d2dEndDraw               = 49
	d2dBindDC                = 57
	writeSetTextAlignment    = 3 // IDWriteTextFormat
	writeSetReadingDirection = 6
	writeGetLineMetrics      = 59
	writeGetMetrics          = 60
	writeGetClusterMetrics   = 62
	writeSetCharacterSpacing = 69
	writeSetUnderline        = 36
	writeSetStrikethrough    = 37
)

// D2D1_TEXT_ANTIALIAS_MODE, D2D1_ALPHA_MODE and D2D1_DRAW_TEXT_OPTIONS.
const (
	d2dAntialiasDefault   = 0
	d2dAntialiasGrayscale = 2
	d2dAlphaPremultiplied = 1
	d2dAlphaIgnore        = 3
	d2dEnableColorFont    = 4
)

type d2dColor struct{ R, G, B, A float32 }
type d2dRenderTargetProperties struct {
	Type              uint32
	Format, AlphaMode uint32
	DPIX, DPIY        float32
	Usage, MinLevel   uint32
}

// directTarget is a Direct2D render target that draws into a GDI bitmap. Binding a bitmap
// and synchronizing it with the target cost more than drawing a line of text, so one
// bitmap is bound and kept (scratch), grown when a label needs more, and every label is drawn
// into the corner of it.
type directTarget struct {
	target, brush uintptr
	scratch       *textBitmap
	width, height int // of the scratch bitmap
}

// d2dMatrix is D2D1_MATRIX_3X2_F: x' = M11*x + M21*y + DX, y' = M12*x + M22*y + DY.
type d2dMatrix struct {
	M11, M12, M21, M22, DX, DY float32
}

var identityMatrix = d2dMatrix{M11: 1, M22: 1}

func newDirectTarget(factory uintptr, alphaMode uint32) (*directTarget, error) {
	// Software rendering, so that the cost and the result do not depend on a graphics
	// device; 96 dpi: label sizes are device pixels already.
	properties := d2dRenderTargetProperties{Type: 1, Format: 87, AlphaMode: alphaMode, DPIX: 96, DPIY: 96}
	var target uintptr
	if err := hresult(comCall(factory, d2dFactoryCreateDCTarget, uintptr(unsafe.Pointer(&properties)), uintptr(unsafe.Pointer(&target)))); err != nil {
		return nil, fmt.Errorf("windows: create Direct2D target: %w", err)
	}
	white := d2dColor{R: 1, G: 1, B: 1, A: 1}
	direct := &directTarget{target: target}
	if err := hresult(comCall(target, d2dCreateSolidColorBrush, uintptr(unsafe.Pointer(&white)), 0, uintptr(unsafe.Pointer(&direct.brush)))); err != nil {
		direct.close()
		return nil, fmt.Errorf("windows: create Direct2D brush: %w", err)
	}
	return direct, nil
}

func (target *directTarget) close() {
	release(&target.brush)
	release(&target.target)
	if target.scratch != nil {
		target.scratch.close()
		target.scratch = nil
	}
}

// bind makes the top left width by height pixels of a bitmap the target draws into, growing
// the scratch bitmap when it is too small, and returns it.
func (target *directTarget) bind(width, height int) (*textBitmap, error) {
	if target.scratch == nil || width > target.width || height > target.height {
		grown, err := newTextBitmap(max(width, target.width, 256), max(height, target.height, 32))
		if err != nil {
			return nil, err
		}
		bounds := rect{Right: int32(grown.Width), Bottom: int32(grown.Height)}
		if err := hresult(comCall(target.target, d2dBindDC, grown.dc, uintptr(unsafe.Pointer(&bounds)))); err != nil {
			grown.close()
			return nil, fmt.Errorf("windows: bind Direct2D target: %w", err)
		}
		if target.scratch != nil {
			target.scratch.close()
		}
		target.scratch, target.width, target.height = grown, grown.Width, grown.Height
	}
	return target.scratch, nil
}

// begin sets the color and the antialiasing mode and starts drawing. The scratch bitmap
// holds the background: Direct2D draws over it.
func (target *directTarget) begin(foreground d2dColor, mode uint32) {
	comCall(target.brush, d2dSetColor, uintptr(unsafe.Pointer(&foreground)))
	comCall(target.target, d2dSetTextAntialiasMode, uintptr(mode))
	comCall(target.target, d2dBeginDraw)
}

func (target *directTarget) setColor(color d2dColor) {
	comCall(target.brush, d2dSetColor, uintptr(unsafe.Pointer(&color)))
}

func (target *directTarget) setTransform(matrix d2dMatrix) {
	comCall(target.target, d2dSetTransform, uintptr(unsafe.Pointer(&matrix)))
}

func (target *directTarget) draw(x, y float32, layout uintptr, options uint32) {
	comCall(target.target, d2dDrawTextLayout, pointArgument(x, y), layout, target.brush, uintptr(options))
}

func (target *directTarget) end() error {
	if err := hresult(comCall(target.target, d2dEndDraw, 0, 0)); err != nil {
		return fmt.Errorf("windows: end Direct2D drawing: %w", err)
	}
	return nil
}

func pointArgument(x, y float32) uintptr {
	return uintptr(math.Float32bits(x)) | uintptr(math.Float32bits(y))<<32
}

// directRasterizer draws labels with DirectWrite. It is not safe for concurrent use.
type directRasterizer struct {
	// services is the process's DirectWrite factory, shared with the text engine.
	services    *textServices
	engine      *textEngine
	factory     uintptr
	opaque      *directTarget // alpha ignored: ClearType is allowed
	translucent *directTarget // premultiplied alpha: color glyphs, grayscale
	spacing     func(this uintptr, leading, trailing, minimum float32, span uint64) uintptr
	output      []byte // the pixels of the last label, reused
	pieces      []gridPiece
	spans       []colorSpan // the colors of the last ClearType grid label, valid until the next call
	fallbacks   []faceFallback
}

func newDirectRasterizer() (*directRasterizer, error) {
	engine, err := theTextEngine()
	if err != nil {
		return nil, err
	}
	rasterizer := &directRasterizer{engine: engine, services: &engine.services}
	options := struct{ Level uint32 }{}
	status, _, _ := d2dFactoryCreate.Call(1, uintptr(unsafe.Pointer(&d2dFactoryIID)), uintptr(unsafe.Pointer(&options)), uintptr(unsafe.Pointer(&rasterizer.factory)))
	if err := hresult(status); err != nil {
		return nil, fmt.Errorf("windows: create Direct2D factory: %w", err)
	}
	if rasterizer.opaque, err = newDirectTarget(rasterizer.factory, d2dAlphaIgnore); err == nil {
		rasterizer.translucent, err = newDirectTarget(rasterizer.factory, d2dAlphaPremultiplied)
	}
	if err != nil {
		rasterizer.close()
		return nil, err
	}
	return rasterizer, nil
}

func (rasterizer *directRasterizer) close() {
	if rasterizer.opaque != nil {
		rasterizer.opaque.close()
	}
	if rasterizer.translucent != nil {
		rasterizer.translucent.close()
	}
	rasterizer.releaseFontFallbacks()
	release(&rasterizer.factory)
}

// textBitmap is a 32-bit GDI bitmap selected into a device context, which Direct2D
// draws into and the label texture is read from.
type textBitmap struct {
	dc, bitmap, previous uintptr
	Width, Height        int
	Pixels               []byte
}

func newTextBitmap(width, height int) (*textBitmap, error) {
	dc, _, dcErr := createCompatibleDC.Call(0)
	if dc == 0 {
		return nil, fmt.Errorf("windows: create text device context: %w", dcErr)
	}
	info := bitmapInfo{Header: bitmapInfoHeader{Size: uint32(unsafe.Sizeof(bitmapInfoHeader{})), Width: int32(width), Height: -int32(height), Planes: 1, Bits: 32}}
	var bits unsafe.Pointer
	bitmap, _, bitmapErr := createDIBSection.Call(dc, uintptr(unsafe.Pointer(&info)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if bitmap == 0 || bits == nil {
		deleteDC.Call(dc)
		return nil, fmt.Errorf("windows: create text bitmap: %w", bitmapErr)
	}
	previous, _, selectErr := selectObject.Call(dc, bitmap)
	if previous == 0 || previous == ^uintptr(0) {
		deleteObject.Call(bitmap)
		deleteDC.Call(dc)
		return nil, fmt.Errorf("windows: select text bitmap: %w", selectErr)
	}
	return &textBitmap{dc: dc, bitmap: bitmap, previous: previous, Width: width, Height: height, Pixels: unsafe.Slice((*byte)(bits), width*height*4)}, nil
}

func (bitmap *textBitmap) close() {
	selectObject.Call(bitmap.dc, bitmap.previous)
	deleteObject.Call(bitmap.bitmap)
	deleteDC.Call(bitmap.dc)
}

// rasterize draws the label's words into a width by height bitmap and returns its pixels,
// which stay valid until the next call. With lcd the pixels are the per-channel coverage of
// ClearType text (red, green and blue in the texture's channels, alpha their luminance), to be
// blended with the label color per channel (textMode 1); otherwise premultiplied BGRA in the
// label's color. skip is the whole device columns of the line left of the bitmap, for a line
// whose clip cuts that much of its start; the text is drawn that many columns left, so the
// bitmap holds only the visible part.
func (rasterizer *directRasterizer) rasterize(label platform.TextLabel, scale float32, width, height int, allowLCD bool, skip int) (pixels []byte, lcd bool, err error) {
	if cap(rasterizer.output) < width*height*4 {
		rasterizer.output = make([]byte, width*height*4)
	}
	pixels = rasterizer.output[:width*height*4]
	clear(pixels)
	if label.Value == "" {
		return pixels, false, nil
	}
	size := label.FontSize * scale
	if size <= 0 || math.IsNaN(float64(size)) || math.IsInf(float64(size), 0) {
		return nil, false, fmt.Errorf("windows: invalid label font size")
	}
	rasterizer.spans = rasterizer.spans[:0]
	if label.Mono || len(label.Runs) > 0 {
		if allowLCD && !platform.ColorGlyphs(label.Value) {
			return pixels, true, rasterizer.gridLCD(pixels, label, size, width, height)
		}
		return pixels, false, rasterizer.grid(pixels, label, size, width, height)
	}
	lcd = allowLCD && !platform.ColorGlyphs(label.Value)
	baseline := float32(platform.LabelBaseline(height, float64(size)))
	// The line is the one layout used for its width, at the device size.
	err = rasterizer.engine.draw(label.Value, size, label.Weight, func(line *shapedLine) error {
		return rasterizer.wholeLine(pixels, line, label, width, height, baseline, lcd, skip)
	})
	if err != nil {
		return nil, false, err
	}
	return pixels, lcd, nil
}

// wholeLine draws one shaped line into pixels, as one piece: with grayscale color glyphs,
// or as the per-channel coverage of ClearType text. skip is the device columns shifted off
// the left, for a bitmap that holds only the part of the line its clip shows.
func (rasterizer *directRasterizer) wholeLine(pixels []byte, line *shapedLine, label platform.TextLabel, width, height int, baseline float32, lcd bool, skip int) error {
	if !lcd {
		foreground := d2dColor{R: float32(label.Color.Red), G: float32(label.Color.Green), B: float32(label.Color.Blue), A: float32(label.Color.Alpha)}
		scratch, err := rasterizer.pass(rasterizer.translucent, line, width, height, baseline, d2dColor{}, foreground, d2dAntialiasGrayscale, d2dEnableColorFont, skip)
		if err != nil {
			return err
		}
		for row := 0; row < height; row++ {
			copy(pixels[row*width*4:(row+1)*width*4], scratch.Pixels[row*scratch.Width*4:row*scratch.Width*4+width*4])
		}
		return nil
	}
	// The coverage of ClearType text does not depend on the color of the text but on the
	// brightness of each of its channels: Direct2D thins light text and thickens dark text
	// (gamma and enhanced contrast). Light text is drawn white on black and dark text black on
	// white, and the coverage of a channel is read from the drawing made for its brightness.
	// Text of one brightness (gray text, nearly all text) takes one drawing.
	var light [3]bool // blue, green, red: the order of the bitmap
	var anyLight, anyDark bool
	for channel, value := range [3]float64{label.Color.Blue, label.Color.Green, label.Color.Red} {
		light[channel] = value > 0.5
		anyLight, anyDark = anyLight || light[channel], anyDark || !light[channel]
	}
	for _, drawLight := range [2]bool{true, false} {
		if drawLight && !anyLight || !drawLight && !anyDark {
			continue
		}
		background, foreground := d2dColor{A: 1}, d2dColor{R: 1, G: 1, B: 1, A: 1}
		if !drawLight {
			background, foreground = foreground, background
		}
		scratch, err := rasterizer.pass(rasterizer.opaque, line, width, height, baseline, background, foreground, d2dAntialiasDefault, 0, skip)
		if err != nil {
			return err
		}
		if anyLight && anyDark {
			coverageOfChannels(pixels, scratch, width, height, drawLight, light)
		} else {
			coverageOfPixels(pixels, scratch, width, height, drawLight)
		}
	}
	if anyLight && anyDark {
		for index := 0; index+3 < len(pixels); index += 4 {
			pixels[index+3] = luminance(pixels[index+2], pixels[index+1], pixels[index])
		}
	}
	return nil
}

// pass draws the shaped line on the target's scratch bitmap, filled with background, and returns it.
func (rasterizer *directRasterizer) pass(target *directTarget, line *shapedLine, width, height int, baseline float32, background, foreground d2dColor, mode, options uint32, skip int) (*textBitmap, error) {
	scratch, err := target.bind(width, height)
	if err != nil {
		return nil, err
	}
	fill(scratch, width, height, background)
	target.begin(foreground, mode)
	// The glyphs start at the layout's left edge (0 but for a layout whose text is aligned
	// away from it), less the whole columns before the clip, with the baseline of the line
	// on row baseline.
	target.draw(-line.left-float32(skip), baseline-line.ascent, line.layout, options)
	if err := target.end(); err != nil {
		return nil, err
	}
	return scratch, nil
}

// fill sets the top left width by height pixels of the bitmap to color.
func fill(bitmap *textBitmap, width, height int, color d2dColor) {
	value := [4]byte{byte(color.B*255 + 0.5), byte(color.G*255 + 0.5), byte(color.R*255 + 0.5), byte(color.A*255 + 0.5)}
	for row := 0; row < height; row++ {
		line := bitmap.Pixels[row*bitmap.Width*4 : row*bitmap.Width*4+width*4]
		for index := 0; index+3 < len(line); index += 4 {
			copy(line[index:index+4], value[:])
		}
	}
}

// luminance is the alpha a pixel of per-channel coverage has.
func luminance(red, green, blue byte) byte {
	return byte((uint32(red)*77 + uint32(green)*151 + uint32(blue)*28 + 128) >> 8)
}

// coverageOfPixels takes the coverage of all channels, and the alpha, from what Direct2D drew
// on an opaque background: white text on black is the coverage itself, black text on white its
// complement.
func coverageOfPixels(pixels []byte, bitmap *textBitmap, width, height int, drawnLight bool) {
	var flip byte
	if !drawnLight {
		flip = 255
	}
	for row := 0; row < height; row++ {
		source := bitmap.Pixels[row*bitmap.Width*4 : row*bitmap.Width*4+width*4]
		out := pixels[row*width*4 : (row+1)*width*4]
		for index := 0; index+3 < len(out); index += 4 {
			blue, green, red := source[index]^flip, source[index+1]^flip, source[index+2]^flip
			out[index], out[index+1], out[index+2], out[index+3] = blue, green, red, luminance(red, green, blue)
		}
	}
}

// coverageOfChannels takes the coverage of the channels (blue, green, red) whose brightness
// the drawing is for.
func coverageOfChannels(pixels []byte, bitmap *textBitmap, width, height int, drawnLight bool, channelLight [3]bool) {
	var flip byte
	if !drawnLight {
		flip = 255
	}
	for row := 0; row < height; row++ {
		source := bitmap.Pixels[row*bitmap.Width*4:]
		out := pixels[row*width*4 : (row+1)*width*4]
		for index := 0; index+3 < len(out); index += 4 {
			for channel := 0; channel < 3; channel++ {
				if channelLight[channel] == drawnLight {
					out[index+channel] = source[index+channel] ^ flip
				}
			}
		}
	}
}

// grid draws a fixed-pitch label into pixels: a terminal line, whose runs have a color, a
// weight and decorations of their own, each character on its cell of the grid (Viva's, not the
// font's), or the whole value of a monospaced label. Colors are applied as the text is drawn
// (premultiplied BGRA, grayscale antialiasing), so a line is one texture however many colors it holds.
func (rasterizer *directRasterizer) grid(pixels []byte, label platform.TextLabel, size float32, width, height int) error {
	family := monoFamily(label.MonoFace)
	fallback := rasterizer.fontFallback(label.MonoFace)
	pitch := label.CellWidth * float32(size/label.FontSize)
	baseline := float32(platform.LabelBaseline(height, float64(size)))
	target := rasterizer.translucent
	scratch, err := target.bind(width, height)
	if err != nil {
		return err
	}
	fill(scratch, width, height, d2dColor{})
	target.begin(d2dColor{A: 1}, d2dAntialiasGrayscale)
	var drawErr error
	for _, piece := range rasterizer.gridPieces(label, pitch) {
		target.setColor(piece.color)
		if err := rasterizer.drawRun(target, family, fallback, size, piece.weight, piece.run, piece.text, piece.left, piece.span, pitch, baseline); err != nil && drawErr == nil {
			drawErr = err
		}
	}
	if err := target.end(); err != nil && drawErr == nil {
		drawErr = err
	}
	if drawErr != nil {
		return drawErr
	}
	for row := 0; row < height; row++ {
		copy(pixels[row*width*4:(row+1)*width*4], scratch.Pixels[row*scratch.Width*4:row*scratch.Width*4+width*4])
	}
	return nil
}

// drawRun draws a run of a grid line starting at left: printable ASCII characters advance by
// exactly the pitch of the grid, a single wider character is centered on its cells, and
// the run's decorations are DirectWrite's own.
func (rasterizer *directRasterizer) drawRun(target *directTarget, family string, fallback uintptr, size, weight float32, run ui.TextRun, piece string, left, span, pitch, baseline float32) error {
	format, err := rasterizer.services.formatStyle(family, size, weight, run.Italic)
	if err != nil {
		return err
	}
	defer release(&format)
	setFontFallback(format, fallback)
	units, err := syscall.UTF16FromString(piece)
	if err != nil {
		return err
	}
	var layout uintptr
	if err := hresult(rasterizer.services.createLayout(rasterizer.services.write, &units[0], uint32(len(units)-1), format, 1<<20, 1<<20, &layout)); err != nil {
		return fmt.Errorf("windows: lay out grid text: %w", err)
	}
	defer release(&layout)
	var metrics textMetrics
	if err := hresult(comCall(layout, writeGetMetrics, uintptr(unsafe.Pointer(&metrics)))); err != nil {
		return err
	}
	var line lineMetrics
	var lines uint32
	if err := hresult(comCall(layout, writeGetLineMetrics, uintptr(unsafe.Pointer(&line)), 1, uintptr(unsafe.Pointer(&lines)))); err != nil && lines == 0 {
		return err
	}
	all := uint64(len(units)-1) << 32
	if run.Underline {
		comCall(layout, writeSetUnderline, 1, uintptr(all))
	}
	if run.Strike {
		comCall(layout, writeSetStrikethrough, 1, uintptr(all))
	}
	x := left
	scaled := false
	switch {
	case run.Ligatures && pitch > 0:
		// Shape the run as one line and scale it into its cell span. SetTransform
		// is the only way to fit a substituted glyph that is not an exact
		// multiple of the cell width; it also disables ClearType, which is why
		// it is used only when the widths differ.
		// Terminal padding occupies cells too. Width excludes trailing spaces
		// and would stretch a short command across its following blank cells.
		scaled = metrics.WidthTrailing > 0 && span > 0 && metrics.WidthTrailing != span
		if scaled {
			scale := span / metrics.WidthTrailing
			target.setTransform(d2dMatrix{M11: scale, M22: 1, DX: left * (1 - scale)})
		}
	case pitch > 0 && printableASCII(piece):
		// Every character stands on its cell, whatever the advance of the font.
		advance := metrics.WidthTrailing / float32(len(units)-1)
		if reduction := advance - pitch; reduction != 0 {
			var extended uintptr
			if comCall(layout, 0, uintptr(unsafe.Pointer(&writeLayout1IID)), uintptr(unsafe.Pointer(&extended))) == 0 && extended != 0 {
				if rasterizer.spacing == nil {
					purego.RegisterFunc(&rasterizer.spacing, vtable(extended, writeSetCharacterSpacing))
				}
				rasterizer.spacing(extended, 0, -reduction, 0, all)
				release(&extended)
			}
		}
	case run.Cells > 0 && isSingleCharacter(piece):
		x = left + (span-metrics.Width)/2
	}
	target.draw(x, baseline-line.Baseline, layout, d2dEnableColorFont)
	if scaled {
		target.setTransform(identityMatrix)
	}
	return nil
}
