package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var columnResizeComponent = Component{
	ID: "column-resize", Title: "Column resize", Category: "Layout",
	Summary: "Columns whose dividers drag to resize a pair and double-click to fit the left one.",
	paint:   (*State).paintColumnResize, pointer: (*State).pointerColumnResize, initialize: (*State).initializeColumnResize,
	Examples: []Example{{
		ID: "file-list", Title: "File list", Description: "Drag a divider to move it, or double-click it to fit the column to its content.",
		Snippet: `widths := []float32{260, 90, 140}
columns := make([]gfx.Rect, 3)
dividers := make([]gfx.Rect, 2)
hits := make([]gfx.Rect, 2)
layout := components.BeginColumnResize(canvas, theme, stage.Rect(), widths).
    Dividers(dividerIDs).Minimums([]float32{120, 54, 76}).
    Fits([]float32{180, 64, 100}).Gap(8).
    Paint(columns, dividers, hits, []string{"Resize name", "Resize size"})
edit, ok := components.BeginColumnResize(nil, theme, stage.Rect(), widths).
    Gap(8).Edit(0, pointer.X)`,
	}},
}

var (
	columnResizeDemoUI      = tree.Feature("column-resize")
	columnResizeDemoDivider = tree.Sub(columnResizeDemoUI, tree.Name("divider"))
)

func (state *State) initializeColumnResize() {
	state.points[0], state.points[1], state.points[2] = 260, 90, 140
	state.values[0] = 0
}

func (state *State) columnResizeWidths() [3]float32 {
	return [3]float32{state.points[0], state.points[1], state.points[2]}
}

func (state *State) columnResizeDrag(divider int, x float32) bool {
	bounds := state.bag().placed
	if bounds.Width <= 0 || divider < 0 || divider > 1 {
		return false
	}
	widths := state.columnResizeWidths()
	edit, ok := components.BeginColumnResize(nil, state.themeFor(), bounds, widths[:]).
		Gap(8).Edit(divider, x)
	if !ok {
		return false
	}
	if divider == 0 {
		state.points[1] = edit.Right
	} else {
		state.points[1], state.points[2] = edit.Left, edit.Right
	}
	return true
}

func (state *State) pointerColumnResize(event gfx.PointerEvent) gfx.EventResult {
	handled := gfx.EventResult{Handled: true, Repaint: true}
	if state.values[0] > 0 {
		switch event.Kind {
		case gfx.PointerMove:
			state.columnResizeDrag(int(state.values[0])-1, event.X)
			return handled
		case gfx.PointerUp, gfx.PointerExit:
			state.values[0] = 0
			return handled
		}
	}
	if event.Kind != gfx.PointerDown || event.Button != gfx.PointerButtonPrimary {
		return gfx.EventResult{}
	}
	divider, ok := event.Target.In(columnResizeDemoDivider)
	if !ok || divider > 1 {
		return gfx.EventResult{}
	}
	if event.ClickCount >= 2 {
		widths := state.columnResizeWidths()
		fit := components.BeginColumnResize(nil, state.themeFor(), state.bag().placed, widths[:]).
			Gap(8).Fits([]float32{180, 64, 100})
		edit, ok := fit.Fit(divider)
		if ok {
			if divider == 0 {
				state.points[1] = edit.Right
			} else {
				state.points[1], state.points[2] = edit.Left, edit.Right
			}
		}
		return handled
	}
	state.values[0] = int32(divider + 1)
	return handled
}

func (state *State) paintColumnResize(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	var stage ui.Box
	state.drawPage(canvas, theme, size, 620, "File list", func(c ui.C) {
		stage = ui.Slot(c).Height(180)
		caption(c, state.Text("拖动分隔线调整两列宽度；双击按内容自适应。", "Drag a divider to resize two columns; double-click it to fit the left one."))
	})
	bounds := stage.Rect()
	state.bag().placed = bounds
	widths := state.columnResizeWidths()
	nameWidth := max(bounds.Width-2*8-widths[1]-widths[2], 40)
	widths[0] = nameWidth
	var columns [3]gfx.Rect
	var dividers, hits [2]gfx.Rect
	ids := [2]gfx.ID{columnResizeDemoDivider.At(0), columnResizeDemoDivider.At(1)}
	labels := [2]string{state.Text("调整名称列", "Resize name"), state.Text("调整大小列", "Resize size")}
	layout := components.BeginColumnResize(canvas, theme, bounds, widths[:]).
		Dividers(ids[:]).
		Minimums([]float32{120, 54, 76}).
		Fits([]float32{180, 64, 100}).
		Gap(8).
		Paint(columns[:], dividers[:], hits[:], labels[:])
	for _, column := range layout.Columns {
		canvas.Rounded(column.Inset(gfx.All(1)), 4, theme.Muted)
	}
	names := [3]string{"name", "size", "modified"}
	for index, column := range layout.Columns {
		canvas.LabelCenter(names[index], column, gfx.TextStyle{Size: theme.Metrics.FontSizeM, Color: theme.Foreground})
	}
}
