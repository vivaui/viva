// A table of servers: a fixed header, sortable columns, one selected row and a
// body that scrolls under the header when the window is short.
package main

import (
	"log"
	"sort"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

type model struct {
	table ui.TableState
	rows  ui.TableRows
}

func newModel() *model {
	return &model{rows: ui.TableRows{
		{"web-01", "Running", "eu-west"},
		{"web-02", "Running", "us-east"},
		{"db-primary", "Degraded", "eu-west"},
		{"cache", "Stopped", "us-east"},
	}}
}

// page declares the table. A header click sorts the rows; a row click and the
// arrow keys select a row, and the table keeps the selection and the scroll
// position itself.
func (m *model) page(c ui.C) {
	ui.Column(c).Fill().Pad(24).Gap(12).Children(func() {
		ui.Text(c, "Servers").FontSize(20).Bold()
		table := ui.Table(c, &m.table, ui.TableSpec{
			Source:  &m.rows,
			Striped: true,
			Columns: []ui.TableColumn{
				{Title: "Name", Width: 200, Sortable: true},
				{Title: "State", Width: 120, Sortable: true},
				{Title: "Region", Sortable: true},
			},
		})
		if column, ok := table.HeaderClicked(); ok {
			m.table.SortBy(column)
			m.sortRows()
		}
		if row := m.table.Selected(); row >= 0 {
			ui.Text(c, "Selected: "+m.rows[row][0]).Muted()
		}
	})
}

// sortRows orders the rows by the column the table reports as sorted. The table
// paints the arrow; the application owns the data and its order.
func (m *model) sortRows() {
	column, descending, ok := m.table.Sort()
	if !ok {
		return
	}
	sort.SliceStable(m.rows, func(i, j int) bool {
		less := m.rows[i][column] < m.rows[j][column]
		if descending {
			return !less
		}
		return less
	})
}

func main() {
	if err := ui.Run(app.Config{Title: "Servers", Width: 720, Height: 480}, ui.PageConfig{View: newModel().page}); err != nil {
		log.Fatal(err)
	}
}
