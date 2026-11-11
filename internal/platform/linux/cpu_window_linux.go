package linux

import (
	"fmt"
	"math"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/raster"
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
)

// cpuImageStrip bounds one PutImage request at the same size the cursor picture
// uses, under any server's request limit.
const cpuImageStrip = 32 * 1024

// cpuWindowRenderer draws a window with the CPU rasterizer when the machine has
// no usable hardware Vulkan device (ADR 0075). The retained scene tracks the
// damage; a present paints the damage into the canvas and copies it to the X
// window with PutImage, which needs no graphics driver. A window with a drawn
// frame also composes the content over its shadow in a 32-bit canvas and masks
// the content's rounded corners (ADR 0076). It is not safe for concurrent use,
// like the rest of the window.
type cpuWindowRenderer struct {
	scene    *softScene
	services *textServices
	win      *window
	gc       uint32
	// depth is the window's drawable depth, fixed at creation: the root depth,
	// or 32 for a window whose frame Viva draws with a 32-bit visual.
	depth uint8
	// margin is the frame margin in device pixels, 0 without a drawn frame.
	// width and height are the window's device size, including the margin.
	margin        uint32
	width, height uint32
	scale         float32
	// out is the composed window picture when there is a margin: the content
	// over the shadow, with the corners round. Without a margin the scene's
	// canvas is presented directly.
	out         *raster.Canvas
	frameDrawer raster.Rasterizer
	// cornerBox and cornerShadow keep the shadow under the four corner squares,
	// captured before the content covers it, so a damaged corner is composed
	// again from the content and the shadow instead of from the previous result.
	cornerBox    [4]pixelBox
	cornerShadow [4][]byte
	// full says the next present composes the whole window again.
	full bool
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
	contentWidth, contentHeight := contentSize(win.width, win.margin), contentSize(win.height, win.margin)
	scene, err := newSoftScene(services, int(contentWidth), int(contentHeight), scale, clear, rectangles, labels)
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
	renderer := &cpuWindowRenderer{
		scene: scene, services: services, win: win, gc: gc, depth: win.depth,
		margin: win.margin, width: win.width, height: win.height, scale: max(scale, 1), full: true,
	}
	if renderer.margin > 0 {
		renderer.out = raster.NewCanvas(int(win.width), int(win.height))
	}
	return renderer, nil
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
	if renderer.margin == 0 {
		if damage.Right <= damage.Left || damage.Bottom <= damage.Top {
			return nil
		}
		renderer.presents++
		if renderer.full {
			renderer.full = false
			renderer.rebuilds++
		} else {
			renderer.partials++
		}
		return renderer.push(renderer.scene.canvas, damage)
	}
	if renderer.full {
		renderer.composeAll()
		renderer.full = false
		renderer.presents++
		renderer.rebuilds++
		return renderer.push(renderer.out, renderer.out.Bounds())
	}
	if damage.Right <= damage.Left || damage.Bottom <= damage.Top {
		return nil
	}
	renderer.composeDamage(damage)
	renderer.presents++
	renderer.partials++
	offset := int(renderer.margin)
	return renderer.push(renderer.out, render.Scissor{
		Left: damage.Left + offset, Top: damage.Top + offset,
		Right: damage.Right + offset, Bottom: damage.Bottom + offset,
	})
}

func (renderer *cpuWindowRenderer) presentFull() error {
	renderer.scene.damageAll()
	renderer.full = true
	return renderer.present()
}

// composeAll draws the whole window again: the shadow, the content over it, and
// the rounded corners where the two meet.
func (renderer *cpuWindowRenderer) composeAll() {
	if renderer.out == nil {
		return
	}
	renderer.frameDrawer.Draw(renderer.out,
		raster.Params{Scale: renderer.scale, Damage: renderer.out.Bounds()},
		[]render.Rectangle{renderer.shadowRectangle()}, nil)
	renderer.cacheCorners()
	renderer.copyContent()
	for index := range renderer.cornerBox {
		renderer.maskCorner(index, scissorOfBox(renderer.cornerBox[index]))
	}
}

// composeDamage copies a damaged content rectangle into the composed window
// picture and puts the shadow back where the rectangle meets a rounded corner.
func (renderer *cpuWindowRenderer) composeDamage(damage render.Scissor) {
	src := renderer.scene.canvas
	left, top := max(damage.Left, 0), max(damage.Top, 0)
	right, bottom := min(damage.Right, src.Width), min(damage.Bottom, src.Height)
	if right <= left || bottom <= top {
		return
	}
	offset := int(renderer.margin)
	for row := top; row < bottom; row++ {
		from := row*src.Stride + left*4
		to := (offset+row)*renderer.out.Stride + (offset+left)*4
		copy(renderer.out.Pix[to:to+(right-left)*4], src.Pix[from:from+(right-left)*4])
	}
	dest := render.Scissor{Left: left + offset, Top: top + offset, Right: right + offset, Bottom: bottom + offset}
	for index := range renderer.cornerBox {
		part := intersectDamage(dest, scissorOfBox(renderer.cornerBox[index]))
		if part.Right > part.Left && part.Bottom > part.Top {
			renderer.maskCorner(index, part)
		}
	}
}

func (renderer *cpuWindowRenderer) copyContent() {
	src, out := renderer.scene.canvas, renderer.out
	offset := int(renderer.margin)
	for row := 0; row < src.Height; row++ {
		from := row * src.Stride
		to := (offset+row)*out.Stride + offset*4
		copy(out.Pix[to:to+src.Width*4], src.Pix[from:from+src.Width*4])
	}
}

// shadowRectangle is the shadow the drawn frame casts: the content's rounded
// rectangle, offset down and blurred, with the caster cut out by the rasterizer.
func (renderer *cpuWindowRenderer) shadowRectangle() render.Rectangle {
	box := frameContent(renderer.width, renderer.height, renderer.margin)
	scale := renderer.scale
	return render.Rectangle{
		Kind:   scene.KindShadow,
		Bounds: scene.Rect{X: float32(box.x) / scale, Y: float32(box.y) / scale, Width: float32(box.width) / scale, Height: float32(box.height) / scale},
		Radius: frameRadiusLogical,
		Color:  scene.Color{Alpha: frameShadowAlpha},
		Shadow: scene.Shadow{OffsetY: frameShadowOffset, Blur: frameShadowBlur},
	}
}

// cacheCorners keeps the shadow under the four corner squares before the
// content covers it, and the boxes those squares occupy.
func (renderer *cpuWindowRenderer) cacheCorners() {
	renderer.cornerBox = frameCorners(renderer.width, renderer.height, renderer.margin, frameRadiusLogical*renderer.scale)
	for index, box := range renderer.cornerBox {
		box = clampBox(box, renderer.out)
		renderer.cornerBox[index] = box
		width, height := int(box.width), int(box.height)
		if width <= 0 || height <= 0 {
			renderer.cornerShadow[index] = nil
			continue
		}
		need := width * height * 4
		if cap(renderer.cornerShadow[index]) < need {
			renderer.cornerShadow[index] = make([]byte, need)
		}
		shadow := renderer.cornerShadow[index][:need]
		for row := 0; row < height; row++ {
			from := (int(box.y)+row)*renderer.out.Stride + int(box.x)*4
			copy(shadow[row*width*4:(row+1)*width*4], renderer.out.Pix[from:from+width*4])
		}
		renderer.cornerShadow[index] = shadow
	}
}

// maskCorner composes the content over the cached shadow in one corner square,
// where the rounded corner's antialiased edge blends the two.
func (renderer *cpuWindowRenderer) maskCorner(index int, area render.Scissor) {
	box, shadow := renderer.cornerBox[index], renderer.cornerShadow[index]
	if len(shadow) == 0 {
		return
	}
	width := int(box.width)
	if width <= 0 {
		return
	}
	src, out := renderer.scene.canvas, renderer.out
	offset := int(renderer.margin)
	rect := scene.Rect{X: float32(offset) / renderer.scale, Y: float32(offset) / renderer.scale, Width: float32(src.Width) / renderer.scale, Height: float32(src.Height) / renderer.scale}
	for y := area.Top; y < area.Bottom; y++ {
		for x := area.Left; x < area.Right; x++ {
			coverage := raster.Coverage(rect, frameRadiusLogical, renderer.scale, x, y)
			shadowAt := ((y-int(box.y))*width + (x - int(box.x))) * 4
			contentAt := (y-offset)*src.Stride + (x-offset)*4
			pixel := y*out.Stride + x*4
			for channel := 0; channel < 4; channel++ {
				content := src.Pix[contentAt+channel]
				out.Pix[pixel+channel] = byte(float32(content)*coverage + float32(shadow[shadowAt+channel])*(1-coverage) + 0.5)
			}
		}
	}
}

// push copies one damaged region of canvas to the window. A region as wide as
// the canvas is sent straight from its rows; a narrower one is packed into the
// scratch buffer first. Both go in bounded strips.
func (renderer *cpuWindowRenderer) push(canvas *raster.Canvas, damage render.Scissor) error {
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
	if width == renderer.width && height == renderer.height && !renderer.full {
		return nil
	}
	renderer.width, renderer.height = width, height
	contentWidth, contentHeight := contentSize(width, renderer.margin), contentSize(height, renderer.margin)
	if int(contentWidth) != renderer.scene.canvas.Width || int(contentHeight) != renderer.scene.canvas.Height {
		if err := renderer.scene.resize(int(contentWidth), int(contentHeight)); err != nil {
			return err
		}
	}
	if renderer.margin > 0 {
		if renderer.out == nil {
			renderer.out = raster.NewCanvas(int(width), int(height))
		} else {
			renderer.out.Resize(int(width), int(height))
		}
	}
	renderer.full = true
	return nil
}

func (renderer *cpuWindowRenderer) setScale(scale float32) error {
	if err := renderer.scene.setScale(scale); err != nil {
		return err
	}
	renderer.scale = max(scale, 1)
	renderer.full = true
	return nil
}

// setFrameMargin gives the window a frame margin, or takes it away: the scene
// is resized to the content and the next present composes the whole window.
func (renderer *cpuWindowRenderer) setFrameMargin(margin uint32) error {
	if margin == renderer.margin {
		return nil
	}
	renderer.margin = margin
	contentWidth, contentHeight := contentSize(renderer.width, margin), contentSize(renderer.height, margin)
	if int(contentWidth) != renderer.scene.canvas.Width || int(contentHeight) != renderer.scene.canvas.Height {
		if err := renderer.scene.resize(int(contentWidth), int(contentHeight)); err != nil {
			return err
		}
	}
	switch {
	case margin == 0:
		renderer.out = nil
	case renderer.out == nil:
		renderer.out = raster.NewCanvas(int(renderer.width), int(renderer.height))
	}
	renderer.full = true
	return nil
}

func (renderer *cpuWindowRenderer) surfaceSize(width, height uint32) (uint32, uint32) {
	return width, height
}

func (renderer *cpuWindowRenderer) refreshText() error { return renderer.scene.refreshText() }

func (renderer *cpuWindowRenderer) forgetSurfaceCapabilities() {}

func (renderer *cpuWindowRenderer) alpha() bool { return renderer.margin > 0 }

func (renderer *cpuWindowRenderer) frameMargin() uint32 { return renderer.margin }

func (renderer *cpuWindowRenderer) deviceSize() (uint32, uint32) {
	return renderer.width, renderer.height
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
	renderer.out = nil
	renderer.cornerShadow = [4][]byte{}
}

func scissorOfBox(box pixelBox) render.Scissor {
	return render.Scissor{Left: int(box.x), Top: int(box.y), Right: int(box.x + box.width), Bottom: int(box.y + box.height)}
}

func clampBox(box pixelBox, canvas *raster.Canvas) pixelBox {
	left, top := max(box.x, 0), max(box.y, 0)
	right, bottom := min(box.x+box.width, int32(canvas.Width)), min(box.y+box.height, int32(canvas.Height))
	return pixelBox{x: left, y: top, width: max(right-left, 0), height: max(bottom-top, 0)}
}

func intersectDamage(a, b render.Scissor) render.Scissor {
	return render.Scissor{Left: max(a.Left, b.Left), Top: max(a.Top, b.Top), Right: min(a.Right, b.Right), Bottom: min(a.Bottom, b.Bottom)}
}
