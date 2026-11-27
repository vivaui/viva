package style

import "github.com/vivaui/viva/gfx"

// TreeItem describes one already-flattened visible tree row. Branches use a
// distinct DisclosureID so expanding a branch never also activates its row.
// Depth is clamped to 31 to keep malformed input from pushing content away.
type TreeItem struct {
	ID           gfx.ID
	DisclosureID gfx.ID
	Label        string
	Depth        uint8
	Branch       bool
	Expanded     bool
	Selected     bool
	Disabled     bool
	// Dragging marks the row being dragged: it keeps its chosen fill but gives
	// up its focus ring, which otherwise reads as the drop target.
	Dragging bool
	// Loading shows a spinner in place of a branch's chevron: its children are
	// being fetched. Phase is the spinner's frame, 0 to 7.
	Loading bool
	Phase   uint8
	// Background paints a full-width, square-cornered wash behind the row
	// (underneath indentation, disclosure arrow, hover and selection highlight)
	// so consecutive tinted rows form one continuous band. Zero paints nothing.
	Background gfx.Color
}
