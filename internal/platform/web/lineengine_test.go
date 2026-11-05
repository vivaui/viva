package web

import (
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/vivaui/viva/internal/textenginetest"
	"github.com/vivaui/viva/ui"
)

// fakeBrowser lays a line out the way a browser does for a left-to-right paragraph:
// characters left to right, except that each run of Hebrew is displayed reversed in
// the place its logical start would be. Combining marks add no advance.
type fakeBrowser struct {
	widths, rectCalls int
	invalidate        func()
}

func (browser *fakeBrowser) setInvalidate(invalidate func()) { browser.invalidate = invalidate }

func advance(r rune, size float32) float32 {
	switch {
	case unicode.Is(unicode.Mn, r), r == 0x200d, r >= 0x1f3fb && r <= 0x1f3ff:
		return 0
	case unicode.Is(unicode.Hebrew, r):
		return size * 0.6
	case r >= 0x2e80 || r >= 0x1f000:
		return size
	}
	return size * 0.5
}

func (browser *fakeBrowser) width(value string, size, weight float32, rtl bool) float32 {
	browser.widths++
	var total float32
	for _, r := range value {
		total += advance(r, size)
	}
	return total
}

// layout is each rune's left and right edge, by UTF-16 offset of the rune.
func (browser *fakeBrowser) layout(value string, size float32) (starts []int32, left, right []float32) {
	runes := []rune(value)
	left, right = make([]float32, len(runes)), make([]float32, len(runes))
	var x float32
	for i := 0; i < len(runes); {
		hebrew := unicode.Is(unicode.Hebrew, runes[i])
		j := i
		var run float32
		for j < len(runes) && unicode.Is(unicode.Hebrew, runes[j]) == hebrew {
			run += advance(runes[j], size)
			j++
		}
		var at float32
		for k := i; k < j; k++ {
			a := advance(runes[k], size)
			if hebrew {
				right[k], left[k] = x+run-at, x+run-at-a
			} else {
				left[k], right[k] = x+at, x+at+a
			}
			at += a
		}
		x += run
		i = j
	}
	units := int32(0)
	for _, r := range runes {
		starts = append(starts, units)
		units += utf16Units(r)
	}
	return starts, left, right
}

func (browser *fakeBrowser) rects(value string, size, weight float32, rtl bool, bounds []int32, lefts, rights []float32) {
	browser.rectCalls++
	starts, left, right := browser.layout(value, size)
	for i := range lefts {
		lefts[i], rights[i] = 1e9, -1e9
		for k, start := range starts {
			if start >= bounds[i] && start < bounds[i+1] {
				lefts[i], rights[i] = min(lefts[i], left[k]), max(rights[i], right[k])
			}
		}
	}
}

type italicBrowser struct{ fakeBrowser }

func (browser *italicBrowser) widthStyle(value string, size, weight float32, italic, rtl bool) float32 {
	width := browser.fakeBrowser.width(value, size, weight, rtl)
	if italic {
		return width * 2
	}
	return width
}

func (browser *italicBrowser) rectsStyle(value string, size, weight float32, italic, rtl bool, bounds []int32, lefts, rights []float32) {
	browser.fakeBrowser.rects(value, size, weight, rtl, bounds, lefts, rights)
	if italic {
		for index := range lefts {
			lefts[index] *= 2
			rights[index] *= 2
		}
	}
}

func TestItalicBrowserMetricsAndCacheStaySeparate(t *testing.T) {
	browser := &italicBrowser{}
	engine := newLineEngine(browser)
	if regular, italic := engine.Width("abcd", 10, 0), engine.WidthStyle("abcd", 10, 0, true); regular != 20 || italic != 40 {
		t.Fatalf("regular/italic width = %v/%v", regular, italic)
	}
	if caret := engine.CaretStyle("abcd", 10, 0, true, 2); caret != 20 {
		t.Fatalf("italic caret = %v", caret)
	}
	if offset := engine.OffsetAtStyle("abcd", 10, 0, true, 26); offset != 3 {
		t.Fatalf("italic hit = %d", offset)
	}
	if browser.widths != 2 {
		t.Fatalf("regular and italic cache entries measured %d times", browser.widths)
	}
	browser.invalidate()
	engine.Width("abcd", 10, 0)
	engine.WidthStyle("abcd", 10, 0, true)
	if browser.widths != 4 {
		t.Fatalf("font invalidation measured %d widths, want both styles remeasured", browser.widths)
	}
}

func TestLineEnginePassesTheTextEngineContract(t *testing.T) {
	textenginetest.Run(t, newLineEngine(&fakeBrowser{}))
}

// "abc" then two Hebrew letters: the Hebrew pair is displayed reversed, so the
// selection of the last Hebrew letter and the "c" before it are two pieces, not one.
func TestASelectionAcrossAChangeOfDirectionIsSeveralSpans(t *testing.T) {
	engine := newLineEngine(&fakeBrowser{})
	const value = "abcאב" // widths at size 10: 5, 5, 5, 6, 6
	whole := engine.Selection(nil, value, 10, 0, 0, len(value))
	if len(whole) != 1 || whole[0].X != 0 || whole[0].Width != 27 {
		t.Fatalf("whole line = %+v, want one span [0, 27)", whole)
	}
	// "c" (logical 2) and the first Hebrew letter (logical 3), which is displayed at the
	// far right of the Hebrew run: not adjacent.
	got := engine.Selection(nil, value, 10, 0, 2, 5)
	if len(got) != 2 || got[0].X >= got[1].X {
		t.Fatalf("selection across the boundary = %+v, want two spans left to right", got)
	}
	// The caret of a right-to-left letter is its right edge.
	if got, want := engine.Caret(value, 10, 0, len("abc")), float32(27); got != want {
		t.Errorf("caret before the first Hebrew letter = %v, want %v (its right edge, the Hebrew run being 15 to 27)", got, want)
	}
	if got, want := engine.Caret(value, 10, 0, len(value)), float32(15); got != want {
		t.Errorf("caret at the end = %v, want %v (the left edge of the last Hebrew letter)", got, want)
	}
}

func TestAClusterIsOneCaretStop(t *testing.T) {
	engine := newLineEngine(&fakeBrowser{})
	const value = "éx" // e with a combining acute, then x
	if got, want := engine.Caret(value, 10, 0, 1), engine.Caret(value, 10, 0, 0); got != want {
		t.Errorf("a caret inside a cluster is at %v, its start is %v", got, want)
	}
	if got := engine.OffsetAt(value, 10, 0, 4); got != 0 && got != len("é") {
		t.Errorf("OffsetAt = %d, which is inside the cluster", got)
	}
}

func TestAMeasuredLineIsNotMeasuredAgainAndAnInvalidationForgetsIt(t *testing.T) {
	browser := &fakeBrowser{}
	engine := newLineEngine(browser)
	const value = "192.168.1.1 主机"
	dst := make([]ui.TextSpan, 0, 4)
	query := func() {
		engine.Width(value, 14, 0)
		engine.Caret(value, 14, 0, 4)
		engine.OffsetAt(value, 14, 0, 30)
		dst = engine.Selection(dst[:0], value, 14, 0, 2, 8)
	}
	query()
	if browser.widths != 1 || browser.rectCalls != 1 {
		t.Fatalf("first queries measured %d widths and %d geometries, want 1 and 1", browser.widths, browser.rectCalls)
	}
	if allocations := testing.AllocsPerRun(100, query); allocations != 0 {
		t.Errorf("warm queries allocated %v times", allocations)
	}
	if browser.widths != 1 || browser.rectCalls != 1 {
		t.Errorf("warm queries went back to the browser: %d widths, %d geometries", browser.widths, browser.rectCalls)
	}
	// A font that finished loading changes every width.
	browser.invalidate()
	query()
	if browser.widths != 2 {
		t.Errorf("after an invalidation the line was measured %d times, want 2", browser.widths)
	}
}

func TestUTF16OffsetsCountSurrogatePairs(t *testing.T) {
	var got []int32
	browser := &recordingBrowser{fakeBrowser: &fakeBrowser{}, record: func(bounds []int32) { got = bounds }}
	engine := newLineEngine(browser)
	value := "a👍🏽b" // a, thumbs-up + skin tone modifier (one cluster, two surrogate pairs), b
	engine.Caret(value, 10, 0, 1)
	want := []int32{0, 1, 5, 6}
	if len(got) != len(want) {
		t.Fatalf("bounds = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("bounds = %v, want %v", got, want)
		}
	}
	if utf8.RuneCountInString(value) != 4 {
		t.Fatal("test string changed")
	}
}

type recordingBrowser struct {
	*fakeBrowser
	record func([]int32)
}

func (browser *recordingBrowser) rects(value string, size, weight float32, rtl bool, bounds []int32, lefts, rights []float32) {
	browser.record(append([]int32(nil), bounds...))
	browser.fakeBrowser.rects(value, size, weight, rtl, bounds, lefts, rights)
}
