package ui

// scratch is the memory the layout borrows beyond the node tree, reused from frame to
// frame so a warm page allocates nothing.
type scratch struct {
	abs   []int32   // the absolute elements, in the order declared
	rows  []float32 // the height of each row of the Grid being laid out
	rowAt []float32 // the offset of each of its rows
	cells []bool    // which cells of a Grid row-spanning elements have taken
}

func (s *scratch) reset() { s.abs = s.abs[:0] }

// trimFlexCapacity gives back a slice's backing array when it is far larger
// than its content. A page that once had a large table should not keep that
// capacity for the View's whole life.
func trimFlexCapacity[T any](items []T) []T {
	const minCapacity = 4096
	const ratio = 4
	if cap(items) <= minCapacity || len(items)*ratio >= cap(items) {
		return items
	}
	next := make([]T, len(items), max(len(items)*2, minCapacity/64))
	copy(next, items)
	return next
}
