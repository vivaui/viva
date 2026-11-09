// Package textenginetest is the contract every gfx.TextEngine passes: the
// reference metrics, and the engine of each backend, run Run from their own tests.
package textenginetest

import (
	"fmt"
	"math"
	"testing"
	"unicode/utf8"

	"github.com/vivaui/viva/gfx"
)

const tolerance = 0.5 // logical pixels: hinting and rounding in a platform's own advances

// Latin and mixed-script lines that read left to right, with combining marks, a
// ligature candidate, CJK and an emoji with a modifier.
var leftToRight = []string{
	"Hello",
	"2213123中文",
	"192.168.1.1 主机",
	"office affine ffi",
	"a b  c",
	"é café",
	"日本語のテキスト",
	"👍🏽 ok",
}

// Lines that change direction.
var mixedDirection = []string{
	"abc שלום def",
	"שלום",
}

var weights = []float32{gfx.WeightRegular, gfx.WeightBold}

// Run checks engine against the contract of gfx.TextEngine.
func Run(t *testing.T, engine gfx.TextEngine) {
	t.Helper()
	t.Run("EmptyLine", func(t *testing.T) { emptyLine(t, engine) })
	t.Run("LeftToRight", func(t *testing.T) {
		for _, value := range leftToRight {
			for _, weight := range weights {
				for _, size := range []float32{12, 14} {
					name := fmt.Sprintf("%q/%g/%g", value, weight, size)
					t.Run(name, func(t *testing.T) { leftToRightLine(t, engine, value, size, weight) })
				}
			}
		}
	})
	t.Run("MixedDirection", func(t *testing.T) {
		for _, value := range mixedDirection {
			t.Run(fmt.Sprintf("%q", value), func(t *testing.T) { mixedLine(t, engine, value, 14, gfx.WeightRegular) })
		}
	})
	t.Run("Scales", func(t *testing.T) { scales(t, engine) })
	t.Run("WarmQueriesDoNotAllocate", func(t *testing.T) { allocations(t, engine) })
}

func emptyLine(t *testing.T, engine gfx.TextEngine) {
	if got := engine.Width("", 14, gfx.WeightRegular); got != 0 {
		t.Errorf("Width of an empty line = %v", got)
	}
	if got := engine.Caret("", 14, gfx.WeightRegular, 0); got != 0 {
		t.Errorf("Caret of an empty line = %v", got)
	}
	if got := engine.OffsetAt("", 14, gfx.WeightRegular, 50); got != 0 {
		t.Errorf("OffsetAt of an empty line = %v", got)
	}
	if total := total(engine.Selection(nil, "", 14, gfx.WeightRegular, 0, 0)); total != 0 {
		t.Errorf("Selection of an empty line has width %v", total)
	}
}

func leftToRightLine(t *testing.T, engine gfx.TextEngine, value string, size, weight float32) {
	width := engine.Width(value, size, weight)
	if width <= 0 {
		t.Fatalf("Width = %v", width)
	}
	if again := engine.Width(value, size, weight); again != width {
		t.Errorf("Width changed between calls: %v then %v", width, again)
	}
	if got := engine.Caret(value, size, weight, 0); math.Abs(float64(got)) > 0.01 {
		t.Errorf("Caret at 0 = %v", got)
	}
	if got := engine.Caret(value, size, weight, len(value)); math.Abs(float64(got-width)) > tolerance {
		t.Errorf("Caret at the end = %v, width %v", got, width)
	}
	if got, want := engine.Caret(value, size, weight, -5), engine.Caret(value, size, weight, 0); got != want {
		t.Errorf("Caret at -5 = %v, at 0 = %v", got, want)
	}
	if got, want := engine.Caret(value, size, weight, len(value)+5), engine.Caret(value, size, weight, len(value)); got != want {
		t.Errorf("Caret past the end = %v, at the end = %v", got, want)
	}

	previous := float32(-1)
	for offset := range value {
		caret := engine.Caret(value, size, weight, offset)
		if caret < previous-0.01 {
			t.Errorf("Caret moved back at %d: %v after %v", offset, caret, previous)
		}
		previous = caret
		// A click on a caret position puts the caret at that same position.
		at := engine.OffsetAt(value, size, weight, caret)
		if at < 0 || at > len(value) || (at < len(value) && !utf8.RuneStart(value[at])) {
			t.Errorf("OffsetAt(Caret(%d)) = %d is not a character boundary", offset, at)
			continue
		}
		if back := engine.Caret(value, size, weight, at); math.Abs(float64(back-caret)) > tolerance {
			t.Errorf("OffsetAt(Caret(%d)) = %d, whose caret %v is not %v", offset, at, back, caret)
		}
		// Inside a character the caret is where that character starts.
		for inside := offset + 1; inside < len(value) && !utf8.RuneStart(value[inside]); inside++ {
			if got := engine.Caret(value, size, weight, inside); got != caret {
				t.Errorf("Caret inside the character at %d: %v at %d, %v at its start", offset, got, inside, caret)
			}
		}
	}
	if got := engine.OffsetAt(value, size, weight, -100); got != 0 {
		t.Errorf("OffsetAt left of the line = %d", got)
	}
	if got := engine.OffsetAt(value, size, weight, width+100); got != len(value) {
		t.Errorf("OffsetAt right of the line = %d, want %d", got, len(value))
	}

	if got := total(engine.Selection(nil, value, size, weight, 0, len(value))); math.Abs(float64(got-width)) > tolerance {
		t.Errorf("Selection of the whole line has width %v, the line %v", got, width)
	}
	if got := total(engine.Selection(nil, value, size, weight, 0, 0)); got > 0.01 {
		t.Errorf("An empty selection has width %v", got)
	}
	for offset := range value {
		left := total(engine.Selection(nil, value, size, weight, 0, offset))
		right := total(engine.Selection(nil, value, size, weight, offset, len(value)))
		if math.Abs(float64(left+right-width)) > tolerance {
			t.Errorf("Selections [0,%d) and [%d,end) are %v and %v wide, the line %v", offset, offset, left, right, width)
		}
		swapped := total(engine.Selection(nil, value, size, weight, len(value), offset))
		if math.Abs(float64(swapped-right)) > 0.01 {
			t.Errorf("Selection(end, %d) = %v, Selection(%d, end) = %v", offset, swapped, offset, right)
		}
	}
}

func mixedLine(t *testing.T, engine gfx.TextEngine, value string, size, weight float32) {
	width := engine.Width(value, size, weight)
	if width <= 0 {
		t.Fatalf("Width = %v", width)
	}
	for _, span := range engine.Selection(nil, value, size, weight, 0, len(value)) {
		if span.X < -tolerance || span.X+span.Width > width+tolerance || span.Width < 0 {
			t.Errorf("selection span %+v lies outside the line [0, %v]", span, width)
		}
	}
	if got := total(engine.Selection(nil, value, size, weight, 0, len(value))); math.Abs(float64(got-width)) > tolerance {
		t.Errorf("Selection of the whole line has width %v, the line %v", got, width)
	}
	for offset := range value {
		caret := engine.Caret(value, size, weight, offset)
		if caret < -tolerance || caret > width+tolerance {
			t.Errorf("Caret(%d) = %v lies outside the line [0, %v]", offset, caret, width)
		}
	}
	for _, x := range []float32{-50, 0, width / 3, width / 2, width, width + 50} {
		at := engine.OffsetAt(value, size, weight, x)
		if at < 0 || at > len(value) || (at < len(value) && !utf8.RuneStart(value[at])) {
			t.Errorf("OffsetAt(%v) = %d is not a character boundary", x, at)
		}
	}
}

func scales(t *testing.T, engine gfx.TextEngine) {
	const value = "The quick brown fox 2213123"
	regular := engine.Width(value, 14, gfx.WeightRegular)
	// A font's spacing changes with its optical size (San Francisco is tighter at
	// large sizes: 1.88x at twice the size), so only the order of magnitude is held.
	if double := engine.Width(value, 28, gfx.WeightRegular); double/regular < 1.7 || double/regular > 2.3 {
		t.Errorf("twice the size is %vx as wide", double/regular)
	}
	if bold := engine.Width(value, 14, gfx.WeightBold); bold < regular-tolerance {
		t.Errorf("bold (%v) is narrower than regular (%v)", bold, regular)
	}
	if cjk := engine.Width("日本語", 14, gfx.WeightRegular); cjk < 2*14 {
		t.Errorf("three CJK characters are %v wide: the fallback font is not measured", cjk)
	}
}

func allocations(t *testing.T, engine gfx.TextEngine) {
	if raceEnabled {
		t.Skip("race instrumentation intentionally drops sync.Pool entries; allocation budgets run without -race")
	}
	const value = "192.168.1.1 主机 abc שלום"
	dst := make([]gfx.TextSpan, 0, 8)
	query := func() {
		engine.Width(value, 14, gfx.WeightRegular)
		engine.Caret(value, 14, gfx.WeightRegular, 5)
		engine.OffsetAt(value, 14, gfx.WeightRegular, 40)
		dst = engine.Selection(dst[:0], value, 14, gfx.WeightRegular, 2, 12)
	}
	query()
	if allocs := testing.AllocsPerRun(50, query); allocs != 0 {
		t.Errorf("warm queries allocated %v times per run", allocs)
	}
}

func total(spans []gfx.TextSpan) float32 {
	var sum float32
	for _, span := range spans {
		sum += span.Width
	}
	return sum
}
