package ui_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var testIcon = gfx.Icon{ViewBox: 24, Stroke: 2, Commands: []gfx.PathCommand{gfx.MoveTo(6, 6), gfx.LineTo(18, 18)}}

// A field's Icon takes room before its text and its Action is a button of its own
// at the end: a press on it is the action's, not the field's, and typing still
// reaches the field.
func TestFieldIconAndAction(t *testing.T) {
	query, clears := "", 0
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(16).Items(ui.Start).Children(func() {
			field := ui.TextField(c, &query, "Search", "Search hosts").Width(240).Icon(testIcon).Action(testIcon, "Clear")
			if field.ActionClicked() {
				clears++
				query = ""
			}
		})
	})
	d.click("Search")
	d.typeText("web")
	if query != "web" {
		t.Fatalf("typing reached %q", query)
	}
	field, clear := d.hit("Search").Bounds, d.hit("Clear").Bounds
	if clear.X < field.X+field.Width/2 || clear.X+clear.Width > field.X+field.Width {
		t.Fatalf("the action %v is not at the end of the field %v", clear, field)
	}
	if text := d.driver.Snapshot().Editor; text == nil || text.Bounds.X <= field.X+16 {
		t.Fatalf("the text does not start after the icon: editor %+v, field %v", text, field)
	}
	d.click("Clear")
	if clears != 1 || query != "" {
		t.Fatalf("clears %d, query %q", clears, query)
	}
}

func BenchmarkFieldAddons(b *testing.B) {
	query := "web"
	page, canvas := ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas()
	view := func(c ui.C) {
		ui.TextField(c, &query, "Search", "").Width(240).Icon(testIcon).Action(testIcon, "Clear")
	}
	b.ReportAllocs()
	for b.Loop() {
		drawPage(page, canvas, view)
	}
}
