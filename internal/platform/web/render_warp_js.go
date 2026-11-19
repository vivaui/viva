//go:build js && wasm

package web

import (
	"math"
	"syscall/js"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

// warpKey identifies a rasterized label reused for a warped draw.
type warpKey struct {
	value          string
	size, weight   float32
	italic, mono   bool
	face           gfx.MonoFace
	cell, baseline float32
	color          platform.Color
	fontGen        uint32
	runs           uint64
	width, height  int
	scale          float32
	bitmapID       uint64
	bitmapVersion  uint32
	bitmapW        int
	bitmapH        int
	bitmapStride   int
	bitmapHash     uint64
}

// drawWarpedLabel paints a label whose picture lands on four arbitrary corners.
// Canvas2D has no projective transform, so the picture is divided into
// horizontal strips and each strip is drawn with the affine transform of its
// top and left edges. The result is an approximation of the quad, with the
// label's per-corner tint applied as a gradient.
func (runtime *runtime) drawWarpedLabel(label platform.TextLabel) {
	source, sourceWidth, sourceHeight := runtime.warpLabelCanvas(label)
	if source.IsUndefined() || source.IsNull() {
		return
	}
	image := runtime.tintedWarp(label, source, sourceWidth, sourceHeight)
	corners := label.Quad
	context := runtime.context
	context.Call("save")
	defer context.Call("restore")
	if label.Clipped {
		clipRect(context, gfx.Rect(label.Clip))
	}
	strips := int(math.Ceil(float64(float32(sourceHeight) / max(4, 2*runtime.dpr))))
	strips = min(max(strips, 1), 32)
	for index := 0; index < strips; index++ {
		v0 := float32(index) / float32(strips)
		v1 := float32(index+1) / float32(strips)
		topLeft := bilinear(corners[0], corners[1], corners[2], corners[3], 0, v0)
		topRight := bilinear(corners[0], corners[1], corners[2], corners[3], 1, v0)
		bottomLeft := bilinear(corners[0], corners[1], corners[2], corners[3], 0, v1)
		sourceTop := v0 * float32(sourceHeight)
		sourceBottom := v1 * float32(sourceHeight)
		sourceStrip := sourceBottom - sourceTop
		if sourceStrip <= 0 {
			continue
		}
		a := (topRight.X - topLeft.X) / float32(sourceWidth)
		b := (topRight.Y - topLeft.Y) / float32(sourceWidth)
		c := (bottomLeft.X - topLeft.X) / sourceStrip
		d := (bottomLeft.Y - topLeft.Y) / sourceStrip
		context.Call("save")
		context.Call("transform", a, b, c, d, topLeft.X, topLeft.Y)
		context.Call("drawImage", image, 0, sourceTop, float32(sourceWidth), sourceStrip, 0, 0, float32(sourceWidth), sourceStrip)
		context.Call("restore")
	}
}

func bilinear(topLeft, topRight, bottomLeft, bottomRight gfx.Point, u, v float32) gfx.Point {
	top := gfx.Point{X: topLeft.X + (topRight.X-topLeft.X)*u, Y: topLeft.Y + (topRight.Y-topLeft.Y)*u}
	bottom := gfx.Point{X: bottomLeft.X + (bottomRight.X-bottomLeft.X)*u, Y: bottomLeft.Y + (bottomRight.Y-bottomLeft.Y)*u}
	return gfx.Point{X: top.X + (bottom.X-top.X)*v, Y: top.Y + (bottom.Y-top.Y)*v}
}

// warpLabelCanvas rasterizes the label once, in its own device-pixel canvas,
// and retains it for later warped frames.
func (runtime *runtime) warpLabelCanvas(label platform.TextLabel) (js.Value, int, int) {
	width := int(math.Ceil(float64(label.Width) * float64(runtime.dpr)))
	height := int(math.Ceil(float64(label.Height) * float64(runtime.dpr)))
	if width <= 0 || height <= 0 || width > 16384 || height > 16384 || width*height > 16*1024*1024 {
		return js.Undefined(), 0, 0
	}
	key := warpKey{
		value: label.Value, size: label.FontSize, weight: label.Weight, italic: label.Italic,
		mono: label.Mono, face: label.MonoFace, cell: label.CellWidth, baseline: label.Baseline,
		color: label.Color, fontGen: label.FontGeneration, runs: hashRuns(label.Runs),
		width: width, height: height, scale: runtime.dpr,
		bitmapID: label.BitmapID, bitmapVersion: label.BitmapVersion,
		bitmapW: label.BitmapWidth, bitmapH: label.BitmapHeight, bitmapStride: label.BitmapStride,
	}
	if len(label.Bitmap) > 0 {
		key.bitmapHash = fnvBytes(label.Bitmap)
	}
	if canvas, ok := runtime.warps[key]; ok {
		return canvas, width, height
	}
	var canvas js.Value
	if len(label.Bitmap) > 0 {
		canvas = runtime.bitmapCanvas(label)
		if canvas.IsUndefined() || canvas.IsNull() {
			return js.Undefined(), 0, 0
		}
		width, height = label.BitmapWidth, label.BitmapHeight
	} else {
		canvas = runtime.document.Call("createElement", "canvas")
		canvas.Set("width", width)
		canvas.Set("height", height)
		context := canvas.Call("getContext", "2d")
		context.Call("scale", runtime.dpr, runtime.dpr)
		context.Call("translate", -label.X, -label.Y)
		local := label
		local.Clipped = false
		local.Clip = gfx.Rect{}
		if len(label.Runs) > 0 {
			runtime.drawLabelRunsContext(context, local)
		} else {
			runtime.drawTextContext(context, gfx.Text{
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
			})
		}
	}
	if runtime.warps == nil {
		runtime.warps = make(map[warpKey]js.Value)
	}
	if len(runtime.warps) > 64 {
		runtime.warps = make(map[warpKey]js.Value)
	}
	runtime.warps[key] = canvas
	return canvas, width, height
}

// tintedWarp applies a label's per-corner tint to its picture. Canvas2D has no
// per-vertex tint, so a gradient from the top-left to the bottom-right corner
// approximates the four tints.
func (runtime *runtime) tintedWarp(label platform.TextLabel, source js.Value, width, height int) js.Value {
	white := true
	for _, tint := range label.Tint {
		if tint.Red < 1 || tint.Green < 1 || tint.Blue < 1 || tint.Alpha < 1 {
			white = false
			break
		}
	}
	if white {
		return source
	}
	if runtime.warpTint.IsUndefined() || runtime.warpTint.IsNull() {
		runtime.warpTint = runtime.document.Call("createElement", "canvas")
	}
	runtime.warpTint.Set("width", width)
	runtime.warpTint.Set("height", height)
	context := runtime.warpCtx
	if !context.Truthy() {
		context = runtime.warpTint.Call("getContext", "2d")
		runtime.warpCtx = context
	}
	context.Set("globalCompositeOperation", "source-over")
	context.Call("clearRect", 0, 0, width, height)
	context.Call("drawImage", source, 0, 0)
	gradient := context.Call("createLinearGradient", 0, 0, width, height)
	topLeft := platformColor(label.Tint[0])
	bottomRight := platformColor(label.Tint[3])
	gradient.Call("addColorStop", 0, cssColor(topLeft))
	gradient.Call("addColorStop", 1, cssColor(bottomRight))
	context.Set("globalCompositeOperation", "multiply")
	context.Set("fillStyle", gradient)
	context.Call("fillRect", 0, 0, width, height)
	context.Set("globalCompositeOperation", "destination-in")
	context.Call("drawImage", source, 0, 0)
	context.Set("globalCompositeOperation", "source-over")
	return runtime.warpTint
}
