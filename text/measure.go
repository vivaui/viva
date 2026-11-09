package text

// Metrics contains one shaped line's logical metrics.
type Metrics struct {
	Width   float32
	Height  float32
	Ascent  float32
	Descent float32
	Leading float32
}

// MeasureFunc shapes and measures one line at a logical font size and weight
// (gfx.TextStyle.Weight: 0 is regular) with a platform's own text engine: how wide
// the platform would draw it, which is not how wide Viva lays it out (layout uses the
// reference metrics every platform shares). Tools compare the two.
type MeasureFunc func(value string, fontSize, weight float32) (Metrics, error)
