package platform

import "math"

// DevicePosition is where, in whole device pixels, a label whose layout position is
// logical (the painter's rounded X plus the OriginX it took off) starts at a display
// scale. A label texture holds one texel per device pixel, so drawing it anywhere but
// on a whole device pixel makes the sampler blend neighbouring texels: at 125% or
// 150% that is most labels, and it blurs the text. Rounding the layout position
// moves the text by at most half a device pixel.
func DevicePosition(logical, origin, scale float32) int {
	return int(math.Floor(float64(logical+origin)*float64(scale) + 0.5))
}

// DeviceExtent is the width in whole device pixels of the span from logical to
// logical+length: both edges are rounded as DevicePosition rounds them, so labels
// that meet at a logical edge meet at a device edge, with no gap and no overlap.
func DeviceExtent(logical, length, scale float32) int {
	return DevicePosition(logical+length, 0, scale) - DevicePosition(logical, 0, scale)
}

// LabelTextureLimit is the widest and tallest texture a label is rasterized into, in
// device pixels: the 2D texture limit of the Direct3D 11 and Vulkan devices Viva runs
// on, and the limit native.m keeps for Metal. LabelTexturePixels bounds the pixels of
// one such texture.
const (
	LabelTextureLimit  = 16384
	LabelTexturePixels = 16 * 1024 * 1024
)

// LabelTextureWidth is the widest texture a label height device pixels high is
// rasterized into: LabelTextureLimit, narrowed for a tall label so that its texture
// stays within LabelTexturePixels. It is 0 for a label too tall for any texture,
// which is not drawn.
func LabelTextureWidth(height int) int {
	if height <= 0 || height > LabelTextureLimit {
		return 0
	}
	return min(LabelTextureLimit, LabelTexturePixels/height)
}

// LabelTextureFits reports whether a label texture width by height device pixels is
// one a backend allocates: width is at most LabelTextureWidth(height). It is tested
// without a division, as renderers ask it of every label every frame.
func LabelTextureFits(width, height int) bool {
	return width > 0 && height > 0 && width <= LabelTextureLimit && height <= LabelTextureLimit && width*height <= LabelTexturePixels
}

// LabelVisibleColumns is the part of a label full device pixels wide, drawn at device
// column origin, that the device columns from left to right show: the columns from
// first to last, counted from the label's left edge. ok is false when they show none.
func LabelVisibleColumns(origin, full, left, right int) (first, last int, ok bool) {
	first = min(max(left-origin, 0), full)
	last = min(max(right-origin, 0), full)
	return first, last, last > first
}

// WideLabelWindow chooses the device columns that the texture of a label full columns
// wide, more than one texture of limit columns holds, is rasterized from: the columns
// from skip to skip+width of the label. The columns from left to right, with left
// below right, are the ones that can be seen, and haveSkip and haveWidth the window of
// the texture the label has, or a width of 0 when it has none.
//
// A texture that still holds every column that can be seen is kept, so a scroll inside
// it does not rasterize the line again. Otherwise the visible columns are kept with as
// much margin on both sides as they are wide, so a scroll of up to their width keeps
// the new texture; at an end of the label the margin goes to the other side. This is
// the window viva_label_window in native.m chooses. Visible columns wider than limit
// keep their first limit columns: these backends draw one texture per label, and a
// viewport is wider than limit only for a label so tall that its limit is narrowed.
func WideLabelWindow(full, limit, left, right, haveSkip, haveWidth int) (skip, width int) {
	if haveWidth > 0 && haveWidth <= limit && haveSkip <= left && right <= haveSkip+haveWidth && haveSkip+haveWidth <= full {
		return haveSkip, haveWidth
	}
	span := right - left
	if span >= limit {
		return left, limit
	}
	margin := min(span, (limit-span)/2)
	width = span + 2*margin
	skip = max(left-margin, 0)
	if skip+width > full {
		skip = full - width
	}
	return skip, width
}

// WideTextLabelWindow is the window a text label's texture is rasterized from when one
// texture cannot hold the whole label. The label is full device columns wide and height
// high, drawn at the device column DevicePosition puts it at, on a target viewport device
// columns wide, or 0 when the caller has none. The columns that can be seen are the
// viewport's, cut to the label's clip; haveSkip and haveWidth are the window of the
// texture the label has, a width of 0 when it has none (see WideLabelWindow). wide is
// false for a label one texture holds whole, which is left to the caller. A wide label
// is not drawn, ok false, when none of it can be seen or it is too tall for any texture:
// an error for it would stop the window drawing anything.
func WideTextLabelWindow(label TextLabel, scale float32, full, height, viewport, haveSkip, haveWidth int) (skip, width int, wide, ok bool) {
	limit := LabelTextureWidth(height)
	if full <= limit {
		return 0, full, false, full > 0
	}
	if limit <= 0 {
		return 0, 0, true, false
	}
	origin := DevicePosition(label.X, label.OriginX, scale)
	left, right := 0, origin+full
	if viewport > 0 {
		right = min(right, viewport)
	}
	if label.Clipped {
		left = max(left, int(math.Floor(float64(label.Clip.X*scale))))
		right = min(right, int(math.Ceil(float64((label.Clip.X+label.Clip.Width)*scale))))
	}
	first, last, visible := LabelVisibleColumns(origin, full, left, right)
	if !visible {
		return 0, 0, true, false
	}
	skip, width = WideLabelWindow(full, limit, first, last, haveSkip, haveWidth)
	return skip, width, true, true
}

// LabelWindow is the part of a label's texture that a clip can show, in device
// pixels: the columns from skip to skip+width of a texture drawn at origin, whose
// whole width is full, when the clip covers left to right. A clip that starts left
// of the label keeps the whole texture. One that starts inside it skips the columns
// it cuts, so a long scrolled line costs its visible width, not its whole width,
// without moving the text by a fraction of a device pixel. ok is false when the
// clip shows none of those columns. It is for a label no wider than its texture
// limit; a wider one takes the window WideLabelWindow chooses.
func LabelWindow(origin, full, left, right int) (skip, width int, ok bool) {
	if full <= 0 || right <= left {
		return 0, 0, false
	}
	if left <= origin {
		return 0, full, true
	}
	right = min(right, origin+full)
	if right <= left {
		return 0, 0, false
	}
	return left - origin, right - left, true
}

// ResampleBGRA shrinks premultiplied 8-bit BGRA pixels from srcWidth by srcHeight to
// width by height by averaging the source area under each destination pixel, with
// fractional weights at the edges, so a bitmap drawn smaller than it is stored keeps
// its detail without the aliasing a bilinear sample of a few texels gives. A
// destination larger than the source in either direction is not supported: the
// result is nil, and the sampler enlarges such a bitmap.
func ResampleBGRA(pixels []byte, stride, srcWidth, srcHeight, width, height int) []byte {
	if width <= 0 || height <= 0 || width > srcWidth || height > srcHeight || stride < srcWidth*4 ||
		len(pixels) < (srcHeight-1)*stride+srcWidth*4 {
		return nil
	}
	// Horizontal pass into float rows, then vertical: the filter is separable.
	horizontal := make([]float32, width*4*srcHeight)
	weights := areaWeights(srcWidth, width)
	for row := range srcHeight {
		line := pixels[row*stride : row*stride+srcWidth*4]
		out := horizontal[row*width*4 : (row+1)*width*4]
		for column := range width {
			span := weights[column]
			var sum [4]float32
			for source := span.first; source <= span.last; source++ {
				weight := span.weight(source)
				base := source * 4
				sum[0] += float32(line[base]) * weight
				sum[1] += float32(line[base+1]) * weight
				sum[2] += float32(line[base+2]) * weight
				sum[3] += float32(line[base+3]) * weight
			}
			copy(out[column*4:], sum[:])
		}
	}
	result := make([]byte, width*height*4)
	rows := areaWeights(srcHeight, height)
	for row := range height {
		span := rows[row]
		out := result[row*width*4 : (row+1)*width*4]
		for index := range width * 4 {
			var sum float32
			for source := span.first; source <= span.last; source++ {
				sum += horizontal[source*width*4+index] * span.weight(source)
			}
			out[index] = byte(min(max(sum+0.5, 0), 255))
		}
	}
	return result
}

// areaSpan is the run of source pixels a destination pixel covers: whole pixels in
// between and a fraction of the first and last. Weights sum to one.
type areaSpan struct {
	first, last int
	head, tail  float32 // the share of the first and of the last source pixel
	scale       float32 // 1 over the number of source pixels one destination pixel covers
}

func (span areaSpan) weight(source int) float32 {
	switch source {
	case span.first:
		return span.head * span.scale
	case span.last:
		return span.tail * span.scale
	}
	return span.scale
}

func areaWeights(source, destination int) []areaSpan {
	ratio := float64(source) / float64(destination)
	spans := make([]areaSpan, destination)
	for index := range spans {
		start, end := float64(index)*ratio, float64(index+1)*ratio
		first, last := int(math.Floor(start)), int(math.Ceil(end))-1
		last = min(max(last, first), source-1)
		span := areaSpan{first: first, last: last, scale: float32(1 / ratio)}
		if first == last {
			span.head = float32(end - start)
		} else {
			span.head = float32(float64(first+1) - start)
			span.tail = float32(end - float64(last))
		}
		spans[index] = span
	}
	return spans
}

// BitmapTextureExtent is the size, in device pixels, of the texture an image label is drawn
// from: the stored image, averaged down to the device pixels the label covers when it is
// drawn smaller than it is stored, because sampling a few texels of a large image would
// alias. The darwin backend computes the same size in native.m.
func BitmapTextureExtent(label TextLabel, scale float32) (width, height int) {
	width, height = label.BitmapWidth, label.BitmapHeight
	shown, high := DeviceExtent(label.X, label.Width, scale), DeviceExtent(label.Y, label.Height, scale)
	if shown > 0 && high > 0 && shown <= width && high <= height && (shown < width || high < height) {
		return shown, high
	}
	return width, height
}

// BitmapTextureStale reports whether an image label's texture, haveWidth by haveHeight, no
// longer suits the size the label is drawn at: a texture smaller than needed would blur the
// image and one more than twice as large would alias it. Between those it stays, so a size
// that animates does not rebuild the texture every frame.
func BitmapTextureStale(label TextLabel, scale float32, haveWidth, haveHeight int) bool {
	width, height := BitmapTextureExtent(label, scale)
	return extentStale(haveWidth, width) || extentStale(haveHeight, height)
}

func extentStale(have, need int) bool { return need > have || have >= 2*need }
