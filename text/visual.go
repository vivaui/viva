package text

import (
	"unicode/utf8"

	"github.com/vivaui/viva/internal/bidi"
	"github.com/vivaui/viva/internal/grapheme"
)

// stepLeft is the rune index one step to the left of caret on the screen: the previous
// grapheme cluster in left-to-right text, and in a line with right-to-left text the next
// place in the order UAX #9 displays the line, so that the Left key always moves the
// caret to the left even through Hebrew or Arabic.
func (editor *Editor) stepLeft(caret int) int {
	if next, ok := editor.visualStep(caret, -1); ok {
		return next
	}
	return previousBoundary(editor.runes, caret)
}

// stepRight is stepLeft to the right.
func (editor *Editor) stepRight(caret int) int {
	if next, ok := editor.visualStep(caret, 1); ok {
		return next
	}
	return nextBoundary(editor.runes, caret)
}

// visualStep moves the caret to the nearest place to its left or right on the screen in a
// line with text that does not read from left to right; ok is false for any other line,
// and when the caret is not on a cluster boundary of the line.
//
// Each boundary between clusters is drawn where textmetrics.Caret draws it. The step goes
// to the boundary with the next place along the screen; boundaries drawn at the same place
// are one stop.
func (editor *Editor) visualStep(caret, direction int) (int, bool) {
	lineStart, lineEnd := caret, caret
	for lineStart > 0 && editor.runes[lineStart-1] != '\n' {
		lineStart--
	}
	for lineEnd < len(editor.runes) && editor.runes[lineEnd] != '\n' {
		lineEnd++
	}
	lineRunes := editor.runes[lineStart:lineEnd]
	if !bidi.NeedsBidiRunes(lineRunes) {
		return 0, false
	}
	line := string(lineRunes)
	resolver := &editor.visualResolve
	paragraph := resolver.Resolve(line, bidi.Auto)
	runs := resolver.LineRuns(paragraph, 0, len(line), nil)

	// The clusters in logical order, with where each is on the screen and whether it reads
	// from right to left. The scratch belongs to the editor; a step reuses it instead of
	// allocating three slices on every arrow key.
	starts := editor.visualStarts[:0] // byte offset of every cluster, then the length of the line
	for at := 0; at < len(line); at = grapheme.Next(line, at) {
		starts = append(starts, at)
	}
	starts = append(starts, len(line))
	editor.visualStarts = starts
	n := len(starts) - 1
	if n == 0 {
		return 0, false
	}
	visual := editor.visualOrder[:0] // the index of each cluster along the screen
	odd := editor.visualOdd[:0]      // whether each cluster reads from right to left
	for index := 0; index < n; index++ {
		visual = append(visual, 0)
		odd = append(odd, false)
	}
	editor.visualOrder, editor.visualOdd = visual, odd
	index := func(offset int) int { // the cluster that starts at offset
		lo, hi := 0, n
		for lo < hi {
			mid := (lo + hi) / 2
			if starts[mid] < offset {
				lo = mid + 1
			} else {
				hi = mid
			}
		}
		return lo
	}
	seen := 0
	for _, run := range runs {
		first, last := index(run.Start), index(run.End) // clusters [first, last)
		for k := first; k < last; k++ {
			odd[k] = run.Level&1 != 0
			if odd[k] {
				visual[k] = seen + (last - 1 - k)
			} else {
				visual[k] = seen + (k - first)
			}
		}
		seen += last - first
	}
	// The place along the screen of the boundary before cluster k (the end of the line for
	// k = n): places 0 to n, place j being the left edge of the j-th cluster on the screen.
	// A boundary is drawn at the trailing edge of the cluster before it when that reads in
	// the direction of the paragraph (typing there continues it) and at the leading edge of
	// the cluster after it otherwise; at the ends of the line there is only one choice. This
	// is the rule of textmetrics.Caret, so every place along the screen is a stop.
	trailing := func(c int) int {
		if odd[c] {
			return visual[c]
		}
		return visual[c] + 1
	}
	leading := func(c int) int {
		if odd[c] {
			return visual[c] + 1
		}
		return visual[c]
	}
	place := func(k int) int {
		switch {
		case k == 0:
			return leading(0)
		case k == n:
			return trailing(n - 1)
		case odd[k-1] == (paragraph.Level&1 != 0):
			return trailing(k - 1)
		}
		return leading(k)
	}
	byteOffset := 0
	for _, r := range editor.runes[lineStart:caret] {
		byteOffset += utf8.RuneLen(r)
	}
	current := index(byteOffset)
	if current > n || starts[current] != byteOffset {
		return 0, false
	}
	here, best, bestPlace := place(current), -1, 0
	for k := 0; k <= n; k++ {
		p := place(k)
		if direction > 0 && p > here && (best < 0 || p < bestPlace) || direction < 0 && p < here && (best < 0 || p > bestPlace) {
			best, bestPlace = k, p
		}
	}
	if best < 0 {
		return caret, true
	}
	return lineStart + utf8.RuneCountInString(line[:starts[best]]), true
}
