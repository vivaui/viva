package components

import (
	"github.com/vivaui/viva/base"
	"github.com/vivaui/viva/gfx"
)

// TreeNavigationItem describes one visible row of a caller-owned, flattened
// tree. Depth must describe the actual hierarchy, not its visual indentation.
type TreeNavigationItem = base.TreeNavigationItem

// TreeNavigationAction tells the caller what to do with the current branch.
type TreeNavigationAction = base.TreeNavigationAction

const (
	TreeNavigationNone     = base.TreeNavigationNone
	TreeNavigationMove     = base.TreeNavigationMove
	TreeNavigationExpand   = base.TreeNavigationExpand
	TreeNavigationCollapse = base.TreeNavigationCollapse
)

// NavigateTree applies desktop tree keyboard navigation to visible rows. It is
// base.NavigateTree; the implementation lives in the engine with the other
// input behaviour.
func NavigateTree(items []TreeNavigationItem, current int, key gfx.Key, pageRows int) (int, TreeNavigationAction, bool) {
	return base.NavigateTree(items, current, key, pageRows)
}
