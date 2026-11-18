package ui

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/style"
)

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
	start := int32(len(widgetsOf(f).PaginationItems))
	for _, item := range Items {
		widgetsOf(f).PaginationItems = append(widgetsOf(f).PaginationItems, style.PaginationItem{
			ID: item.ID, Label: item.Label, Kind: components.PaginationKind(item.Kind),
			Current: item.Current, Disabled: item.Disabled,
		})
	}
	widgetsOf(f).paginations = append(widgetsOf(f).paginations, paginationDecl{start: start, count: int32(len(Items))})
	return b
}

// paintPagination paints the row's items into the element's rectangle, which the
// component's layout keeps inside its bounds.
func (f *frame) paintPagination(v ControlView) {
	d := &widgetsOf(f).paginations[v.Aux]
	items := widgetsOf(f).PaginationItems[d.start : d.start+d.count]
	bar := components.BeginPagination(f.Surface, f.Palette, v.Rect)
	for _, item := range items {
		bar.Add(item)
	}
}
