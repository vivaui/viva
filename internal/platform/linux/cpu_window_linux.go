package linux

import (
	"fmt"
	"math"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
)

// cpuImageStrip bounds one PutImage request at the same size the cursor picture
// uses, under any server's request limit.
const cpuImageStrip = 32 * 1024

// cpuWindowRenderer draws a window with the CPU rasterizer when the machine has
// no usable hardware Vulkan device (ADR 0075). The retained scene tracks the
// damage; a present paints the damage into the canvas and copies it to the X
// window with PutImage, which needs no graphics driver. It is not safe for
// concurrent use, like the rest of the window.
type cpuWindowRenderer struct {
	scene    *softScene
	services *textServices
	win      *window
	gc       uint32
	// depth is the window's drawable depth, fixed at creation: the root depth,
	// or 32 for a window whose frame Viva draws with a 32-bit visual.
	depth uint8
	// scratch is where a damaged rectangle narrower than the canvas is packed
	// row by row, because PutImage takes a rectangle without a stride.
	scratch []byte
	// checked is set after the first request was checked with the server, so a
	// wrong depth or drawable fails once with the server's reason instead of
	// quietly drawing nothing.
	checked  bool
	presents uint64
	rebuilds uint64
	partials uint64
}

func newCPUWindowRenderer(win *window, scale float32, clear platform.Color, rectangles []render.Rectangle, labels []platform.TextLabel) (*cpuWindowRenderer, error) {
	if win.width == 0 || win.height == 0 || win.width > math.MaxUint16 || win.height > math.MaxUint16 {
		return nil, fmt.Errorf("linux: invalid CPU window size %dx%d", win.width, win.height)
	}
	services, err := newTextServices()
	if err != nil {
		return nil, err
	}
	scene, err := newSoftScene(services, int(win.width), int(win.height), scale, clear, rectangles, labels)
	if err != nil {
		services.close()
		return nil, err
	}
	gc := win.api.generateID(win.connection)
	if err := win.checkRequest("create window drawing context", win.api.createGC(win.connection, gc, win.id, 0, nil)); err != nil {
		scene.close()
		services.close()
		return nil, err
	}
	return &cpuWindowRenderer{scene: scene, services: services, win: win, gc: gc, depth: win.depth}, nil
}

// apply takes a frame into the retained scene. Painting waits for present, as
// the GPU renderer's command recording does.
func (renderer *cpuWindowRenderer) apply(frame platform.Frame) (bool, error) {
	return renderer.scene.apply(frame)
}

// stage is apply: staging exists to defer GPU command recording, and the CPU
// records nothing until present.
func (renderer *cpuWindowRenderer) stage(frame platform.Frame) (bool, error) {
	return renderer.scene.apply(frame)
}

func (renderer *cpuWindowRenderer) present() error {
	damage := renderer.scene.paint()
	if damage.Right <= damage.Left || damage.Bottom <= damage.Top {
		return nil
	}
	renderer.presents++
	canvas := renderer.scene.canvas
	if damage.Left <= 0 && damage.Top <= 0 && damage.Right >= canvas.Width && damage.Bottom >= canvas.Height {
		renderer.rebuilds++
	} else {
		renderer.partials++
	}
	return renderer.push(damage)
}

func (renderer *cpuWindowRenderer) presentFull() error {
	renderer.scene.damageAll()
	return renderer.present()
}

// push copies one damaged region of the canvas to the window. A region as wide
// as the canvas is sent straight from the canvas rows; a narrower one is packed
// into the scratch buffer first. Both go in bounded strips.
func (renderer *cpuWindowRenderer) push(damage render.Scissor) error {
	canvas := renderer.scene.canvas
	left, top := max(damage.Left, 0), max(damage.Top, 0)
	right, bottom := min(damage.Right, canvas.Width), min(damage.Bottom, canvas.Height)
	if right <= left || bottom <= top {
		return nil
	}
	width, rows := right-left, bottom-top
	if left > math.MaxInt16 || top+rows-1 > math.MaxInt16 {
		return fmt.Errorf("linux: window image at %d,%d is beyond the X11 coordinate range", left, top)
	}
	var pixels []byte
	if left == 0 && width == canvas.Width {
		pixels = canvas.Pix[top*canvas.Stride : (top+rows)*canvas.Stride]
	} else {
		need := width * 4 * rows
		if cap(renderer.scratch) < need {
			renderer.scratch = make([]byte, need)
		}
		pixels = renderer.scratch[:need]
		for row := range rows {
			from := (top+row)*canvas.Stride + left*4
			copy(pixels[row*width*4:(row+1)*width*4], canvas.Pix[from:from+width*4])
		}
	}
	stripRows := max(cpuImageStrip/(width*4), 1)
	for row := 0; row < rows; row += stripRows {
		count := min(stripRows, rows-row)
		strip := pixels[row*width*4 : (row+count)*width*4]
		cookie := renderer.win.api.putImage(renderer.win.connection, xcbImageFormatZPixmap, renderer.win.id,
			renderer.gc, uint16(width), uint16(count), int16(left), int16(top+row), 0, renderer.depth,
			uint32(len(strip)), &strip[0])
		if !renderer.checked {
			renderer.checked = true
			if err := renderer.win.checkRequest("put window image", cookie); err != nil {
				return err
			}
		}
	}
	renderer.win.api.flush(renderer.win.connection)
	return nil
}

func (renderer *cpuWindowRenderer) resize(width, height uint32) error {
	if width == 0 || height == 0 {
		return nil
	}
	if int(width) == renderer.scene.canvas.Width && int(height) == renderer.scene.canvas.Height {
		return nil
	}
	return renderer.scene.resize(int(width), int(height))
}

func (renderer *cpuWindowRenderer) setScale(scale float32) error {
	return renderer.scene.setScale(scale)
}

// setFrameMargin is empty: the CPU path cannot composite a transparent margin,
// and the window has already taken it away when alpha is false.
func (renderer *cpuWindowRenderer) setFrameMargin(uint32) error { return nil }

func (renderer *cpuWindowRenderer) surfaceSize(width, height uint32) (uint32, uint32) {
	return width, height
}

func (renderer *cpuWindowRenderer) refreshText() error { return renderer.scene.refreshText() }

func (renderer *cpuWindowRenderer) forgetSurfaceCapabilities() {}

func (renderer *cpuWindowRenderer) alpha() bool { return false }

func (renderer *cpuWindowRenderer) frameMargin() uint32 { return 0 }

func (renderer *cpuWindowRenderer) deviceSize() (uint32, uint32) {
	return uint32(renderer.scene.canvas.Width), uint32(renderer.scene.canvas.Height)
}

func (renderer *cpuWindowRenderer) firstRectangle() (render.Rectangle, bool) {
	if len(renderer.scene.rects) == 0 {
		return render.Rectangle{}, false
	}
	return renderer.scene.rects[0], true
}

func (renderer *cpuWindowRenderer) stats() rendererStats {
	return rendererStats{backend: "cpu", presents: renderer.presents, rebuilds: renderer.rebuilds, partials: renderer.partials}
}

func (renderer *cpuWindowRenderer) close() {
	if renderer.scene != nil {
		renderer.scene.close()
		renderer.scene = nil
	}
	if renderer.services != nil {
		renderer.services.close()
		renderer.services = nil
	}
	if renderer.gc != 0 && renderer.win != nil && renderer.win.api != nil && renderer.win.connection != 0 {
		renderer.win.api.freeGC(renderer.win.connection, renderer.gc)
		renderer.win.api.flush(renderer.win.connection)
		renderer.gc = 0
	}
	renderer.scratch = nil
}
