//go:build linux

package headless

import (
	"errors"
	"image"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/platform/linux"
	"github.com/vivaui/viva/internal/render"
)

// offscreen is what draws a frame to pixels: the Vulkan pipelines, or the CPU rasterizer on a
// machine with no Vulkan driver.
type offscreen interface {
	Set([]render.Rectangle, []platform.TextLabel) error
	Image() (*image.RGBA, error)
	Close()
}

// renderer keeps one offscreen renderer, recreated when the frame's size, scale or clear color
// changes. It draws with Vulkan when there is a driver and with the CPU when there is none.
type renderer struct {
	offscreen     offscreen
	width, height int
	scale         float64
	clear         app.Color
}

// rendererAvailable says whether this build can draw screenshots.
// defaultScale is the screenshot scale when Options.Scale is zero.
// rendersTextEditor says whether screenshots show the text being edited, which the
// platform's native editor draws over the scene; the Windows and Linux renderers do not
// draw that overlay yet.
const rendersTextEditor = false

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
		clear := platform.Color{Red: snapshot.Clear.Red, Green: snapshot.Clear.Green, Blue: snapshot.Clear.Blue, Alpha: snapshot.Clear.Alpha}
		var target offscreen
		gpu, err := linux.NewOffscreen(snapshot.Width, snapshot.Height, scale, clear)
		if err == nil {
			target = gpu
		} else if errors.Is(err, platform.ErrNoGraphics) {
			cpu, cpuErr := linux.NewCPUOffscreen(snapshot.Width, snapshot.Height, scale, clear)
			if cpuErr != nil {
				return nil, errors.Join(err, cpuErr)
			}
			target = cpu
		} else {
			return nil, err
		}
		r.offscreen, r.width, r.height, r.scale, r.clear = target, snapshot.Width, snapshot.Height, scale, snapshot.Clear
	}
	rectangles, labels, _ := scenery(snapshot)
	if err := r.offscreen.Set(rectangles, labels); err != nil {
		return nil, err
	}
	picture, err := r.offscreen.Image()
	if err != nil {
		return nil, err
	}
	for index := 3; index < len(picture.Pix); index += 4 {
		picture.Pix[index] = 255
	}
	return picture, nil
}
