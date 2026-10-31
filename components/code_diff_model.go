package components

import "strings"

// CodeDiffKind describes how one aligned row changed from the left source to
// the right source.
type CodeDiffKind uint8

const (
	CodeDiffUnchanged CodeDiffKind = iota
	CodeDiffAdded
	CodeDiffRemoved
	CodeDiffChanged
)

// CodeDiffRow aligns one line from each source. A negative index means that the
// row has no line on that side, so a pure insertion or removal keeps the other
// side aligned.
type CodeDiffRow struct {
	Kind      CodeDiffKind
	LeftLine  int
	RightLine int
}

// CodeDiffStats counts changed lines. Added describes lines present only on the
// right, while Removed describes lines present only on the left. Coarse reports
// that the comparison reached its work bound and grouped the remaining middle
// as one changed region.
type CodeDiffStats struct {
	Added   int
	Removed int
	Coarse  bool
}

// CodeDiffResult contains the split sources and their aligned rows.
type CodeDiffResult struct {
	Left  []string
	Right []string
	Rows  []CodeDiffRow
	Stats CodeDiffStats
}

const codeDiffWorkLimit = 4_000_000

type codeDiffAnchor struct {
	left, right int
}

type codeDiffBuilder struct {
	result CodeDiffResult
	work   int
	limit  int
}

// DiffCodeLines computes a bounded, deterministic line comparison. Common
// prefixes and suffixes are handled directly. Unique lines anchor recursive
// subdivisions; a region without anchors is represented as paired removals and
// additions. If the work budget or recursion depth is exhausted, the remaining
// region is emitted as one coarse change instead of blocking the caller for an
// unbounded time.
func DiffCodeLines(left, right string) CodeDiffResult {
	return diffCodeLines(left, right, codeDiffWorkLimit)
}

func diffCodeLines(left, right string, limit int) CodeDiffResult {
	builder := codeDiffBuilder{result: CodeDiffResult{
		Left:  strings.Split(left, "\n"),
		Right: strings.Split(right, "\n"),
	}, limit: limit}
	builder.result.Rows = make([]CodeDiffRow, 0, len(builder.result.Left)+len(builder.result.Right))
	builder.compare(0, len(builder.result.Left), 0, len(builder.result.Right), 0)
	return builder.result
}

func (builder *codeDiffBuilder) compare(leftStart, leftEnd, rightStart, rightEnd, depth int) {
	for leftStart < leftEnd && rightStart < rightEnd &&
		builder.result.Left[leftStart] == builder.result.Right[rightStart] {
		builder.equal(leftStart, rightStart)
		leftStart++
		rightStart++
	}
	suffix := 0
	for leftStart < leftEnd-suffix && rightStart < rightEnd-suffix &&
		builder.result.Left[leftEnd-suffix-1] == builder.result.Right[rightEnd-suffix-1] {
		suffix++
	}
	leftMiddleEnd, rightMiddleEnd := leftEnd-suffix, rightEnd-suffix
	switch {
	case leftStart == leftMiddleEnd:
		builder.addedRange(rightStart, rightMiddleEnd)
	case rightStart == rightMiddleEnd:
		builder.removedRange(leftStart, leftMiddleEnd)
	default:
		builder.compareMiddle(leftStart, leftMiddleEnd, rightStart, rightMiddleEnd, depth)
	}
	for index := suffix; index > 0; index-- {
		builder.equal(leftEnd-index, rightEnd-index)
	}
}

func (builder *codeDiffBuilder) compareMiddle(leftStart, leftEnd, rightStart, rightEnd, depth int) {
	if depth >= 128 || builder.spend(leftEnd-leftStart+rightEnd-rightStart) {
		builder.coarseChange(leftStart, leftEnd, rightStart, rightEnd)
		return
	}
	anchors, ok := builder.uniqueAnchors(leftStart, leftEnd, rightStart, rightEnd)
	if !ok {
		builder.coarseChange(leftStart, leftEnd, rightStart, rightEnd)
		return
	}
	if len(anchors) == 0 {
		builder.changedRange(leftStart, leftEnd, rightStart, rightEnd)
		return
	}
	lastLeft, lastRight := leftStart, rightStart
	for _, anchor := range anchors {
		builder.compare(lastLeft, anchor.left, lastRight, anchor.right, depth+1)
		builder.equal(anchor.left, anchor.right)
		lastLeft, lastRight = anchor.left+1, anchor.right+1
	}
	builder.compare(lastLeft, leftEnd, lastRight, rightEnd, depth+1)
}

// uniqueAnchors returns the longest increasing sequence of lines that occur
// exactly once in both current ranges. ok is false when the work bound is hit.
func (builder *codeDiffBuilder) uniqueAnchors(leftStart, leftEnd, rightStart, rightEnd int) ([]codeDiffAnchor, bool) {
	leftPositions := make(map[string]int, leftEnd-leftStart)
	for index := leftStart; index < leftEnd; index++ {
		if builder.spend(1) {
			return nil, false
		}
		line := builder.result.Left[index]
		if previous, exists := leftPositions[line]; exists {
			leftPositions[line] = -1
			if previous == -1 {
				continue
			}
		} else {
			leftPositions[line] = index
		}
	}
	rightPositions := make(map[string]int, rightEnd-rightStart)
	for index := rightStart; index < rightEnd; index++ {
		if builder.spend(1) {
			return nil, false
		}
		line := builder.result.Right[index]
		if _, exists := rightPositions[line]; exists {
			rightPositions[line] = -1
		} else {
			rightPositions[line] = index
		}
	}
	pairs := make([]codeDiffAnchor, 0, min(len(leftPositions), len(rightPositions)))
	for index := leftStart; index < leftEnd; index++ {
		if builder.spend(1) {
			return nil, false
		}
		line := builder.result.Left[index]
		leftPosition, leftUnique := leftPositions[line]
		if !leftUnique || leftPosition < 0 {
			continue
		}
		rightPosition, rightUnique := rightPositions[line]
		if rightUnique && rightPosition >= 0 {
			pairs = append(pairs, codeDiffAnchor{left: index, right: rightPosition})
		}
	}
	return builder.longestIncreasing(pairs)
}

// longestIncreasing keeps the LIS of right-side positions. The input is already
// ordered by left-side position.
func (builder *codeDiffBuilder) longestIncreasing(pairs []codeDiffAnchor) ([]codeDiffAnchor, bool) {
	if len(pairs) == 0 {
		return nil, true
	}
	// The slice is already sorted by left. Keep track of both the smallest right
	// endpoint for each LIS length and the pair selected at that length.
	tails := make([]codeDiffAnchor, 0, len(pairs))
	previous := make([]int, len(pairs))
	for index := range previous {
		previous[index] = -1
	}
	for index, pair := range pairs {
		if builder.spend(1) {
			return nil, false
		}
		low := 0
		high := len(tails)
		for low < high {
			middle := (low + high) / 2
			if tails[middle].right < pair.right {
				low = middle + 1
			} else {
				high = middle
			}
		}
		if low > 0 {
			previous[index] = tails[low-1].left
		}
		if low == len(tails) {
			tails = append(tails, codeDiffAnchor{left: index, right: pair.right})
		} else {
			tails[low] = codeDiffAnchor{left: index, right: pair.right}
		}
	}
	result := make([]codeDiffAnchor, len(tails))
	cursor := tails[len(tails)-1].left
	for index := len(result) - 1; index >= 0; index-- {
		result[index] = pairs[cursor]
		cursor = previous[cursor]
	}
	return result, true
}

func (builder *codeDiffBuilder) spend(work int) bool {
	if work <= 0 {
		return false
	}
	builder.work += work
	if builder.work <= builder.limit {
		return false
	}
	builder.result.Stats.Coarse = true
	return true
}

func (builder *codeDiffBuilder) equal(left, right int) {
	builder.result.Rows = append(builder.result.Rows, CodeDiffRow{
		Kind: CodeDiffUnchanged, LeftLine: left, RightLine: right,
	})
}

func (builder *codeDiffBuilder) addedRange(start, end int) {
	for index := start; index < end; index++ {
		builder.result.Rows = append(builder.result.Rows, CodeDiffRow{
			Kind: CodeDiffAdded, LeftLine: -1, RightLine: index,
		})
		builder.result.Stats.Added++
	}
}

func (builder *codeDiffBuilder) removedRange(start, end int) {
	for index := start; index < end; index++ {
		builder.result.Rows = append(builder.result.Rows, CodeDiffRow{
			Kind: CodeDiffRemoved, LeftLine: index, RightLine: -1,
		})
		builder.result.Stats.Removed++
	}
}

func (builder *codeDiffBuilder) changedRange(leftStart, leftEnd, rightStart, rightEnd int) {
	rows := max(leftEnd-leftStart, rightEnd-rightStart)
	for index := 0; index < rows; index++ {
		left, right := -1, -1
		kind := CodeDiffChanged
		if leftStart+index < leftEnd {
			left = leftStart + index
		}
		if rightStart+index < rightEnd {
			right = rightStart + index
		}
		switch {
		case left < 0:
			kind = CodeDiffAdded
			builder.result.Stats.Added++
		case right < 0:
			kind = CodeDiffRemoved
			builder.result.Stats.Removed++
		case builder.result.Left[left] == builder.result.Right[right]:
			// A hunk can contain lines that the unique-anchor pass could not use
			// as anchors, for example repeated YAML keys. Once the two sides are
			// paired in source order, identical lines are not changes and must not
			// be counted or coloured as such.
			kind = CodeDiffUnchanged
		default:
			builder.result.Stats.Added++
			builder.result.Stats.Removed++
		}
		builder.result.Rows = append(builder.result.Rows, CodeDiffRow{Kind: kind, LeftLine: left, RightLine: right})
	}
}

// coarseChange emits a region after a work or depth bound. The result remains
// useful and always pairs lines in source order.
func (builder *codeDiffBuilder) coarseChange(leftStart, leftEnd, rightStart, rightEnd int) {
	builder.result.Stats.Coarse = true
	builder.changedRange(leftStart, leftEnd, rightStart, rightEnd)
}
