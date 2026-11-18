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

// Pagination adds a row of page items painted by components.Pagination: page
// numbers, the previous and next controls and omission markers, each with the
// application's identity. Chosen reports the item a paint is delivering a click
// on. Keep the items in the model, as a page number row is kept:
//
//	ui.Pagination(c, []ui.PaginationItem{
//		{ID: prevID, Label: "Previous", Kind: ui.PaginationPrevious, Disabled: m.page == 0},
//		{Label: strconv.Itoa(m.page + 1), Kind: ui.PaginationPage, Current: true},
//		{ID: nextID, Label: "Next", Kind: ui.PaginationNext, Disabled: m.page+1 == m.pages},
//	})
//	if id, ok := row.Chosen(); ok && id == nextID { ... }
func (f *frame) Pagination(Items []PaginationItem) Box {
	b := f.Box(kindPagination)
	n := &f.Nodes[b.i]
	n.Aux = int32(len(widgetsOf(f).paginations))
	start := int32(len(widgetsOf(f).paginationItems))
	for _, item := range Items {
		widgetsOf(f).paginationItems = append(widgetsOf(f).paginationItems, components.PaginationItem{
			ID: item.ID, Label: item.Label, Kind: components.PaginationKind(item.Kind),
			Current: item.Current, Disabled: item.Disabled,
		})
	}
	widgetsOf(f).paginations = append(widgetsOf(f).paginations, paginationDecl{start: start, count: int32(len(Items))})
	return b
}

// Pagination adds a row of page items to a page.
func Pagination(c C, Items []PaginationItem) Box {
	if c == nil || c.page == nil || c.f == nil {
		panic("ui: Pagination is for a page")
	}
	return c.f.Pagination(Items)
}

// paintPagination paints the row's items into the element's rectangle, which the
// component's layout keeps inside its bounds.
func (f *frame) paintPagination(v ControlView) {
	d := &widgetsOf(f).paginations[v.Aux]
	items := widgetsOf(f).paginationItems[d.start : d.start+d.count]
	bar := components.BeginPagination(f.Surface, f.Palette, v.Rect)
	for _, item := range items {
		bar.Add(item)
	}
}

// paginationItems is the converted items of element n (for the measure and the
// click report).
func (f *frame) paginationItems(n *node) []components.PaginationItem {
	d := &widgetsOf(f).paginations[n.Aux]
	return widgetsOf(f).paginationItems[d.start : d.start+d.count]
}
