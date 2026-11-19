package ui_test

import (
	"testing"

	"github.com/vivaui/viva/base"
	"github.com/vivaui/viva/ui"
)

// AutoGrow sizes a TextArea from the rows its text needs: the control's minimum
// for one row, one line per row in between, and the caller's cap once the text
// passes it.
func TestTextAreaAutoGrowMeasuresItsRows(t *testing.T) {
	height := func(lines int) float32 {
		var notes ui.Field
		value := "line"
		for i := 1; i < lines; i++ {
			value += "\nline"
		}
		notes.SetText(value)
		var box ui.Box
		view := func(c ui.C) {
			ui.Column(c).Pad(16).Children(func() {
				box = ui.TextArea(c, &notes, "Notes", "").Width(300).AutoGrow(5)
			})
		}
		page := ui.NewPage(ui.PageConfig{View: view})
		drawPage(page, benchCanvas(), view)
		return box.Rect().Height
	}
	rowHeight := pageTheme.Metrics.FontSizeM + 7
	rowsHeight := func(n float32) float32 {
		return n*rowHeight + 2*base.TextareaPadY
	}
	if got := height(1); got != 64 {
		t.Errorf("one row is %v tall, want the 64 minimum", got)
	}
	if got, want := height(3), rowsHeight(3); got != want {
		t.Errorf("three rows are %v tall, want %v", got, want)
	}
	if got, want := height(8), rowsHeight(5); got != want {
		t.Errorf("eight rows are %v tall, want the five-row cap %v", got, want)
	}
}

// The rows measured by AutoGrow are the ones the paint lays out, so a warm box
// neither rewraps nor allocates.
func TestTextAreaAutoGrowAllocatesNothingOnceWarm(t *testing.T) {
	var notes ui.Field
	notes.SetText("one\ntwo\nthree")
	view := func(c ui.C) {
		ui.Column(c).Pad(16).Children(func() {
			ui.TextArea(c, &notes, "Notes", "").Width(300).AutoGrow(5)
		})
	}
	page := ui.NewPage(ui.PageConfig{View: view})
	canvas := benchCanvas()
	draw := func() { drawPage(page, canvas, view) }
	draw()
	if allocations := testing.AllocsPerRun(100, draw); allocations != 0 {
		t.Fatalf("a warm AutoGrow textarea allocates %v times a frame, want 0", allocations)
	}
}
