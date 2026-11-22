package base

import "unsafe"

// scratch is the memory the layout borrows beyond the node tree, reused from frame to
// frame so a warm page allocates nothing.
type scratch struct {
	theme Theme     // the theme of the element a recipe is applied to
	abs   []int32   // the absolute elements, in the order declared
	rows  []float32 // the height of each row of the Grid being laid out
	rowAt []float32 // the offset of each of its rows
	cells []bool    // which cells of a Grid row-spanning elements have taken
}

func (s *scratch) reset() { s.abs = s.abs[:0] }

// frameTails is the length each pointer-holding slice of a Frame had when the
// last frame ended.
type frameTails struct {
	nodes, exts, images, lines, addons, states, stops, wraps, ellipses int
}

// releaseTail clears items[len(items):*end], what the last frame held past this
// one's length, and records this length for the next. Everything past it was
// cleared before, so the cost is what the slice shrank by, not its capacity.
func releaseTail[T any](items []T, end *int) {
	if *end > len(items) && *end <= cap(items) {
		clear(items[len(items):*end])
	}
	*end = len(items)
}

// trimFlexCapacity gives back a slice's backing array when it is far larger
// than its content. A page that once had a large table should not keep that
// capacity for the view's whole life. Large is counted in elements and in
// bytes: 4096 small ones, or 64 KiB of large ones such as a 776-byte Extra.
func trimFlexCapacity[T any](Items []T) []T {
	const minCapacity, minBytes = 4096, 64 << 10
	const ratio = 4
	var zero T
	large := cap(Items) > minCapacity || uintptr(cap(Items))*unsafe.Sizeof(zero) > minBytes
	if !large || len(Items)*ratio >= cap(Items) {
		return Items
	}
	next := make([]T, len(Items), max(len(Items)*2, minCapacity/64))
	copy(next, Items)
	return next
}
