package platform

import (
	"math"

	"github.com/vivaui/viva/ui"
)

// CursorPixels is a pointer picture in the device pixels of a window drawn at windowScale device
// pixels per logical pixel: the rows top down, four bytes a pixel in blue, green, red, alpha
// order, with the color multiplied by alpha when premultiplied is set (macOS, X11 RENDER) and
// not (Windows, whose cursor bitmaps are read as plain ARGB). An image whose own scale already
// matches is only converted; otherwise it is resampled, bilinear and in premultiplied color so
// that a soft edge does not take the color of the transparent pixels around it. The hotspot is
// the device pixel the mouse is at.
func CursorPixels(image *ui.CursorImage, windowScale float32, premultiplied bool) (width, height, hotX, hotY int, bgra []byte) {
	sourceWidth, sourceHeight := image.Size()
	source := image.Pixels()
	factor := float64(windowScale) / float64(image.Scale())
	if windowScale <= 0 || factor <= 0 || math.Abs(factor-1) < 1e-3 {
		factor = 1
	}
	width = max(int(math.Round(float64(sourceWidth)*factor)), 1)
	height = max(int(math.Round(float64(sourceHeight)*factor)), 1)
	hotSourceX, hotSourceY := image.Hotspot()
	hotX = min(int(float64(hotSourceX)*factor+0.5*factor), width-1)
	hotY = min(int(float64(hotSourceY)*factor+0.5*factor), height-1)
	bgra = make([]byte, width*height*4)
	if factor == 1 && !premultiplied {
		copy(bgra, source)
		return width, height, hotX, hotY, bgra
	}
	if factor == 1 {
		for index := 0; index < len(source); index += 4 {
			alpha := uint32(source[index+3])
			bgra[index] = byte((uint32(source[index])*alpha + 127) / 255)
			bgra[index+1] = byte((uint32(source[index+1])*alpha + 127) / 255)
			bgra[index+2] = byte((uint32(source[index+2])*alpha + 127) / 255)
			bgra[index+3] = byte(alpha)
		}
		return width, height, hotX, hotY, bgra
	}
	// Sample the source at the center of each target pixel, in premultiplied color.
	sample := func(x, y int) (b, g, r, a float64) {
		x, y = min(max(x, 0), sourceWidth-1), min(max(y, 0), sourceHeight-1)
		offset := (y*sourceWidth + x) * 4
		alpha := float64(source[offset+3])
		return float64(source[offset]) * alpha / 255, float64(source[offset+1]) * alpha / 255, float64(source[offset+2]) * alpha / 255, alpha
	}
	for row := 0; row < height; row++ {
		sourceY := (float64(row)+0.5)/factor - 0.5
		y0 := int(math.Floor(sourceY))
		fy := sourceY - float64(y0)
		for column := 0; column < width; column++ {
			sourceX := (float64(column)+0.5)/factor - 0.5
			x0 := int(math.Floor(sourceX))
			fx := sourceX - float64(x0)
			var sums [4]float64
			for _, tap := range [4]struct {
				x, y int
				w    float64
			}{
				{x0, y0, (1 - fx) * (1 - fy)}, {x0 + 1, y0, fx * (1 - fy)},
				{x0, y0 + 1, (1 - fx) * fy}, {x0 + 1, y0 + 1, fx * fy},
			} {
				b, g, r, a := sample(tap.x, tap.y)
				sums[0] += b * tap.w
				sums[1] += g * tap.w
				sums[2] += r * tap.w
				sums[3] += a * tap.w
			}
			offset := (row*width + column) * 4
			if !premultiplied && sums[3] > 0 {
				for channel := 0; channel < 3; channel++ {
					sums[channel] = sums[channel] * 255 / sums[3]
				}
			}
			for channel := range sums {
				bgra[offset+channel] = byte(min(sums[channel]+0.5, 255))
			}
		}
	}
	return width, height, hotX, hotY, bgra
}
