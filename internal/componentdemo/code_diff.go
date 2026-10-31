package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

var (
	codeDiffUI         = tree.Feature("code-diff")
	codeDiffEditor     = tree.ID(codeDiffUI, "editor")
	codeDiffVertical   = tree.ID(codeDiffUI, "vertical-scrollbar")
	codeDiffHorizontal = tree.ID(codeDiffUI, "horizontal-scrollbar")
	codeDiffViewport   = tree.ID(codeDiffUI, "viewport")
)

const (
	codeDiffLeftValue = "server: production\n" +
		"replicas: 2\n" +
		"logLevel: info\n" +
		"timeout: 20s\n" +
		"healthCheck:\n" +
		"  enabled: true\n" +
		"  interval: 10s\n"
	codeDiffRightValue = "server: production\n" +
		"replicas: 4\n" +
		"logLevel: warning\n" +
		"timeout: 30s\n" +
		"healthCheck:\n" +
		"  enabled: true\n" +
		"  interval: 10s\n" +
		"  path: /ready\n"
)

type codeDiffDemoState struct {
	component components.CodeDiff
	left      string
	right     string
	result    components.CodeDiffResult
}

var codeDiffComponent = Component{
	ID: "code-diff", Title: "Code Diff", Category: "Forms",
	Summary: "服务器版本与本地版本的行对齐双栏对比，右侧复用原生文本编辑。",
	initialize: func(state *State) {
		state.codeDiff = &codeDiffDemoState{
			left:   codeDiffLeftValue,
			right:  codeDiffRightValue,
			result: components.DiffCodeLines(codeDiffLeftValue, codeDiffRightValue),
		}
	},
	paint: (*State).paintCodeDiff,
	pointer: func(state *State, event ui.PointerEvent) ui.EventResult {
		return state.codeDiff.component.Pointer(event)
	},
	scroll: func(state *State, event ui.ScrollEvent) ui.EventResult {
		return state.codeDiff.component.Scroll(event)
	},
	editable: func(string) (editableField, bool) {
		return editableField{ID: codeDiffEditor, Initial: codeDiffRightValue}, true
	},
	editor: func(*State) *text.Editor {
		return text.NewEditor(codeDiffRightValue)
	},
	Examples: []Example{{
		ID: "default", Title: "双栏差异", Description: "绿色表示本地新增，红色表示服务器内容已删除；两栏共享滚动并保留行号对应。",
		Height: 310,
		Snippet: `diff.Paint(canvas, theme, bounds, components.CodeDiffSnapshot{
    Result: components.DiffCodeLines(serverValue, localValue),
    ID: field, Value: localValue,
    VerticalScrollbarID: vertical,
    HorizontalScrollbarID: horizontal,
})`,
	}},
}

func (state *State) paintCodeDiff(canvas *ui.Canvas, theme ui.Theme, size ui.Size) {
	demo := state.codeDiff
	value := state.fieldValue(demo.right)
	if value != demo.right {
		demo.result = components.DiffCodeLines(demo.left, value)
		demo.right = value
	}
	focused := state.editing && canvas.Focused() == codeDiffEditor
	snapshot := components.CodeDiffSnapshot{
		Result: demo.result,
		ID:     codeDiffEditor, Label: state.Text("本地编辑版本", "Local edited version"),
		Hint:  state.Text("编辑右侧文本，左侧保持为服务器基准版本。", "Edit the right pane; the left pane remains the server baseline."),
		Value: value, Language: "yaml", Highlight: demoCodeHighlight,
		ReadOnly: !state.editing, Focused: focused, ShowCaret: focused,
		VerticalScrollbarID: codeDiffVertical, HorizontalScrollbarID: codeDiffHorizontal,
		ViewportID: codeDiffViewport,
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

	var stage flex.Box
	state.page(canvas, theme, size, 660, state.Text("Code Diff · 保存前对比", "Code Diff · Compare before saving"), func(f *flex.Frame) {
		stage = f.Slot().FillWidth().Height(min(float32(230), max(size.Height-105, 120)))
		caption(f, state.Text(
			"差异统计根据当前右侧文本重新计算；滚动和编辑状态由调用方持有。",
			"Diff statistics follow the current right text; scrolling and editing stay caller-owned.",
		))
	})
	demo.component.Paint(canvas, theme, stage.Rect(), snapshot)
}
