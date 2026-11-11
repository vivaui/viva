package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var paginationComponent = Component{
	ID: "pagination", Title: "Pagination", Category: "Navigation", Summary: "调用方持有当前页的紧凑分页控件。",
	paint: (*State).paintPagination, initialize: (*State).initializePagination,
	Examples: []Example{
		{ID: "default", Title: "分页导航", Description: "上一页、页码与下一页共同更新当前页。", Snippet: `pages := tree.Sub(page, tree.Name("page"))
pagination := components.BeginPagination(canvas, theme, bounds)
pagination.AddPrevious(tree.ID(page, "previous"), "Previous", current == 1)
pagination.AddPage(pages.At(0), "1", current == 1)
pagination.AddPage(pages.At(1), "2", current == 2)
pagination.AddPage(pages.At(2), "3", current == 3)
pagination.AddEllipsis()
pagination.AddPage(pages.At(4), "8", current == 8)
pagination.AddNext(tree.ID(page, "next"), "Next", current == 8)
if index, ok := canvas.ClickedIn(pages); ok {
    current = int32(index + 1)
    if index == 4 {
        current = 8
    }
}`},
		{ID: "boundaries", Title: "边界状态", Description: "第一页禁用上一页，并保留省略号。", Snippet: `pages := tree.Sub(page, tree.Name("page"))
current := int32(1)
pagination := components.BeginPagination(canvas, theme, bounds)
pagination.AddPrevious(tree.ID(page, "previous"), "Previous", current == 1)
pagination.AddPage(pages.At(0), "1", current == 1)
pagination.AddPage(pages.At(1), "2", current == 2)
pagination.AddPage(pages.At(2), "3", current == 3)
pagination.AddEllipsis()
pagination.AddPage(pages.At(4), "8", current == 8)
pagination.AddNext(tree.ID(page, "next"), "Next", current == 8)`},
	},
}

var (
	paginationUI       = tree.Feature("pagination")
	paginationPrevious = tree.ID(paginationUI, "previous")
	paginationNext     = tree.ID(paginationUI, "next")
	// paginationCurrent is the lone page button of the compact bar; the current
	// page is not interactive, so a click on it changes nothing.
	paginationCurrent = tree.ID(paginationUI, "current")
	// paginationPage's item i is the i-th visible page button.
	paginationPage = tree.Sub(paginationUI, tree.Name("page"))
)

func (state *State) initializePagination() {
	if state.exampleID == "boundaries" {
		state.values[0] = 1
		return
	}
	state.values[0] = 3
}

func (state *State) paintPagination(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	page := state.values[0]
	var bar ui.Box
	state.drawPage(canvas, theme, size, 620, "Results", func(c ui.C) {
		ui.Text(c, paginationPageStatus(page)).FontSize(theme.Metrics.FontSizeS).Muted()
		bar = ui.Slot(c).Height(theme.Metrics.ControlHeight)
		caption(c, state.Text("窄视口自动减少页码；当前页保持非交互。", "Narrow viewports show fewer pages; the current page stays non-interactive."))
	})

	bounds := bar.Rect()
	pagination := components.BeginPagination(canvas, theme, bounds)
	compact := bounds.Width < 430
	pagination.AddPrevious(paginationPrevious, "Previous", page == 1)
	if compact {
		pagination.AddPage(paginationCurrent, paginationPageLabel(page), true)
	} else {
		for visiblePage := int32(1); visiblePage <= 3; visiblePage++ {
			pagination.AddPage(paginationPage.At(int(visiblePage-1)), paginationPageLabel(visiblePage), page == visiblePage)
		}
		pagination.AddEllipsis()
		pagination.AddPage(paginationPage.At(4), "8", page == 8)
	}
	pagination.AddNext(paginationNext, "Next", page == 8)

	if canvas.Clicked(paginationPrevious) && page > 1 {
		state.values[0]--
	}
	if canvas.Clicked(paginationNext) && page < 8 {
		state.values[0]++
	}
	if index, ok := canvas.ClickedIn(paginationPage); ok {
		state.values[0] = int32(index) + 1
		if index == 4 {
			state.values[0] = 8
		}
	}
}

func paginationPageLabel(page int32) string {
	switch page {
	case 1:
		return "1"
	case 2:
		return "2"
	case 3:
		return "3"
	case 4:
		return "4"
	case 5:
		return "5"
	case 6:
		return "6"
	case 7:
		return "7"
	default:
		return "8"
	}
}

func paginationPageStatus(page int32) string {
	switch page {
	case 1:
		return "Page 1 of 8"
	case 2:
		return "Page 2 of 8"
	case 3:
		return "Page 3 of 8"
	case 4:
		return "Page 4 of 8"
	case 5:
		return "Page 5 of 8"
	case 6:
		return "Page 6 of 8"
	case 7:
		return "Page 7 of 8"
	default:
		return "Page 8 of 8"
	}
}
