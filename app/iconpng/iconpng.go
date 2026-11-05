// Package iconpng decodes a PNG into an app.WindowIcon. It is separate from
// package app so an application that supplies raw RGBA pixels, or no icon at
// all, does not link image/png and its compression dependencies.
package iconpng

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/png"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/internal/iconshape"
)

// Decode decodes a square PNG as an app.WindowIcon. It is what an application
// embeds with go:embed, usually the very file viva.toml names as [app] icon, so
// the packaged icon and the running window's come from one image.
func Decode(data []byte) (app.WindowIcon, error) { return DecodeFit(data, 1024) }

// DecodeFit is Decode that shrinks a picture larger than limit pixels to limit
// by area averaging, so an application that embeds a 1024 pixel master does not
// keep 4 MB of pixels for as long as it runs. 512 is sharp wherever a desktop
// shows an icon large. A picture that already fits is not resampled.
func DecodeFit(data []byte, limit int) (app.WindowIcon, error) {
	if limit < 1 || limit > 1024 {
		return app.WindowIcon{}, fmt.Errorf("iconpng: limit must be between 1 and 1024 pixels, got %d", limit)
	}
	decoded, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return app.WindowIcon{}, fmt.Errorf("iconpng: %w", err)
	}
	bounds := decoded.Bounds()
	if bounds.Dx() != bounds.Dy() {
		return app.WindowIcon{}, fmt.Errorf("iconpng: icon must be square, got %dx%d", bounds.Dx(), bounds.Dy())
	}
	picture, ok := decoded.(*image.NRGBA)
	if !ok || picture.Stride != bounds.Dx()*4 || bounds.Min != (image.Point{}) {
		picture = image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
		draw.Draw(picture, picture.Bounds(), decoded, bounds.Min, draw.Src)
	}
	if bounds.Dx() > limit && bounds.Dx() <= 1024 {
		picture = iconshape.Scale(picture, limit)
	}
	icon := app.WindowIcon{Size: picture.Rect.Dx(), RGBA: picture.Pix}
	if bounds.Dx() < 1 || bounds.Dx() > 1024 {
		return app.WindowIcon{}, fmt.Errorf("iconpng: icon must be between 1 and 1024 pixels, got %d", bounds.Dx())
	}
	return icon, nil
}
