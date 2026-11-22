package base

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

// trimFlexCapacity gives back a slice's backing array when it is far larger
// than its content. A page that once had a large table should not keep that
// capacity for the view's whole life.
func trimFlexCapacity[T any](Items []T) []T {
	const minCapacity = 4096
	const ratio = 4
	if cap(Items) <= minCapacity || len(Items)*ratio >= cap(Items) {
		return Items
	}
	next := make([]T, len(Items), max(len(Items)*2, minCapacity/64))
	copy(next, Items)
	return next
}
