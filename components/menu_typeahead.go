package components

import "github.com/vivaui/viva/base"

// MenuTypeaheadReset is how long a typed prefix lasts before a new one starts.
const MenuTypeaheadReset = base.MenuTypeaheadReset

// MenuTypeahead is caller-owned keyboard text matching for a menu or a list.
type MenuTypeahead = base.MenuTypeahead

// MenuTypeaheadItem is one choice for TypeItems.
type MenuTypeaheadItem = base.MenuTypeaheadItem

// MenuTypeaheadRows is a list for TypeRows, read a row at a time.
type MenuTypeaheadRows = base.MenuTypeaheadRows

// MenuRows is what Type matches against; MenuScroll implements it.
type MenuRows = base.MenuRows
