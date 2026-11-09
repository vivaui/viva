package windows

import (
	"errors"
	"runtime"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

// BenchmarkLabelRasterize is a label whose text changes each frame, drawn and read
// back: the cost of rasterizing proportional text through the renderer, on the
// software rasterizer of the test machine.
func BenchmarkLabelRasterize(benchmark *testing.B) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	for _, test := range []struct{ name, a, b string }{
		{"latin", "Connect to host-01 as ada, port 22", "Reconnect to host-02 as bob, port 2222"},
		{"cjk", "界面一致性测试。日本語のテキスト", "连接到主机，端口二十二的测试界面"},
		{"emoji", "Deployed \U0001F680 to production", "Rolled back \U0001F4A5 on staging"},
	} {
		benchmark.Run(test.name, func(benchmark *testing.B) {
			offscreen, err := NewOffscreen(400, 28, 1, platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1})
			if errors.Is(err, platform.ErrNoGraphics) {
				benchmark.Skip(err)
			}
			if err != nil {
				benchmark.Fatal(err)
			}
			defer offscreen.Close()
			label := platform.TextLabel{Width: 400, Height: 28, FontSize: 13, Color: platform.Color{Alpha: 1}}
			benchmark.ReportAllocs()
			benchmark.ResetTimer()
			for index := range benchmark.N {
				label.Value = test.a
				if index%2 == 1 {
					label.Value = test.b
				}
				if err := offscreen.Set(nil, []platform.TextLabel{label}); err != nil {
					benchmark.Fatal(err)
				}
				if _, err := offscreen.Image(); err != nil {
					benchmark.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkRasterizeOnly is one sentence rasterized with nothing else: the part of the cost
// that is the text engine.
func BenchmarkRasterizeOnly(benchmark *testing.B) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rasterizer, err := newDirectRasterizer()
	if err != nil {
		benchmark.Fatal(err)
	}
	defer rasterizer.close()
	label := platform.TextLabel{Value: "Connect to host-01 as ada, port 22", Width: 400, Height: 28, FontSize: 13, Color: platform.Color{Alpha: 1}}
	benchmark.ReportAllocs()
	benchmark.ResetTimer()
	for range benchmark.N {
		if _, _, err := rasterizer.rasterize(label, 1, 400, 28, true, 0); err != nil {
			benchmark.Fatal(err)
		}
	}
}

// terminalLine is a line of a terminal: cells columns of fixed pitch in several colors.
func terminalLine(cells int, colors []gfx.Color) platform.TextLabel {
	value := make([]byte, cells)
	for index := range value {
		value[index] = byte('a' + index%26)
	}
	label := platform.TextLabel{Value: string(value), Mono: true, CellWidth: 8, Width: float32(cells) * 8, Height: 20, FontSize: 14, Color: platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}}
	per := cells / len(colors)
	for index, color := range colors {
		label.Runs = append(label.Runs, gfx.TextRun{Column: int32(index * per), Bytes: int32(per), Cells: int32(per), Color: color})
	}
	return label
}

var terminalColors = []gfx.Color{{Red: 1, Green: 1, Blue: 1, Alpha: 1}, {Green: 1, Alpha: 1}, {Red: 1, Alpha: 1}, {Red: 0.2, Green: 0.4, Blue: 1, Alpha: 1}}

// BenchmarkGridRasterize is one terminal line of 100 cells in four colors rasterized, as
// grayscale (what it was) and as ClearType.
func BenchmarkGridRasterize(benchmark *testing.B) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rasterizer, err := newDirectRasterizer()
	if err != nil {
		benchmark.Fatal(err)
	}
	defer rasterizer.close()
	label := terminalLine(100, terminalColors)
	for _, test := range []struct {
		name string
		lcd  bool
	}{{"grayscale", false}, {"cleartype", true}} {
		benchmark.Run(test.name, func(benchmark *testing.B) {
			benchmark.ReportAllocs()
			for range benchmark.N {
				if _, _, err := rasterizer.rasterize(label, 1, 800, 20, test.lcd, 0); err != nil {
					benchmark.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkTerminalScreenUnchanged is a screen of 40 terminal lines in four colors drawn
// again with no line changed: the steady-state cost of a repaint, where a ClearType line
// is drawn once per color.
func BenchmarkTerminalScreenUnchanged(benchmark *testing.B) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	offscreen, err := NewOffscreen(800, 800, 1, platform.Color{Alpha: 1})
	if errors.Is(err, platform.ErrNoGraphics) {
		benchmark.Skip(err)
	}
	if err != nil {
		benchmark.Fatal(err)
	}
	defer offscreen.Close()
	labels := make([]platform.TextLabel, 40)
	for index := range labels {
		labels[index] = terminalLine(100, terminalColors)
		labels[index].Y = float32(index * 20)
	}
	if err := offscreen.Set(nil, labels); err != nil {
		benchmark.Fatal(err)
	}
	benchmark.ReportAllocs()
	benchmark.ResetTimer()
	for range benchmark.N {
		if err := offscreen.Set(nil, labels); err != nil {
			benchmark.Fatal(err)
		}
		if _, err := offscreen.Image(); err != nil {
			benchmark.Fatal(err)
		}
	}
}
