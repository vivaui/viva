// Package iconshape gives an application's square picture the shape a desktop
// draws its icons in. An application supplies one full-bleed square image; the
// platform's own convention (the macOS tile with its margin, the plain rounded
// tile of Windows and Linux) is applied here, once, for the packager and for the
// running window alike, so a packaged icon and a window's icon never differ.
package iconshape

import (
	"image"
	"image/draw"
	"math"
)

// MacInset is the share of the canvas the macOS icon grid gives the tile: the
// 824 px body of a 1024 px icon. The rest is the transparent margin every Dock
// icon has.
const MacInset = 824.0 / 1024

const (
	// exponent of the superellipse that approximates the continuous corners of
	// the tile.
	exponent = 5
	// deep is how far from the center, in half-sides, a point is certainly inside
	// the tile: 2 * 0.8^5 < 1.
	deep   = 0.8
	supers = 4 // samples per axis along the edge
)

// cornerAlpha is the most opacity a corner may have for the picture to count as
// already shaped.
const cornerAlpha = 32

// Shaped reports whether the picture already has its own shape: all four
// corners are (nearly) transparent. A full-bleed square, which the framework
// is meant to shape, has opaque corners.
func Shaped(picture image.Image) bool {
	bounds := picture.Bounds()
	if bounds.Empty() {
		return false
	}
	for _, point := range [...]image.Point{
		{bounds.Min.X, bounds.Min.Y}, {bounds.Max.X - 1, bounds.Min.Y},
		{bounds.Min.X, bounds.Max.Y - 1}, {bounds.Max.X - 1, bounds.Max.Y - 1},
	} {
		if _, _, _, alpha := picture.At(point.X, point.Y).RGBA(); alpha>>8 > cornerAlpha {
			return false
		}
	}
	return true
}

// Tile draws the picture into a size x size canvas as the tile of a desktop
// icon: scaled to inset of the canvas (1 fills it) and clipped to the rounded
// shape, with a transparent margin around it. A picture that is Shaped is only
// scaled to the whole canvas, whatever the inset: its artwork decides its own
// margin.
func Tile(picture image.Image, size int, inset float64) *image.NRGBA {
	canvas := image.NewNRGBA(image.Rect(0, 0, size, size))
	if size < 1 || picture.Bounds().Empty() {
		return canvas
	}
	shaped := Shaped(picture)
	body := size
	if !shaped {
		body = max(1, int(math.Round(float64(size)*inset)))
	}
	margin := (size - body) / 2
	scaled := scale(picture, body)
	for y := 0; y < body; y++ {
		for x := 0; x < body; x++ {
			from := scaled.Pix[(y*body+x)*4:]
			coverage := 255
			if !shaped {
				coverage = edgeCoverage(x, y, body)
			}
			if coverage == 0 {
				continue
			}
			to := canvas.Pix[((y+margin)*size+x+margin)*4:]
			to[0], to[1], to[2] = from[0], from[1], from[2]
			to[3] = uint8((int(from[3])*coverage + 127) / 255)
		}
	}
	return canvas
}

// Mark returns the picture cropped to a square around its mark, with a margin of
// an eighth of the side, so the mark fills the icon instead of the padding the
// canvas carries for a desktop that draws it large. The square is centered on
// the mark and, when the mark is near an edge, slid inside the picture. A picture
// that already has a shape of its own, whose mark reaches both canvas edges, or
// which has no mark at all, is returned as it is: there is no margin to spend.
func Mark(picture image.Image) image.Image {
	bounds := picture.Bounds()
	extent, ok := markExtent(picture)
	if !ok {
		return picture
	}
	side := max(extent.Dx(), extent.Dy())
	side += side / 8
	if side >= bounds.Dx() || side >= bounds.Dy() {
		return picture
	}
	x := extent.Min.X + extent.Dx()/2 - side/2
	y := extent.Min.Y + extent.Dy()/2 - side/2
	x = min(max(x, bounds.Min.X), bounds.Max.X-side)
	y = min(max(y, bounds.Min.Y), bounds.Max.Y-side)
	crop := image.NewNRGBA(image.Rect(0, 0, side, side))
	draw.Draw(crop, crop.Bounds(), picture, image.Pt(x, y), draw.Src)
	return crop
}

// TileMark returns the picture as a size x size desktop tile whose mark fills
// it: Mark crops away the canvas margin around the mark, and the crop takes the
// same rounded tile Tile gives. It is the form a desktop uses where it draws the
// icon small, a taskbar button or a status-area item, so the surface's few
// pixels carry the mark rather than the margin. A picture Mark does not crop
// gives exactly Tile's tile.
func TileMark(picture image.Image, size int) *image.NRGBA {
	return Tile(Mark(picture), size, 1)
}

// markExtent is the bounds of the picture's mark: the pixels markAlpha puts
// above a quarter-visible. ok is false when nothing stands out from the
// background.
func markExtent(picture image.Image) (extent image.Rectangle, ok bool) {
	bounds := picture.Bounds()
	if bounds.Empty() {
		return image.Rectangle{}, false
	}
	shaped := Shaped(picture)
	br, bg, bb, _ := pixelAt(picture, bounds.Min.X, bounds.Min.Y)
	left, top, right, bottom := bounds.Dx(), bounds.Dy(), -1, -1
	for y := 0; y < bounds.Dy(); y++ {
		for x := 0; x < bounds.Dx(); x++ {
			r, g, b, a := pixelAt(picture, bounds.Min.X+x, bounds.Min.Y+y)
			if markAlpha(r, g, b, a, shaped, br, bg, bb) <= 24 {
				continue
			}
			left, top, right, bottom = min(left, x), min(top, y), max(right, x), max(bottom, y)
		}
	}
	if right < left {
		return image.Rectangle{}, false
	}
	return image.Rect(bounds.Min.X+left, bounds.Min.Y+top, bounds.Min.X+right+1, bounds.Min.Y+bottom+1), true
}

// pixelAt is the picture's color at (x, y) as a pixel's RGBA gives it, reading
// the bytes of an NRGBA or RGBA picture directly: scanning a whole icon through
// image.Image.At allocates a color for every pixel.
func pixelAt(picture image.Image, x, y int) (r, g, b, a uint32) {
	switch source := picture.(type) {
	case *image.NRGBA:
		pixel := source.Pix[source.PixOffset(x, y):]
		alpha := uint32(pixel[3])
		return uint32(pixel[0]) * 0x101 * alpha / 0xff, uint32(pixel[1]) * 0x101 * alpha / 0xff, uint32(pixel[2]) * 0x101 * alpha / 0xff, alpha * 0x101
	case *image.RGBA:
		pixel := source.Pix[source.PixOffset(x, y):]
		return uint32(pixel[0]) * 0x101, uint32(pixel[1]) * 0x101, uint32(pixel[2]) * 0x101, uint32(pixel[3]) * 0x101
	}
	return picture.At(x, y).RGBA()
}

// markAlpha is how much one pixel belongs to the picture's mark: a shaped
// picture's own alpha, or how far its color is from the background with a little
// gain, so a near-black mark on white reads as fully opaque.
func markAlpha(r, g, b, a uint32, shaped bool, br, bg, bb uint32) uint8 {
	if shaped {
		return uint8(a >> 8)
	}
	return uint8(min(255, max(distance(r, br), distance(g, bg), distance(b, bb))*115/100))
}

// edgeCoverage is how much of pixel (x, y) of a body x body tile lies inside
// its shape, 0 to 255.
func edgeCoverage(x, y, body int) int {
	half := float64(body) / 2
	u, v := (float64(x)+0.5-half)/half, (float64(y)+0.5-half)/half
	if math.Abs(u) <= deep && math.Abs(v) <= deep {
		return 255
	}
	inside := 0
	for sy := 0; sy < supers; sy++ {
		for sx := 0; sx < supers; sx++ {
			u := (float64(x) + (float64(sx)+0.5)/supers - half) / half
			v := (float64(y) + (float64(sy)+0.5)/supers - half) / half
			if pow5(math.Abs(u))+pow5(math.Abs(v)) <= 1 {
				inside++
			}
		}
	}
	return 255 * inside / (supers * supers)
}

func pow5(value float64) float64 { return value * value * value * value * value }

// scale resamples the picture to size x size by area averaging when it shrinks
// and bilinear sampling when it grows, on premultiplied color so a transparent
// edge does not darken the result.
func scale(picture image.Image, size int) *image.NRGBA {
	bounds := picture.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	out := image.NewNRGBA(image.Rect(0, 0, size, size))
	source, isNRGBA := picture.(*image.NRGBA)
	at := func(x, y int) (red, green, blue, alpha float64) {
		x, y = min(max(x, 0), width-1), min(max(y, 0), height-1)
		var pixel [4]uint8
		if isNRGBA {
			copy(pixel[:], source.Pix[source.PixOffset(bounds.Min.X+x, bounds.Min.Y+y):])
		} else {
			r, g, b, a := picture.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			if a != 0 {
				pixel = [4]uint8{uint8(r * 0xffff / a >> 8), uint8(g * 0xffff / a >> 8), uint8(b * 0xffff / a >> 8), uint8(a >> 8)}
			}
		}
		weight := float64(pixel[3])
		return float64(pixel[0]) * weight, float64(pixel[1]) * weight, float64(pixel[2]) * weight, weight
	}
	scaleX, scaleY := float64(width)/float64(size), float64(height)/float64(size)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			var red, green, blue, alpha, count float64
			if scaleX >= 1 {
				left, right := int(float64(x)*scaleX), max(int(float64(x+1)*scaleX), int(float64(x)*scaleX)+1)
				top, bottom := int(float64(y)*scaleY), max(int(float64(y+1)*scaleY), int(float64(y)*scaleY)+1)
				for sy := top; sy < bottom; sy++ {
					for sx := left; sx < right; sx++ {
						r, g, b, a := at(sx, sy)
						red, green, blue, alpha, count = red+r, green+g, blue+b, alpha+a, count+1
					}
				}
			} else {
				fx, fy := (float64(x)+0.5)*scaleX-0.5, (float64(y)+0.5)*scaleY-0.5
				x0, y0 := int(math.Floor(fx)), int(math.Floor(fy))
				dx, dy := fx-float64(x0), fy-float64(y0)
				for _, tap := range [4]struct {
					x, y   int
					weight float64
				}{{x0, y0, (1 - dx) * (1 - dy)}, {x0 + 1, y0, dx * (1 - dy)}, {x0, y0 + 1, (1 - dx) * dy}, {x0 + 1, y0 + 1, dx * dy}} {
					r, g, b, a := at(tap.x, tap.y)
					red, green, blue, alpha, count = red+r*tap.weight, green+g*tap.weight, blue+b*tap.weight, alpha+a*tap.weight, count+tap.weight
				}
			}
			if alpha == 0 {
				continue
			}
			pixel := out.Pix[(y*size+x)*4:]
			pixel[0] = uint8(red/alpha + 0.5)
			pixel[1] = uint8(green/alpha + 0.5)
			pixel[2] = uint8(blue/alpha + 0.5)
			pixel[3] = uint8(alpha/count + 0.5)
		}
	}
	return out
}

// Scale resamples the picture to size x size, by area averaging when it shrinks
// and bilinear sampling when it grows, without shaping it.
func Scale(picture image.Image, size int) *image.NRGBA { return scale(picture, size) }

// Silhouette is the picture's mark as a one-color shape for a status area that
// tints its icons (a macOS template): black, with an alpha that says how far
// each pixel is from the background. The background is the color of the top-left
// pixel; a picture with a shape of its own (see Shaped) already says it in alpha.
// The mark is cropped to its extent and centered in a size x size canvas with a
// small margin, because a status icon is drawn tiny and an empty border wastes
// its pixels.
func Silhouette(picture image.Image, size int) *image.NRGBA {
	canvas := image.NewNRGBA(image.Rect(0, 0, size, size))
	bounds := picture.Bounds()
	if size < 1 || bounds.Empty() {
		return canvas
	}
	shaped := Shaped(picture)
	br, bg, bb, _ := pixelAt(picture, bounds.Min.X, bounds.Min.Y)
	mask := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	left, top, right, bottom := bounds.Dx(), bounds.Dy(), -1, -1
	for y := 0; y < bounds.Dy(); y++ {
		for x := 0; x < bounds.Dx(); x++ {
			r, g, b, a := pixelAt(picture, bounds.Min.X+x, bounds.Min.Y+y)
			alpha := uint32(markAlpha(r, g, b, a, shaped, br, bg, bb))
			if alpha == 0 {
				continue
			}
			mask.Pix[(y*bounds.Dx()+x)*4+3] = uint8(alpha)
			if alpha > 24 {
				left, top, right, bottom = min(left, x), min(top, y), max(right, x), max(bottom, y)
			}
		}
	}
	if right < left {
		return canvas
	}
	// A square around the mark, with a margin of an eighth of its side.
	side := max(right-left+1, bottom-top+1)
	side += side / 8
	centerX, centerY := (left+right+1)/2, (top+bottom+1)/2
	square := image.NewNRGBA(image.Rect(0, 0, side, side))
	for y := 0; y < side; y++ {
		for x := 0; x < side; x++ {
			sx, sy := centerX-side/2+x, centerY-side/2+y
			if sx >= 0 && sy >= 0 && sx < bounds.Dx() && sy < bounds.Dy() {
				square.Pix[(y*side+x)*4+3] = mask.Pix[(sy*bounds.Dx()+sx)*4+3]
			}
		}
	}
	return scale(square, size)
}

// distance is how far two 16-bit channel values are apart, as 0 to 255.
func distance(a, b uint32) uint32 {
	if a < b {
		a, b = b, a
	}
	return (a - b) >> 8
}
