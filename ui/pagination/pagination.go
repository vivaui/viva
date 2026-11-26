// Package pagination is the opt-in page control for moving through pages of
// something: the page numbers with the previous and next controls, and
// optionally the total, a choice of page size and a field to jump to a page. A
// page keeps a State in its model and declares the row with a Spec:
//
//	m.pager = pagination.State{PageSize: 20}
//	// in the page:
//	pagination.View(c, "Pages", &m.pager, pagination.Spec{Total: len(m.rows), SizeOptions: []int{10, 20, 50}, ShowTotal: true})
//	if m.pager.Changed() {
//		// m.pager.Page or m.pager.PageSize moved: load the rows
//	}
//
// Page is the current page, counted from one; PageSize is how many rows a page
// holds. The row shows the first and last page and a window around the current
// one, with an ellipsis for what is left out. Both are brought into range for
// Total before they are read, so a Total that shrinks never leaves the user on a
// page that no longer exists. A change of page size keeps the first row of the
// page in view.
//
// ui.Pagination, below this package, is the row of items a page assembles itself;
// this package assembles them from Total and keeps the state.
//
// It is a separate package so an application that does not import it pays
// nothing for it. Keep the State in the model: it keeps its buffers across paints,
// so a warm paint allocates nothing. Do not copy a State after it has been
// declared.
package pagination

import (
	"strconv"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// Labels are the words of the row.
type Labels struct {
	// Previous and Next name the two controls; Total precedes the number of rows;
	// GoTo names the field that jumps to a page; PerPage names the page size choice.
	Previous, Next, Total, GoTo, PerPage string
}

// English is the default words.
var English = Labels{Previous: "Previous", Next: "Next", Total: "Total", GoTo: "Go to page", PerPage: "Rows per page"}

// Spec declares the row.
type Spec struct {
	// Total is the number of rows across all pages.
	Total int
	// SizeOptions, when it has more than one value, adds a choice of page size.
	SizeOptions []int
	// ShowTotal shows the total before the pages.
	ShowTotal bool
	// Jumper adds a field to type a page number into; Enter goes to it.
	Jumper bool
	// Labels are the words; English when zero.
	Labels Labels
	// JumperID gives the jump field an identity of the application's own instead of
	// the one its place gives it, for a runner that resolves the platform text
	// service by an application identity, such as a component preview.
	JumperID gfx.ID
}

// State is the position in the pages and what the row keeps between paints.
// Its zero value is the first page of ten rows.
type State struct {
	// Page is the current page, from one.
	Page int
	// PageSize is how many rows a page holds; ten when zero or less.
	PageSize int

	jump     ui.Field
	jumpOpt  [1]ui.Option
	changed  bool
	reported bool
	items    []ui.PaginationItem
	numbers  []string // the words of the page numbers, made once as they are first needed
	sizes    map[int]string
	sizePick int              // the page size the select edits, a field because its address is bound
	sizeName func(int) string // sizeLabel, made once: a method value made in each paint allocates
	total    struct {
		rows int
		text string
	}
}

// Jumper is the field behind the jump box, for a runner that binds the editor the
// platform edits to it with Field.Bind. A page does not need it.
func (s *State) Jumper() *ui.Field { return &s.jump }

// Changed reports whether the user moved to another page or page size in the
// paint that delivered it.
func (s *State) Changed() bool { return s.reported }

// pages is how many pages Total rows make.
func (s *State) pages(total int) int {
	return max((total+s.PageSize-1)/s.PageSize, 1)
}

func (s *State) number(page int) string {
	for len(s.numbers) <= page {
		s.numbers = append(s.numbers, strconv.Itoa(len(s.numbers)))
	}
	return s.numbers[page]
}

func (s *State) sizeLabel(size int) string {
	if text, ok := s.sizes[size]; ok {
		return text
	}
	if s.sizes == nil {
		s.sizes = map[int]string{}
	}
	s.sizes[size] = strconv.Itoa(size)
	return s.sizes[size]
}

// window appends the items of the page row: the first page, the last page and
// the pages around the current one, with an ellipsis where pages are left out.
func (s *State) window(c ui.C, pages int, labels Labels) {
	add := func(page int) {
		s.items = append(s.items, ui.PaginationItem{ID: ui.ID(c), Label: s.number(page), Kind: ui.PaginationPage, Current: page == s.Page})
	}
	s.items = append(s.items, ui.PaginationItem{ID: ui.ID(c), Label: labels.Previous, Kind: ui.PaginationPrevious, Disabled: s.Page <= 1})
	if pages <= 7 {
		for page := 1; page <= pages; page++ {
			add(page)
		}
	} else {
		from, to := max(s.Page-1, 2), min(s.Page+1, pages-1)
		// Near an end the window grows toward it, so the row keeps its length.
		if s.Page <= 4 {
			from, to = 2, 5
		}
		if s.Page >= pages-3 {
			from, to = pages-4, pages-1
		}
		add(1)
		if from > 2 {
			s.items = append(s.items, ui.PaginationItem{Kind: ui.PaginationEllipsis})
		}
		for page := from; page <= to; page++ {
			add(page)
		}
		if to < pages-1 {
			s.items = append(s.items, ui.PaginationItem{Kind: ui.PaginationEllipsis})
		}
		add(pages)
	}
	s.items = append(s.items, ui.PaginationItem{ID: ui.ID(c), Label: labels.Next, Kind: ui.PaginationNext, Disabled: s.Page >= pages})
}

// View declares the row at c. label names the group for assistive technology.
// Declare the view on its own, not inside a Memo.
func View(c ui.C, label string, state *State, spec Spec) ui.Box {
	if state == nil {
		panic("pagination: View needs its State")
	}
	state.reported, state.changed = state.changed, false
	labels := spec.Labels
	if labels == (Labels{}) {
		labels = English
	}
	if state.PageSize <= 0 {
		state.PageSize = 10
	}
	total := max(spec.Total, 0)
	pages := state.pages(total)
	state.Page = min(max(state.Page, 1), pages)
	theme := c.Theme()
	metrics := theme.Metrics
	row := ui.Flow(c).Items(ui.Center).Gap(metrics.GapItem * 1.5).LineGap(metrics.GapItem)
	row = row.HitWith(ui.ID(c), label, gfx.HitOptions{Role: gfx.RoleGroup, SkipFocus: true})
	row.Children(func() {
		if spec.ShowTotal {
			if state.total.text == "" || state.total.rows != total {
				state.total.rows = total
				state.total.text = labels.Total + " " + strconv.Itoa(total)
			}
			ui.Text(c, state.total.text).Muted().Shrink(0)
		}
		state.items = state.items[:0]
		state.window(c, pages, labels)
		items := ui.Pagination(c, state.items)
		if id, ok := items.Chosen(); ok {
			for _, item := range state.items {
				if item.ID != id || item.Disabled {
					continue
				}
				switch item.Kind {
				case ui.PaginationPrevious:
					state.goTo(state.Page - 1)
				case ui.PaginationNext:
					state.goTo(state.Page + 1)
				default:
					if page, err := strconv.Atoi(item.Label); err == nil {
						state.goTo(page)
					}
				}
			}
		}
		if len(spec.SizeOptions) > 1 {
			if state.sizeName == nil {
				state.sizeName = state.sizeLabel
			}
			state.sizePick = state.PageSize
			choice := ui.Select(c, &state.sizePick, spec.SizeOptions, state.sizeName).Width(88)
			choice.Label(labels.PerPage)
			if size := state.sizePick; size != state.PageSize {
				first := (state.Page - 1) * state.PageSize
				state.PageSize, state.changed = size, true
				state.Page = min(first/size+1, state.pages(total))
			}
		}
		if spec.Jumper {
			var options []ui.Option
			if !spec.JumperID.IsZero() {
				state.jumpOpt[0] = ui.WithID(spec.JumperID)
				options = state.jumpOpt[:]
			}
			field := ui.TextField(c, &state.jump, labels.GoTo, labels.GoTo, options...).Width(112)
			if field.Submitted() {
				if page, err := strconv.Atoi(state.jump.Text()); err == nil {
					state.goTo(min(max(page, 1), pages))
				}
				state.jump.SetText("")
			}
		}
	})
	return row
}

// goTo moves to a page, and notes it when that is another page.
func (s *State) goTo(page int) {
	if page != s.Page {
		s.Page, s.changed = page, true
	}
}
