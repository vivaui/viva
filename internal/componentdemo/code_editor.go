package componentdemo

import (
	"strings"
	"unicode/utf8"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

var (
	codeEditorField = tree.ID(tree.Feature("code-editor"), "field")
	codeEditorBar   = tree.ID(tree.Feature("code-editor"), "scroll")
	codeEditorBarX  = tree.ID(tree.Feature("code-editor"), "scroll-x")
)

// codeEditorTemplate is the sample source; WIDE is replaced by two wide
// characters in the Chinese preview, so that the grid shows a double-width cell.
const codeEditorTemplate = `package main

import "fmt"

// greet prints one line for every name it is given.
func greet(names []string) {
	for index, name := range names {
		if name == "" {
			continue
		}
		fmt.Printf("%d: hello, %s\n", index+1, name)
	}
}

func main() {
	greet([]string{"Viva", "WIDE", "Motty"})
}
`

// codeEditorSample is the sample text of one preview language, with the marks and
// selection of the static example computed once, so that painting builds nothing.
type codeEditorSample struct {
	source         string
	marks          []text.Span
	current        text.Span
	selectionStart int
	selectionEnd   int
}

func newCodeEditorSample(wide string) codeEditorSample {
	source := strings.Replace(codeEditorTemplate, "WIDE", wide, 1)
	sample := codeEditorSample{
		source:         source,
		selectionStart: strings.Index(source, "func greet"),
		selectionEnd:   strings.Index(source, "\t\tif name"),
	}
	for index, start := range allIndexes(source, "name") {
		span := text.Span{Start: start, End: start + len("name")}
		sample.marks = append(sample.marks, span)
		if index == 1 {
			sample.current = span
		}
	}
	return sample
}

var codeEditorSamples = [...]codeEditorSample{newCodeEditorSample("中文"), newCodeEditorSample("wide")}

func (state *State) codeEditorSample() *codeEditorSample {
	if state.english {
		return &codeEditorSamples[1]
	}
	return &codeEditorSamples[0]
}

var codeEditorOptions = text.CodeOptions{TabWidth: 4, HardTabs: true, AutoIndent: true, AutoClose: true, IndentAfter: "{[(", LineComment: "//"}

var codeEditorComponent = Component{
	ID: "code-editor", Title: "Code Editor", Category: "Forms", Summary: "等宽网格代码编辑器：行号、括号配对、查找标记；文本与撤销由调用方的 text.Editor 持有。",
	initialize: func(state *State) { state.codeEditor = new(components.CodeEditor) },
	editable: func(exampleID string) (editableField, bool) {
		return editableField{ID: codeEditorField, Initial: codeEditorSamples[0].source}, exampleID == "default" || exampleID == "wrap"
	},
	editor: func(state *State) *text.Editor {
		editor := text.NewEditor(state.codeEditorSample().source)
		editor.SetCode(codeEditorOptions)
		editor.SetSelection(text.Range{})
		return editor
	},
	paint:   (*State).paintCodeEditor,
	pointer: func(state *State, event gfx.PointerEvent) gfx.EventResult { return state.codeEditor.Pointer(event) },
	key:     (*State).codeEditorKey,
	scroll:  func(state *State, event gfx.ScrollEvent) gfx.EventResult { return state.codeEditor.Scroll(event) },
	Examples: []Example{
		{ID: "default", Title: "编辑代码", Description: "每个字符占一个单元格（CJK 占两格），行号、当前行和括号配对都按单元格计算。Tab 缩进、Shift+Tab 反缩进、Cmd/Ctrl+/ 注释、Alt+↑↓ 移动行、Enter 自动缩进、括号自动闭合都是 text.Editor 的操作。", Snippet: `editor := text.NewEditor(source)
editor.SetCode(text.CodeOptions{TabWidth: 4, HardTabs: true, AutoIndent: true, AutoClose: true, IndentAfter: "{[(", LineComment: "//"})

view.Paint(canvas, theme, bounds, components.CodeEditorSnapshot{
    ID: field, Value: editor.String(), LineNumbers: true, CurrentLine: true, MatchBrackets: true,
    Caret: editor.ByteOffset(editor.Caret()), Highlight: highlight, ScrollbarID: bar,
})`},
		{ID: "wrap", Title: "软换行", Description: "Wrap 让长行在单词处折到下一行，行号只标在每行的第一行；不换行时视图横向滚动并跟随插入点。", Snippet: `view.Paint(canvas, theme, bounds, components.CodeEditorSnapshot{
    ID: field, Value: value, Wrap: true, LineNumbers: true,
})`},
		{ID: "find", Title: "查找与替换", Description: "PaintFindBar 只画条和声明命中：查询、选项、计数、上一个/下一个、替换都由应用用 text/find.Session 驱动；匹配通过 Marks 画在编辑器里，当前匹配更亮。", Snippet: `session.Update(editor.String())
layout := components.PaintFindBar(canvas, theme, bar, components.FindBarSnapshot{
    Find: findInput, Replacement: replaceInput, Replacing: true,
    PreviousID: previous, NextID: next, CloseID: closeBar,
    MatchCaseID: matchCase, WholeWordID: wholeWord, RegexpID: regexp,
    ReplaceOneID: replaceOne, ReplaceAllID: replaceAll,
    Status: "2 of 4",
})`},
		{ID: "marks", Title: "查找标记与选区", Description: "Marks 在文本后面画查找匹配，当前匹配更亮；选区跨行时行尾多画半格表示换行也被选中。", Snippet: `view.Paint(canvas, theme, bounds, components.CodeEditorSnapshot{
    ID: field, Value: value,
    Marks: session.Matches(), CurrentMark: current, // []text.Span of the find session
    SelectionStart: 0, SelectionEnd: 20,
})`},
	},
}

func (state *State) paintCodeEditor(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	bounds := gfx.Rect{X: 24, Y: 24, Width: max(0, size.Width-48), Height: max(64, size.Height-48)}
	sample := state.codeEditorSample()
	value := sample.source
	snapshot := components.CodeEditorSnapshot{
		ID: codeEditorField, Label: "Go source", Value: value, ReadOnly: true,
		LineNumbers: true, CurrentLine: true, MatchBrackets: true, Wrap: state.exampleID == "wrap",
		Language: "go", Highlight: demoCodeHighlight, ScrollbarID: codeEditorBar, HorizontalScrollbarID: codeEditorBarX,
		TabWidth: codeEditorOptions.TabWidth,
	}
	switch {
	case state.exampleID == "marks" || state.exampleID == "find":
		snapshot.Marks, snapshot.CurrentMark = sample.marks, sample.current
		snapshot.Focused, snapshot.ShowCaret = true, true
		snapshot.SelectionStart, snapshot.SelectionEnd = sample.selectionStart, sample.selectionEnd
		snapshot.Caret = sample.selectionEnd
		snapshot.CurrentLine = false
	case state.editing:
		value = state.editor.String()
		focused := canvas.Focused() == codeEditorField
		selection := state.editor.Selection()
		snapshot.Value, snapshot.ReadOnly, snapshot.Focused = value, false, focused
		snapshot.SelectionStart, snapshot.SelectionEnd = state.editor.ByteOffset(selection.Start), state.editor.ByteOffset(selection.End)
		snapshot.Caret = state.editor.ByteOffset(state.editor.Caret())
		snapshot.ShowCaret = focused && selection.Start == selection.End
		if marked, composing := state.editor.Composition(); composing {
			snapshot.CompositionStart, snapshot.CompositionEnd = state.editor.ByteOffset(marked.Start), state.editor.ByteOffset(marked.End)
		}
	}
	state.codeEditor.Paint(canvas, theme, bounds, snapshot)
	if state.exampleID == "find" {
		state.paintCodeEditorFindBar(canvas, theme, bounds, sample)
	}
}

var (
	codeFindPage       = tree.Feature("code-editor-find")
	codeFindField      = tree.ID(codeFindPage, "find")
	codeFindReplace    = tree.ID(codeFindPage, "replace")
	codeFindPrevious   = tree.ID(codeFindPage, "previous")
	codeFindNext       = tree.ID(codeFindPage, "next")
	codeFindClose      = tree.ID(codeFindPage, "close")
	codeFindMatchCase  = tree.ID(codeFindPage, "match-case")
	codeFindWholeWord  = tree.ID(codeFindPage, "whole-word")
	codeFindRegexp     = tree.ID(codeFindPage, "regexp")
	codeFindToggle     = tree.ID(codeFindPage, "toggle")
	codeFindReplaceOne = tree.ID(codeFindPage, "replace-one")
	codeFindReplaceAll = tree.ID(codeFindPage, "replace-all")
)

// paintCodeEditorFindBar lays a find bar over the top right of the editor, as an
// editor's own sits, in front of the marks it paints.
func (state *State) paintCodeEditorFindBar(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, sample *codeEditorSample) {
	width := min(bounds.Width-32, 460)
	bar := gfx.Rect{X: bounds.X + bounds.Width - width - 22, Y: bounds.Y + 8, Width: width}
	canvas.PushLayer(2)
	{
		components.PaintFindBar(canvas, theme, bar, components.FindBarSnapshot{
			Find:        components.InputSnapshot{ID: codeFindField, Label: state.Text("查找", "Find"), Value: "name", Placeholder: state.Text("查找", "Find"), ReadOnly: true},
			Replacement: components.InputSnapshot{ID: codeFindReplace, Label: state.Text("替换", "Replace"), Value: "title", Placeholder: state.Text("替换", "Replace"), ReadOnly: true},
			Replacing:   true,
			PreviousID:  codeFindPrevious, NextID: codeFindNext, CloseID: codeFindClose,
			MatchCaseID: codeFindMatchCase, WholeWordID: codeFindWholeWord, RegexpID: codeFindRegexp,
			ToggleReplaceID: codeFindToggle, ReplaceOneID: codeFindReplaceOne, ReplaceAllID: codeFindReplaceAll,
			MatchCase: true,
			Status:    state.Text("2 / 4", "2 of 4"),
			Labels: components.FindBarLabels{
				Previous: state.Text("上一个", "Previous match"), Next: state.Text("下一个", "Next match"), Close: state.Text("关闭", "Close"),
				MatchCase: state.Text("区分大小写", "Match case"), WholeWord: state.Text("全字匹配", "Whole word"), Regexp: state.Text("正则表达式", "Regular expression"),
				ToggleReplace: state.Text("显示替换", "Toggle replace"), ReplaceOne: state.Text("替换", "Replace"), ReplaceAll: state.Text("全部替换", "Replace all"),
			},
		})
	}
	canvas.PopLayer()
}

// codeEditorKey is what the application does with the keys of a code editor: the
// editing commands, then the movements that depend on how the rows are laid out.
func (state *State) codeEditorKey(event gfx.KeyEvent) gfx.EventResult {
	if !state.editing || event.Target != codeEditorField || state.codeEditor == nil {
		return gfx.EventResult{}
	}
	if command, ok := components.CodeEditorCommandForKey(event); ok && command.Apply(state.editor) {
		return gfx.EventResult{Handled: true, Repaint: true}
	}
	movement, extend := components.TextareaMovementForKey(event.Key, event.Modifiers)
	if movement == components.TextareaMoveNone {
		return gfx.EventResult{}
	}
	value := state.editor.String()
	to := state.codeEditor.Move(value, state.editor.ByteOffset(state.editor.Caret()), movement)
	state.editor.MoveTo(utf8.RuneCountInString(value[:to]), extend)
	return gfx.EventResult{Handled: true, Repaint: true}
}

func allIndexes(value, needle string) []int {
	var out []int
	for from := 0; ; {
		index := strings.Index(value[from:], needle)
		if index < 0 {
			return out
		}
		out = append(out, from+index)
		from += index + len(needle)
	}
}

var demoKeywords = [...]string{"package", "import", "func", "for", "range", "if", "continue", "return", "var", "const", "type", "else"}

// demoCodeHighlight colors comments, strings and keywords of one Go line: a
// stand-in for the grammar an application owns.
func demoCodeHighlight(_, line string, dst []components.CodeEditorRun) []components.CodeEditorRun {
	keyword, comment, literal := gfx.RGB(170, 59, 180), gfx.RGB(110, 130, 118), gfx.RGB(30, 120, 80)
	end := len(line)
	if at := strings.Index(line, "//"); at >= 0 {
		end = at
	}
	for _, word := range demoKeywords {
		for from := 0; from < end; {
			at := strings.Index(line[from:end], word)
			if at < 0 {
				break
			}
			start := from + at
			from = start + len(word)
			before := start == 0 || !isDemoWord(line[start-1])
			after := from >= len(line) || !isDemoWord(line[from])
			if before && after {
				dst = append(dst, components.CodeEditorRun{Start: start, End: from, Color: keyword})
			}
		}
	}
	for from := 0; from < end; {
		open := strings.IndexByte(line[from:end], '"')
		if open < 0 {
			break
		}
		open += from
		closing := strings.IndexByte(line[open+1:end], '"')
		if closing < 0 {
			break
		}
		from = open + closing + 2
		dst = append(dst, components.CodeEditorRun{Start: open, End: from, Color: literal})
	}
	if end < len(line) {
		dst = append(dst, components.CodeEditorRun{Start: end, End: len(line), Color: comment})
	}
	// The highlighter may return its runs in any order of discovery, but the
	// editor wants them in order.
	for i := 1; i < len(dst); i++ {
		for j := i; j > 0 && dst[j].Start < dst[j-1].Start; j-- {
			dst[j], dst[j-1] = dst[j-1], dst[j]
		}
	}
	return dst
}

func isDemoWord(b byte) bool {
	return b == '_' || b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}
