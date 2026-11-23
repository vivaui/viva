//go:build darwin && vivahead && !ios

package headless

import (
	"image"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/platform/darwin"
)

// renderer keeps one Metal offscreen renderer, recreated when the frame's
// size, scale or clear color changes.
type renderer struct {
	offscreen     *darwin.Offscreen
	width, height int
	scale         float64
	clear         app.Color
}

// rendererAvailable says whether this build can draw screenshots.
// defaultScale is the screenshot scale when Options.Scale is zero.
// rendersTextEditor says whether screenshots show the text being edited, which the
// platform's native editor draws over the scene; the Windows and Linux renderers do not
// draw that overlay yet.
const rendersTextEditor = true

const defaultScale = 2

const rendererAvailable = true

func newRenderer() *renderer { return &renderer{} }

func (r *renderer) close() {
	if r.offscreen != nil {
		r.offscreen.Close()
		r.offscreen = nil
	}
}

func (r *renderer) render(snapshot app.Snapshot, scale float64) (*image.RGBA, error) {
	if r.offscreen == nil || r.width != snapshot.Width || r.height != snapshot.Height || r.scale != scale || r.clear != snapshot.Clear {
		r.close()
		offscreen, err := darwin.NewOffscreen(snapshot.Width, snapshot.Height, scale, platform.Color{
			Red: snapshot.Clear.Red, Green: snapshot.Clear.Green, Blue: snapshot.Clear.Blue, Alpha: snapshot.Clear.Alpha,
		})
		if err != nil {
			return nil, err
		}
		r.offscreen, r.width, r.height, r.scale, r.clear = offscreen, snapshot.Width, snapshot.Height, scale, snapshot.Clear
	}
	rectangles, labels, input := scenery(snapshot)
	if err := r.offscreen.Set(rectangles, labels, input); err != nil {
		return nil, err
	}
	pixels, width, height, err := r.offscreen.Render()
	if err != nil {
		return nil, err
	}
	// Metal wrote BGRA; the frame is opaque.
	picture := image.NewRGBA(image.Rect(0, 0, width, height))
	for index := 0; index+3 < len(pixels); index += 4 {
		picture.Pix[index], picture.Pix[index+1], picture.Pix[index+2], picture.Pix[index+3] =
			pixels[index+2], pixels[index+1], pixels[index], 255
	}
	return picture, nil
}
