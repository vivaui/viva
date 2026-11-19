package codeeditor_test

import (
	"fmt"
	"testing"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/codeeditor"
)

// The page control adds identity and input routing, not pixels: painting the
// same snapshot through the component and through codeeditor.View produces the
// same rectangles and labels.
func TestPageViewPaintsTheSameAsTheComponent(t *testing.T) {
	snapshot := components.CodeEditorSnapshot{
		Value: source, LineNumbers: true, CurrentLine: true, MatchBrackets: true,
		Language: "go", TabWidth: 4, Focused: true, ShowCaret: false,
		SelectionStart: 2, SelectionEnd: 8, Caret: 8,
	}
	var direct, wrapped *gfx.Canvas
	directPage := ui.NewPage(ui.PageConfig{Theme: testTheme(), View: func(c ui.C) {
		ui.Slot(c).Fill().Draw(func(d *ui.DrawContext) {
			var view components.CodeEditor
			view.Paint(d.Canvas, *d.Theme, d.Rect, snapshot)
		})
	}})
	var page codeeditor.Editor
	page.Snapshot = snapshot
	wrappedPage := ui.NewPage(ui.PageConfig{Theme: testTheme(), View: func(c ui.C) {
		codeeditor.View(c, "Source", &page).Fill()
	}})
	direct, wrapped = new(gfx.Canvas), new(gfx.Canvas)
	drawPage(directPage, direct, func(c ui.C) {
		ui.Slot(c).Fill().Draw(func(d *ui.DrawContext) {
			var view components.CodeEditor
			view.Paint(d.Canvas, *d.Theme, d.Rect, snapshot)
		})
	})
	drawPage(wrappedPage, wrapped, func(c ui.C) {
		codeeditor.View(c, "Source", &page).Fill()
	})
	if got, want := describeCanvas(wrapped), describeCanvas(direct); got != want {
		t.Fatalf("page view and component paint differently\n%s\nwant\n%s", got, want)
	}
}

func describeCanvas(canvas *gfx.Canvas) string {
	var out string
	for _, rectangle := range canvas.Rectangles() {
		out += fmt.Sprintf("rect %+v %+v %g\n", rectangle.Bounds, rectangle.Color, rectangle.Radius)
	}
	for _, text := range canvas.Texts() {
		out += fmt.Sprintf("text %q %+v %+v\n", text.Value, text.Bounds, text.Style)
	}
	return out
}
