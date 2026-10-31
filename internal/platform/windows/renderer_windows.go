package windows

import (
	"fmt"
	"math"
	"os"
	"slices"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
)

// rectangleInstance is one instance: bounds, color and style, then a triangle's
// colors at its second and third corner.
type rectangleInstance struct{ Bounds, Color, Style, Shade1, Shade2 [4]float32 }

const rectangleInstanceBytes = 80

type bufferDesc struct{ Bytes, Usage, Bind, CPU, Misc, Stride uint32 }
type textureDesc struct{ Width, Height, Mips, Array, Format, Samples, Quality, Usage, Bind, CPU, Misc uint32 }
type resourceData struct {
	Data              unsafe.Pointer
	Pitch, SlicePitch uint32
}
type resourceViewDesc struct {
	Format, Dimension uint32
	Fields            [4]uint32
}
type swapDesc struct {
	Width, Height, RateNumerator, RateDenominator, Format, Scanline, Scaling, Samples, Quality, Usage, Buffers uint32
	Window                                                                                                     uintptr
	Windowed                                                                                                   int32
	SwapEffect, Flags                                                                                          uint32
}
type renderBlend struct {
	Enabled                                                                       int32
	Source, Destination, Operation, SourceAlpha, DestinationAlpha, AlphaOperation uint32
	Mask                                                                          uint8
	Padding                                                                       [3]byte
}
type blendDesc struct {
	Coverage, Independent int32
	Targets               [8]renderBlend
}
type rasterDesc struct {
	Fill, Cull                             uint32
	Front, Bias                            int32
	Clamp, Slope                           float32
	DepthClip, Scissor, Multisample, Lines int32
}
type samplerDesc struct {
	Filter, AddressU, AddressV, AddressW uint32
	Bias                                 float32
	Anisotropy, Comparison               uint32
	Border                               [4]float32
	MinLOD, MaxLOD                       float32
}
type viewport struct{ X, Y, Width, Height, MinDepth, MaxDepth float32 }

// scissorRect is a D3D11_RECT: left, top, right and bottom in target pixels.
type scissorRect struct{ Left, Top, Right, Bottom int32 }

// labelScissor is the part of the width by height target a label may paint:
// all of it, or the label's clip, given in logical pixels, when a canvas clip
// cuts it partway. visible is false when that clip leaves no pixel.
func labelScissor(label platform.TextLabel, scale float32, width, height int) (rect scissorRect, visible bool) {
	if !label.Clipped {
		return scissorRect{Right: int32(width), Bottom: int32(height)}, true
	}
	left := max(int64(math.Floor(float64(label.Clip.X*scale))), 0)
	top := max(int64(math.Floor(float64(label.Clip.Y*scale))), 0)
	right := min(int64(math.Ceil(float64((label.Clip.X+label.Clip.Width)*scale))), int64(width))
	bottom := min(int64(math.Ceil(float64((label.Clip.Y+label.Clip.Height)*scale))), int64(height))
	if right <= left || bottom <= top {
		return scissorRect{}, false
	}
	return scissorRect{Left: int32(left), Top: int32(top), Right: int32(right), Bottom: int32(bottom)}, true
}

type box struct{ Left, Top, Front, Right, Bottom, Back uint32 }
type frameConstants struct {
	Width, Height  float32
	InstanceOffset uint32
	// TextMode is 0 for a label texture that is premultiplied color, 1 for one that
	// holds the coverage of ClearType text per channel, blended with TextColor.
	TextMode  uint32
	Bounds    [4]float32
	TextColor [4]float32
	PixelSize float32
	Padding   [3]float32
}

type compositingBatchKind uint8

const (
	compositeRectangles compositingBatchKind = iota + 1
	compositeText
)

type compositingBatch struct {
	Kind         compositingBatchKind
	Layer        uint16
	Clipped      bool
	Clip         scene.Rect
	First, Count int
}

// rectangleScissor is the part of the width by height target a rectangle batch
// may paint: all of it, or the batch's clip, given in logical pixels, rounded to
// the nearest pixel boundary. visible is false when the clip leaves no pixel.
func rectangleScissor(batch compositingBatch, scale float32, width, height int) (rect scissorRect, visible bool) {
	if !batch.Clipped {
		return scissorRect{Right: int32(width), Bottom: int32(height)}, true
	}
	scissor, visible := render.ScissorOf(batch.Clip, scale, width, height)
	if !visible {
		return scissorRect{}, false
	}
	return scissorRect{Left: int32(scissor.Left), Top: int32(scissor.Top), Right: int32(scissor.Right), Bottom: int32(scissor.Bottom)}, true
}

type renderer struct {
	compositionState
	window, device, context, swap, target                        uintptr
	rectangleVertex, rectangleFragment, textVertex, textFragment uintptr
	buffer, view, constants, blend, textBlend, raster, sampler   uintptr
	direct                                                       *directRasterizer
	instances                                                    []rectangleInstance
	rectangleClips                                               []render.Rectangle // only Layer, Clipped and Clip are kept
	rectangleBatches                                             []render.Batch
	labels                                                       []labelTexture
	labelOrder                                                   []int
	compositingBatches                                           []compositingBatch
	color                                                        platform.Color
	capacity, width, height                                      int
	submissions, rebuilds, partials, labelRebuilds               uint64
}

var createDevice = systemDLL("d3d11.dll").NewProc("D3D11CreateDeviceAndSwapChain")
var textureIID = guid{0x6f15aaf2, 0xd208, 0x4e89, [8]byte{0x9a, 0xb4, 0x48, 0x95, 0x35, 0xd3, 0x4f, 0x9c}}

func initialUpdate(count int) render.Update { return render.Update{Count: count, Rebuild: true} }

func groupTextLayers(labels []labelTexture, order []int) []int {
	order = order[:0]
	for index := range labels {
		order = append(order, index)
	}
	slices.SortStableFunc(order, func(left, right int) int {
		return int(labels[left].label.Layer) - int(labels[right].label.Layer)
	})
	return order
}

func groupCompositingLayers(rectangles []render.Batch, labels []labelTexture, labelOrder []int, batches []compositingBatch) []compositingBatch {
	batches = batches[:0]
	for rectangleIndex, labelIndex := 0, 0; rectangleIndex < len(rectangles) || labelIndex < len(labelOrder); {
		layer := ^uint16(0)
		if rectangleIndex < len(rectangles) {
			layer = rectangles[rectangleIndex].Layer
		}
		if labelIndex < len(labelOrder) && labels[labelOrder[labelIndex]].label.Layer < layer {
			layer = labels[labelOrder[labelIndex]].label.Layer
		}
		for rectangleIndex < len(rectangles) && rectangles[rectangleIndex].Layer == layer {
			rectangle := rectangles[rectangleIndex]
			batches = append(batches, compositingBatch{Kind: compositeRectangles, Layer: layer, Clipped: rectangle.Clipped, Clip: rectangle.Clip, First: rectangle.First, Count: rectangle.Count})
			rectangleIndex++
		}
		firstLabel := labelIndex
		for labelIndex < len(labelOrder) && labels[labelOrder[labelIndex]].label.Layer == layer {
			labelIndex++
		}
		if labelIndex != firstLabel {
			batches = append(batches, compositingBatch{Kind: compositeText, Layer: layer, First: firstLabel, Count: labelIndex - firstLabel})
		}
	}
	return batches
}

func (renderer *renderer) initialize(window uintptr, color platform.Color) error {
	renderer.window, renderer.color = window, color
	// VIVA_SOFTWARE_RENDERING=1 skips the graphics driver and draws on the
	// software rasterizer (WARP): a way to tell whether a fault is the driver's.
	return renderer.createDeviceAndResources(os.Getenv("VIVA_SOFTWARE_RENDERING") == "1")
}

// createDeviceAndResources makes the Direct3D device, the window's swap chain and
// everything drawn with them. It tries the GPU and then the software device
// (WARP), or only the software one when software is set.
func (renderer *renderer) createDeviceAndResources(software bool) error {
	if renderer.initializeComposition(software) {
		return nil
	}
	description := swapDesc{Format: 87, Samples: 1, Usage: 0x20, Buffers: 2, Window: renderer.window, Windowed: 1, SwapEffect: 3}
	levels := [...]uint32{0xb000}
	var status uintptr
	drivers := []uintptr{1, 5} // D3D_DRIVER_TYPE_HARDWARE, then _WARP
	if software {
		drivers = drivers[1:]
	}
	for _, driver := range drivers {
		status, _, _ = createDevice.Call(0, driver, 0, 0x20, uintptr(unsafe.Pointer(&levels[0])), 1, 7, uintptr(unsafe.Pointer(&description)), uintptr(unsafe.Pointer(&renderer.swap)), uintptr(unsafe.Pointer(&renderer.device)), 0, uintptr(unsafe.Pointer(&renderer.context)))
		if hresult(status) == nil {
			break
		}
		release(&renderer.context)
		release(&renderer.device)
		release(&renderer.swap)
	}
	if err := hresult(status); err != nil {
		return fmt.Errorf("create D3D11 device and swap chain, hardware and software (WARP) both failed: %w: %w", platform.ErrNoGraphics, err)
	}
	return renderer.createResources()
}

// createResources builds the shaders, constant buffer and pipeline state both
// the window and the offscreen path draw with, on the already created device.
func (renderer *renderer) createResources() error {
	var status uintptr
	for index, shader := range [...]struct {
		Code        []byte
		Destination *uintptr
	}{{rectangleVertexCode, &renderer.rectangleVertex}, {rectangleFragmentCode, &renderer.rectangleFragment}, {textVertexCode, &renderer.textVertex}, {textFragmentCode, &renderer.textFragment}} {
		code := shader.Code
		method := 12
		if index%2 != 0 {
			method = 15
		}
		status = comCall(renderer.device, method, uintptr(unsafe.Pointer(&code[0])), uintptr(len(code)), 0, uintptr(unsafe.Pointer(shader.Destination)))
		if err := hresult(status); err != nil {
			return fmt.Errorf("create shader %d: %w", index, err)
		}
	}
	buffer := bufferDesc{Bytes: uint32(unsafe.Sizeof(frameConstants{})), Bind: 4}
	if err := hresult(comCall(renderer.device, 3, uintptr(unsafe.Pointer(&buffer)), 0, uintptr(unsafe.Pointer(&renderer.constants)))); err != nil {
		return err
	}
	blending := blendDesc{}
	blending.Targets[0] = renderBlend{Enabled: 1, Source: 2, Destination: 6, Operation: 1, SourceAlpha: 2, DestinationAlpha: 6, AlphaOperation: 1, Mask: 15}
	if err := hresult(comCall(renderer.device, 20, uintptr(unsafe.Pointer(&blending)), uintptr(unsafe.Pointer(&renderer.blend)))); err != nil {
		return err
	}
	// Text is blended per channel: the second output of the text shader is the factor
	// that keeps the background (dual-source blending, SRC1 = 16..19 in D3D11_BLEND).
	textBlending := blendDesc{}
	textBlending.Targets[0] = renderBlend{Enabled: 1, Source: 2, Destination: 17, Operation: 1, SourceAlpha: 2, DestinationAlpha: 19, AlphaOperation: 1, Mask: 15}
	if err := hresult(comCall(renderer.device, 20, uintptr(unsafe.Pointer(&textBlending)), uintptr(unsafe.Pointer(&renderer.textBlend)))); err != nil {
		return err
	}
	raster := rasterDesc{Fill: 3, Cull: 1, DepthClip: 1, Scissor: 1}
	if err := hresult(comCall(renderer.device, 22, uintptr(unsafe.Pointer(&raster)), uintptr(unsafe.Pointer(&renderer.raster)))); err != nil {
		return err
	}
	sampler := samplerDesc{Filter: 0x15, AddressU: 3, AddressV: 3, AddressW: 3, MaxLOD: math.MaxFloat32}
	return hresult(comCall(renderer.device, 23, uintptr(unsafe.Pointer(&sampler)), uintptr(unsafe.Pointer(&renderer.sampler))))
}

// storeRectangles copies the changed instances and their layer and clip, then
// regroups the batches. It returns the range of instances to upload.
func (renderer *renderer) storeRectangles(frame platform.Frame) (first, count int, err error) {
	update := frame.Update
	if update.First < 0 || update.Count < 0 || update.First > len(frame.Rectangles) || update.Count > len(frame.Rectangles)-update.First {
		return 0, 0, fmt.Errorf("windows: invalid rectangle update")
	}
	if update.Rebuild {
		if len(frame.Rectangles) < len(renderer.instances) {
			clear(renderer.instances[len(frame.Rectangles):])
			clear(renderer.rectangleClips[len(frame.Rectangles):])
		}
		if cap(renderer.instances) < len(frame.Rectangles) {
			renderer.instances = make([]rectangleInstance, len(frame.Rectangles))
		} else {
			renderer.instances = renderer.instances[:len(frame.Rectangles)]
		}
		if cap(renderer.rectangleClips) < len(frame.Rectangles) {
			renderer.rectangleClips = make([]render.Rectangle, len(frame.Rectangles))
		} else {
			renderer.rectangleClips = renderer.rectangleClips[:len(frame.Rectangles)]
		}
	} else if len(renderer.instances) < len(frame.Rectangles) {
		// A pure append grows the retained instance store. The update range is
		// the new tail, so existing instances keep their slots.
		if update.First > len(renderer.instances) || update.First+update.Count != len(frame.Rectangles) {
			return 0, 0, fmt.Errorf("windows: rectangle count grew outside a suffix update")
		}
		old := len(renderer.instances)
		if cap(renderer.instances) < len(frame.Rectangles) {
			next := make([]rectangleInstance, len(frame.Rectangles))
			copy(next, renderer.instances)
			renderer.instances = next
			nextClips := make([]render.Rectangle, len(frame.Rectangles))
			copy(nextClips, renderer.rectangleClips)
			renderer.rectangleClips = nextClips
		} else {
			renderer.instances = renderer.instances[:len(frame.Rectangles)]
			renderer.rectangleClips = renderer.rectangleClips[:len(frame.Rectangles)]
			clear(renderer.instances[old:])
			clear(renderer.rectangleClips[old:])
		}
	} else if len(renderer.instances) != len(frame.Rectangles) {
		return 0, 0, fmt.Errorf("windows: rectangle count shrank without rebuild")
	}
	first, count = update.First, update.Count
	if update.Rebuild {
		first, count = 0, len(frame.Rectangles)
		renderer.rebuilds++
	} else {
		renderer.partials++
	}
	for index := first; index < first+count; index++ {
		renderer.instances[index] = makeRectangleInstance(frame.Rectangles[index])
		rectangle := &frame.Rectangles[index]
		renderer.rectangleClips[index] = render.Rectangle{Layer: rectangle.Layer, Clipped: rectangle.Clipped, Clip: rectangle.Clip}
	}
	renderer.rectangleBatches = render.GroupBatches(renderer.rectangleBatches, renderer.rectangleClips)
	return first, count, nil
}

func (renderer *renderer) apply(frame platform.Frame) error {
	compositingChanged := false
	update := frame.Update
	if !update.Empty() {
		first, count, err := renderer.storeRectangles(frame)
		if err != nil {
			return err
		}
		compositingChanged = true
		if len(renderer.instances) > renderer.capacity {
			if err := renderer.growInstanceBuffer(max(len(renderer.instances), renderer.capacity*2)); err != nil {
				return err
			}
			first, count = 0, len(renderer.instances)
		}
		if count > 0 {
			region := box{Left: uint32(first * rectangleInstanceBytes), Right: uint32((first + count) * rectangleInstanceBytes), Bottom: 1, Back: 1}
			comCall(renderer.context, 48, renderer.buffer, 0, uintptr(unsafe.Pointer(&region)), uintptr(unsafe.Pointer(&renderer.instances[first])), 0, 0)
		}
	}
	if frame.TextLabels != nil {
		for index := len(frame.TextLabels); index < len(renderer.labels); index++ {
			renderer.labels[index].close()
		}
		if len(frame.TextLabels) > cap(renderer.labels) {
			renderer.labels = append(renderer.labels, make([]labelTexture, len(frame.TextLabels)-len(renderer.labels))...)
		} else {
			renderer.labels = renderer.labels[:len(frame.TextLabels)]
		}
		for index, label := range frame.TextLabels {
			renderer.labels[index].update(label)
		}
		renderer.labelOrder = groupTextLayers(renderer.labels, renderer.labelOrder)
		compositingChanged = true
	}
	if compositingChanged {
		renderer.compositingBatches = groupCompositingLayers(renderer.rectangleBatches, renderer.labels, renderer.labelOrder, renderer.compositingBatches)
	}
	return nil
}

// growInstanceBuffer replaces the GPU buffer of rectangle instances by one that
// holds capacity of them. It is empty: the caller uploads what it holds.
func (renderer *renderer) growInstanceBuffer(capacity int) error {
	release(&renderer.view)
	release(&renderer.buffer)
	if capacity > int(^uint32(0))/rectangleInstanceBytes {
		return fmt.Errorf("windows: rectangle buffer exceeds device limit")
	}
	description := bufferDesc{Bytes: uint32(capacity * rectangleInstanceBytes), Bind: 8, Misc: 0x40, Stride: rectangleInstanceBytes}
	if err := hresult(comCall(renderer.device, 3, uintptr(unsafe.Pointer(&description)), 0, uintptr(unsafe.Pointer(&renderer.buffer)))); err != nil {
		return err
	}
	view := resourceViewDesc{Dimension: 1, Fields: [4]uint32{0, uint32(capacity)}}
	if err := hresult(comCall(renderer.device, 7, renderer.buffer, uintptr(unsafe.Pointer(&view)), uintptr(unsafe.Pointer(&renderer.view)))); err != nil {
		return err
	}
	renderer.capacity = capacity
	return nil
}

func makeRectangleInstance(rectangle render.Rectangle) rectangleInstance {
	instance := rectangleInstance{
		Bounds: [4]float32{rectangle.Bounds.X, rectangle.Bounds.Y, rectangle.Bounds.Width, rectangle.Bounds.Height},
		Color:  [4]float32{rectangle.Color.Red, rectangle.Color.Green, rectangle.Color.Blue, rectangle.Color.Alpha},
		Style:  [4]float32{rectangle.Radius},
	}
	if rectangle.Kind == scene.KindTriangle {
		// Same triangle slot convention as the Metal backend:
		// bounds=(p0, p1), style=(p2.x, layer, p2.y, edgeMask+1).
		instance.Bounds = [4]float32{rectangle.Points[0].X, rectangle.Points[0].Y, rectangle.Points[1].X, rectangle.Points[1].Y}
		instance.Style = [4]float32{rectangle.Points[2].X, float32(rectangle.Layer), rectangle.Points[2].Y, float32(rectangle.EdgeMask + 1)}
		colors := rectangle.VertexColors()
		instance.Shade1 = [4]float32{colors[1].Red, colors[1].Green, colors[1].Blue, colors[1].Alpha}
		instance.Shade2 = [4]float32{colors[2].Red, colors[2].Green, colors[2].Blue, colors[2].Alpha}
	}
	if rectangle.Kind == scene.KindShadow && rectangle.Shadow.Blur > 0 {
		// The casting rectangle keeps the slot's bounds and radius; the blur is the
		// third style value and the offset and spread the first three shade1 values.
		instance.Style[2] = rectangle.Shadow.Blur
		instance.Shade1 = [4]float32{rectangle.Shadow.OffsetX, rectangle.Shadow.OffsetY, rectangle.Shadow.Spread}
	}
	return instance
}

func (renderer *renderer) present(width, height int, scale float32) error {
	if width <= 0 || height <= 0 {
		return nil
	}
	if renderer.width != width || renderer.height != height || renderer.target == 0 {
		comCall(renderer.context, 33, 0, 0, 0)
		release(&renderer.target)
		if err := hresult(comCall(renderer.swap, 13, 0, uintptr(width), uintptr(height), 0, 0)); err != nil {
			return fmt.Errorf("resize swap chain: %w", err)
		}
		var back uintptr
		if err := hresult(comCall(renderer.swap, 9, 0, uintptr(unsafe.Pointer(&textureIID)), uintptr(unsafe.Pointer(&back)))); err != nil {
			return fmt.Errorf("get swap-chain texture: %w", err)
		}
		status := comCall(renderer.device, 9, back, 0, uintptr(unsafe.Pointer(&renderer.target)))
		release(&back)
		if err := hresult(status); err != nil {
			return err
		}
		renderer.width, renderer.height = width, height
	}
	if err := renderer.draw(width, height, scale); err != nil {
		return err
	}
	return hresult(comCall(renderer.swap, 8, 1, 0))
}

// draw clears the bound-size render target to the clear color and paints every
// compositing batch onto it. The window's present and the offscreen render
// share it, so a screenshot is made of the pixels the window would show.
func (renderer *renderer) draw(width, height int, scale float32) error {
	clear := [4]float32{float32(renderer.color.Red), float32(renderer.color.Green), float32(renderer.color.Blue), float32(renderer.color.Alpha)}
	if renderer.usesComposition() {
		clear[0] *= clear[3]
		clear[1] *= clear[3]
		clear[2] *= clear[3]
	}
	comCall(renderer.context, 50, renderer.target, uintptr(unsafe.Pointer(&clear)))
	comCall(renderer.context, 33, 1, uintptr(unsafe.Pointer(&renderer.target)), 0)
	comCall(renderer.context, 35, renderer.blend, 0, uintptr(^uint32(0)))
	comCall(renderer.context, 43, renderer.raster)
	viewport := viewport{Width: float32(width), Height: float32(height), MaxDepth: 1}
	comCall(renderer.context, 44, 1, uintptr(unsafe.Pointer(&viewport)))
	// Scissoring is on for every draw, so the whole target is set first and a
	// rectangle batch or label cut by a canvas clip narrows it for its own draw.
	whole := scissorRect{Right: int32(width), Bottom: int32(height)}
	comCall(renderer.context, 45, 1, uintptr(unsafe.Pointer(&whole)))
	comCall(renderer.context, 24, 4)
	frame := frameConstants{Width: float32(width) / scale, Height: float32(height) / scale, PixelSize: 1 / scale}
	comCall(renderer.context, 48, renderer.constants, 0, 0, uintptr(unsafe.Pointer(&frame)), 0, 0)
	comCall(renderer.context, 7, 0, 1, uintptr(unsafe.Pointer(&renderer.constants)))  // the vertex shaders
	comCall(renderer.context, 16, 0, 1, uintptr(unsafe.Pointer(&renderer.constants))) // the text pixel shader
	boundKind := compositingBatchKind(0)
	current := whole
	var cut scissorRect // the columns of one span of a grid line; one variable, since its address escapes
	for _, batch := range renderer.compositingBatches {
		switch batch.Kind {
		case compositeRectangles:
			if boundKind != compositeRectangles {
				comCall(renderer.context, 35, renderer.blend, 0, uintptr(^uint32(0)))
				comCall(renderer.context, 11, renderer.rectangleVertex, 0, 0)
				comCall(renderer.context, 9, renderer.rectangleFragment, 0, 0)
				comCall(renderer.context, 25, 0, 1, uintptr(unsafe.Pointer(&renderer.view)))
				boundKind = compositeRectangles
			}
			scissor, visible := rectangleScissor(batch, scale, width, height)
			if !visible {
				continue
			}
			if scissor != current {
				comCall(renderer.context, 45, 1, uintptr(unsafe.Pointer(&scissor)))
				current = scissor
			}
			frame.InstanceOffset = uint32(batch.First)
			comCall(renderer.context, 48, renderer.constants, 0, 0, uintptr(unsafe.Pointer(&frame)), 0, 0)
			comCall(renderer.context, 21, 6, uintptr(batch.Count), 0, uintptr(batch.First))
		case compositeText:
			if boundKind != compositeText {
				comCall(renderer.context, 35, renderer.textBlend, 0, uintptr(^uint32(0)))
				comCall(renderer.context, 11, renderer.textVertex, 0, 0)
				comCall(renderer.context, 9, renderer.textFragment, 0, 0)
				comCall(renderer.context, 10, 0, 1, uintptr(unsafe.Pointer(&renderer.sampler)))
				boundKind = compositeText
			}
			for labelOffset := batch.First; labelOffset < batch.First+batch.Count; labelOffset++ {
				index := renderer.labelOrder[labelOffset]
				label := &renderer.labels[index]
				if label.label.Width <= 0 || label.label.Height <= 0 || label.label.Value == "" && len(label.label.Bitmap) == 0 {
					continue
				}
				skip, windowWidth, shown := label.window(scale)
				if !shown {
					label.close()
					continue
				}
				if label.view == 0 || label.scale != scale || label.skip != skip || skip > 0 && label.width != windowWidth {
					if err := renderer.rasterize(label, scale); err != nil {
						return fmt.Errorf("rasterize label %d: %w", index, err)
					}
				}
				scissor, visible := labelScissor(label.label, scale, width, height)
				if !visible {
					continue
				}
				frame.Bounds = label.quad(scale)
				frame.TextMode = 0
				if label.lcd {
					frame.TextMode = 1
					frame.TextColor = [4]float32{float32(label.label.Color.Red), float32(label.label.Color.Green), float32(label.label.Color.Blue), float32(label.label.Color.Alpha)}
				}
				comCall(renderer.context, 8, 1, 1, uintptr(unsafe.Pointer(&label.view)))
				if len(label.spans) == 0 {
					comCall(renderer.context, 48, renderer.constants, 0, 0, uintptr(unsafe.Pointer(&frame)), 0, 0)
					if scissor != current {
						comCall(renderer.context, 45, 1, uintptr(unsafe.Pointer(&scissor)))
						current = scissor
					}
					comCall(renderer.context, 13, 6, 0)
					continue
				}
				// A grid line has a color per run: its coverage is drawn once for each, cut to the
				// columns of the run.
				origin, _ := label.pixelOrigin(scale)
				for _, span := range label.spans {
					cut = scissor
					cut.Left, cut.Right = max(cut.Left, int32(origin+span.left)), min(cut.Right, int32(origin+span.right))
					if cut.Right <= cut.Left {
						continue
					}
					frame.TextColor = [4]float32{span.color.R, span.color.G, span.color.B, span.color.A}
					comCall(renderer.context, 48, renderer.constants, 0, 0, uintptr(unsafe.Pointer(&frame)), 0, 0)
					if cut != current {
						comCall(renderer.context, 45, 1, uintptr(unsafe.Pointer(&cut)))
						current = cut
					}
					comCall(renderer.context, 13, 6, 0)
				}
			}
		}
	}
	var empty uintptr
	comCall(renderer.context, 8, 1, 1, uintptr(unsafe.Pointer(&empty)))
	renderer.submissions++
	return nil
}

func (renderer *renderer) close() {
	for index := range renderer.labels {
		renderer.labels[index].close()
	}
	if renderer.direct != nil {
		renderer.direct.close()
		renderer.direct = nil
	}
	renderer.labels = nil
	renderer.labelOrder = nil
	renderer.instances = nil
	renderer.rectangleClips = nil
	renderer.rectangleBatches = nil
	renderer.compositingBatches = nil
	renderer.releaseDevice()
}

// releaseDevice frees the device and every GPU object made on it: the swap chain,
// the pipeline, the instance buffer and the label textures. What the renderer
// keeps in memory, the instances, the labels and their batches, is left, so a
// device made again can draw the same scene.
func (renderer *renderer) releaseDevice() {
	if renderer.context != 0 {
		comCall(renderer.context, 110)
		comCall(renderer.context, 111)
	}
	for index := range renderer.labels {
		release(&renderer.labels[index].view)
		release(&renderer.labels[index].texture)
	}
	renderer.capacity, renderer.width, renderer.height = 0, 0, 0
	renderer.releaseComposition()
	for _, object := range [...]*uintptr{&renderer.target, &renderer.swap, &renderer.view, &renderer.buffer, &renderer.constants, &renderer.blend, &renderer.textBlend, &renderer.raster, &renderer.sampler, &renderer.rectangleVertex, &renderer.rectangleFragment, &renderer.textVertex, &renderer.textFragment, &renderer.context, &renderer.device} {
		release(object)
	}
}
