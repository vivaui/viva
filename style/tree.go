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
}
