package linux

import (
	"fmt"
	"image"
	"math"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/raster"
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
)

// CPUOffscreen is Offscreen without a GPU: the scene is drawn by the CPU rasterizer
// (internal/raster) and the labels by the system's Pango, so a machine with no Vulkan driver, or a
// build that does not want to load one, still renders. The pixels agree with the Vulkan pipelines
// to within a few levels (raster_parity_linux_test.go) except for text, which is always grayscale
// antialiased here: the Vulkan path blends LCD text when the desktop's font settings ask for it.
// It is not safe for concurrent use.
type CPUOffscreen struct {
	services *textServices
	canvas   *raster.Canvas
	raster   raster.Rasterizer
	scale    float32
	clear    scene.Color
	rects    []render.Rectangle
	labels   []raster.Label
}

// NewCPUOffscreen is NewOffscreen for the CPU: a width x height scene in logical pixels, drawn at
// scale device pixels per logical pixel and cleared to clear before each render.
func NewCPUOffscreen(width, height int, scale float64, clear platform.Color) (*CPUOffscreen, error) {
	if width < 1 || height < 1 || width > offscreenLimit || height > offscreenLimit || math.IsNaN(scale) || scale < 1 || scale > maxScale {
		return nil, fmt.Errorf("linux: invalid offscreen size %dx%d at %gx", width, height, scale)
	}
	deviceWidth, deviceHeight := int(math.Ceil(float64(width)*scale)), int(math.Ceil(float64(height)*scale))
	if deviceWidth > offscreenLimit || deviceHeight > offscreenLimit {
		return nil, fmt.Errorf("linux: offscreen size %dx%d at %gx is beyond %d device pixels", width, height, scale, offscreenLimit)
	}
	services, err := newTextServices()
	if err != nil {
		return nil, err
	}
	return &CPUOffscreen{
		services: services,
		canvas:   raster.NewCanvas(deviceWidth, deviceHeight),
		scale:    float32(scale),
		clear:    scene.Color{Red: float32(clear.Red), Green: float32(clear.Green), Blue: float32(clear.Blue), Alpha: float32(clear.Alpha)},
	}, nil
}

// Set replaces the scene: rectangle and triangle instances in paint order and text labels.
func (offscreen *CPUOffscreen) Set(rectangles []render.Rectangle, labels []platform.TextLabel) error {
	if offscreen.canvas == nil {
		return fmt.Errorf("linux: offscreen renderer is closed")
	}
	offscreen.rects = append(offscreen.rects[:0], rectangles...)
	offscreen.labels = offscreen.labels[:0]
	for index := range labels {
		prepared, ok, err := prepareLabel(offscreen.services, labels[index], offscreen.scale, offscreen.canvas.Width, offscreen.canvas.Height)
		if err != nil {
			return fmt.Errorf("linux: label %d: %w", index, err)
		}
		if ok {
			offscreen.labels = append(offscreen.labels, prepared)
		}
	}
	return nil
}

// Image draws the scene and returns its pixels, top row first, premultiplied RGBA.
func (offscreen *CPUOffscreen) Image() (*image.RGBA, error) {
	if offscreen.canvas == nil {
		return nil, fmt.Errorf("linux: offscreen renderer is closed")
	}
	offscreen.raster.Draw(offscreen.canvas, raster.Params{Scale: offscreen.scale, Clear: offscreen.clear}, offscreen.rects, offscreen.labels)
	canvas := offscreen.canvas
	picture := image.NewRGBA(image.Rect(0, 0, canvas.Width, canvas.Height))
	for y := 0; y < canvas.Height; y++ {
		source := canvas.Pix[y*canvas.Stride : y*canvas.Stride+canvas.Width*4]
		target := picture.Pix[y*picture.Stride : y*picture.Stride+canvas.Width*4]
		for i := 0; i < len(source); i += 4 {
			target[i], target[i+1], target[i+2], target[i+3] = source[i+2], source[i+1], source[i], source[i+3]
		}
	}
	return picture, nil
}

// Close releases the renderer.
func (offscreen *CPUOffscreen) Close() {
	if offscreen.services != nil {
		offscreen.services.close()
		offscreen.services = nil
	}
	offscreen.canvas, offscreen.rects, offscreen.labels = nil, nil, nil
}

// prepareLabel turns one text label (or image) into the picture the rasterizer composites, placed
// on the device pixel grid the way the Vulkan backend places it: the texture's top left is a whole
// device pixel (platform.DevicePosition), a clip takes every device pixel it touches. ok is false
// when the label draws nothing.
func prepareLabel(services *textServices, l platform.TextLabel, scale float32, canvasWidth, canvasHeight int) (label raster.Label, ok bool, err error) {
	image, ok, err := rasterizeLabel(services, l, scale)
	if err != nil || !ok {
		return raster.Label{}, false, err
	}
	label, ok = placeLabel(l, scale, image, canvasWidth, canvasHeight)
	return label, ok, nil
}

// labelImage is a label's picture at one scale, premultiplied BGRA with no padding between rows.
type labelImage struct {
	pixels        []byte
	width, height int
}

// rasterizeLabel draws a label's text, or resamples its image to the device pixels it covers. ok is
// false when it draws nothing. The picture does not depend on where the label is, except that an
// image smaller than its bitmap does, through the fraction of a pixel it starts at.
func rasterizeLabel(services *textServices, l platform.TextLabel, scale float32) (image labelImage, ok bool, err error) {
	if l.Width <= 0 || l.Height <= 0 || (l.Value == "" && len(l.Bitmap) == 0) {
		return labelImage{}, false, nil
	}
	if len(l.Bitmap) > 0 {
		width, height := l.BitmapWidth, l.BitmapHeight
		shownWidth, shownHeight := platform.DeviceExtent(l.X, l.Width, scale), platform.DeviceExtent(l.Y, l.Height, scale)
		switch {
		case shownWidth <= 0 || shownHeight <= 0:
			return labelImage{}, false, nil
		case shownWidth <= width && shownHeight <= height && (shownWidth < width || shownHeight < height):
			return labelImage{platform.ResampleBGRA(l.Bitmap, l.BitmapStride, width, height, shownWidth, shownHeight), shownWidth, shownHeight}, true, nil
		}
		return labelImage{tightBitmap(l.Bitmap, width, height, l.BitmapStride), width, height}, true, nil
	}
	width := int(math.Ceil(float64(l.Width * scale)))
	height := int(math.Ceil(float64(l.Height * scale)))
	pixels, _, err := services.rasterize(l, scale, width, height, false)
	if err != nil {
		return labelImage{}, false, err
	}
	return labelImage{pixels, width, height}, true, nil
}

// placeLabel puts a label's picture on the canvas: its top left on a whole device pixel and its
// clip, if a canvas clip cuts it, on the device pixels the clip touches. ok is false when the clip
// leaves nothing.
func placeLabel(l platform.TextLabel, scale float32, image labelImage, canvasWidth, canvasHeight int) (label raster.Label, ok bool) {
	x, y := platform.DevicePosition(l.X, l.OriginX, scale), platform.DevicePosition(l.Y, l.OriginY, scale)
	if len(l.Bitmap) > 0 {
		x, y = platform.DevicePosition(l.X, 0, scale), platform.DevicePosition(l.Y, 0, scale)
	}
	label = raster.Label{Pix: image.pixels, Stride: image.width * 4, Width: image.width, Height: image.height, X: x, Y: y, Layer: l.Layer}
	if l.Clipped {
		left := max(int(math.Floor(float64(l.Clip.X*scale))), 0)
		top := max(int(math.Floor(float64(l.Clip.Y*scale))), 0)
		right := min(int(math.Ceil(float64((l.Clip.X+l.Clip.Width)*scale))), canvasWidth)
		bottom := min(int(math.Ceil(float64((l.Clip.Y+l.Clip.Height)*scale))), canvasHeight)
		if right <= left || bottom <= top {
			return raster.Label{}, false
		}
		label.Clipped, label.Clip = true, render.Scissor{Left: left, Top: top, Right: right, Bottom: bottom}
	}
	return label, true
}
