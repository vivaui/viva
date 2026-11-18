package ui

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

// PaginationKind is what one item of a Pagination is: a page number, the
// previous or next control, or an omission marker.
type PaginationKind uint8

const (
	PaginationPage PaginationKind = iota
	PaginationPrevious
	PaginationNext
	PaginationEllipsis
)

// PaginationItem is one item a Pagination paints. An item with no identity and
// not an ellipsis paints inert, as a current page does.
type PaginationItem struct {
	ID       gfx.ID
	Label    string
	Kind     PaginationKind
	Current  bool
	Disabled bool
}

type paginationDecl struct {
	start, count int32 // this page's items in the frame's converted items
}

// Pagination adds a row of page items to a page.
func Pagination(c C, Items []PaginationItem) Box {
	if c == nil || c.page == nil || c.f == nil {
		panic("ui: Pagination is for a page")
	}
	return c.f.Pagination(Items)
}

// paginationItems is the converted items of element n (for the measure and the
// click report).
func (f *frame) PaginationItems(n *node) []components.PaginationItem {
	d := &widgetsOf(f).paginations[n.Aux]
	return widgetsOf(f).PaginationItems[d.start : d.start+d.count]
}
