//go:build js && wasm

package web

import (
	"syscall/js"
	"unicode/utf8"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/bidi"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
)

// bitmapKey identifies a retained bitmap picture.
type bitmapKey struct {
	id            uint64
	version       uint32
	width, height int
	stride        int
	hash          uint64
}

// render repaints the frame the painter built into the 2D context.
func (runtime *runtime) render() {
	clock := &runtime.clock
	clock.begin()
	runtime.paths.beginFrame()
	if runtime.painter.dirty {
		runtime.painter.dirty = false
		runtime.a11y.sync(runtime.painter.accessibility, runtime.painter.focus.Focused())
		id, input := runtime.painter.activeTextInput()
		runtime.a11y.text.sync(id, input, runtime.textInputChanged)
	}
	clock.lap(&clock.a11y)
	if err := runtime.painter.frameErr; err != nil {
		js.Global().Get("console").Call("error", "viva: frame: "+err.Error())
		runtime.painter.frameErr = nil
	}
	if runtime.gl != nil {
		if err := runtime.gl.Draw(runtime.painter.frame, runtime.painter.labels, runtime.painter.clear); err != nil {
			if err == errContextLost {
				runtime.fallbackToCanvas2D(err.Error())
			} else {
				js.Global().Get("console").Call("error", "viva: webgl2: "+err.Error())
			}
		}
	} else {
		context := runtime.context
		context.Call("save")
		context.Call("scale", runtime.dpr, runtime.dpr)
		context.Set("fillStyle", cssColor(runtime.painter.clear))
		context.Call("fillRect", 0, 0, runtime.painter.width, runtime.painter.height)
		runtime.drawFrame()
		context.Call("restore")
	}
	runtime.paths.endFrame()
	rectangles, triangles, texts := frameCounts(runtime.painter.frame, runtime.painter.labels)
	clock.end(rectangles, triangles, texts, len(runtime.painter.accessibility), runtime.paths.built, runtime.paths.reused)
	if delay, ok := runtime.canvas.NextFrame(); ok {
		runtime.scheduleFrame(delay)
	}
}

// frameCounts reports how many primitives the last frame carried, for the
// page's statistics.
func frameCounts(frame platform.Frame, labels []platform.TextLabel) (rectangles, triangles, texts int) {
	for _, instance := range frame.Rectangles {
		switch instance.Kind {
		case scene.KindTriangle:
			triangles++
		default:
			rectangles++
		}
	}
	for _, label := range labels {
		if label.BitmapWidth <= 0 || label.BitmapHeight <= 0 || len(label.Bitmap) == 0 && label.BitmapID == 0 {
			texts++
		}
	}
	return rectangles, triangles, texts
}

// drawFrame interleaves the shape batches and labels in the order the
// application painted them.
func (runtime *runtime) drawFrame() {
	frame := runtime.painter.frame
	labels := runtime.painter.labels
	batches := render.GroupBatches(runtime.batches[:0], frame.Rectangles)
	runtime.batches = batches
	order := runtime.labelOrder[:0]
	for _, label := range labels {
		order = append(order, render.LabelOrder{Layer: label.Layer, ShapesBefore: label.ShapesBefore})
	}
	runtime.labelOrder = order
	plan, scratch := render.PlanWithOrder(runtime.plan[:0], runtime.planOrder, batches, order)
	runtime.plan, runtime.planOrder = plan, scratch
	for _, op := range plan {
		switch op.Kind {
		case render.PlanShapes:
			runtime.drawShapes(frame.Rectangles[op.First:op.First+op.Count], op.Clipped, op.Clip)
		case render.PlanLabels:
			if op.Label >= 0 && op.Label < len(labels) {
				runtime.drawLabel(labels[op.Label])
			}
		}
	}
}

// drawShapes paints one batch of instances, clipped to the batch's clip.
func (runtime *runtime) drawShapes(instances []render.Rectangle, clipped bool, clip scene.Rect) {
	context := runtime.context
	if clipped {
		context.Call("save")
		clipRect(context, gfx.Rect{X: clip.X, Y: clip.Y, Width: clip.Width, Height: clip.Height})
	}
	for index := 0; index < len(instances); {
		instance := instances[index]
		switch instance.Kind {
		case scene.KindShadow:
			drawShadow(context, gfx.Shadow{
				Bounds: gfx.Rect{X: instance.Bounds.X, Y: instance.Bounds.Y, Width: instance.Bounds.Width, Height: instance.Bounds.Height},
				Radius: instance.Radius,
				Color:  gfxColor(instance.Color),
				Style: gfx.ShadowStyle{
					OffsetX: instance.Shadow.OffsetX, OffsetY: instance.Shadow.OffsetY,
					Blur: instance.Shadow.Blur, Spread: instance.Shadow.Spread,
				},
			}, runtime.dpr)
			index++
		case scene.KindTriangle:
			if instance.Shade.On {
				runtime.drawShadedTriangle(gfxTriangle(instance))
				index++
				continue
			}
			first := index
			run := runtime.triangles[:0]
			for index < len(instances) && instances[index].Kind == scene.KindTriangle &&
				!instances[index].Shade.On && instances[index].Color == instance.Color {
				run = append(run, gfxTriangle(instances[index]))
				index++
			}
			runtime.triangles = run
			if index > first {
				runtime.drawSolidRun(run)
			}
		default:
			drawRectangle(context, gfx.Rectangle{
				Bounds: gfx.Rect{X: instance.Bounds.X, Y: instance.Bounds.Y, Width: instance.Bounds.Width, Height: instance.Bounds.Height},
				Color:  gfxColor(instance.Color),
				Radius: instance.Radius,
			})
			index++
		}
	}
	if clipped {
		context.Call("restore")
	}
}

func gfxColor(color scene.Color) gfx.Color {
	return gfx.Color{Red: color.Red, Green: color.Green, Blue: color.Blue, Alpha: color.Alpha}
}

func platformColor(color platform.Color) gfx.Color {
	return gfx.Color{Red: float32(color.Red), Green: float32(color.Green), Blue: float32(color.Blue), Alpha: float32(color.Alpha)}
}

func gfxTriangle(instance render.Rectangle) gfx.Triangle {
	return gfx.Triangle{
		Points:   [3]gfx.Point{{X: instance.Points[0].X, Y: instance.Points[0].Y}, {X: instance.Points[1].X, Y: instance.Points[1].Y}, {X: instance.Points[2].X, Y: instance.Points[2].Y}},
		Color:    gfxColor(instance.Color),
		Shade:    [2]gfx.Color{gfxColor(instance.Shade.Colors[0]), gfxColor(instance.Shade.Colors[1])},
		Shaded:   instance.Shade.On,
		EdgeMask: instance.EdgeMask,
	}
}

// sameRun reports whether next continues the run that first began.
func sameRun(first, next gfx.Triangle) bool {
	return next.Layer == first.Layer && !next.Shaded && next.Color == first.Color &&
		next.HasClip == first.HasClip && next.ClipOf == first.ClipOf
}

// drawRectangle paints one rectangle.
func drawRectangle(context js.Value, rectangle gfx.Rectangle) {
	context.Set("fillStyle", cssColor(rectangle.Color))
	if rectangle.Radius > 0 {
		traceRoundedRect(context, rectangle.Bounds, rectangle.Radius)
		context.Call("fill")
	} else {
		context.Call("fillRect", rectangle.Bounds.X, rectangle.Bounds.Y, rectangle.Bounds.Width, rectangle.Bounds.Height)
	}
}

func (runtime *runtime) drawSolidRun(run []gfx.Triangle) {
	context := runtime.context
	first := run[0]
	context.Call("save")
	context.Call("translate", first.Points[0].X, first.Points[0].Y)
	context.Set("fillStyle", cssColor(first.Color))
	context.Call("fill", runtime.paths.path(run))
	context.Call("restore")
}

func (runtime *runtime) drawShadedTriangle(triangle gfx.Triangle) {
	context := runtime.context
	context.Set("fillStyle", triangleFill(context, triangle))
	context.Call("beginPath")
	context.Call("moveTo", triangle.Points[0].X, triangle.Points[0].Y)
	context.Call("lineTo", triangle.Points[1].X, triangle.Points[1].Y)
	context.Call("lineTo", triangle.Points[2].X, triangle.Points[2].Y)
	context.Call("closePath")
	context.Call("fill")
}

// drawLabel paints one platform text label: a bitmap, a fixed-pitch run line,
// or proportional text.
func (runtime *runtime) drawLabel(label platform.TextLabel) {
	if label.HasQuad {
		runtime.drawWarpedLabel(label)
		return
	}
	if label.BitmapWidth > 0 && label.BitmapHeight > 0 && (len(label.Bitmap) > 0 || label.BitmapID != 0) {
		runtime.drawBitmapLabel(label)
		return
	}
	if len(label.Runs) > 0 {
		runtime.drawLabelRuns(label)
		return
	}
	runtime.drawText(gfx.Text{
		Value: label.Value,
		Bounds: gfx.Rect{
			X: label.X + label.OriginX, Y: label.Y + label.OriginY,
			Width: label.Width, Height: label.Height,
		},
		Style: gfx.TextStyle{
			Size: label.FontSize, Weight: label.Weight, Italic: label.Italic,
			Color: platformColor(label.Color), Mono: label.Mono, MonoFace: label.MonoFace,
		},
		Baseline: label.Baseline,
		HasClip:  label.Clipped, ClipOf: gfx.Rect(label.Clip),
	})
}

// drawLabelRuns paints a fixed-pitch grid line from the labels a frame carries.
func (runtime *runtime) drawLabelRuns(label platform.TextLabel) {
	runtime.drawLabelRunsContext(runtime.context, label)
}

// drawLabelRunsContext is drawLabelRuns into an explicit 2D context, which the
// WebGL2 renderer uses to rasterize a grid label texture.
func (runtime *runtime) drawLabelRunsContext(context js.Value, label platform.TextLabel) {
	context.Call("save")
	defer context.Call("restore")
	if label.Clipped {
		clipRect(context, gfx.Rect(label.Clip))
	}
	clipRect(context, gfx.Rect{X: label.X + label.OriginX, Y: label.Y + label.OriginY, Width: label.Width, Height: label.Height})
	metrics := monoBaseline(context, label.FontSize, label.MonoFace)
	baseline := label.Y + label.OriginY + label.Baseline
	if label.Baseline <= 0 {
		baseline = label.Y + label.OriginY + (label.Height-metrics.Ascent-metrics.Descent)*0.5 + metrics.Ascent
	}
	context.Set("textBaseline", "alphabetic")
	offset := 0
	for _, run := range label.Runs {
		end := offset + int(run.Bytes)
		if run.Bytes < 0 || end > len(label.Value) {
			break
		}
		piece := label.Value[offset:end]
		offset = end
		left := label.X + label.OriginX + float32(run.Column)*label.CellWidth
		span := float32(run.Cells) * label.CellWidth
		context.Set("fillStyle", cssColor(run.Color))
		context.Set("font", cssMonoFont(label.FontSize, run.Weight, run.Italic, label.MonoFace))
		x := left
		if run.Cells > 0 && utf8.RuneCountInString(piece) == 1 {
			x = left + (span-float32(context.Call("measureText", piece).Get("width").Float()))/2
		}
		if run.Ligatures && printableASCII(piece) && len(piece) > 1 {
			if width := float32(context.Call("measureText", piece).Get("width").Float()); width > 0 && span > 0 {
				context.Call("save")
				context.Call("translate", left, 0)
				context.Call("scale", span/width, 1)
				context.Call("fillText", piece, 0, baseline)
				context.Call("restore")
			}
		} else if printableASCII(piece) && len(piece) > 1 {
			for index := range len(piece) {
				context.Call("fillText", piece[index:index+1], left+float32(index)*label.CellWidth, baseline)
			}
		} else {
			context.Call("fillText", piece, x, baseline)
		}
		if span > 0 {
			thickness := max(1, label.FontSize/14)
			if run.Underline {
				context.Call("fillRect", left, baseline+thickness, span, thickness)
			}
			if run.Strike {
				context.Call("fillRect", left, baseline-metrics.Ascent*0.35, span, thickness)
			}
		}
	}
}

// drawBitmapLabel paints an image label from its retained premultiplied BGRA
// pixels, converted once into an offscreen canvas.
func (runtime *runtime) drawBitmapLabel(label platform.TextLabel) {
	key := bitmapKey{id: label.BitmapID, version: label.BitmapVersion, width: label.BitmapWidth, height: label.BitmapHeight, stride: label.BitmapStride}
	if key.id == 0 {
		key.hash = fnvBytes(label.Bitmap)
	}
	canvas, ok := runtime.bitmaps[key]
	if !ok {
		if len(label.Bitmap) == 0 {
			return
		}
		canvas = runtime.bitmapCanvas(label)
		if canvas.IsUndefined() || canvas.IsNull() {
			return
		}
		if runtime.bitmaps == nil {
			runtime.bitmaps = make(map[bitmapKey]js.Value)
		}
		if len(runtime.bitmaps) > 64 {
			runtime.bitmaps = make(map[bitmapKey]js.Value)
		}
		runtime.bitmaps[key] = canvas
	}
	context := runtime.context
	context.Call("drawImage", canvas, label.X+label.OriginX, label.Y+label.OriginY, label.Width, label.Height)
}

// bitmapCanvas converts a label's premultiplied BGRA pixels into an offscreen
// canvas holding straight RGBA, which Canvas2D composites as it drew the source.
func (runtime *runtime) bitmapCanvas(label platform.TextLabel) js.Value {
	width, height := label.BitmapWidth, label.BitmapHeight
	if width <= 0 || height <= 0 || label.BitmapStride < width*4 {
		return js.Undefined()
	}
	rgba := make([]byte, width*height*4)
	for y := 0; y < height; y++ {
		source := label.Bitmap[y*label.BitmapStride:]
		target := rgba[y*width*4:]
		for x := 0; x < width; x++ {
			b, g, r, a := source[x*4], source[x*4+1], source[x*4+2], source[x*4+3]
			if a == 255 {
				target[x*4], target[x*4+1], target[x*4+2], target[x*4+3] = r, g, b, 255
				continue
			}
			if a == 0 {
				continue
			}
			target[x*4] = byte(int(r) * 255 / int(a))
			target[x*4+1] = byte(int(g) * 255 / int(a))
			target[x*4+2] = byte(int(b) * 255 / int(a))
			target[x*4+3] = a
		}
	}
	canvas := runtime.document.Call("createElement", "canvas")
	canvas.Set("width", width)
	canvas.Set("height", height)
	context := canvas.Call("getContext", "2d")
	bytes := js.Global().Get("Uint8ClampedArray").New(len(rgba))
	js.CopyBytesToJS(bytes, rgba)
	image := js.Global().Get("ImageData").New(bytes, width, height)
	context.Call("putImageData", image, 0, 0)
	return canvas
}

func fnvBytes(data []byte) uint64 {
	const (
		offset = 14695981039346656037
		prime  = 1099511628211
	)
	hash := uint64(offset)
	for _, b := range data {
		hash ^= uint64(b)
		hash *= prime
	}
	return hash
}

// drawText paints one text line. A line that cannot overflow its box (no outer
// clip, left to right, narrower and no taller than the box) is drawn directly;
// any other line is clipped to its box inside its own saved state.
func (runtime *runtime) drawText(text gfx.Text) {
	runtime.drawTextContext(runtime.context, text)
}

// drawTextContext is drawText into an explicit 2D context, which the WebGL2
// renderer uses to rasterize a label texture.
func (runtime *runtime) drawTextContext(context js.Value, text gfx.Text) {
	context.Call("save")
	defer context.Call("restore")
	context.Set("textBaseline", "alphabetic")
	context.Set("textAlign", "left")
	context.Set("direction", "ltr")
	if text.RunCount > 0 {
		runtime.drawRuns(text)
		return
	}
	var style textStyleState
	metrics := runtime.metrics.baseline(text.Style.Size)
	baseline := runtime.canvas.LabelBaseline(gfx.Rect{Height: text.Bounds.Height}, gfx.TextStyle{Size: text.Style.Size})
	if text.Style.Mono {
		baseline = text.Baseline
		if baseline <= 0 {
			mono := monoBaseline(context, text.Style.Size, text.Style.MonoFace)
			baseline = (text.Bounds.Height-mono.Ascent-mono.Descent)*0.5 + mono.Ascent
		}
	}
	y := text.Bounds.Y + baseline
	// The line is drawn whole, in the font and direction the text engine measured it
	// with. The base direction of a right-to-left paragraph puts the
	// punctuation around it on the side the paragraph puts it.
	rtl := bidi.BaseDirection(text.Value) == bidi.RTL
	// Italic text takes the clipped path below: its width is not the regular face's that
	// the fit test here measures, and the path draws it in the font the engine measured.
	if !text.HasClip && !rtl && !text.Style.Italic && metrics.Ascent+metrics.Descent <= text.Bounds.Height &&
		runtime.metrics.width(text.Value, text.Style.Size, text.Style.Weight)+fitMargin <= text.Bounds.Width {
		style.apply(context, text.Style)
		context.Call("fillText", text.Value, text.Bounds.X, y)
		return
	}
	context.Call("save")
	if text.HasClip {
		clipRect(context, text.ClipOf)
	}
	clipRect(context, text.Bounds)
	context.Set("fillStyle", cssColor(text.Style.Color))
	if text.Style.Mono {
		context.Set("font", cssMonoFont(text.Style.Size, text.Style.Weight, text.Style.Italic, text.Style.MonoFace))
	} else {
		context.Set("font", cssFontStyle(text.Style.Size, text.Style.Weight, text.Style.Italic))
	}
	if rtl {
		context.Set("direction", "rtl")
	}
	context.Call("fillText", text.Value, text.Bounds.X, y)
	context.Call("restore")
}

// fitMargin is the room, in pixels, a line must leave in its box to skip the clip:
// glyphs overhang their advance a little, and a clip would cut that.
const fitMargin = 2

// textStyleState is the fill and font a context currently holds, so a run of lines
// in one style sets them once. The zero value matches nothing.
type textStyleState struct {
	set    bool
	color  gfx.Color
	size   float32
	weight float32
	italic bool
}

func (state *textStyleState) apply(context js.Value, style gfx.TextStyle) {
	if state.set && state.color == style.Color && state.size == style.Size && state.weight == style.Weight && state.italic == style.Italic {
		return
	}
	if !state.set || state.color != style.Color {
		context.Set("fillStyle", cssColor(style.Color))
	}
	if !state.set || state.size != style.Size || state.weight != style.Weight || state.italic != style.Italic {
		if style.Mono {
			context.Set("font", cssMonoFont(style.Size, style.Weight, style.Italic, style.MonoFace))
		} else {
			context.Set("font", cssFontStyle(style.Size, style.Weight, style.Italic))
		}
	}
	*state = textStyleState{set: true, color: style.Color, size: style.Size, weight: style.Weight, italic: style.Italic}
}
