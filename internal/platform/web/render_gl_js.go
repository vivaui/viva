//go:build js && wasm

package web

import (
	"errors"
	"math"
	"syscall/js"
	"unsafe"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
)

var errContextLost = errors.New("webgl2: context lost")

type textKey struct {
	bitmap    bool
	id        uint64
	version   uint32
	hash      uint64
	value     string
	size      float32
	weight    float32
	italic    bool
	color     platform.Color
	fontGen   uint32
	mono      bool
	monoFace  gfx.MonoFace
	cellWidth float32
	baseline  float32
	runs      uint64
	width     int
	height    int
	stride    int
	deviceW   int
	deviceH   int
	tileLeft  int
	tileRight int
	scale     float32
	identity  gfx.TextIdentity
}

type labelTile struct {
	texture js.Value
	left    int
	right   int
	width   int
	height  int
	corners [4][2]float32
	tints   [4][4]float32
}

type glTexture struct {
	texture js.Value
	width   int
	height  int
	bytes   int64
	used    uint64
}

type glRenderer struct {
	ctx       js.Value
	program   js.Value
	text      js.Value
	attribs   [5]int
	uView     js.Value
	uPixel    js.Value
	uTarget   js.Value
	uOrigin   js.Value
	uCorners  [4]js.Value
	uTints    [4]js.Value
	uTexture  js.Value
	uLabel    js.Value
	uMask     js.Value
	vbo       js.Value
	floats    []float32
	batches   []render.Batch
	orders    []render.LabelOrder
	plan      []render.PlanOp
	planOrder []int
	tiles     []labelTile
	width     int
	height    int
	scale     float32
	maxTex    int
	frame     uint64
	textures  map[textKey]*glTexture
	bytes     int64
	runtime   *runtime
}

func newGLRenderer(runtime *runtime, canvas js.Value) (*glRenderer, error) {
	ctx := canvas.Call("getContext", "webgl2", map[string]any{
		"alpha": true, "depth": false, "stencil": false, "antialias": false,
		"premultipliedAlpha": true, "preserveDrawingBuffer": false,
		"powerPreference": "default",
	})
	if !ctx.Truthy() || ctx.IsNull() {
		return nil, errors.New("webgl2: no context")
	}
	vertexSource, err := shaderSource("scene.vert.glsl")
	if err != nil {
		return nil, err
	}
	fragmentSource, err := shaderSource("scene.frag.glsl")
	if err != nil {
		return nil, err
	}
	vertex, err := compileShader(ctx, "VERTEX_SHADER", vertexSource)
	if err != nil {
		return nil, err
	}
	fragment, err := compileShader(ctx, "FRAGMENT_SHADER", fragmentSource)
	if err != nil {
		return nil, err
	}
	program, err := linkProgram(ctx, vertex, fragment)
	if err != nil {
		return nil, err
	}
	textVertexSource, err := shaderSource("text.vert.glsl")
	if err != nil {
		return nil, err
	}
	textFragmentSource, err := shaderSource("text.frag.glsl")
	if err != nil {
		return nil, err
	}
	textVertex, err := compileShader(ctx, "VERTEX_SHADER", textVertexSource)
	if err != nil {
		return nil, err
	}
	textFragment, err := compileShader(ctx, "FRAGMENT_SHADER", textFragmentSource)
	if err != nil {
		return nil, err
	}
	textProgram, err := linkProgram(ctx, textVertex, textFragment)
	if err != nil {
		return nil, err
	}
	renderer := &glRenderer{
		ctx: ctx, program: program, text: textProgram,
		attribs:  [5]int{0, 1, 2, 3, 4},
		uView:    ctx.Call("getUniformLocation", program, "uViewport"),
		uPixel:   ctx.Call("getUniformLocation", program, "uPixelSize"),
		uTarget:  ctx.Call("getUniformLocation", program, "uTargetPixels"),
		uOrigin:  ctx.Call("getUniformLocation", program, "uOrigin"),
		uTexture: ctx.Call("getUniformLocation", textProgram, "uTexture"),
		uLabel:   ctx.Call("getUniformLocation", textProgram, "uLabel"),
		uMask:    ctx.Call("getUniformLocation", textProgram, "uMask"),
		runtime:  runtime,
		textures: make(map[textKey]*glTexture),
	}
	for index := range renderer.uCorners {
		renderer.uCorners[index] = ctx.Call("getUniformLocation", textProgram, "uCorners["+itoa(index)+"]")
		renderer.uTints[index] = ctx.Call("getUniformLocation", textProgram, "uTint["+itoa(index)+"]")
	}
	renderer.vbo = ctx.Call("createBuffer")
	ctx.Call("bindBuffer", ctx.Get("ARRAY_BUFFER"), renderer.vbo)
	ctx.Call("enable", ctx.Get("BLEND"))
	ctx.Call("blendFunc", ctx.Get("ONE"), ctx.Get("ONE_MINUS_SRC_ALPHA"))
	ctx.Call("disable", ctx.Get("DEPTH_TEST"))
	ctx.Call("disable", ctx.Get("CULL_FACE"))
	ctx.Call("enable", ctx.Get("SCISSOR_TEST"))
	ctx.Call("pixelStorei", ctx.Get("UNPACK_ALIGNMENT"), 1)
	renderer.maxTex = ctx.Call("getParameter", ctx.Get("MAX_TEXTURE_SIZE")).Int()
	if renderer.maxTex <= 0 {
		renderer.maxTex = 2048
	}
	// A test can lower the limit to exercise tiling on a small viewport.
	if override := js.Global().Get("vivaMaxTexture"); override.Type() == js.TypeNumber {
		if limit := override.Int(); limit > 0 && limit < renderer.maxTex {
			renderer.maxTex = limit
		}
	}
	return renderer, nil
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	var digits [3]byte
	position := len(digits)
	for value > 0 {
		position--
		digits[position] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[position:])
}

func (renderer *glRenderer) Resize(deviceWidth, deviceHeight int, scale float32) {
	renderer.width, renderer.height, renderer.scale = deviceWidth, deviceHeight, scale
	renderer.ctx.Call("viewport", 0, 0, deviceWidth, deviceHeight)
	renderer.ctx.Call("useProgram", renderer.program)
	renderer.ctx.Call("uniform2f", renderer.uView, float32(deviceWidth)/scale, float32(deviceHeight)/scale)
	renderer.ctx.Call("uniform1f", renderer.uPixel, 1/scale)
	renderer.ctx.Call("uniform2f", renderer.uTarget, float32(deviceWidth), float32(deviceHeight))
	renderer.ctx.Call("uniform2f", renderer.uOrigin, 0, 0)
	renderer.ctx.Call("useProgram", renderer.text)
	renderer.ctx.Call("uniform2f", renderer.ctx.Call("getUniformLocation", renderer.text, "uViewport"), float32(deviceWidth)/scale, float32(deviceHeight)/scale)
}

func (renderer *glRenderer) Draw(frame platform.Frame, labels []platform.TextLabel, clear gfx.Color) error {
	if renderer.ctx.Call("isContextLost").Bool() {
		return errContextLost
	}
	renderer.frame++
	ctx := renderer.ctx
	ctx.Call("clearColor", float64(clear.Red*clear.Alpha), float64(clear.Green*clear.Alpha), float64(clear.Blue*clear.Alpha), float64(clear.Alpha))
	ctx.Call("clear", ctx.Get("COLOR_BUFFER_BIT"))
	if err := renderer.upload(frame); err != nil {
		return err
	}
	batches := render.GroupBatches(renderer.batches[:0], frame.Rectangles)
	renderer.batches = batches
	orders := renderer.orders[:0]
	for _, label := range labels {
		orders = append(orders, render.LabelOrder{Layer: label.Layer, ShapesBefore: label.ShapesBefore})
	}
	renderer.orders = orders
	plan, order := render.PlanWithOrder(renderer.plan[:0], renderer.planOrder, batches, orders)
	renderer.plan, renderer.planOrder = plan, order
	for _, op := range plan {
		switch op.Kind {
		case render.PlanShapes:
			renderer.drawShapes(op.First, op.Count, op.Clipped, op.Clip)
		case render.PlanLabels:
			if op.Label >= 0 && op.Label < len(labels) {
				renderer.drawLabel(labels[op.Label])
			}
		}
	}
	return nil
}

func (renderer *glRenderer) upload(frame platform.Frame) error {
	ctx := renderer.ctx
	count := len(frame.Rectangles)
	needed := count * 20
	if cap(renderer.floats) < needed {
		grown := make([]float32, needed, needed*2)
		copy(grown, renderer.floats)
		renderer.floats = grown
	}
	rebuild := frame.Update.Rebuild || len(renderer.floats) < needed || renderer.frame == 1
	if rebuild {
		renderer.floats = renderer.floats[:needed]
		for index, instance := range frame.Rectangles {
			packInstance(renderer.floats[index*20:], instance)
		}
		ctx.Call("bindBuffer", ctx.Get("ARRAY_BUFFER"), renderer.vbo)
		ctx.Call("bufferData", ctx.Get("ARRAY_BUFFER"), jsFloat32(renderer.floats), ctx.Get("DYNAMIC_DRAW"))
		return nil
	}
	if frame.Update.Count <= 0 {
		renderer.floats = renderer.floats[:needed]
		return nil
	}
	first, last := frame.Update.First, frame.Update.First+frame.Update.Count
	if first < 0 {
		first = 0
	}
	if last > count {
		last = count
	}
	if first >= last {
		return nil
	}
	renderer.floats = renderer.floats[:needed]
	for index := first; index < last; index++ {
		packInstance(renderer.floats[index*20:], frame.Rectangles[index])
	}
	ctx.Call("bindBuffer", ctx.Get("ARRAY_BUFFER"), renderer.vbo)
	ctx.Call("bufferSubData", ctx.Get("ARRAY_BUFFER"), first*80, jsFloat32(renderer.floats[first*20:last*20]))
	return nil
}

func (renderer *glRenderer) drawShapes(first, count int, clipped bool, clip scene.Rect) {
	if count <= 0 {
		return
	}
	ctx := renderer.ctx
	renderer.scissor(clipped, clip)
	ctx.Call("useProgram", renderer.program)
	ctx.Call("bindBuffer", ctx.Get("ARRAY_BUFFER"), renderer.vbo)
	for location := 0; location < 5; location++ {
		ctx.Call("vertexAttribPointer", location, 4, ctx.Get("FLOAT"), false, 80, first*80+location*16)
		ctx.Call("enableVertexAttribArray", location)
		ctx.Call("vertexAttribDivisor", location, 1)
	}
	ctx.Call("drawArraysInstanced", ctx.Get("TRIANGLES"), 0, 6, count)
}

// labelScissor is the device-pixel rectangle a clipped label may paint into.
// Labels use floor/ceil, like the native label backends, so partial coverage at
// the clip edge is kept.
func labelScissor(clip scene.Rect, scale float32, width, height int) (left, top, right, bottom int) {
	clamp := func(value float64, limit int) int {
		return int(min(max(value, 0), float64(limit)))
	}
	left = clamp(math.Floor(float64(clip.X*scale)), width)
	top = clamp(math.Floor(float64(clip.Y*scale)), height)
	right = clamp(math.Ceil(float64((clip.X+clip.Width)*scale)), width)
	bottom = clamp(math.Ceil(float64((clip.Y+clip.Height)*scale)), height)
	return left, top, right, bottom
}

func (renderer *glRenderer) scissor(clipped bool, clip scene.Rect) {
	ctx := renderer.ctx
	if !clipped {
		ctx.Call("scissor", 0, 0, renderer.width, renderer.height)
		return
	}
	box, visible := render.ScissorOf(clip, renderer.scale, renderer.width, renderer.height)
	if !visible {
		ctx.Call("scissor", 0, 0, 0, 0)
		return
	}
	ctx.Call("scissor", box.Left, renderer.height-box.Bottom, box.Right-box.Left, box.Bottom-box.Top)
}

func (renderer *glRenderer) drawLabel(label platform.TextLabel) {
	deviceWidth := int(math.Ceil(float64(label.Width) * float64(renderer.scale)))
	deviceHeight := int(math.Ceil(float64(label.Height) * float64(renderer.scale)))
	if deviceWidth <= 0 || deviceHeight <= 0 {
		return
	}
	ctx := renderer.ctx
	if label.Clipped {
		left, top, right, bottom := labelScissor(scene.Rect{X: label.Clip.X, Y: label.Clip.Y, Width: label.Clip.Width, Height: label.Clip.Height}, renderer.scale, renderer.width, renderer.height)
		if right <= left || bottom <= top {
			return
		}
		ctx.Call("scissor", left, renderer.height-bottom, right-left, bottom-top)
	} else {
		ctx.Call("scissor", 0, 0, renderer.width, renderer.height)
	}
	for _, tile := range renderer.labelTiles(label, deviceWidth, deviceHeight) {
		if tile.texture.IsUndefined() || tile.texture.IsNull() {
			continue
		}
		ctx.Call("useProgram", renderer.text)
		ctx.Call("activeTexture", ctx.Get("TEXTURE0"))
		ctx.Call("bindTexture", ctx.Get("TEXTURE_2D"), tile.texture)
		ctx.Call("uniform1i", renderer.uTexture, 0)
		radius := label.ImageRadius
		if label.HasQuad {
			// A warped label's quad is not its box: the rounded mask has no
			// meaning on it, so the page sees a square picture there.
			radius = 0
		}
		ctx.Call("uniform4f", renderer.uLabel,
			label.X+label.OriginX, label.Y+label.OriginY, label.Width, label.Height)
		ctx.Call("uniform2f", renderer.uMask, radius, label.ImageFade)
		for index := 0; index < 4; index++ {
			ctx.Call("uniform4f", renderer.uCorners[index], tile.corners[index][0], tile.corners[index][1], 0, 0)
			ctx.Call("uniform4f", renderer.uTints[index], tile.tints[index][0], tile.tints[index][1], tile.tints[index][2], tile.tints[index][3])
		}
		ctx.Call("drawArrays", ctx.Get("TRIANGLES"), 0, 6)
	}
}

// labelTiles splits a label wider than the GPU's largest texture into
// horizontal tiles. Each tile carries its own texture, destination quad and
// corner tints. A label one texture wide gets one tile.
func (renderer *glRenderer) labelTiles(label platform.TextLabel, deviceWidth, deviceHeight int) []labelTile {
	renderer.tiles = renderer.tiles[:0]
	for _, bounds := range tileRanges(deviceWidth, renderer.maxTex) {
		tile := renderer.labelTile(label, deviceWidth, deviceHeight, bounds[0], bounds[1])
		if tile.width <= 0 || tile.height <= 0 {
			continue
		}
		renderer.tiles = append(renderer.tiles, tile)
	}
	return renderer.tiles
}

func (renderer *glRenderer) labelTile(label platform.TextLabel, deviceWidth, deviceHeight, left, right int) labelTile {
	tile := labelTile{left: left, right: right}
	x0 := label.X + label.OriginX + float32(left)/renderer.scale
	x1 := label.X + label.OriginX + float32(right)/renderer.scale
	y0 := label.Y + label.OriginY
	y1 := y0 + label.Height
	tile.corners = [4][2]float32{{x0, y0}, {x1, y0}, {x0, y1}, {x1, y1}}
	for index := range tile.tints {
		tile.tints[index] = [4]float32{1, 1, 1, 1}
	}
	if label.HasQuad {
		u0 := float32(left) / float32(deviceWidth)
		u1 := float32(right) / float32(deviceWidth)
		tile.corners[0] = quadPoint(label.Quad, u0, 0)
		tile.corners[1] = quadPoint(label.Quad, u1, 0)
		tile.corners[2] = quadPoint(label.Quad, u0, 1)
		tile.corners[3] = quadPoint(label.Quad, u1, 1)
		tile.tints[0] = lerpTint(label.Tint[0], label.Tint[1], u0)
		tile.tints[1] = lerpTint(label.Tint[0], label.Tint[1], u1)
		tile.tints[2] = lerpTint(label.Tint[2], label.Tint[3], u0)
		tile.tints[3] = lerpTint(label.Tint[2], label.Tint[3], u1)
	}
	tile.texture, tile.width, tile.height = renderer.tileTexture(label, deviceWidth, deviceHeight, left, right)
	return tile
}

func quadPoint(quad [4]gfx.Point, u, v float32) [2]float32 {
	top := gfx.Point{X: quad[0].X + (quad[1].X-quad[0].X)*u, Y: quad[0].Y + (quad[1].Y-quad[0].Y)*u}
	bottom := gfx.Point{X: quad[2].X + (quad[3].X-quad[2].X)*u, Y: quad[2].Y + (quad[3].Y-quad[2].Y)*u}
	return [2]float32{top.X + (bottom.X-top.X)*v, top.Y + (bottom.Y-top.Y)*v}
}

func lerpTint(left, right platform.Color, u float32) [4]float32 {
	return [4]float32{
		float32(left.Red) + (float32(right.Red)-float32(left.Red))*u,
		float32(left.Green) + (float32(right.Green)-float32(left.Green))*u,
		float32(left.Blue) + (float32(right.Blue)-float32(left.Blue))*u,
		float32(left.Alpha) + (float32(right.Alpha)-float32(left.Alpha))*u,
	}
}

// tileTexture rasterizes one horizontal tile of a label and retains it.
func (renderer *glRenderer) tileTexture(label platform.TextLabel, deviceWidth, deviceHeight, left, right int) (js.Value, int, int) {
	key := renderer.key(label)
	key.tileLeft, key.tileRight = left, right
	key.deviceW, key.deviceH = deviceWidth, deviceHeight
	if texture, ok := renderer.textures[key]; ok {
		texture.used = renderer.frame
		return texture.texture, texture.width, texture.height
	}
	if len(label.Bitmap) == 0 && label.BitmapID != 0 {
		return js.Undefined(), 0, 0
	}
	width, height, pixels, ok := renderer.rasterizeTile(label, deviceWidth, deviceHeight, left, right)
	if !ok {
		return js.Undefined(), 0, 0
	}
	ctx := renderer.ctx
	texture := ctx.Call("createTexture")
	ctx.Call("bindTexture", ctx.Get("TEXTURE_2D"), texture)
	ctx.Call("texParameteri", ctx.Get("TEXTURE_2D"), ctx.Get("TEXTURE_MIN_FILTER"), ctx.Get("LINEAR"))
	ctx.Call("texParameteri", ctx.Get("TEXTURE_2D"), ctx.Get("TEXTURE_MAG_FILTER"), ctx.Get("LINEAR"))
	ctx.Call("texParameteri", ctx.Get("TEXTURE_2D"), ctx.Get("TEXTURE_WRAP_S"), ctx.Get("CLAMP_TO_EDGE"))
	ctx.Call("texParameteri", ctx.Get("TEXTURE_2D"), ctx.Get("TEXTURE_WRAP_T"), ctx.Get("CLAMP_TO_EDGE"))
	ctx.Call("pixelStorei", ctx.Get("UNPACK_FLIP_Y_WEBGL"), false)
	ctx.Call("texImage2D", ctx.Get("TEXTURE_2D"), 0, ctx.Get("RGBA"), width, height, 0, ctx.Get("RGBA"), ctx.Get("UNSIGNED_BYTE"), jsUint8(pixels))
	entry := &glTexture{texture: texture, width: width, height: height, bytes: int64(width) * int64(height) * 4, used: renderer.frame}
	renderer.textures[key] = entry
	renderer.bytes += entry.bytes
	if len(renderer.textures) > 512 || renderer.bytes > 64<<20 {
		for oldKey, old := range renderer.textures {
			if len(renderer.textures) <= 384 && renderer.bytes <= 48<<20 {
				break
			}
			if old.used+120 < renderer.frame || renderer.bytes > 64<<20 {
				ctx.Call("deleteTexture", old.texture)
				renderer.bytes -= old.bytes
				delete(renderer.textures, oldKey)
			}
		}
	}
	return texture, width, height
}

func (renderer *glRenderer) rasterizeTile(label platform.TextLabel, deviceWidth, deviceHeight, left, right int) (int, int, []byte, bool) {
	width, height := right-left, deviceHeight
	if width <= 0 || height <= 0 || width > renderer.maxTex || height > renderer.maxTex {
		return 0, 0, nil, false
	}
	if len(label.Bitmap) > 0 {
		source := renderer.runtime.bitmapCanvas(label)
		if source.IsUndefined() || source.IsNull() {
			return 0, 0, nil, false
		}
		canvas := renderer.runtime.document.Call("createElement", "canvas")
		canvas.Set("width", width)
		canvas.Set("height", height)
		context := canvas.Call("getContext", "2d")
		sx := float64(left) * float64(label.BitmapWidth) / float64(deviceWidth)
		sw := float64(width) * float64(label.BitmapWidth) / float64(deviceWidth)
		context.Call("drawImage", source, sx, 0, sw, float64(label.BitmapHeight), 0, 0, width, height)
		data := context.Call("getImageData", 0, 0, width, height).Get("data")
		rgba := make([]byte, width*height*4)
		if copied := js.CopyBytesToGo(rgba, data); copied != len(rgba) {
			return 0, 0, nil, false
		}
		return width, height, premultipliedBGRA(rgba), true
	}
	canvas := renderer.runtime.document.Call("createElement", "canvas")
	canvas.Set("width", width)
	canvas.Set("height", height)
	context := canvas.Call("getContext", "2d")
	context.Call("scale", renderer.scale, renderer.scale)
	// Move the tile's device-pixel left edge to the texture's left edge.
	context.Call("translate", -label.X-(label.OriginX+float32(left)/renderer.scale), -label.Y)
	local := label
	local.Clipped = false
	local.Clip = gfx.Rect{}
	if len(label.Runs) > 0 {
		renderer.runtime.drawLabelRunsContext(context, local)
	} else {
		renderer.runtime.drawTextContext(context, gfx.Text{
			Value:  label.Value,
			Bounds: gfx.Rect{X: label.X + label.OriginX, Y: label.Y + label.OriginY, Width: label.Width, Height: label.Height},
			Style: gfx.TextStyle{
				Size: label.FontSize, Weight: label.Weight, Italic: label.Italic,
				Color: platformColor(label.Color), Mono: label.Mono, MonoFace: label.MonoFace,
			},
			Baseline: label.Baseline,
		})
	}
	data := context.Call("getImageData", 0, 0, width, height).Get("data")
	rgba := make([]byte, width*height*4)
	if copied := js.CopyBytesToGo(rgba, data); copied != len(rgba) {
		return 0, 0, nil, false
	}
	return width, height, premultipliedBGRA(rgba), true
}

func premultipliedBGRA(rgba []byte) []byte {
	pixels := make([]byte, len(rgba))
	for index := 0; index < len(rgba); index += 4 {
		alpha := rgba[index+3]
		pixels[index] = byte(int(rgba[index+2]) * int(alpha) / 255)
		pixels[index+1] = byte(int(rgba[index+1]) * int(alpha) / 255)
		pixels[index+2] = byte(int(rgba[index]) * int(alpha) / 255)
		pixels[index+3] = alpha
	}
	return pixels
}

func (renderer *glRenderer) key(label platform.TextLabel) textKey {
	key := textKey{
		id: label.BitmapID, version: label.BitmapVersion,
		value: label.Value, size: label.FontSize, weight: label.Weight, italic: label.Italic,
		color: label.Color, fontGen: label.FontGeneration, mono: label.Mono, monoFace: label.MonoFace,
		cellWidth: label.CellWidth, baseline: label.Baseline, identity: label.Identity,
		width: label.BitmapWidth, height: label.BitmapHeight, stride: label.BitmapStride,
		scale: renderer.scale,
	}
	if len(label.Bitmap) > 0 {
		key.bitmap = true
		if key.id == 0 {
			key.hash = fnvBytes(label.Bitmap)
		}
		return key
	}
	key.deviceW = int(math.Ceil(float64(label.Width) * float64(renderer.scale)))
	key.deviceH = int(math.Ceil(float64(label.Height) * float64(renderer.scale)))
	key.runs = hashRuns(label.Runs)
	return key
}

func hashRuns(runs []gfx.TextRun) uint64 {
	hash := uint64(14695981039346656037)
	for _, run := range runs {
		hash ^= uint64(run.Column)
		hash *= 1099511628211
		hash ^= uint64(run.Cells)
		hash *= 1099511628211
		hash ^= uint64(run.Bytes)
		hash *= 1099511628211
		hash ^= uint64(run.Color.Red * 255)
		hash *= 1099511628211
		hash ^= uint64(run.Color.Green * 255)
		hash *= 1099511628211
		hash ^= uint64(run.Color.Blue * 255)
		hash *= 1099511628211
		hash ^= uint64(run.Color.Alpha * 255)
		hash *= 1099511628211
		hash ^= uint64(run.Weight * 255)
		hash *= 1099511628211
		if run.Italic {
			hash ^= 3
		}
		hash *= 1099511628211
		if run.Ligatures {
			hash ^= 5
		}
		hash *= 1099511628211
		if run.Underline {
			hash ^= 1
		}
		hash *= 1099511628211
		if run.Strike {
			hash ^= 2
		}
		hash *= 1099511628211
	}
	return hash
}

func (renderer *glRenderer) Destroy() {
	ctx := renderer.ctx
	for _, texture := range renderer.textures {
		ctx.Call("deleteTexture", texture.texture)
	}
	renderer.textures = nil
	ctx.Call("deleteBuffer", renderer.vbo)
	ctx.Call("deleteProgram", renderer.program)
	ctx.Call("deleteProgram", renderer.text)
}

func jsFloat32(values []float32) js.Value {
	array := js.Global().Get("Float32Array").New(len(values))
	if len(values) == 0 {
		return array
	}
	bytes := js.Global().Get("Uint8Array").New(array.Get("buffer"))
	js.CopyBytesToJS(bytes, unsafe.Slice((*byte)(unsafe.Pointer(&values[0])), len(values)*4))
	return array
}

func jsUint8(values []byte) js.Value {
	array := js.Global().Get("Uint8Array").New(len(values))
	if len(values) > 0 {
		js.CopyBytesToJS(array, values)
	}
	return array
}
