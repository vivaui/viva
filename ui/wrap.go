package ui

import (
	"math"
	"strings"
	"unicode/utf8"

	"github.com/vivaui/viva/internal/grapheme"
	"github.com/vivaui/viva/internal/linebreak"
	"github.com/vivaui/viva/internal/uniprops"
)

// WrapLines breaks value into lines no wider than width at font size size and
// weight (TextStyle.Weight) and appends them to dst, at most maxLines of them.
//
// Where a line may break follows UAX #14 (internal/linebreak): after spaces, hyphens and
// most punctuation as the standard's pair rules say, between ideographs, kana and
// emoji with the line-start and line-end rules of East Asian typesetting, between the
// syllables of the dictionary-less scripts (Thai, Lao, Khmer, Myanmar) at the places their
// writing allows, and at a newline in the text (a trailing CR is dropped). Two
// tailorings: Hangul syllables break at spaces as Korean text is set (a Korean word is
// kept whole when it fits a line), and a word wider than a whole line (a URL, a path, a
// hash) breaks after the last / \ - _ . & ? = that fits and failing that after the last
// grapheme cluster (UAX #29) that fits, so a base with its marks, an emoji sequence, a
// flag, a conjunct is never split.
//
// A line always holds at least one character, however narrow the width. When the
// text needs more than maxLines, the last line ends with an ellipsis. The
// lines are slices of value, so wrapping text that fits builds no strings; only
// the ellipsized last line is new (see WrapLinesCached). Draw the result one
// Label per line.
//
//	lines := canvas.WrapLines(scratch[:0], message, 12, ui.WeightRegular, bounds.Width, 3)
//	for index, line := range lines {
//		canvas.Label(line, ui.Rect{X: bounds.X, Y: bounds.Y + float32(index)*17, Width: bounds.Width, Height: 17}, style)
//	}
//
// Widths are measured at weight, the one the lines are drawn with: a heavier
// line is wider.
func (canvas *Canvas) WrapLines(dst []string, value string, size, weight, width float32, maxLines int) []string {
	return canvas.WrapLinesCached(dst, value, size, weight, width, maxLines, nil)
}

// EllipsisCache remembers the ellipsized last line of one wrapped text, so that
// wrapping the same text at the same width again, frame after frame, builds no
// string. The zero value is ready; keep one for each text that is wrapped every
// frame. Wrapping something else simply replaces what it holds.
type EllipsisCache struct {
	line         string
	size, weight float32
	width        float32
	result       string
	filled       bool
	generation   uint32 // the canvas's FontGeneration when result was cut
}

// WrapLinesCached is WrapLines that takes the ellipsized last line from cache when
// it is the one wanted, and stores it there when it is not. A nil cache builds the
// string every time.
func (canvas *Canvas) WrapLinesCached(dst []string, value string, size, weight, width float32, maxLines int, cache *EllipsisCache) []string {
	if maxLines <= 0 || width <= 0 {
		return dst
	}
	first := len(dst)
	for value != "" {
		line, rest := splitLine(value)
		more := rest != "" // another paragraph follows
		for {
			line = trimSpaceRight(line)
			last := len(dst)-first == maxLines-1
			fits := canvas.TextWidthWeight(line, size, weight) <= width
			if fits && !(last && more) {
				dst = append(dst, line)
				break
			}
			if last {
				return append(dst, canvas.ellipsizeCached(line, size, weight, width, cache))
			}
			cut := canvas.breakPoint(line, size, weight, width)
			dst = append(dst, trimSpaceRight(line[:cut]))
			if line = trimSpaceLeft(line[cut:]); line == "" {
				break
			}
		}
		value = rest
	}
	return dst
}

// FitLine is line when it fits width at font size size and weight (TextStyle.Weight),
// else its start ending in an ellipsis, cut as WrapLines cuts the last line; nothing
// when width leaves no room. It is the one-line text of a table cell or a list row,
// and draws as one Label.
//
// A cut line is remembered by the canvas, so painting the same cell at the same width
// again, frame after frame, builds no string: only the first paint of a line, or one
// that another line of the same slot replaced, allocates. Lines longer than 512 bytes
// are cut afresh every time.
func (canvas *Canvas) FitLine(line string, size, weight, width float32) string {
	if width <= 0 {
		return ""
	}
	if canvas.TextWidthWeight(line, size, weight) <= width {
		return line
	}
	return canvas.cutLine(line, size, weight, width)
}

// cutLine is ellipsize remembered by the canvas: the same line at the same size,
// weight and width is cut once, so a control that shows it again frame after
// frame builds no string. Lines longer than fitMemoMaxLine are cut afresh every
// time, since remembering them would hold a document open on the canvas.
func (canvas *Canvas) cutLine(line string, size, weight, width float32) string {
	if len(line) > fitMemoMaxLine {
		return canvas.ellipsize(line, size, weight, width)
	}
	hash := uint32(2166136261)
	for index := 0; index < len(line); index++ {
		hash = (hash ^ uint32(line[index])) * 16777619
	}
	// Mix every byte: integer widths usually have identical low float bits.
	// Hashing the whole word at once sends the same label at distinct widths
	// to the same slot, making an input and its popup evict each other.
	for _, dimension := range [3]float32{width, size, weight} {
		bits := math.Float32bits(dimension)
		for shift := uint(0); shift < 32; shift += 8 {
			hash = (hash ^ uint32(byte(bits>>shift))) * 16777619
		}
	}
	if canvas.fits == nil {
		canvas.fits = make([]fitEntry, fitMemoSize)
	}
	entry := &canvas.fits[hash&(fitMemoSize-1)]
	generation := canvas.FontGeneration()
	if entry.result == "" || entry.size != size || entry.weight != weight || entry.width != width || entry.line != line || entry.generation != generation {
		*entry = fitEntry{line: line, result: canvas.ellipsize(line, size, weight, width), size: size, weight: weight, width: width, generation: generation}
	}
	return entry.result
}

// fitEntry is one remembered cut line of FitLine.
type fitEntry struct {
	line, result        string
	size, weight, width float32
	generation          uint32 // the canvas's FontGeneration when result was cut
}

const (
	fitMemoSize    = 512 // a power of two: the slot is the hash masked
	fitMemoMaxLine = 512
)

func (canvas *Canvas) ellipsizeCached(line string, size, weight, width float32, cache *EllipsisCache) string {
	if cache == nil {
		// No caller-owned cache: the canvas remembers the cut instead, so a
		// control that wraps the same text every frame builds no string. This is
		// the same memo FitLine uses, and it is what keeps a text field whose
		// placeholder is wider than the field from allocating on every paint.
		return canvas.cutLine(line, size, weight, width)
	}
	generation := canvas.FontGeneration()
	if !cache.filled || cache.size != size || cache.weight != weight || cache.width != width || cache.line != line || cache.generation != generation {
		cache.line, cache.size, cache.weight, cache.width, cache.generation = line, size, weight, width, generation
		cache.result, cache.filled = canvas.ellipsize(line, size, weight, width), true
	}
	return cache.result
}

// splitLine cuts value at its first newline.
func splitLine(value string) (line, rest string) {
	line, rest = value, ""
	for index := 0; index < len(value); index++ {
		if value[index] == '\n' {
			line, rest = value[:index], value[index+1:]
			break
		}
	}
	if len(line) > 0 && line[len(line)-1] == '\r' {
		line = line[:len(line)-1]
	}
	return line, rest
}

// breakPoint is where to cut line (wider than width) so the first part fits: at the
// last UAX #14 break opportunity that fits; else, between Hangul syllables or, for a word
// wider than the line, after the last path or hyphen separator that fits, whichever
// fits more; else after the last whole grapheme cluster that fits, but never before the
// first one.
func (canvas *Canvas) breakPoint(line string, size, weight, width float32) int {
	// The longest prefix that fits, found by bisection on rune boundaries: widths
	// grow with the text, so the answer is the largest boundary whose prefix fits.
	lo, hi := 0, len(line) // line[:lo] fits (the empty prefix does), line[:hi] does not
	for hi-lo > 1 {
		mid := runeFloor(line, (lo+hi)/2)
		if mid <= lo {
			_, w := utf8.DecodeRuneInString(line[lo:])
			if mid = lo + w; mid >= hi {
				break
			}
		}
		if canvas.TextWidthWeight(trimSpaceRight(line[:mid]), size, weight) <= width {
			lo = mid
		} else {
			hi = mid
		}
	}
	limit := clusterFloor(line, lo)
	var strong, weak int
	var scanner linebreak.Scanner
	scanner.Fallback = true
	scanner.Init(line)
	for end, kind, ok := scanner.Next(); ok && end <= limit && end < len(line); end, kind, ok = scanner.Next() {
		if before := trimSpaceRight(line[:end]); before == "" || strings.Trim(before, "-") == "" {
			continue // nothing before it, or only the dashes of an option such as --verbose
		}
		switch {
		case kind == linebreak.Mandatory:
			return end
		case betweenHangul(line, end):
			weak = end
		default:
			strong = end
		}
	}
	if strong > 0 {
		return strong
	}
	var prev, prev2 rune
	seenText := false
	for index, r := range line {
		if index > limit {
			break
		}
		if index > 0 && seenText && breaksInToken(prev2, prev, r) && grapheme.Boundary(line, index) {
			weak = max(weak, index)
		}
		if r != ' ' {
			seenText = true
		}
		prev2, prev = prev, r
	}
	switch {
	case weak > 0:
		return weak
	case limit > 0:
		return limit
	}
	// Not even the first character fits: it gets the line to itself.
	return nextCluster(line, 0)
}

// betweenHangul reports whether the break at end separates two Hangul syllables or jamo.
func betweenHangul(line string, end int) bool {
	before, _ := utf8.DecodeLastRuneInString(line[:end])
	after, _ := utf8.DecodeRuneInString(line[end:])
	return isHangul(before) && isHangul(after)
}

func isHangul(r rune) bool {
	switch uniprops.LineBreak(r) {
	case uniprops.LBH2, uniprops.LBH3, uniprops.LBJL, uniprops.LBJV, uniprops.LBJT:
		return true
	}
	return false
}

// breaksInToken reports whether a long word may be cut between prev and next:
// after a path or hyphen separator that has something before it and a letter or
// digit after it.
func breaksInToken(prev2, prev, next rune) bool {
	switch prev {
	case '/', '\\', '_', '.', '&', '?', '=', '-':
	default:
		return false
	}
	if prev2 == 0 || prev2 == ' ' || prev == '-' && prev2 == '-' {
		return false
	}
	if next == ' ' || next == prev || closesLine(next) && next != '.' || nonStarter(next) {
		return false
	}
	if prev == '.' && next >= '0' && next <= '9' {
		return false // 1.5, 10.0.0.1
	}
	return true
}

// ellipsize ends line with an ellipsis, shortening it as far as it takes to
// fit width, never inside a character (a base with its marks, an emoji sequence).
func (canvas *Canvas) ellipsize(line string, size, weight, width float32) string {
	// Find, by bisection on the width of the prefix alone, the longest prefix that
	// leaves room for the ellipsis; only the winner is built as a string.
	room := width - canvas.TextWidthWeight("…", size, weight)
	lo, hi := 0, len(line) // the answer is the largest character boundary in [lo, hi] whose prefix fits
	for lo < hi {
		mid := clusterFloor(line, (lo+hi+1)/2)
		if mid <= lo { // no boundary in the upper half: try the next one after lo
			if mid = nextCluster(line, lo); mid > hi {
				break
			}
		}
		if canvas.TextWidthWeight(trimSpaceRight(line[:mid]), size, weight) <= room {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	// Shaping can make prefix plus ellipsis wider than the sum of their widths: check
	// the real string, and give up a character at a time if it does not fit.
	for end := lo; end > 0; {
		if candidate := trimSpaceRight(line[:end]) + "…"; canvas.TextWidthWeight(candidate, size, weight) <= width {
			return candidate
		}
		end = prevCluster(line, end)
	}
	return "…"
}

// runeFloor is the largest rune boundary of value at or before i.
func runeFloor(value string, i int) int {
	if i >= len(value) {
		return len(value)
	}
	for i > 0 && !utf8.RuneStart(value[i]) {
		i--
	}
	return i
}

// clusterFloor is the largest character boundary of value at or before i: a
// place where no combining mark, variation selector, joiner or second half of a
// flag follows.
func clusterFloor(value string, i int) int {
	i = runeFloor(value, i)
	for i > 0 && i < len(value) && !clusterBoundary(value, i) {
		_, w := utf8.DecodeLastRuneInString(value[:i])
		i -= w
	}
	return i
}

// GraphemeStart returns the nearest whole-character byte boundary at or before
// offset, clamped to value. It handles combining marks and emoji sequences.
func GraphemeStart(value string, offset int) int {
	return clusterFloor(value, min(max(offset, 0), len(value)))
}

// nextCluster is the end of the character that starts at i.
func nextCluster(value string, i int) int {
	if i >= len(value) {
		return len(value)
	}
	_, w := utf8.DecodeRuneInString(value[i:])
	for i += w; i < len(value) && !clusterBoundary(value, i); i += w {
		_, w = utf8.DecodeRuneInString(value[i:])
	}
	return i
}

// prevCluster is the start of the character that ends at end.
func prevCluster(value string, end int) int {
	_, w := utf8.DecodeLastRuneInString(value[:end])
	end -= w
	for end > 0 && !clusterBoundary(value, end) {
		_, w = utf8.DecodeLastRuneInString(value[:end])
		end -= w
	}
	return end
}

// clusterBoundary reports whether value may be cut at i (0 < i < len(value))
// without splitting a character as a reader sees it: UAX #29 extended grapheme
// clusters, so a mark, a variation selector, an emoji sequence, a flag or an Indic
// conjunct stays whole.
func clusterBoundary(value string, i int) bool { return grapheme.Boundary(value, i) }

func trimSpaceRight(value string) string {
	for len(value) > 0 && value[len(value)-1] == ' ' {
		value = value[:len(value)-1]
	}
	return value
}

func trimSpaceLeft(value string) string {
	for len(value) > 0 && value[0] == ' ' {
		value = value[1:]
	}
	return value
}

// closesLine reports whether r is punctuation that may not start a line.
func closesLine(r rune) bool {
	switch r {
	case '，', '。', '、', '；', '：', '！', '？', '）', '」', '』', '》', '】', '〉', '〕', '〗', '〙', '〛', '］', '｝', '．', '‥', '…', '”', '’', '〟',
		',', '.', ';', ':', '!', '?', ')', ']', '}', '%', '％':
		return true
	}
	return false
}

// nonStarter reports whether r is one of the characters Japanese and Chinese
// typesetting keeps off the start of a line besides closing punctuation: small
// kana, the prolonged sound mark, iteration marks, wave dashes and middle dots.
func nonStarter(r rune) bool {
	switch r {
	case 'ぁ', 'ぃ', 'ぅ', 'ぇ', 'ぉ', 'っ', 'ゃ', 'ゅ', 'ょ', 'ゎ', 'ゕ', 'ゖ',
		'ァ', 'ィ', 'ゥ', 'ェ', 'ォ', 'ッ', 'ャ', 'ュ', 'ョ', 'ヮ', 'ヵ', 'ヶ',
		'ー', 'ｰ', 'ゝ', 'ゞ', 'ヽ', 'ヾ', '々', '〻', '〜', '～', '・', '･', '゠', '‐', '–':
		return true
	}
	return r >= 0x31F0 && r <= 0x31FF || r == 0x3005 || r == 0x303B
}
