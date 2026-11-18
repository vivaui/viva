package ui

import "github.com/vivaui/viva/components"

type paginationDecl struct {
	items []components.PaginationItem
}

// Pagination adds a row of page items painted by components.Pagination: page
// numbers, the previous and next controls and omission markers, each with the
// application's identity. Chosen reports the item a paint is delivering a click
// on. Keep the items in the model, as a page number row is kept:
//
//	ui.Pagination(c, []components.PaginationItem{
//		{ID: prevID, Label: "Previous", Kind: components.PaginationPrevious, Disabled: m.page == 0},
//		{ID: gfx.ID{}, Label: strconv.Itoa(m.page + 1), Kind: components.PaginationPage, Current: true},
//		{ID: nextID, Label: "Next", Kind: components.PaginationNext, Disabled: m.page+1 == m.pages},
//	})
//	if id, ok := row.Chosen(); ok && id == nextID { ... }
func (f *frame) Pagination(items []components.PaginationItem) Box {
	b := f.box(kindPagination)
	n := &f.nodes[b.i]
	n.aux = int32(len(f.widgets.paginations))
	f.widgets.paginations = append(f.widgets.paginations, paginationDecl{items: items})
	return b
}

// Pagination adds a row of page items to a page.
func Pagination(c C, items []components.PaginationItem) Box {
	if c == nil || c.page == nil || c.f == nil {
		panic("ui: Pagination is for a page")
	}
	return c.f.Pagination(items)
}

// paintPagination paints the row's items into the element's rectangle, which the
// component's layout keeps inside its bounds.
func (f *frame) paintPagination(n *node) {
	d := &f.widgets.paginations[n.aux]
	bar := components.BeginPagination(f.canvas, f.theme, n.rect)
	for _, item := range d.items {
		bar.Add(item)
	}
}
