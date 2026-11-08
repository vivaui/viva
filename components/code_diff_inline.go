package components

import "github.com/vivaui/viva/text"

// CodeDiffInline marks one changed grapheme range in a paired row. Offsets are
// UTF-8 byte offsets within the source lines; zero-width ranges are insertions.
type CodeDiffInline struct {
	Row                  int
	LeftStart, LeftEnd   int
	RightStart, RightEnd int
}

const codeDiffMaxInlineClusters = 128

func codeDiffBoundaries(line string, boundaries []int) []int {
	boundaries = append(boundaries[:0], 0)
	for at := 0; at < len(line) && len(boundaries) <= codeDiffMaxInlineClusters; {
		at = text.NextGraphemeBoundary(line, at)
		boundaries = append(boundaries, at)
	}
	return boundaries
}

func diffCodeInline(dst []CodeDiffInline, row int, left, right string, budget *int, scratch *[]uint16) []CodeDiffInline {
	leftAt, rightAt := 0, 0
	for leftAt < len(left) && rightAt < len(right) {
		leftEnd, rightEnd := text.NextGraphemeBoundary(left, leftAt), text.NextGraphemeBoundary(right, rightAt)
		if left[leftAt:leftEnd] != right[rightAt:rightEnd] {
			break
		}
		leftAt, rightAt = leftEnd, rightEnd
	}
	leftEnd, rightEnd := len(left), len(right)
	for leftEnd > leftAt && rightEnd > rightAt {
		leftStart, rightStart := text.PreviousGraphemeBoundary(left, leftEnd), text.PreviousGraphemeBoundary(right, rightEnd)
		if left[leftStart:leftEnd] != right[rightStart:rightEnd] {
			break
		}
		leftEnd, rightEnd = leftStart, rightStart
	}
	if leftAt == leftEnd && rightAt == rightEnd {
		return dst
	}
	leftMiddle, rightMiddle := left[leftAt:leftEnd], right[rightAt:rightEnd]
	if *budget <= 0 {
		return append(dst, CodeDiffInline{Row: row, LeftStart: leftAt, LeftEnd: leftEnd, RightStart: rightAt, RightEnd: rightEnd})
	}
	var leftStorage, rightStorage [codeDiffMaxInlineClusters + 1]int
	leftBounds := codeDiffBoundaries(leftMiddle, leftStorage[:0])
	rightBounds := codeDiffBoundaries(rightMiddle, rightStorage[:0])
	leftCount, rightCount := len(leftBounds)-1, len(rightBounds)-1
	work := (leftCount + 1) * (rightCount + 1)
	if leftBounds[leftCount] != len(leftMiddle) || rightBounds[rightCount] != len(rightMiddle) || work > *budget {
		return append(dst, CodeDiffInline{Row: row, LeftStart: leftAt, LeftEnd: leftEnd, RightStart: rightAt, RightEnd: rightEnd})
	}
	*budget -= work
	stride := rightCount + 1
	if cap(*scratch) < work {
		*scratch = make([]uint16, work)
	}
	lengths := (*scratch)[:work]
	clear(lengths)
	for leftIndex := leftCount - 1; leftIndex >= 0; leftIndex-- {
		for rightIndex := rightCount - 1; rightIndex >= 0; rightIndex-- {
			cell := leftIndex*stride + rightIndex
			if leftMiddle[leftBounds[leftIndex]:leftBounds[leftIndex+1]] == rightMiddle[rightBounds[rightIndex]:rightBounds[rightIndex+1]] {
				lengths[cell] = lengths[cell+stride+1] + 1
			} else {
				lengths[cell] = max(lengths[cell+stride], lengths[cell+1])
			}
		}
	}
	startLeft, startRight := 0, 0
	for leftIndex, rightIndex := 0, 0; leftIndex < leftCount || rightIndex < rightCount; {
		if leftIndex < leftCount && rightIndex < rightCount &&
			leftMiddle[leftBounds[leftIndex]:leftBounds[leftIndex+1]] == rightMiddle[rightBounds[rightIndex]:rightBounds[rightIndex+1]] {
			if startLeft < leftIndex || startRight < rightIndex {
				dst = append(dst, CodeDiffInline{Row: row,
					LeftStart: leftAt + leftBounds[startLeft], LeftEnd: leftAt + leftBounds[leftIndex],
					RightStart: rightAt + rightBounds[startRight], RightEnd: rightAt + rightBounds[rightIndex]})
			}
			leftIndex++
			rightIndex++
			startLeft, startRight = leftIndex, rightIndex
		} else if rightIndex == rightCount || leftIndex < leftCount && lengths[(leftIndex+1)*stride+rightIndex] >= lengths[leftIndex*stride+rightIndex+1] {
			leftIndex++
		} else {
			rightIndex++
		}
	}
	if startLeft < leftCount || startRight < rightCount {
		dst = append(dst, CodeDiffInline{Row: row,
			LeftStart: leftAt + leftBounds[startLeft], LeftEnd: leftEnd,
			RightStart: rightAt + rightBounds[startRight], RightEnd: rightEnd})
	}
	return dst
}
