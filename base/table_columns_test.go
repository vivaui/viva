package base

import "github.com/vivaui/viva/style"

// titledColumns is columns with the titles and the sortable flags a test keeps in
// slices of their own, one per column; a nil columns is one column of no width for
// each title.
func titledColumns(columns []style.TableColumn, titles []string, sortable []bool) []style.TableColumn {
	out := make([]style.TableColumn, max(len(columns), len(titles)))
	copy(out, columns)
	for i, title := range titles {
		out[i].Title = title
	}
	for i := range min(len(sortable), len(out)) {
		out[i].Sortable = sortable[i]
	}
	return out
}
