package base

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/style"
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

type PaginationDecl struct {
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
func (f *Frame) PaginationItems(n *Node) []style.PaginationItem {
	d := &WidgetsOf(f).paginations[n.Aux]
	return WidgetsOf(f).PaginationItems[d.start : d.start+d.count]
}

// Pagination adds a row of page items painted by components.Pagination: page
// numbers, the previous and next controls and omission markers, each with the
// application's identity. Chosen reports the item a paint is delivering a click
// on. Keep the items in the model, as a page number row is kept:
//
//	ui.Pagination(c, []ui.PaginationItem{
//		{ID: prevID, Label: "Previous", Kind: ui.PaginationPrevious, Disabled: m.page == 0},
//		{Label: strconv.Itoa(m.page + 1), Kind: ui.PaginationPage, Current: true},
//		{ID: nextID, Label: "Next", Kind: ui.PaginationNext, Disabled: m.page+1 == m.pages},
//	}

//	if id, ok := row.Chosen(); ok && id == nextID { ... }

func (f *Frame) Pagination(Items []PaginationItem) Box {
	b := f.Box(kindPagination)
	n := &f.Nodes[b.i]
	n.Aux = int32(len(WidgetsOf(f).paginations))
	start := int32(len(WidgetsOf(f).PaginationItems))
	for _, item := range Items {
		WidgetsOf(f).PaginationItems = append(WidgetsOf(f).PaginationItems, style.PaginationItem{
			ID: item.ID, Label: item.Label, Kind: style.PaginationKind(item.Kind),
			Current: item.Current, Disabled: item.Disabled,
		})
	}
	WidgetsOf(f).paginations = append(WidgetsOf(f).paginations, PaginationDecl{start: start, count: int32(len(Items))})
	return b
}
