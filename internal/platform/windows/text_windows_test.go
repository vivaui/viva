package windows

import (
	"math"
	"runtime"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

func TestMeasureTextSystemFallback(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	measure, close, err := NewTextMeasurer()
	if err != nil {
		t.Fatal(err)
	}
	defer close()
	for _, value := range []string{"Viva", "原生界面", "Viva 原生界面 👩‍💻", "a\nb"} {
		metrics, err := measure(value, 16, 0)
		if err != nil {
			t.Fatalf("%q: %v", value, err)
		}
		if metrics.Width <= 0 || metrics.Height <= 0 || metrics.Ascent <= 0 || metrics.Descent < 0 {
			t.Fatalf("%q: invalid metrics: %+v", value, metrics)
		}
	}
}

func TestMeasureTextInvalidFontSize(t *testing.T) {
	services := textServices{}
	for _, size := range []float32{0, -1, float32(math.NaN()), float32(math.Inf(1))} {
		if _, err := services.measure("Viva", size, 0); err == nil {
			t.Errorf("invalid font size %v accepted", size)
		}
	}
}

func TestNativeFontWeight(t *testing.T) {
	for _, test := range []struct {
		weight float32
		want   uint32
	}{{0, 400}, {0.1, 500}, {0.2, 600}, {0.35, 700}} {
		if got := nativeFontWeight(test.weight); got != test.want {
			t.Errorf("weight %v: got %d, want %d", test.weight, got, test.want)
		}
	}
}

// rasterizeProportional draws a label as the renderer does; lcd is whether
// the texture holds per-channel coverage (the text may be blended with ClearType).
func rasterizeProportional(t testing.TB, label platform.TextLabel, scale float32, width, height int, allowLCD bool) (pixels []byte, lcd bool, closePixels func()) {
	t.Helper()
	rasterizer, err := newDirectRasterizer()
	if err != nil {
		t.Fatal(err)
	}
	pixels, lcd, err = rasterizer.rasterize(label, scale, width, height, allowLCD, 0)
	if err != nil {
		rasterizer.close()
		t.Fatal(err)
	}
	return pixels, lcd, rasterizer.close
}

func TestRasterizeTextSystemFallback(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	label := platform.TextLabel{Value: "Viva 原生界面", Width: 240, Height: 32, FontSize: 16, Color: platform.Color{Red: 0.8, Green: 0.6, Blue: 0.4, Alpha: 0.75}}
	for _, allowLCD := range []bool{false, true} {
		pixels, lcd, closePixels := rasterizeProportional(t, label, 1, 240, 32, allowLCD)
		if len(pixels) != 240*32*4 || lcd != allowLCD {
			t.Fatalf("pixel length %d, lcd %v with ClearType allowed %v", len(pixels), lcd, allowLCD)
		}
		visible := false
		for index := 0; index < len(pixels); index += 4 {
			alpha := pixels[index+3]
			if alpha != 0 {
				visible = true
			}
			if !lcd && (pixels[index] > alpha || pixels[index+1] > alpha || pixels[index+2] > alpha) {
				t.Fatal("text pixels are not premultiplied")
			}
		}
		closePixels()
		if !visible {
			t.Fatalf("text rasterization produced an empty bitmap (ClearType allowed %v)", allowLCD)
		}
	}
}

// A color emoji is drawn in its own colors, which are not coverage to blend per
// channel: the label is premultiplied color, with ClearType or not.
func TestColorEmojiKeepsItsColors(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	label := platform.TextLabel{Value: "Done \U0001F600", Width: 120, Height: 32, FontSize: 20, Color: platform.Color{Red: 0.1, Green: 0.1, Blue: 0.1, Alpha: 1}}
	pixels, lcd, closePixels := rasterizeProportional(t, label, 1, 120, 32, true)
	defer closePixels()
	if lcd {
		t.Fatal("a label with a color emoji was rasterized for per-channel blending")
	}
	colored := 0
	for index := 0; index < len(pixels); index += 4 {
		alpha := int(pixels[index+3])
		blue, green, red := int(pixels[index]), int(pixels[index+1]), int(pixels[index+2])
		if alpha > 200 && max(red, green, blue)-min(red, green, blue) > 60 {
			colored++
		}
	}
	if colored < 20 {
		t.Fatalf("the emoji has %d strongly colored pixels, it was drawn monochrome", colored)
	}
}

func BenchmarkMeasureTextMixed(benchmark *testing.B) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	measure, close, err := NewTextMeasurer()
	if err != nil {
		benchmark.Fatal(err)
	}
	defer close()
	benchmark.ReportAllocs()
	benchmark.ResetTimer()
	for range benchmark.N {
		if _, err := measure("Viva 原生界面 👩‍💻", 16, 0); err != nil {
			benchmark.Fatal(err)
		}
	}
}

// A heavier weight is a wider line, and a label fitted at the regular weight
// would clip at the heavier one.
func TestMeasureTextIsWiderAtAHeavierWeight(t *testing.T) {
	measure, close, err := NewTextMeasurer()
	if err != nil {
		t.Fatal(err)
	}
	defer close()
	regular, err := measure("Import SSH Config", 14, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, weight := range []float32{0.23, 0.40} {
		heavier, err := measure("Import SSH Config", 14, weight)
		if err != nil {
			t.Fatal(err)
		}
		if heavier.Width <= regular.Width {
			t.Fatalf("weight %.2f: %.2f is not wider than regular %.2f", weight, heavier.Width, regular.Width)
		}
	}
}

// Semibold text is Segoe UI Semibold, a few percent wider than regular as on the
// other platforms. GDI asked for weight 600 of "Segoe UI" emboldens the regular
// face instead, which is about a fifth wider and measured and drawn too heavy.
func TestSemiboldIsTheRealSemiboldFace(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	measure, close, err := NewTextMeasurer()
	if err != nil {
		t.Fatal(err)
	}
	defer close()
	const sample = "The quick brown fox jumps over the lazy dog"
	regular, err := measure(sample, 14, 0)
	if err != nil {
		t.Fatal(err)
	}
	semibold, err := measure(sample, 14, 0.23)
	if err != nil {
		t.Fatal(err)
	}
	if ratio := semibold.Width / regular.Width; ratio < 1 || ratio > 1.10 {
		t.Errorf("semibold line is %.3f times as wide as regular, want a real semibold's 1.02 to 1.08", ratio)
	}
}

func TestLightFontWeight(t *testing.T) {
	if got := nativeFontWeight(gfx.WeightLight); got != 300 {
		t.Fatalf("light weight = %d, want 300", got)
	}
}
