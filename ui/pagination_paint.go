package ui

import (
	"github.com/vivaui/viva/components"
)

// paintPagination paints the row's items into the element's rectangle, which the
// component's layout keeps inside its bounds.
func paintPagination(f *Frame, v ControlView) {
	d := &WidgetsOf(f).GetPaginations()[v.Aux]
	items := WidgetsOf(f).PaginationItems[d.GetStart() : d.GetStart()+d.GetCount()]
	bar := components.BeginPagination(f.Surface, f.Palette, v.Rect)
	for _, item := range items {
		bar.Add(item)
	}
}
