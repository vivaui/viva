package ui

import "math"

// GaussianBlurBitmap returns a new tightly packed bitmap whose premultiplied
// BGRA pixels are blurred with a separable Gaussian kernel of standard deviation
// sigma, measured in bitmap pixels.
//
// The kernel reaches three sigma on each side, so the work is proportional to
// width*height*sigma: at sigma 200 a 1920x1080 bitmap is about five billion
// multiply-adds and will stall whatever thread paints it. Sigma is meant for a
// few pixels of softness; a caller that wants a much wider blur should scale the
// bitmap down first.
//
// The source bitmap is left untouched. A non-positive or non-finite sigma
// returns source unchanged. Invalid bitmap storage returns the zero Bitmap. The blurred result
// has no bitmap identity: callers that retain and redraw it should assign a
// stable ID once, as they would for any other generated Bitmap.
func GaussianBlurBitmap(source Bitmap, sigma float32) Bitmap {
	maxInt := int(^uint(0) >> 1)
	if source.Width <= 0 || source.Height <= 0 || source.Width > maxInt/4 {
		return Bitmap{}
	}
	rowBytes := source.Width * 4
	if source.Stride < rowBytes || source.Height > maxInt/source.Width/4 {
		return Bitmap{}
	}
	if source.Height > 1 && source.Stride > (maxInt-rowBytes)/(source.Height-1) {
		return Bitmap{}
	}
	required := (source.Height-1)*source.Stride + rowBytes
	if len(source.Pix) < required {
		return Bitmap{}
	}
	if sigma <= 0 || math.IsNaN(float64(sigma)) || math.IsInf(float64(sigma), 0) {
		return source
	}

	radius := max(int(math.Ceil(float64(sigma)*3)), 1)
	kernel := gaussianKernel(radius, float64(sigma))
	width, height := source.Width, source.Height
	scratch := make([]float32, width*height*4)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			dst := (y*width + x) * 4
			for tap := -radius; tap <= radius; tap++ {
				sx := min(max(x+tap, 0), width-1)
				src := y*source.Stride + sx*4
				weight := kernel[tap+radius]
				scratch[dst+0] += float32(source.Pix[src+0]) * weight
				scratch[dst+1] += float32(source.Pix[src+1]) * weight
				scratch[dst+2] += float32(source.Pix[src+2]) * weight
				scratch[dst+3] += float32(source.Pix[src+3]) * weight
			}
		}
	}

	stride := width * 4
	pixels := make([]byte, stride*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			dst := y*stride + x*4
			var sum [4]float32
			for tap := -radius; tap <= radius; tap++ {
				sy := min(max(y+tap, 0), height-1)
				src := (sy*width + x) * 4
				weight := kernel[tap+radius]
				sum[0] += scratch[src+0] * weight
				sum[1] += scratch[src+1] * weight
				sum[2] += scratch[src+2] * weight
				sum[3] += scratch[src+3] * weight
			}
			for channel := range 4 {
				pixels[dst+channel] = byte(min(max(int(sum[channel]+0.5), 0), 255))
			}
		}
	}
	return Bitmap{Pix: pixels, Width: width, Height: height, Stride: stride}
}

func gaussianKernel(radius int, sigma float64) []float32 {
	kernel := make([]float32, 2*radius+1)
	twoSigmaSquared := 2 * sigma * sigma
	total := float64(0)
	for offset := -radius; offset <= radius; offset++ {
		weight := math.Exp(-float64(offset*offset) / twoSigmaSquared)
		kernel[offset+radius] = float32(weight)
		total += weight
	}
	inverse := float32(1 / total)
	for index := range kernel {
		kernel[index] *= inverse
	}
	return kernel
}
