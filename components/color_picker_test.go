package components

import (
	"math"
	"testing"

	"github.com/vivaui/viva/gfx"
)

func pickerTestOptions() ColorPickerOptions {
	return ColorPickerOptions{
		IDs:   ColorPickerIDs{Trigger: gfx.Item(1), Dismiss: gfx.Item(2), Saturation: gfx.Item(3), Hue: gfx.Item(4), Alpha: gfx.Item(5), Hex: gfx.Item(6), Clear: gfx.Item(7), Cancel: gfx.Item(8), Apply: gfx.Item(9)},
		Scope: gfx.Item(10), Layer: 3, Viewport: gfx.Rect{Width: 500, Height: 400}, Open: true,
		Value: ColorPickerValue{Hue: 210, Saturation: .7, Brightness: .8, Alpha: .5},
		Hex:   InputSnapshot{Value: "#3D85CC"},
	}
}

func TestColorPickerHexAndRoundTrip(t *testing.T) {
	for _, test := range []struct {
		in, out string
		alpha   bool
	}{
		{"#123", "#112233", false}, {" #aBf ", "#AABBFF", false}, {"#1234", "#11223344", true},
		{"#1020aF", "#1020AF", false}, {"#12345678", "#12345678", true},
		{"#fff", "#FFFFFF", false}, {"#000", "#000000", false},
	} {
		c, ok := ParseColorHex(test.in, test.alpha)
		if !ok {
			t.Fatal(test.in)
		}
		var buf [9]byte
		got := string((ColorPickerValue{}.WithColor(c)).AppendHex(buf[:0], test.alpha))
		if got != test.out {
			t.Fatalf("%q: %q != %q", test.in, got, test.out)
		}
	}
	for _, value := range []string{"", "123456", "#12", "#12345", "#1234567", "#123456789", "#gg0000", "#１２３", "#ff00ffjunk"} {
		if _, ok := ParseColorHex(value, true); ok {
			t.Fatalf("accepted %q", value)
		}
	}
	for _, value := range []string{"#1234", "#12345678"} {
		if _, ok := ParseColorHex(value, false); ok {
			t.Fatalf("accepted alpha %q", value)
		}
	}
	// Round trip all byte values, primaries and achromatic endpoints.
	for i := range 256 {
		c := gfx.Color{Red: float32(i) / 255, Green: float32(255-i) / 255, Blue: float32(i/2) / 255, Alpha: 1}
		got := (ColorPickerValue{}.WithColor(c)).Color()
		if math.Abs(float64(got.Red-c.Red)) > 1e-6 || math.Abs(float64(got.Green-c.Green)) > 1e-6 || math.Abs(float64(got.Blue-c.Blue)) > 1e-6 {
			t.Fatal(c, got)
		}
	}
	seed := ColorPickerValue{Hue: 270, Saturation: .6, Brightness: .7, Alpha: 1}
	black := seed.WithColor(gfx.Color{Alpha: 1})
	gray := seed.WithColor(gfx.Color{Red: .5, Green: .5, Blue: .5, Alpha: 1})
	if black.Hue != 270 || black.Saturation != .6 || gray.Hue != 270 || gray.Saturation != 0 {
		t.Fatal("achromatic edits lost HSV intent")
	}
	bad := ColorPickerValue{Hue: float32(math.NaN()), Saturation: -1, Brightness: float32(math.Inf(1)), Alpha: float32(math.NaN())}.Color()
	if bad != (gfx.Color{Red: 1, Green: 1, Blue: 1}) {
		t.Fatal("nonfinite input", bad)
	}
}

func TestColorPickerViewportAndInput(t *testing.T) {
	for _, theme := range []gfx.Theme{gfx.LightTheme(), gfx.DarkTheme()} {
		for _, scale := range []float32{1, 1.25, 1.5, 1.75, 2, 3} {
			for _, height := range []float32{300, 400, 600} {
				for _, alpha := range []bool{false, true} {
					var c gfx.Canvas
					c.SetPixelScale(scale)
					o := pickerTestOptions()
					o.Viewport = gfx.Rect{Width: 320, Height: height}
					o.ShowAlpha = alpha
					o.Error = "Invalid hex"
					o.Hex.Invalid = true
					l := BeginColorPicker(&c, theme, gfx.Rect{X: 280, Y: height - 40, Width: 32, Height: 32}, o).Paint()
					if c.TrappedFocusScope() != o.Scope || c.EscapeTarget() != o.IDs.Dismiss {
						t.Fatal("popup routing lost")
					}
					for _, h := range c.Hits() {
						if h.Block || h.ID == o.IDs.Trigger || h.ID == o.IDs.Dismiss {
							continue
						}
						b := h.Bounds
						if b.X < 0 || b.Y < 0 || b.X+b.Width > 320.01 || b.Y+b.Height > height+.01 {
							t.Fatalf("scale %v height %v: clipped hit %+v", scale, height, h)
						}
						if h.ID == o.IDs.Apply && !h.Disabled {
							t.Fatal("invalid hex can be committed")
						}
					}
					v, r := l.Pointer(gfx.PointerEvent{Target: o.IDs.Saturation, Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, X: l.Saturation.X + l.Saturation.Width*.75, Y: l.Saturation.Y + l.Saturation.Height*.25}, o.Value)
					if !r.Handled || math.Abs(float64(v.Saturation-.75)) > .001 || math.Abs(float64(v.Brightness-.75)) > .001 {
						t.Fatal(v, r)
					}
					v, r = l.Pointer(gfx.PointerEvent{Target: o.IDs.Hue, Kind: gfx.PointerMove, Captured: true, X: l.Hue.X + l.Hue.Width + 100}, v)
					if !r.Handled || v.Hue != 360 {
						t.Fatal("captured drag should clamp", v, r)
					}
					v, r = l.Key(gfx.KeyEvent{Target: o.IDs.Hue, Key: gfx.KeyLeft, Modifiers: gfx.ModifierShift}, v)
					if !r.Handled || v.Hue != 350 {
						t.Fatal("keyboard large step", v, r)
					}
					if _, r = l.Key(gfx.KeyEvent{Target: o.IDs.Hex, Key: gfx.KeyLeft}, v); r.Handled {
						t.Fatal("stole hex key")
					}
					if _, r = l.Pointer(gfx.PointerEvent{Target: o.IDs.Hue, Kind: gfx.PointerDown, Button: gfx.PointerButtonSecondary}, v); r.Handled {
						t.Fatal("secondary button edits")
					}
				}
			}
		}
	}
}

func TestColorPickerClosedDisabledAndPaintAllocations(t *testing.T) {
	var c gfx.Canvas
	o := pickerTestOptions()
	paint := func() {
		c.Reset()
		BeginColorPicker(&c, gfx.LightTheme(), gfx.Rect{X: 40, Y: 40, Width: 32, Height: 32}, o).Paint()
	}
	paint()
	if n := testing.AllocsPerRun(100, paint); n != 0 {
		t.Fatalf("open paint allocations %v", n)
	}
	o.RoundTrigger = true
	paint()
	if n := testing.AllocsPerRun(100, paint); n != 0 {
		t.Fatalf("round paint allocations %v", n)
	}
	o.Open = false
	paint()
	if len(c.Hits()) != 1 {
		t.Fatal("closed picker declares popup")
	}
	if n := testing.AllocsPerRun(100, paint); n != 0 {
		t.Fatal(n)
	}
	o.Open, o.Disabled = true, true
	paint()
	if len(c.Hits()) != 1 || !c.Hits()[0].Disabled {
		t.Fatal("disabled picker declares popup")
	}
}

func BenchmarkColorPicker(b *testing.B) {
	for _, open := range []bool{false, true} {
		name := "closed"
		if open {
			name = "open"
		}
		b.Run(name, func(b *testing.B) {
			var c gfx.Canvas
			o := pickerTestOptions()
			o.Open = open
			paint := func() {
				c.Reset()
				BeginColorPicker(&c, gfx.LightTheme(), gfx.Rect{X: 40, Y: 40, Width: 32, Height: 32}, o).Paint()
			}
			paint()
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				paint()
			}
		})
	}
}
