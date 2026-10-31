package windows

import (
	"fmt"
	"math"
	"syscall"
	"unicode"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

var writeFactory = systemDLL("dwrite.dll").NewProc("DWriteCreateFactory")
var writeIID = guid{0xb859ee5a, 0xd838, 0x4b5b, [8]byte{0xa2, 0xe8, 0x1a, 0xdc, 0x7d, 0x93, 0xdb, 0x48}}
var graphics = systemDLL("gdi32.dll")
var createCompatibleDC = graphics.NewProc("CreateCompatibleDC")
var deleteDC = graphics.NewProc("DeleteDC")
var createDIBSection = graphics.NewProc("CreateDIBSection")
var selectObject = graphics.NewProc("SelectObject")
var deleteObject = graphics.NewProc("DeleteObject")

type textServices struct {
	write        uintptr
	createFormat func(uintptr, *uint16, uintptr, uint32, uint32, uint32, float32, *uint16, *uintptr) uintptr
	createLayout func(uintptr, *uint16, uint32, uintptr, float32, float32, *uintptr) uintptr
}
type labelTexture struct {
	label         platform.TextLabel
	runs          []ui.TextRun // the label's own copy of its runs
	texture, view uintptr
	scale         float32
	// width and height are the texture's size in device pixels.
	width, height int
	// skip is the device columns of the label's full texture that start before its
	// clip and so are not rasterized.
	skip int
	// lcd is set when the texture holds the per-channel coverage of ClearType text, to
	// be blended with the label's color (textMode 1), not premultiplied color.
	lcd bool
	// spans are the columns of an lcd grid label's texture that are text of one color;
	// empty for a label that is all in label.Color.
	spans []colorSpan
}
type bitmapInfoHeader struct {
	Size               uint32
	Width, Height      int32
	Planes, Bits       uint16
	Compression, Image uint32
	XPels, YPels       int32
	Colors, Important  uint32
}
type bitmapInfo struct {
	Header bitmapInfoHeader
	Colors [1]uint32
}
type lineMetrics struct {
	Length, Trailing, Newline uint32
	Height, Baseline          float32
	Trimmed                   int32
}
type textMetrics struct {
	Left, Top, Width, WidthTrailing, Height, LayoutWidth, LayoutHeight float32
	Depth, Lines                                                       uint32
}

// monoFamily is the family a face draws in first. The families that draw what it
// lacks are the face's font fallback (see directRasterizer.fontFallback); the cell is
// always measured in this one.
func monoFamily(face ui.MonoFace) string {
	if name := ui.MonoFamilyName(face); name != "" {
		return name
	}
	if ui.MonoIsClassic(face) {
		return "Courier New"
	}
	return "Consolas"
}

func (services *textServices) initializeWrite() error {
	if services.write != 0 {
		return nil
	}
	status, _, _ := writeFactory.Call(0, uintptr(unsafe.Pointer(&writeIID)), uintptr(unsafe.Pointer(&services.write)))
	if err := hresult(status); err != nil {
		return err
	}
	purego.RegisterFunc(&services.createFormat, vtable(services.write, 15))
	purego.RegisterFunc(&services.createLayout, vtable(services.write, 18))
	return nil
}
func (services *textServices) format(size, weight float32) (uintptr, error) {
	return services.formatFamily("Segoe UI", size, weight)
}
func (services *textServices) formatFamily(name string, size, weight float32) (uintptr, error) {
	return services.formatStyle(name, size, weight, false)
}

// formatStyle is a DirectWrite text format of a family at a Viva weight, in italics or not.
func (services *textServices) formatStyle(name string, size, weight float32, italic bool) (uintptr, error) {
	family, _ := syscall.UTF16PtrFromString(name)
	locale, _ := syscall.UTF16PtrFromString("")
	var format uintptr
	style := uint32(0) // DWRITE_FONT_STYLE_NORMAL
	if italic {
		style = 2 // DWRITE_FONT_STYLE_ITALIC
	}
	status := services.createFormat(services.write, family, 0, nativeFontWeight(weight), style, 5, size, locale, &format)
	return format, hresult(status)
}

// nativeFontWeight is the DirectWrite weight of a Viva weight: regular, medium, semibold and
// bold are the faces Segoe UI and Consolas have.
func nativeFontWeight(weight float32) uint32 {
	switch {
	case weight >= 0.35:
		return 700
	case weight >= 0.20:
		return 600
	case weight >= 0.10:
		return 500
	default:
		return 400
	}
}
func (services *textServices) close() {
	release(&services.write)
	services.createFormat = nil
	services.createLayout = nil
}
func (label *labelTexture) close() {
	release(&label.view)
	release(&label.texture)
	label.label = platform.TextLabel{}
	label.runs = label.runs[:0]
	label.scale = 0
	label.width, label.height = 0, 0
	label.skip = 0
	label.lcd = false
	label.spans = label.spans[:0]
}

// quad is where the label's texture is drawn, in logical pixels: its top left and its size
// are whole device pixels, so every texel lands on one pixel and the sampler blends none
// with its neighbour. At a fractional display scale the label's logical bounds are
// usually between pixels, and drawing there blurs ClearType text. Text takes the texture's
// own size; an image takes the device pixels its logical bounds cover.
func (label *labelTexture) quad(scale float32) [4]float32 {
	l := label.label
	x, y := label.pixelOrigin(scale)
	width, height := label.width, label.height
	if len(l.Bitmap) > 0 {
		width, height = platform.DeviceExtent(l.X, l.Width, scale), platform.DeviceExtent(l.Y, l.Height, scale)
	}
	return [4]float32{float32(x+label.skip) / scale, float32(y) / scale, float32(width) / scale, float32(height) / scale}
}

// pixelOrigin is the device pixel the label's texture starts at.
func (label *labelTexture) pixelOrigin(scale float32) (x, y int) {
	l := label.label
	if len(l.Bitmap) > 0 {
		return platform.DevicePosition(l.X, 0, scale), platform.DevicePosition(l.Y, 0, scale)
	}
	return platform.DevicePosition(l.X, l.OriginX, scale), platform.DevicePosition(l.Y, l.OriginY, scale)
}

// window is the part of the label's full texture that its clip can show, in device
// pixels. A clipped line rasterizes only inside its clip, so a long scrolled
// single-line field costs the visible width, not the whole line. The text does not
// move: the skip is a whole number of device columns, so every glyph texel keeps the
// phase it has when the whole texture is drawn. Labels that are not clipped, and
// images and grid lines, keep their whole texture.
func (label *labelTexture) window(scale float32) (skip, width int, ok bool) {
	full := int(math.Ceil(float64(label.label.Width * scale)))
	if !label.label.Clipped || label.label.Mono || len(label.label.Runs) > 0 || len(label.label.Bitmap) > 0 {
		return 0, full, full > 0
	}
	x, _ := label.pixelOrigin(scale)
	left := max(int64(math.Floor(float64(label.label.Clip.X*scale))), 0)
	right := min(int64(math.Ceil(float64((label.label.Clip.X+label.label.Clip.Width)*scale))), int64(x)+int64(full))
	return platform.LabelWindow(x, full, int(left), int(right))
}

func (label *labelTexture) update(next platform.TextLabel) {
	if labelContentChanged(label.label, next) {
		release(&label.view)
		release(&label.texture)
	}
	// The runs belong to the caller's frame; keep a copy for the next comparison.
	label.runs = append(label.runs[:0], next.Runs...)
	label.label = next
	label.label.Runs = label.runs
}

func labelContentChanged(previous, next platform.TextLabel) bool {
	bitmapChanged := len(previous.Bitmap) > 0 || len(next.Bitmap) > 0
	return previous.Value != next.Value || (!bitmapChanged && (previous.Width != next.Width || previous.Height != next.Height)) || previous.FontSize != next.FontSize || previous.Weight != next.Weight || previous.Color != next.Color ||
		previous.Mono != next.Mono || previous.MonoFace != next.MonoFace || previous.CellWidth != next.CellWidth || !sameRuns(previous.Runs, next.Runs) ||
		previous.BitmapID != next.BitmapID || previous.BitmapVersion != next.BitmapVersion || previous.BitmapWidth != next.BitmapWidth ||
		previous.BitmapHeight != next.BitmapHeight || previous.BitmapStride != next.BitmapStride || !sameBitmapStorage(previous.Bitmap, next.Bitmap)
}

func sameBitmapStorage(left, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	return len(left) == 0 || &left[0] == &right[0]
}

func sameRuns(left, right []ui.TextRun) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
func (renderer *renderer) rasterize(label *labelTexture, scale float32) error {
	skip, width, ok := label.window(scale)
	if !ok {
		release(&label.view)
		release(&label.texture)
		label.scale, label.width, label.height, label.skip = scale, 0, 0, 0
		return nil
	}
	height := int(math.Ceil(float64(label.label.Height * scale)))
	if len(label.label.Bitmap) > 0 {
		width, height = label.label.BitmapWidth, label.label.BitmapHeight
		// A bitmap drawn smaller than it is stored is averaged down to the device pixels it
		// covers; sampling a few of its texels would alias.
		if shown, high := platform.DeviceExtent(label.label.X, label.label.Width, scale), platform.DeviceExtent(label.label.Y, label.label.Height, scale); shown > 0 && high > 0 && shown <= width && high <= height && (shown < width || high < height) {
			width, height = shown, high
		}
	}
	if width <= 0 || height <= 0 || width > 16384 || height > 16384 || width*height > 16*1024*1024 {
		return fmt.Errorf("windows: label texture exceeds bounded size")
	}
	var pixels []byte
	var err error
	label.lcd = false
	label.spans = label.spans[:0]
	if len(label.label.Bitmap) > 0 {
		if pixels, err = bitmapPixels(label.label, width, height); err != nil {
			return err
		}
	} else {
		if renderer.direct == nil {
			if renderer.direct, err = newDirectRasterizer(); err != nil {
				return err
			}
		}
		// ClearType blends per channel with what is under the text: a window's back
		// buffer is opaque, an offscreen target cleared translucent is not.
		pixels, label.lcd, err = renderer.direct.rasterize(label.label, scale, width, height, renderer.window != 0 || renderer.color.Alpha >= 1, skip)
		if err != nil {
			return err
		}
		label.spans = append(label.spans, renderer.direct.spans...)
	}
	description := textureDesc{Width: uint32(width), Height: uint32(height), Mips: 1, Array: 1, Format: 87, Samples: 1, Usage: 1, Bind: 8}
	data := resourceData{Data: unsafe.Pointer(&pixels[0]), Pitch: uint32(width * 4)}
	release(&label.view)
	release(&label.texture)
	if err := hresult(comCall(renderer.device, 5, uintptr(unsafe.Pointer(&description)), uintptr(unsafe.Pointer(&data)), uintptr(unsafe.Pointer(&label.texture)))); err != nil {
		return err
	}
	if err := hresult(comCall(renderer.device, 7, label.texture, 0, uintptr(unsafe.Pointer(&label.view)))); err != nil {
		return err
	}
	label.scale = scale
	label.width, label.height = width, height
	label.skip = skip
	renderer.labelRebuilds++
	return nil
}

// bitmapPixels is the pixels of an image label at width by height: a tight copy of the
// bitmap, or the bitmap averaged down to that size when it is smaller than the bitmap.
func bitmapPixels(label platform.TextLabel, width, height int) ([]byte, error) {
	if width <= 0 || height <= 0 || label.BitmapStride < label.BitmapWidth*4 {
		return nil, fmt.Errorf("windows: invalid bitmap dimensions")
	}
	required := (label.BitmapHeight-1)*label.BitmapStride + label.BitmapWidth*4
	if required < 0 || len(label.Bitmap) < required {
		return nil, fmt.Errorf("windows: short bitmap payload")
	}
	if width != label.BitmapWidth || height != label.BitmapHeight {
		pixels := platform.ResampleBGRA(label.Bitmap, label.BitmapStride, label.BitmapWidth, label.BitmapHeight, width, height)
		if pixels == nil {
			return nil, fmt.Errorf("windows: cannot resample bitmap to %dx%d", width, height)
		}
		return pixels, nil
	}
	pixels := make([]byte, width*height*4)
	for row := 0; row < height; row++ {
		copy(pixels[row*width*4:(row+1)*width*4], label.Bitmap[row*label.BitmapStride:row*label.BitmapStride+width*4])
	}
	return pixels, nil
}

func printableASCII(value string) bool {
	for index := range len(value) {
		if value[index] < 0x20 || value[index] >= 0x7f {
			return false
		}
	}
	return value != ""
}

// isSingleCharacter reports whether text is one rune, possibly followed by
// combining marks.
func isSingleCharacter(text string) bool {
	count := 0
	for _, r := range text {
		if count > 0 && unicode.In(r, unicode.Mn, unicode.Me, unicode.Cf) {
			continue
		}
		count++
	}
	return count == 1
}

// MeasureMono reports the fixed-pitch font's character cell at a size: the
// advance of one character and the height of one line.
func MeasureMono(size float32, face ui.MonoFace) (width, height float32, err error) {
	engine, err := theTextEngine()
	if err != nil {
		return 0, 0, err
	}
	engine.mu.Lock()
	defer engine.mu.Unlock()
	metrics, err := engine.services.measureIn(monoFamily(face), "M", size, 0)
	if err != nil {
		return 0, 0, err
	}
	return metrics.Width, metrics.Height, nil
}

// MeasureText is the width and the vertical metrics of one line of the system UI font,
// shaped as the text engine shapes it.
func MeasureText(value string, size, weight float32) (text.Metrics, error) {
	engine, err := theTextEngine()
	if err != nil {
		return text.Metrics{}, err
	}
	return engine.metrics(value, size, weight)
}

// NewTextMeasurer is MeasureText as a text.MeasureFunc. There is nothing to close: the
// measure shares the process's text engine.
func NewTextMeasurer() (text.MeasureFunc, func(), error) {
	engine, err := theTextEngine()
	if err != nil {
		return nil, nil, err
	}
	return engine.metrics, func() {}, nil
}

func (services *textServices) measure(value string, size, weight float32) (text.Metrics, error) {
	return services.measureIn("Segoe UI", value, size, weight)
}

func (services *textServices) measureIn(typeface, value string, size, weight float32) (text.Metrics, error) {
	if size <= 0 || math.IsNaN(float64(size)) || math.IsInf(float64(size), 0) {
		return text.Metrics{}, fmt.Errorf("windows: invalid font size")
	}
	format, err := services.formatFamily(typeface, size, weight)
	if err != nil {
		return text.Metrics{}, err
	}
	defer release(&format)
	units, err := syscall.UTF16FromString(value)
	if err != nil {
		return text.Metrics{}, err
	}
	var layout uintptr
	status := services.createLayout(services.write, &units[0], uint32(len(units)-1), format, 1000000, 1000000, &layout)
	if err := hresult(status); err != nil {
		return text.Metrics{}, err
	}
	defer release(&layout)
	var metrics textMetrics
	if err := hresult(comCall(layout, 60, uintptr(unsafe.Pointer(&metrics)))); err != nil {
		return text.Metrics{}, err
	}
	if metrics.Lines > 1024 {
		return text.Metrics{}, fmt.Errorf("windows: text measurement exceeds line limit")
	}
	lines := make([]lineMetrics, max(1, int(metrics.Lines)))
	var count uint32
	if err := hresult(comCall(layout, 59, uintptr(unsafe.Pointer(&lines[0])), uintptr(len(lines)), uintptr(unsafe.Pointer(&count)))); err != nil {
		return text.Metrics{}, err
	}
	line := lines[0]
	return text.Metrics{Width: metrics.WidthTrailing, Height: metrics.Height, Ascent: line.Baseline, Descent: line.Height - line.Baseline}, nil
}
