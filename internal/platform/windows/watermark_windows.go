package windows

import (
	"fmt"
	"math"
	"sync"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

const (
	wmSetWatermark       = 0x8057 // follows wmChangeDecoration
	d2dCreateBitmap      = 4
	d2dCreateBitmapBrush = 7
	d2dFillRectangle     = 17
	watermarkClassName   = "VivaWatermark"
)

var (
	watermarkClass       sync.Once
	watermarkClassError  error
	updateWatermarkLayer = user.NewProc("UpdateLayeredWindow")
)

type watermarkRequest struct {
	mu      sync.Mutex
	handle  uintptr
	value   platform.Watermark
	pending bool
}

func (window *nativeWindow) SetWatermark(value platform.Watermark) error {
	if window == nil || window.closed.Load() {
		return fmt.Errorf("windows: window is closed")
	}
	value, err := platform.NormalizeWatermark(value)
	if err != nil {
		return err
	}
	request := &window.state.pendingWatermark
	request.mu.Lock()
	defer request.mu.Unlock()
	if request.handle == 0 {
		return fmt.Errorf("windows: window is closed")
	}
	request.value, request.pending = value, true
	// PostMessage never waits for the window thread. Teardown retires this handle
	// under the same lock, so background setters never read mutable window state.
	if ok, _, err := postMessage.Call(request.handle, wmSetWatermark, 0, 0); ok == 0 {
		return fmt.Errorf("windows: post watermark: %w", err)
	}
	return nil
}

func (state *windowState) applyWatermark() {
	request := &state.pendingWatermark
	request.mu.Lock()
	value, pending := request.value, request.pending
	request.pending = false
	request.mu.Unlock()
	if !pending {
		return
	}
	previous := state.config.Watermark
	state.config.Watermark = value
	if err := state.syncWatermark(); err != nil {
		state.config.Watermark = previous
		if restoreErr := state.syncWatermark(); restoreErr != nil {
			platform.ReportFault("restore window watermark", restoreErr)
		}
		platform.ReportFault("window watermark", err)
	}
}

// watermarkOverlay retains the small text tile independently of the client-sized DIB.
// Moving and ordinary frames never rasterize text or refill the DIB.
type watermarkOverlay struct {
	window                uintptr
	rasterizer            *directRasterizer
	target                *directTarget
	bitmap, brush         uintptr
	config                platform.Watermark
	scale                 float32
	width, height         int
	tileWidth, tileHeight int
}

func watermarkProc(window, kind, first, second uintptr) uintptr {
	switch kind {
	case wmNCHitTest:
		return ^uintptr(0) // HTTRANSPARENT
	case 0x0021:
		return 3 // WM_MOUSEACTIVATE -> MA_NOACTIVATE
	case wmErase:
		return 1
	}
	result, _, _ := defaultWindowProc.Call(window, kind, first, second)
	return result
}

func (state *windowState) newWatermark() (*watermarkOverlay, error) {
	watermarkClass.Do(func() {
		class := windowClass{Size: uint32(unsafe.Sizeof(windowClass{})), Procedure: syscall.NewCallback(watermarkProc), Instance: state.application.instance, Name: wide(watermarkClassName)}
		if ok, _, err := registerClass.Call(uintptr(unsafe.Pointer(&class))); ok == 0 {
			watermarkClassError = fmt.Errorf("windows: register watermark: %w", err)
		}
	})
	if watermarkClassError != nil {
		return nil, watermarkClassError
	}
	handle, _, err := createWindow.Call(wsExLayered|wsExTransparent|wsExNoActivate|wsExToolWindow, uintptr(unsafe.Pointer(wide(watermarkClassName))), 0, wsPopup, 0, 0, 1, 1, state.window, 0, state.application.instance, 0)
	if handle == 0 {
		return nil, fmt.Errorf("windows: create watermark: %w", err)
	}
	return &watermarkOverlay{window: handle}, nil
}

func (overlay *watermarkOverlay) suspend() {
	showWindow.Call(overlay.window, 0)
	release(&overlay.brush)
	release(&overlay.bitmap)
	if overlay.target != nil {
		overlay.target.close()
		overlay.target = nil
	}
	overlay.width, overlay.height = 0, 0
}
func (overlay *watermarkOverlay) close() {
	overlay.suspend()
	if overlay.rasterizer != nil {
		overlay.rasterizer.close()
		overlay.rasterizer = nil
	}
	if overlay.window != 0 {
		destroyWindow.Call(overlay.window)
		overlay.window = 0
	}
}

// raiseWatermark is the only stacking entry point. New native child/owned windows
// must call it after changing the native view tree.
func (state *windowState) raiseWatermark() {
	if state.watermark != nil && state.watermark.window != 0 {
		level := ^uintptr(1) // HWND_NOTOPMOST: match the owner, including demotion.
		if style, err := readDecorationStyle(state.window, ^uintptr(19)); err == nil && style&wsExTopmost != 0 {
			level = ^uintptr(0)
		}
		setWindowPosition.Call(state.watermark.window, level, 0, 0, 0, 0, 0x13) // HWND_TOP, NOMOVE|NOSIZE|NOACTIVATE
	}
}

func (state *windowState) syncWatermark() error {
	value := state.config.Watermark
	if value.Text == "" || value.Color.Alpha <= 0 {
		if state.watermark != nil {
			state.watermark.close()
			state.watermark = nil
		}
		return nil
	}
	if !state.ready || state.window == 0 {
		return nil
	}
	if state.watermark == nil {
		overlay, err := state.newWatermark()
		if err != nil {
			return err
		}
		state.watermark = overlay
	}
	overlay := state.watermark
	if state.minimized || state.hiddenByShow {
		overlay.suspend()
		return nil
	}
	var client rect
	if ok, _, err := getClientRect.Call(state.window, uintptr(unsafe.Pointer(&client))); ok == 0 {
		return err
	}
	area := platform.WatermarkContentRect(gfx.Size{Width: float32(client.Right), Height: float32(client.Bottom)}, value.Insets, state.scale)
	if area.Width <= 0 || area.Height <= 0 {
		overlay.suspend()
		return nil
	}
	origin := point{X: int32(area.X), Y: int32(area.Y)}
	if ok, _, err := clientToScreen.Call(state.window, uintptr(unsafe.Pointer(&origin))); ok == 0 {
		return err
	}
	width, height := int(area.Width), int(area.Height)
	rebuild := watermarkTileChanged(overlay.config, value) || overlay.scale != state.scale || overlay.rasterizer == nil
	if overlay.config != value || overlay.scale != state.scale || overlay.width != width || overlay.height != height || overlay.target == nil {
		if err := overlay.render(value, state.scale, width, height, rebuild); err != nil {
			overlay.suspend()
			return err
		}
		size := point{X: int32(width), Y: int32(height)}
		source := point{}
		blend := struct{ Operation, Flags, Alpha, Format byte }{Alpha: 255, Format: 1}
		if ok, _, err := updateWatermarkLayer.Call(overlay.window, 0, uintptr(unsafe.Pointer(&origin)), uintptr(unsafe.Pointer(&size)), overlay.target.scratch.dc, uintptr(unsafe.Pointer(&source)), 0, uintptr(unsafe.Pointer(&blend)), 2); ok == 0 {
			return fmt.Errorf("windows: present watermark: %w", err)
		}
		overlay.width, overlay.height = width, height
	} else {
		// Translation only: keep the retained surface and its z-order.
		setWindowPosition.Call(overlay.window, 0, uintptr(origin.X), uintptr(origin.Y), 0, 0, 0x15)
	}
	showWindow.Call(overlay.window, swShowNoActivate)
	state.raiseWatermark()
	return nil
}

// Rotation and clipping change composition, not the retained text tile.
func watermarkTileChanged(previous, next platform.Watermark) bool {
	return previous.Text != next.Text || previous.FontFamily != next.FontFamily || previous.Color != next.Color ||
		previous.FontSize != next.FontSize || previous.FontWeight != next.FontWeight || previous.TileWidth != next.TileWidth || previous.TileHeight != next.TileHeight
}

func watermarkSRGB(value float64) float32 {
	if value <= 0.0031308 {
		return float32(12.92 * value)
	}
	return float32(1.055*math.Pow(value, 1/2.4) - 0.055)
}

func (overlay *watermarkOverlay) render(value platform.Watermark, scale float32, width, height int, rebuild bool) error {
	if overlay.rasterizer == nil {
		rasterizer, err := newDirectRasterizer()
		if err != nil {
			return err
		}
		overlay.rasterizer = rasterizer
	}
	if rebuild {
		if err := overlay.renderTile(value, scale); err != nil {
			return err
		}
		release(&overlay.brush)
		release(&overlay.bitmap)
	}
	if overlay.target == nil {
		target, err := newDirectTarget(overlay.rasterizer.factory, d2dAlphaPremultiplied)
		if err != nil {
			return err
		}
		overlay.target = target
	}
	target := overlay.target
	surface, err := target.bind(width, height)
	if err != nil {
		return err
	}
	clear(surface.Pixels)
	if overlay.brush == 0 {
		tile := overlay.rasterizer.translucent.scratch
		properties := struct {
			Format, AlphaMode uint32
			DPIX, DPIY        float32
		}{87, d2dAlphaPremultiplied, 96, 96}
		packedSize := uintptr(uint32(overlay.tileWidth)) | uintptr(uint32(overlay.tileHeight))<<32
		if err := hresult(comCall5(target.target, d2dCreateBitmap, packedSize, uintptr(unsafe.Pointer(&tile.Pixels[0])), uintptr(tile.Width*4), uintptr(unsafe.Pointer(&properties)), uintptr(unsafe.Pointer(&overlay.bitmap)))); err != nil {
			return fmt.Errorf("windows: create watermark tile: %w", err)
		}
		brushProperties := struct{ X, Y, Interpolation uint32 }{1, 1, 1} // WRAP, WRAP, LINEAR
		if err := hresult(comCall4(target.target, d2dCreateBitmapBrush, overlay.bitmap, uintptr(unsafe.Pointer(&brushProperties)), 0, uintptr(unsafe.Pointer(&overlay.brush)))); err != nil {
			return err
		}
	}
	angle := float64(value.Angle) * math.Pi / 180
	matrix := d2dMatrix{M11: float32(math.Cos(angle)), M12: float32(math.Sin(angle)), M21: -float32(math.Sin(angle)), M22: float32(math.Cos(angle))}
	comCall1(overlay.brush, 5, uintptr(unsafe.Pointer(&matrix))) // ID2D1Brush.SetTransform
	bounds := struct{ Left, Top, Right, Bottom float32 }{Right: float32(width), Bottom: float32(height)}
	target.begin(d2dColor{}, d2dAntialiasGrayscale)
	comCall2(target.target, d2dFillRectangle, uintptr(unsafe.Pointer(&bounds)), overlay.brush)
	if err := target.end(); err != nil {
		return err
	}
	overlay.config, overlay.scale = value, scale
	return nil
}

func (overlay *watermarkOverlay) renderTile(value platform.Watermark, scale float32) error {
	width, height := platform.WatermarkTileSize(value, scale)
	if width <= 0 || height <= 0 {
		return fmt.Errorf("windows: invalid watermark tile")
	}
	rasterizer := overlay.rasterizer
	target := rasterizer.translucent
	surface, err := target.bind(width, height)
	if err != nil {
		return err
	}
	clear(surface.Pixels)
	weight := nativeFontWeight(value.FontWeight)
	family := value.FontFamily
	if family == "" {
		family = "Segoe UI"
	}
	format, err := rasterizer.services.formatWeight(family, value.FontSize*scale, weight, false)
	if err != nil {
		return err
	}
	defer release(&format)
	units := utf16.Encode([]rune(value.Text))
	if len(units) == 0 {
		return fmt.Errorf("windows: watermark text is empty")
	}
	var layout uintptr
	if err := hresult(rasterizer.services.createLayout(rasterizer.services.write, &units[0], uint32(len(units)), format, 1<<20, 1<<20, &layout)); err != nil {
		return err
	}
	defer release(&layout)
	var metrics textMetrics
	if err := hresult(comCall1(layout, writeGetMetrics, uintptr(unsafe.Pointer(&metrics)))); err != nil {
		return err
	}
	target.begin(d2dColor{R: watermarkSRGB(value.Color.Red), G: watermarkSRGB(value.Color.Green), B: watermarkSRGB(value.Color.Blue), A: float32(value.Color.Alpha)}, d2dAntialiasGrayscale)
	target.pushClip(0, width, height)
	target.draw((float32(width)-metrics.WidthTrailing)/2-metrics.Left, (float32(height)-metrics.Height)/2-metrics.Top, layout, 0)
	target.popClip()
	if err := target.end(); err != nil {
		return err
	}
	overlay.tileWidth, overlay.tileHeight = width, height
	return nil
}

// moveWatermark only translates the retained surface. Resize messages are paced
// separately by serviceResize, including left/top-edge resize drags.
func (state *windowState) moveWatermark() {
	if state.watermark == nil || state.minimized || state.hiddenByShow {
		return
	}
	insets := state.config.Watermark.Insets
	origin := point{X: int32(math.Ceil(float64(insets.Left * state.scale))), Y: int32(math.Ceil(float64(insets.Top * state.scale)))}
	if ok, _, _ := clientToScreen.Call(state.window, uintptr(unsafe.Pointer(&origin))); ok != 0 {
		setWindowPosition.Call(state.watermark.window, 0, uintptr(origin.X), uintptr(origin.Y), 0, 0, 0x15)
	}
}
