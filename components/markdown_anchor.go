package components

import (
	"strings"
	"unicode/utf8"
)

// MarkdownAnchor ties one block of a laid-out document to the text it came
// from: the bytes Start to End of the text given to markdown.Document.Parse (see
// markdown.Block) and the vertical extent Top to Bottom the block took, relative
// to the origin the document is painted at.
type MarkdownAnchor struct {
	Start, End  int
	Top, Bottom float32
}

// Anchors lists the blocks of the laid-out document in reading order. The
// source ranges do not overlap and the vertical extents do not either, so both
// are sorted. A document longer than the layout's line limit has anchors only for
// the blocks that were laid out. The slice is valid until the next Layout.
func (v *MarkdownView) Anchors() []MarkdownAnchor { return v.anchors }

// YAtSource is the vertical position, relative to the origin the document is
// painted at, that corresponds to the byte offset of source, the text the
// document was parsed from. It is a position in the rendering of the line the
// offset is on, found from where the offset falls in the block's lines and in its
// line: the rendering of a wrapped paragraph or a long code line has no per-
// character geometry to be exact with, so the result is a proportional point
// inside the block. Blank lines between blocks map to the gap between them.
// Offsets before the first block give its top and offsets after the last its
// bottom. It is false when the document has no laid-out block. Its cost is
// proportional to the length of the block holding offset.
func (v *MarkdownView) YAtSource(source string, offset int) (float32, bool) {
	anchors := v.anchors
	if len(anchors) == 0 {
		return 0, false
	}
	offset = max(0, min(offset, len(source)))
	// The first block that ends at or after offset.
	low, high := 0, len(anchors)
	for low < high {
		middle := (low + high) / 2
		if anchors[middle].End < offset {
			low = middle + 1
		} else {
			high = middle
		}
	}
	if low == len(anchors) {
		return anchors[len(anchors)-1].Bottom, true
	}
	block := anchors[low]
	if offset >= block.Start {
		return block.Top + (block.Bottom-block.Top)*blockFraction(source, block, offset), true
	}
	if low == 0 {
		return block.Top, true
	}
	previous := anchors[low-1]
	t := float32(offset-previous.End) / float32(block.Start-previous.End)
	return previous.Bottom + (block.Top-previous.Bottom)*t, true
}

// SourceAtY is the inverse of YAtSource: the byte offset of source whose
// position is y, relative to the origin the document is painted at. The offset is
// on a character boundary, and inside a block it is a point along the line the
// position falls on. It is false when the document has no laid-out block.
func (v *MarkdownView) SourceAtY(source string, y float32) (int, bool) {
	anchors := v.anchors
	if len(anchors) == 0 {
		return 0, false
	}
	if y <= anchors[0].Top {
		return 0, true
	}
	// The first block that ends at or below y.
	low, high := 0, len(anchors)
	for low < high {
		middle := (low + high) / 2
		if anchors[middle].Bottom < y {
			low = middle + 1
		} else {
			high = middle
		}
	}
	if low == len(anchors) {
		return len(source), true
	}
	block := anchors[low]
	end := min(block.End, len(source))
	start := min(block.Start, end)
	if y >= block.Top {
		t := float32(0)
		if block.Bottom > block.Top {
			t = (y - block.Top) / (block.Bottom - block.Top)
		}
		return offsetAtFraction(source, start, end, t), true
	}
	previous := anchors[low-1]
	t := float32(0)
	if block.Top > previous.Bottom {
		t = (y - previous.Bottom) / (block.Top - previous.Bottom)
	}
	return characterStart(source, previous.End+int(t*float32(block.Start-previous.End))), true
}

// blockFraction is how far offset is through the lines of block, from zero at
// its start to one at its end: whole lines count equally, and the offset's place
// within its own line is by bytes.
func blockFraction(source string, block MarkdownAnchor, offset int) float32 {
	end := min(block.End, len(source))
	start := min(block.Start, end)
	offset = max(start, min(offset, end))
	lines := strings.Count(source[start:end], "\n") + 1
	before := source[start:offset]
	line := strings.Count(before, "\n")
	lineStart := start + strings.LastIndexByte(before, '\n') + 1
	lineEnd := end
	if i := strings.IndexByte(source[lineStart:end], '\n'); i >= 0 {
		lineEnd = lineStart + i
	}
	if lineEnd > lineStart && source[lineEnd-1] == '\r' {
		lineEnd--
	}
	within := float32(0)
	if lineEnd > lineStart {
		within = min(1, float32(offset-lineStart)/float32(lineEnd-lineStart))
	}
	return (float32(line) + within) / float32(lines)
}

// offsetAtFraction is the offset blockFraction maps to t, which is clamped to
// the block, rounded down to a character boundary.
func offsetAtFraction(source string, start, end int, t float32) int {
	t = max(0, min(t, 1))
	lines := strings.Count(source[start:end], "\n") + 1
	position := t * float32(lines)
	line := min(int(position), lines-1)
	within := position - float32(line)
	lineStart := start
	for skipped := 0; skipped < line; skipped++ {
		lineStart += strings.IndexByte(source[lineStart:end], '\n') + 1
	}
	lineEnd := end
	if i := strings.IndexByte(source[lineStart:end], '\n'); i >= 0 {
		lineEnd = lineStart + i
	}
	if lineEnd > lineStart && source[lineEnd-1] == '\r' {
		lineEnd--
	}
	return characterStart(source, lineStart+int(within*float32(lineEnd-lineStart)))
}

// characterStart rounds offset down to the start of the character it is in.
func characterStart(source string, offset int) int {
	offset = max(0, min(offset, len(source)))
	for offset > 0 && offset < len(source) && !utf8.RuneStart(source[offset]) {
		offset--
	}
	return offset
}
