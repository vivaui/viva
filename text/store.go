package text

import (
	"bytes"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/vivaui/viva/internal/grapheme"
)

// runeStore is the editor's text: what Range, Caret and Selection count in runes,
// held as UTF-8 in leaves of about leafBytes bytes (ADR 0092). Each leaf knows how
// many runes, UTF-16 units and newlines it holds, and running totals over the
// leaves turn a rune index into a byte offset, a UTF-16 offset or a line in a
// binary search and a walk inside one leaf. An edit rewrites the leaves it
// touches, not the text after it. The editor reads and changes the text only
// through these methods.
type runeStore struct {
	leaves []storeLeaf
	// sums[i] is the totals before leaf i, valid for i <= summed; an edit lowers
	// summed to the first leaf it touched and sums are made again from there
	// when next asked for.
	sums       []storeSums
	summed     int
	total      storeSums
	cursor     storeCursor
	scratch    []byte
	boundaries []rune
}

type storeLeaf struct {
	text  []byte
	sizes storeSums
}

type storeSums struct{ runes, bytes, units, newlines int }

// storeCursor is the last rune found: rune index at byte offset in leaf, so a walk
// that reads neighbours (word and line navigation) decodes from there.
type storeCursor struct{ leaf, index, offset int }

// leafBytes is a leaf's usual size: an edit grows a leaf to at most twice of it
// before splitting it, and a remainder under a quarter of it joins the next leaf.
const leafBytes = 2048

// newRuneStore holds value, each byte that is not UTF-8 read as U+FFFD as a
// conversion to []rune does.
func newRuneStore(value string) runeStore {
	var store runeStore
	text := []byte(value)
	if !utf8.Valid(text) {
		text = text[:0:0]
		for _, r := range value {
			text = utf8.AppendRune(text, r)
		}
	}
	store.leaves = store.split(text)
	for index := range store.leaves {
		store.total.add(store.leaves[index].sizes)
	}
	return store
}

// Len is the number of runes.
func (store *runeStore) Len() int { return store.total.runes }

// Bytes is the length of the text in UTF-8.
func (store *runeStore) Bytes() int { return store.total.bytes }

// Newlines is how many newline runes the text holds.
func (store *runeStore) Newlines() int { return store.total.newlines }

// At is the rune at index.
func (store *runeStore) At(index int) rune {
	leaf, offset := store.seek(index)
	r, _ := utf8.DecodeRune(store.leaves[leaf].text[offset:])
	return r
}

// Slice is runes [start, end), a slice of its own.
func (store *runeStore) Slice(start, end int) []rune {
	return store.AppendSlice(make([]rune, 0, end-start), start, end)
}

// AppendSlice appends runes [start, end) to dst.
func (store *runeStore) AppendSlice(dst []rune, start, end int) []rune {
	if start >= end {
		return dst
	}
	leaf, offset := store.seek(start)
	for count := end - start; count > 0; {
		text := store.leaves[leaf].text
		for offset < len(text) && count > 0 {
			r, size := utf8.DecodeRune(text[offset:])
			dst = append(dst, r)
			offset += size
			count--
		}
		leaf, offset = leaf+1, 0
	}
	return dst
}

// String is the text.
func (store *runeStore) String() string {
	var builder strings.Builder
	builder.Grow(store.total.bytes)
	store.WriteBytes(&builder, 0, store.total.bytes)
	return builder.String()
}

// WriteBytes writes the text's bytes [from, to) to builder; both are offsets of
// rune starts.
func (store *runeStore) WriteBytes(builder *strings.Builder, from, to int) {
	if from >= to {
		return
	}
	store.sum(len(store.leaves))
	leaf := sort.Search(len(store.leaves), func(i int) bool { return store.sums[i+1].bytes > from })
	for offset := from - store.sums[leaf].bytes; from < to; leaf, offset = leaf+1, 0 {
		text := store.leaves[leaf].text[offset:]
		text = text[:min(len(text), to-from)]
		builder.Write(text)
		from += len(text)
	}
}

// ByteOffset is the byte offset of rune index; RuneIndex the index of the rune
// byte offset falls in. Both clamp to the text.
func (store *runeStore) ByteOffset(index int) int {
	if index <= 0 {
		return 0
	}
	if index >= store.total.runes {
		return store.total.bytes
	}
	leaf, offset := store.seek(index)
	return store.sums[leaf].bytes + offset
}

func (store *runeStore) RuneIndex(offset int) int {
	if offset <= 0 {
		return 0
	}
	if offset >= store.total.bytes {
		return store.total.runes
	}
	store.sum(len(store.leaves))
	leaf := sort.Search(len(store.leaves), func(i int) bool { return store.sums[i+1].bytes > offset }) // the leaf holding offset
	text, index, at := store.leaves[leaf].text, store.sums[leaf].runes, store.sums[leaf].bytes
	for i := 0; i < len(text); {
		_, size := utf8.DecodeRune(text[i:])
		if at+i+size > offset {
			break
		}
		i += size
		index++
	}
	return index
}

// UTF16Offset is the number of UTF-16 units before rune index.
func (store *runeStore) UTF16Offset(index int) int {
	if index <= 0 {
		return 0
	}
	if index >= store.total.runes {
		return store.total.units
	}
	leaf, offset := store.seek(index)
	units := store.sums[leaf].units
	text := store.leaves[leaf].text
	for i := 0; i < offset; {
		r, size := utf8.DecodeRune(text[i:])
		units += utf16Len(r)
		i += size
	}
	return units
}

// RuneAtUTF16 is the index of the rune units UTF-16 units into the text, and
// whether units falls on a rune boundary rather than inside a surrogate pair (the
// index is then the rune holding the pair).
func (store *runeStore) RuneAtUTF16(units int) (index int, exact bool) {
	if units <= 0 {
		return 0, true
	}
	if units >= store.total.units {
		return store.total.runes, units == store.total.units
	}
	store.sum(len(store.leaves))
	leaf := sort.Search(len(store.leaves), func(i int) bool { return store.sums[i+1].units > units })
	index, consumed := store.sums[leaf].runes, store.sums[leaf].units
	text := store.leaves[leaf].text
	for i := 0; i < len(text) && consumed < units; {
		r, size := utf8.DecodeRune(text[i:])
		width := utf16Len(r)
		if consumed+width > units {
			return index, false
		}
		consumed += width
		index++
		i += size
	}
	return index, true
}

// LineStart is the index of the first rune of the line holding rune index: just
// after the newline before it, or 0.
func (store *runeStore) LineStart(index int) int {
	index = min(index, store.Len())
	if index <= 0 {
		return 0
	}
	store.sum(len(store.leaves))
	leaf, offset := store.seekEdge(index)
	for counted := index; ; leaf-- {
		text := store.leaves[leaf].text[:offset]
		if at := bytes.LastIndexByte(text, '\n'); at >= 0 {
			return counted - utf8.RuneCount(text[at+1:])
		}
		if leaf == 0 {
			return 0
		}
		counted = store.sums[leaf].runes
		offset = len(store.leaves[leaf-1].text)
	}
}

// LineEnd is the index of the newline at or after rune index, or the length of
// the text.
func (store *runeStore) LineEnd(index int) int {
	if index >= store.Len() {
		return store.Len()
	}
	for leaf, offset := store.seek(max(index, 0)); leaf < len(store.leaves); leaf, offset = leaf+1, 0 {
		text := store.leaves[leaf].text[offset:]
		if at := bytes.IndexByte(text, '\n'); at >= 0 {
			return index + utf8.RuneCount(text[:at])
		}
		index += utf8.RuneCount(text)
	}
	return store.Len()
}

// Line is how many newlines come before rune index: the line it is on.
func (store *runeStore) Line(index int) int {
	if index <= 0 {
		return 0
	}
	if index >= store.Len() {
		return store.Newlines()
	}
	leaf, offset := store.seek(index)
	return store.sums[leaf].newlines + bytes.Count(store.leaves[leaf].text[:offset], newline)
}

var newline = []byte{'\n'}

// Next is the grapheme cluster boundary after start; Prev the one before end.
// A cluster is found in a window of the runes around it, with the context on
// both sides the rules read, widened for a cluster longer than the window.
func (store *runeStore) Next(start int) int {
	// Between two ASCII runes other than CR LF there is always a boundary.
	if start >= 0 && start < store.Len() && store.At(start) < utf8.RuneSelf {
		if start+1 == store.Len() {
			return start + 1
		}
		if next := store.At(start + 1); next < utf8.RuneSelf && (next != '\n' || store.At(start) != '\r') {
			return start + 1
		}
	}
	for width := 64; ; width *= 4 {
		from, to := max(start-width, 0), min(start+width, store.Len())
		store.boundaries = store.AppendSlice(store.boundaries[:0], from, to)
		next := grapheme.NextRunes(store.boundaries, start-from) + from
		if next < to || to == store.Len() {
			return next
		}
	}
}

func (store *runeStore) Prev(end int) int {
	if end > 0 && end <= store.Len() && store.At(end-1) < utf8.RuneSelf {
		if end == 1 {
			return 0
		}
		if previous := store.At(end - 2); previous < utf8.RuneSelf && (previous != '\r' || store.At(end-1) != '\n') {
			return end - 1
		}
	}
	for width := 64; ; width *= 4 {
		from, to := max(end-width, 0), min(end+width, store.Len())
		store.boundaries = store.AppendSlice(store.boundaries[:0], from, to)
		previous := grapheme.PrevRunes(store.boundaries, end-from) + from
		if previous > from || from == 0 {
			return previous
		}
	}
}

// Replace makes runes [start, end) replacement.
func (store *runeStore) Replace(start, end int, replacement []rune) {
	encoded := store.scratch[:0]
	for _, r := range replacement {
		encoded = utf8.AppendRune(encoded, r)
	}
	store.scratch = encoded[:0]
	if len(store.leaves) == 0 {
		store.leaves = store.split(encoded)
		for index := range store.leaves {
			store.total.add(store.leaves[index].sizes)
		}
		return
	}
	first, startOffset := store.seekEdge(start)
	last, endOffset := store.seekEdge(end)
	if first == last {
		// An edit inside one leaf that stays a leaf's size is made in place; the
		// only leaf stays when it is emptied, ready for the next text.
		leaf := &store.leaves[first]
		if size := len(leaf.text) - (endOffset - startOffset) + len(encoded); (size > 0 || len(store.leaves) == 1) && size <= 2*leafBytes {
			removed, added := measure(leaf.text[startOffset:endOffset]), measure(encoded)
			leaf.text = slices.Replace(leaf.text, startOffset, endOffset, encoded...)
			leaf.sizes.sub(removed)
			leaf.sizes.add(added)
			store.total.sub(removed)
			store.total.add(added)
			store.edited(first, startOffset)
			return
		}
	}
	// Otherwise the leaves from the one holding start to the one holding end are
	// made again from their bytes before start, the replacement and their bytes
	// after end.
	buffer := append(store.leaves[first].text[:startOffset:startOffset], encoded...)
	buffer = append(buffer, store.leaves[last].text[endOffset:]...)
	last++
	// A small remainder joins the leaf after it.
	if len(buffer) < leafBytes/4 && last < len(store.leaves) && len(buffer)+len(store.leaves[last].text) <= 2*leafBytes {
		buffer = append(buffer, store.leaves[last].text...)
		last++
	}
	made := store.split(buffer)
	for index := first; index < last; index++ {
		store.total.sub(store.leaves[index].sizes)
	}
	for index := range made {
		store.total.add(made[index].sizes)
	}
	store.leaves = slices.Replace(store.leaves, first, last, made...)
	store.edited(first, 0)
}

// edited drops what an edit from byte offset in leaf changed: the totals after the
// leaf, and the cursor when it is past the edit.
func (store *runeStore) edited(leaf, offset int) {
	if store.summed > leaf {
		store.summed = leaf
		store.sums = store.sums[:leaf+1]
	}
	if c := store.cursor; c.leaf > leaf || c.leaf == leaf && c.offset > offset {
		store.cursor = storeCursor{leaf: -1}
	}
}

// split cuts text into leaves of about leafBytes on rune boundaries.
func (store *runeStore) split(text []byte) []storeLeaf {
	if len(text) == 0 {
		return nil
	}
	count := max((len(text)+leafBytes-1)/leafBytes, 1)
	leaves := make([]storeLeaf, 0, count)
	for len(text) > 0 {
		size := min(len(text), (len(text)+count-len(leaves)-1)/(count-len(leaves)))
		for size < len(text) && !utf8.RuneStart(text[size]) {
			size++
		}
		chunk := append([]byte(nil), text[:size]...)
		leaves = append(leaves, storeLeaf{text: chunk, sizes: measure(chunk)})
		text = text[size:]
	}
	return leaves
}

// measure counts text's runes, bytes, UTF-16 units and newlines.
func measure(text []byte) storeSums {
	sizes := storeSums{bytes: len(text)}
	for i := 0; i < len(text); {
		r, size := utf8.DecodeRune(text[i:])
		sizes.runes++
		sizes.units += utf16Len(r)
		if r == '\n' {
			sizes.newlines++
		}
		i += size
	}
	return sizes
}

func (sums *storeSums) sub(other storeSums) {
	sums.runes -= other.runes
	sums.bytes -= other.bytes
	sums.units -= other.units
	sums.newlines -= other.newlines
}

func (sums *storeSums) add(other storeSums) {
	sums.runes += other.runes
	sums.bytes += other.bytes
	sums.units += other.units
	sums.newlines += other.newlines
}

// sum makes the running totals valid up to leaf count.
func (store *runeStore) sum(count int) {
	if len(store.sums) == 0 {
		store.sums = append(store.sums, storeSums{})
		store.summed = 0
	}
	for store.summed < count {
		next := store.sums[store.summed]
		next.add(store.leaves[store.summed].sizes)
		store.sums = append(store.sums[:store.summed+1], next)
		store.summed++
	}
}

// seek finds rune index, which is inside the text: its leaf and byte offset there.
func (store *runeStore) seek(index int) (leaf, offset int) {
	c := &store.cursor
	if c.leaf >= 0 && c.leaf < len(store.leaves) && c.leaf <= store.summed && len(store.sums) > 0 {
		begin := store.sums[c.leaf].runes
		if index >= begin && index < begin+store.leaves[c.leaf].sizes.runes {
			return store.walk(c.leaf, index)
		}
	}
	store.sum(len(store.leaves))
	leaf = sort.Search(len(store.leaves), func(i int) bool { return store.sums[i+1].runes > index })
	if c.leaf != leaf {
		*c = storeCursor{leaf: leaf, index: store.sums[leaf].runes}
	}
	return store.walk(leaf, index)
}

// walk finds rune index in leaf from the cursor, the start or the end of it.
func (store *runeStore) walk(leaf, index int) (int, int) {
	c := &store.cursor
	text := store.leaves[leaf].text
	begin := store.sums[leaf].runes
	at, offset := c.index, c.offset
	if c.leaf != leaf {
		at, offset = begin, 0
	}
	if end := begin + store.leaves[leaf].sizes.runes; index-begin < abs(index-at) && index-begin <= end-index {
		at, offset = begin, 0
	} else if end-index < abs(index-at) {
		at, offset = end, len(text)
	}
	for at < index {
		_, size := utf8.DecodeRune(text[offset:])
		offset += size
		at++
	}
	for at > index {
		_, size := utf8.DecodeLastRune(text[:offset])
		offset -= size
		at--
	}
	*c = storeCursor{leaf: leaf, index: at, offset: offset}
	return leaf, offset
}

// seekEdge is seek for an edge of an edit, which may be the end of the text: the
// end of the last leaf.
func (store *runeStore) seekEdge(index int) (leaf, offset int) {
	if index >= store.total.runes {
		last := len(store.leaves) - 1
		return last, len(store.leaves[last].text)
	}
	return store.seek(index)
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
