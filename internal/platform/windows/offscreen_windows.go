package windows

import (
	"fmt"
	"image"
	"math"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
)

var createPlainDevice = systemDLL("d3d11.dll").NewProc("D3D11CreateDevice")

// Offscreen renders frames to pixels with the window's own Direct3D 11 draw
// path and text rasterizers but without a window or a swap chain, so a
// screenshot is the pixels the window would show. It tries the hardware
// adapter first and WARP second, as the window does. It is not safe for
// concurrent use.
type Offscreen struct {
	renderer      renderer
	staging       uintptr
	width, height int // logical pixels
	scale         float64
}

// NewOffscreen creates a renderer for a width x height logical-pixel scene at
// the given backing scale, cleared to clear before each render.
func NewOffscreen(width, height int, scale float64, clear platform.Color) (*Offscreen, error) {
	if width < 1 || height < 1 || scale < 1 {
		return nil, fmt.Errorf("windows: invalid offscreen size %dx%d at %gx", width, height, scale)
	}
	offscreen := &Offscreen{width: width, height: height, scale: scale}
	offscreen.renderer.color = clear
	levels := [...]uint32{0xb000}
	var status uintptr
	for _, driver := range [...]uintptr{1, 5} {
		status, _, _ = createPlainDevice.Call(0, driver, 0, 0x20, uintptr(unsafe.Pointer(&levels[0])), 1, 7, uintptr(unsafe.Pointer(&offscreen.renderer.device)), 0, uintptr(unsafe.Pointer(&offscreen.renderer.context)))
		if hresult(status) == nil {
			break
		}
		release(&offscreen.renderer.context)
		release(&offscreen.renderer.device)
	}
	if err := hresult(status); err != nil {
		return nil, fmt.Errorf("create D3D11 device, hardware and software (WARP) both failed: %w: %w", platform.ErrNoGraphics, err)
	}
	if err := offscreen.renderer.createResources(); err != nil {
		offscreen.Close()
		return nil, err
	}
	pixelWidth, pixelHeight := offscreen.size()
	renderer := &offscreen.renderer
	description := textureDesc{Width: uint32(pixelWidth), Height: uint32(pixelHeight), Mips: 1, Array: 1, Format: 87, Samples: 1, Usage: 0, Bind: 0x20}
	var target uintptr
	if err := hresult(comCall3(renderer.device, 5, uintptr(unsafe.Pointer(&description)), 0, uintptr(unsafe.Pointer(&target)))); err != nil {
		offscreen.Close()
		return nil, fmt.Errorf("create offscreen target: %w", err)
	}
	status = comCall3(renderer.device, 9, target, 0, uintptr(unsafe.Pointer(&renderer.target)))
	release(&target)
	if err := hresult(status); err != nil {
		offscreen.Close()
		return nil, fmt.Errorf("create offscreen target view: %w", err)
	}
	description.Usage, description.Bind, description.CPU = 3, 0, 0x20000
	if err := hresult(comCall3(renderer.device, 5, uintptr(unsafe.Pointer(&description)), 0, uintptr(unsafe.Pointer(&offscreen.staging)))); err != nil {
		offscreen.Close()
		return nil, fmt.Errorf("create offscreen read-back texture: %w", err)
	}
	renderer.width, renderer.height = pixelWidth, pixelHeight
	return offscreen, nil
}

func (offscreen *Offscreen) size() (width, height int) {
	return int(math.Ceil(float64(offscreen.width) * offscreen.scale)), int(math.Ceil(float64(offscreen.height) * offscreen.scale))
}

// Set replaces the scene: rectangle and triangle instances in paint order and
// text labels. Labels are rasterized when they are drawn.
func (offscreen *Offscreen) Set(rectangles []render.Rectangle, labels []platform.TextLabel) error {
	if offscreen.renderer.device == 0 {
		return fmt.Errorf("windows: offscreen renderer is closed")
	}
	if labels == nil {
		labels = []platform.TextLabel{} // an empty, non-nil slice replaces the previous labels
	}
	return offscreen.renderer.apply(platform.Frame{Update: initialUpdate(len(rectangles)), Rectangles: rectangles, TextLabels: labels})
}

// Image draws the scene and returns its pixels, in device pixels. The image is
// premultiplied, like the target.
func (offscreen *Offscreen) Image() (*image.RGBA, error) {
	renderer := &offscreen.renderer
	if renderer.device == 0 {
		return nil, fmt.Errorf("windows: offscreen renderer is closed")
	}
	width, height := offscreen.size()
	if err := renderer.draw(width, height, float32(offscreen.scale)); err != nil {
		return nil, err
	}
	var target uintptr
	comCall1(renderer.target, 7, uintptr(unsafe.Pointer(&target))) // ID3D11View::GetResource
	comCall2(renderer.context, 47, offscreen.staging, target)      // CopyResource
	release(&target)
	var mapped resourceData
	if err := hresult(comCall5(renderer.context, 14, offscreen.staging, 0, 1, 0, uintptr(unsafe.Pointer(&mapped)))); err != nil {
		return nil, fmt.Errorf("map offscreen read-back texture: %w", err)
	}
	defer comCall2(renderer.context, 15, offscreen.staging, 0)
	result := image.NewRGBA(image.Rect(0, 0, width, height))
	source := unsafe.Slice((*byte)(mapped.Data), int(mapped.Pitch)*(height-1)+width*4)
	for y := 0; y < height; y++ {
		row := source[y*int(mapped.Pitch):]
		out := result.Pix[y*result.Stride : y*result.Stride+width*4]
		for x := 0; x < width*4; x += 4 {
			out[x], out[x+1], out[x+2], out[x+3] = row[x+2], row[x+1], row[x], row[x+3] // BGRA to RGBA
		}
	}
	return result, nil
}

// Close releases the renderer.
func (offscreen *Offscreen) Close() {
	release(&offscreen.staging)
	offscreen.renderer.close()
}
