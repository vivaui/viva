package platform

import (
	"fmt"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/vivaui/viva/gfx"
)

const (
	watermarkMaxFontSize   = 4096
	watermarkMaxTileSize   = 8192
	watermarkMaxTilePixels = 16 << 20
	watermarkMaxTextBytes  = 16 << 10
)

// Watermark is an input-transparent, native text pattern above all content.
type Watermark struct {
	Text, FontFamily            string
	Color                       Color
	FontSize, FontWeight, Angle float32
	TileWidth, TileHeight       float32
	Insets                      gfx.Insets
}

// NormalizeWatermark validates the complete request before changing native state.
func NormalizeWatermark(w Watermark) (Watermark, error) {
	if len(w.Text) > watermarkMaxTextBytes || len(w.FontFamily) > 1024 || strings.ContainsAny(w.Text, "\r\n") {
		return Watermark{}, fmt.Errorf("watermark: text must be a bounded single line and family name at most 1024 bytes")
	}
	if !utf8.ValidString(w.Text) || strings.IndexByte(w.Text, 0) >= 0 || !utf8.ValidString(w.FontFamily) || strings.IndexByte(w.FontFamily, 0) >= 0 {
		return Watermark{}, fmt.Errorf("watermark: text must be valid UTF-8 without NUL")
	}
	values := []float32{w.FontSize, w.TileWidth, w.TileHeight, w.Insets.Top, w.Insets.Right, w.Insets.Bottom, w.Insets.Left}
	for _, v := range values {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) || v < 0 {
			return Watermark{}, fmt.Errorf("watermark: dimensions must be finite and nonnegative")
		}
	}
	for _, v := range []float64{float64(w.Angle), float64(w.FontWeight), w.Color.Red, w.Color.Green, w.Color.Blue, w.Color.Alpha} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return Watermark{}, fmt.Errorf("watermark: parameters must be finite")
		}
	}
	if w.Color.Red < 0 || w.Color.Red > 1 || w.Color.Green < 0 || w.Color.Green > 1 || w.Color.Blue < 0 || w.Color.Blue > 1 || w.Color.Alpha < 0 || w.Color.Alpha > 1 {
		return Watermark{}, fmt.Errorf("watermark: color components must be between 0 and 1")
	}
	if w.FontWeight < gfx.WeightLight || w.FontWeight > gfx.WeightBold {
		return Watermark{}, fmt.Errorf("watermark: font weight must be between gfx.WeightLight and gfx.WeightBold")
	}
	if w.Text == "" {
		return Watermark{}, nil
	}
	if w.Color == (Color{}) {
		w.Color = Color{Red: 0.5, Green: 0.5, Blue: 0.5, Alpha: 0.12}
	}
	if w.FontSize == 0 {
		w.FontSize = 12
	}
	if w.TileWidth == 0 {
		w.TileWidth = 240
	}
	if w.TileHeight == 0 {
		w.TileHeight = 180
	}
	// Bound native allocations at every backing scale; the pixel-size helper also
	// caps the total area before a platform allocates its retained tile.
	if w.FontSize > watermarkMaxFontSize || w.TileWidth > watermarkMaxTileSize || w.TileHeight > watermarkMaxTileSize {
		return Watermark{}, fmt.Errorf("watermark: text or tile dimensions exceed limits")
	}
	return w, nil
}

// WatermarkTileSize converts logical periods to physical pixels without a 1x intermediate.
func WatermarkTileSize(w Watermark, scale float32) (int, int) {
	if scale <= 0 || math.IsNaN(float64(scale)) || math.IsInf(float64(scale), 0) {
		return 0, 0
	}
	width, height := math.Ceil(float64(w.TileWidth)*float64(scale)), math.Ceil(float64(w.TileHeight)*float64(scale))
	if width <= 0 || height <= 0 || width > watermarkMaxTileSize || height > watermarkMaxTileSize || width*height > watermarkMaxTilePixels {
		return 0, 0
	}
	return int(width), int(height)
}

// WatermarkContentRect clips the overlay to client pixels. Insets are logical.
func WatermarkContentRect(size gfx.Size, insets gfx.Insets, scale float32) gfx.Rect {
	x, y := float32(math.Ceil(float64(insets.Left*scale))), float32(math.Ceil(float64(insets.Top*scale)))
	right, bottom := float32(math.Floor(float64(size.Width-insets.Right*scale))), float32(math.Floor(float64(size.Height-insets.Bottom*scale)))
	return gfx.Rect{X: x, Y: y, Width: max(right-x, 0), Height: max(bottom-y, 0)}
}
