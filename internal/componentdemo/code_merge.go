package componentdemo

import (
	"strings"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

var (
	codeMergeUI       = tree.Feature("code-merge")
	codeMergeEditor   = tree.ID(codeMergeUI, "result")
	codeMergeVertical = tree.ID(codeMergeUI, "vertical-scrollbar")
	codeMergeAccept   = tree.ID(codeMergeUI, "accept-incoming")
)

const (
	codeMergeCurrent  = "head\nlocal\ntail"
	codeMergeIncoming = "head\nremote\ntail"
	codeMergeResult   = "head\nchoice\ntail"
)

type codeMergeDemoState struct {
	component components.CodeMerge
	rows      []components.CodeMergeRow
	current   []string
	incoming  []string
	actions   [1]components.CodeMergeAction
	value     string
	accepted  bool
}

var codeMergeComponent = Component{
	ID: "code-merge", Title: "Code Merge", Category: "Forms",
	Summary: "并排查看 Current、可编辑的 Result 与 Incoming，操作含义由应用决定。",
	initialize: func(state *State) {
		state.codeMerge = &codeMergeDemoState{
			current: strings.Split(codeMergeCurrent, "\n"), incoming: strings.Split(codeMergeIncoming, "\n"),
		}
	},
	paint: (*State).paintCodeMerge,
	pointer: func(state *State, event gfx.PointerEvent) gfx.EventResult {
		return state.codeMerge.component.Pointer(event)
	},
	scroll: func(state *State, event gfx.ScrollEvent) gfx.EventResult {
		return state.codeMerge.component.Scroll(event)
	},
	editable: func(string) (editableField, bool) {
		return editableField{ID: codeMergeEditor, Initial: codeMergeResult}, true
	},
	editor: func(*State) *text.Editor {
		return text.NewEditor(codeMergeResult)
	},
	Examples: []Example{{
		ID: "default", Title: "三方对齐", Description: "中栏可编辑；点击 Incoming 操作由调用方接收并决定如何更新结果。",
		Height: 310,
		Snippet: `merge.Paint(canvas, theme, bounds, components.CodeMergeSnapshot{
    Current: currentLines, Incoming: incomingLines,
    Rows: alignedRows,
    Editor: components.CodeDiffSnapshot{ID: resultID, Value: result},
    Actions: []components.CodeMergeAction{{Row: 1, Incoming: true, ID: acceptID, Label: "Accept"}},
})`,
	}},
}

func (state *State) paintCodeMerge(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	demo := state.codeMerge
	value := state.fieldValue(codeMergeResult)
	if demo.value != value {
		alignment := components.DiffCodeLines(codeMergeCurrent, value)
		demo.rows = demo.rows[:0]
		for _, row := range alignment.Rows {
			incomingLine := row.LeftLine
			demo.rows = append(demo.rows, components.CodeMergeRow{
				CurrentLine: row.LeftLine, ResultLine: row.RightLine, IncomingLine: incomingLine,
				CurrentChanged:  row.Kind != components.CodeDiffUnchanged,
				IncomingChanged: incomingLine == 1,
			})
		}
		demo.value = value
	}
	focused := state.editing && canvas.Focused() == codeMergeEditor
	snapshot := components.CodeDiffSnapshot{
		ID: codeMergeEditor, Label: state.Text("合并结果", "Merge result"), Value: value,
		ReadOnly: !state.editing, Focused: focused, ShowCaret: focused,
		VerticalScrollbarID: codeMergeVertical,
	}
	if state.editing {
		selection := state.editor.Selection()
		snapshot.SelectionStart = byteOffsetOfRune(value, selection.Start)
		snapshot.SelectionEnd = byteOffsetOfRune(value, selection.End)
		snapshot.Caret = byteOffsetOfRune(value, state.editor.Caret())
		snapshot.ShowCaret = focused && selection.Start == selection.End
		if marked, composing := state.editor.Composition(); composing {
			snapshot.CompositionStart = byteOffsetOfRune(value, marked.Start)
			snapshot.CompositionEnd = byteOffsetOfRune(value, marked.End)
		}
	}
	var stage ui.Box
	state.page(canvas, theme, size, 750, state.Text("Code Merge · 三方对比", "Code Merge · Three-way comparison"), func(frame *ui.Frame) {
		stage = frame.Slot().FillWidth().Height(min(float32(230), max(size.Height-105, 120)))
		caption(frame, state.Text("三栏同步滚动，冲突检测和接受策略由调用方负责。", "The three panes scroll together; conflict detection and acceptance remain caller-owned."))
	})
	var actions []components.CodeMergeAction
	for rowIndex, row := range demo.rows {
		if row.IncomingLine == 1 && !demo.accepted {
			demo.actions[0] = components.CodeMergeAction{Row: rowIndex, Incoming: true, ID: codeMergeAccept, Label: state.Text("接受", "Accept")}
			actions = demo.actions[:]
			break
		}
	}
	demo.component.Paint(canvas, theme, stage.Rect(), components.CodeMergeSnapshot{
		Current: demo.current, Incoming: demo.incoming, Rows: demo.rows, Editor: snapshot, Actions: actions,
	})
	if canvas.Clicked(codeMergeAccept) {
		demo.accepted = true
	}
}
