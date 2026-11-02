package ui

import "fmt"

// MaxCursorImageSize is the largest width and height of a CursorImage, in image pixels. Remote
// desktop protocols name pointers up to 384 pixels; operating systems draw them from far less.
const MaxCursorImageSize = 512

// CursorImage is a pointer the application paints itself, as a picture the system draws as its
// own pointer: it follows the mouse with no lag, shows over the window's title bar and edges the
// way any pointer does, and is dropped by the system when the mouse leaves. A remote desktop
// client uses it for the pointer the remote machine names.
//
// A CursorImage never changes after NewCursorImage, so one value is one pointer and platforms
// make their native cursor for it once. Make a new value when the picture changes.
type CursorImage struct {
	width, height int
	hotX, hotY    int
	scale         float32
	bgra          []byte
}

// NewCursorImage is a pointer picture width by height image pixels, with its hotspot at hotX,
// hotY from the top left: the pixel that is the mouse's position. bgra holds the rows top down,
// four bytes a pixel in blue, green, red, alpha order, with the color not multiplied by alpha.
// The image keeps bgra, so it must not be changed afterwards.
//
// scale is how many image pixels one logical pixel of the window covers, 1 when zero: a picture
// made for the screen's own pixels passes the window's pixel scale, and one drawn at 1x passes 1.
func NewCursorImage(width, height, hotX, hotY int, scale float32, bgra []byte) (*CursorImage, error) {
	if width < 1 || height < 1 || width > MaxCursorImageSize || height > MaxCursorImageSize {
		return nil, fmt.Errorf("ui: cursor image %dx%d is outside 1..%d", width, height, MaxCursorImageSize)
	}
	if hotX < 0 || hotY < 0 || hotX >= width || hotY >= height {
		return nil, fmt.Errorf("ui: cursor hotspot %d,%d is outside the %dx%d image", hotX, hotY, width, height)
	}
	if len(bgra) != width*height*4 {
		return nil, fmt.Errorf("ui: cursor image %dx%d needs %d bytes, has %d", width, height, width*height*4, len(bgra))
	}
	if scale < 0 || scale != scale {
		return nil, fmt.Errorf("ui: cursor image scale %v is not positive", scale)
	}
	if scale == 0 {
		scale = 1
	}
	return &CursorImage{width: width, height: height, hotX: hotX, hotY: hotY, scale: scale, bgra: bgra}, nil
}

// Size is the image's width and height in image pixels.
func (image *CursorImage) Size() (width, height int) { return image.width, image.height }

// Hotspot is the pixel of the image that is the mouse's position.
func (image *CursorImage) Hotspot() (x, y int) { return image.hotX, image.hotY }

// Scale is how many image pixels cover one logical pixel.
func (image *CursorImage) Scale() float32 { return image.scale }

// Pixels is the image's rows, top down, in blue, green, red, alpha bytes with the color not
// multiplied by alpha. Callers must not change it.
func (image *CursorImage) Pixels() []byte { return image.bgra }
