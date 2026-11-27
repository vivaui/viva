package platform

import (
	"math"
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestWatermarkValidationAndDefaults(t *testing.T) {
	base := Watermark{Text: "测试用户 10086", Color: Color{Alpha: 0.1}, FontWeight: -0.4}
	got, err := NormalizeWatermark(base)
	if err != nil || got.FontSize != 12 || got.TileWidth != 240 || got.TileHeight != 180 || got.FontWeight != -0.4 {
		t.Fatalf("defaults: %+v, %v", got, err)
	}
	for name, change := range map[string]func(*Watermark){
		"invalid UTF-8":   func(w *Watermark) { w.Text = "\xff" },
		"NUL":             func(w *Watermark) { w.Text = "a\x00b" },
		"negative tile":   func(w *Watermark) { w.TileWidth = -1 },
		"multiline":       func(w *Watermark) { w.Text = "a\nb" },
		"negative weight": func(w *Watermark) { w.FontWeight = -1 },
		"large weight":    func(w *Watermark) { w.FontWeight = 1 },
		"large tile":      func(w *Watermark) { w.TileWidth = 1 << 25 },
		"invalid family":  func(w *Watermark) { w.FontFamily = "bad\x00family" },
		"negative alpha":  func(w *Watermark) { w.Color.Alpha = -1 },
		"negative inset":  func(w *Watermark) { w.Insets.Top = -1 },
		"NaN":             func(w *Watermark) { w.Angle = float32(math.NaN()) },
		"infinite color":  func(w *Watermark) { w.Color.Red = math.Inf(1) },
	} {
		t.Run(name, func(t *testing.T) {
			value := base
			change(&value)
			if _, err := NormalizeWatermark(value); err == nil {
				t.Fatal("invalid request accepted")
			}
		})
	}
	for _, value := range []Watermark{{}} {
		normalized, err := NormalizeWatermark(value)
		if err != nil || normalized != (Watermark{}) {
			t.Fatalf("disabled: %+v, %v", normalized, err)
		}
	}
}

func TestWatermarkPixelGeometry(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		width, height := WatermarkTileSize(Watermark{TileWidth: 240, TileHeight: 180}, scale)
		if width != int(240*scale) || height != int(180*scale) {
			t.Fatal("tile was not rasterized at backing scale", scale, width, height)
		}
		area := WatermarkContentRect(gfx.Size{Width: 1600 * scale, Height: 960 * scale}, gfx.Insets{Top: 42, Left: 120}, scale)
		if area.X != 120*scale || area.Width != (1600-120)*scale || area.Y < float32(42)*scale || area.Height <= 0 {
			t.Fatal("incorrect content clipping", scale, area)
		}
	}
	area := WatermarkContentRect(gfx.Size{Width: 10, Height: 10}, gfx.Insets{Left: 120, Top: 42}, 2)
	if area.Width != 0 || area.Height != 0 {
		t.Fatal("insets outside client must hide the overlay", area)
	}
}

func TestWatermarkTextOnlyIsVisibleAndTileAllocationIsBounded(t *testing.T) {
	value, err := NormalizeWatermark(Watermark{Text: "DRAFT"})
	if err != nil || value.Color.Alpha <= 0 || value.FontSize <= 0 {
		t.Fatal("text-only watermark is invisible", value, err)
	}
	if width, height := WatermarkTileSize(Watermark{TileWidth: 8192, TileHeight: 8192}, 4); width != 0 || height != 0 {
		t.Fatal("oversized backing allocation accepted")
	}
}
