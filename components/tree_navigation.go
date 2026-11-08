package components

import "github.com/vivaui/viva/ui"

// TreeNavigationItem describes one visible row of a caller-owned, flattened
// tree. Depth must describe the actual hierarchy, not its visual indentation.
type TreeNavigationItem struct {
	Depth    uint8
	Branch   bool
	Expanded bool
	Disabled bool
}

// TreeNavigationAction tells the caller what to do with the current branch.
// Expansion and selection remain application-owned.
type TreeNavigationAction uint8

const (
	TreeNavigationNone TreeNavigationAction = iota
	TreeNavigationMove
	TreeNavigationExpand
	TreeNavigationCollapse
)

// NavigateTree applies desktop tree keyboard navigation to visible rows.
// Arrow keys move among enabled rows; Right opens a collapsed branch or moves
// to its first child, and Left closes an open branch or moves to its parent.
// Home, End and Page keys navigate the visible rows without wrapping. pageRows
// is the number of rows in one viewport. An accepted key returns true even
// when navigation is already at an edge.
func NavigateTree(items []TreeNavigationItem, current int, key ui.Key, pageRows int) (int, TreeNavigationAction, bool) {
	switch key {
	case ui.KeyUp, ui.KeyDown, ui.KeyHome, ui.KeyEnd, ui.KeyPageUp, ui.KeyPageDown, ui.KeyLeft, ui.KeyRight:
	default:
		return current, TreeNavigationNone, false
	}
	if current < 0 || current >= len(items) || items[current].Disabled {
		return current, TreeNavigationNone, false
	}
	item := items[current]
	if key == ui.KeyRight {
		if !item.Branch {
			return current, TreeNavigationNone, true
		}
		if !item.Expanded {
			return current, TreeNavigationExpand, true
		}
		for next := current + 1; next < len(items) && items[next].Depth > item.Depth; next++ {
			if int(items[next].Depth) == int(item.Depth)+1 && !items[next].Disabled {
				return next, TreeNavigationMove, true
			}
		}
		return current, TreeNavigationNone, true
	}
	if key == ui.KeyLeft {
		if item.Branch && item.Expanded {
			return current, TreeNavigationCollapse, true
		}
		for index := current - 1; index >= 0; index-- {
			if items[index].Depth < item.Depth {
				if !items[index].Disabled {
					return index, TreeNavigationMove, true
				}
				break
			}
		}
		return current, TreeNavigationNone, true
	}
	if pageRows < 1 {
		pageRows = 1
	}
	step := 0
	switch key {
	case ui.KeyUp:
		step = -1
	case ui.KeyDown:
		step = 1
	case ui.KeyPageUp:
		step = -pageRows
	case ui.KeyPageDown:
		step = pageRows
	case ui.KeyHome:
		step = -current
	case ui.KeyEnd:
		step = len(items) - 1 - current
	}
	target := max(0, min(len(items)-1, current+step))
	if step > 0 {
		for target < len(items)-1 && items[target].Disabled {
			target++
		}
		if items[target].Disabled {
			for target > current && items[target].Disabled {
				target--
			}
		}
	} else if step < 0 {
		for target > 0 && items[target].Disabled {
			target--
		}
		if items[target].Disabled {
			for target < current && items[target].Disabled {
				target++
			}
		}
	}
	if target != current {
		return target, TreeNavigationMove, true
	}
	return current, TreeNavigationNone, true
}
