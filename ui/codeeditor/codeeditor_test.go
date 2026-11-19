package codeeditor_test

import (
	"testing"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/codeeditor"
	"github.com/vivaui/viva/window"
)

const source = `package main

func main() {
	println("hello")
}
`

type model struct {
	editor codeeditor.Editor
	diff   codeeditor.Diff
	merge  codeeditor.Merge
	find   codeeditor.FindBar
	canvas codeeditor.Canvas
	input  window.TextInput

	keys, pointers, scrolls, texts, paints int
}

func (m *model) wire() {
	m.editor.Key = m.key
	m.editor.Pointer = m.pointer
	m.editor.Scroll = m.scroll
	m.editor.Text = m.text

	m.diff.Key = m.key
	m.diff.Pointer = m.pointer
	m.diff.Scroll = m.scroll
	m.diff.Text = m.text

	m.merge.Key = m.key
	m.merge.Pointer = m.pointer
	m.merge.Scroll = m.scroll
	m.merge.Text = m.text

	m.find.Key = m.key
	m.find.Pointer = m.pointer
	m.find.Scroll = m.scroll
	m.find.Text = m.text

	m.canvas.Key = m.key
	m.canvas.Pointer = m.pointer
	m.canvas.Scroll = m.scroll
	m.canvas.Text = m.text
	m.canvas.Draw = m.paint
}

func (m *model) key(gfx.KeyEvent) gfx.EventResult {
	m.keys++
	return gfx.EventResult{Handled: true}
}

func (m *model) pointer(gfx.PointerEvent) gfx.EventResult {
	m.pointers++
	return gfx.EventResult{Handled: true}
}

func (m *model) scroll(gfx.ScrollEvent) gfx.EventResult {
	m.scrolls++
	return gfx.EventResult{Handled: true}
}

func (m *model) text(window.TextInputContext) *window.TextInput {
	m.texts++
	return &m.input
}

func (m *model) paint(d *ui.DrawContext) {
	m.paints++
	d.Canvas.Fill(d.Rect, d.Theme.Card)
}

func (m *model) editorPage(c ui.C) { codeeditor.View(c, "Source", &m.editor).Fill() }

func (m *model) diffPage(c ui.C) { codeeditor.DiffView(c, "Changes", &m.diff).Fill() }

func (m *model) mergePage(c ui.C) { codeeditor.MergeView(c, "Merge", &m.merge).Fill() }

func (m *model) findPage(c ui.C) { codeeditor.FindBarView(c, "Find", &m.find).FillWidth() }

func (m *model) canvasPage(c ui.C) { codeeditor.CanvasView(c, "Canvas", &m.canvas).Fill() }

func testTheme() *gfx.Theme {
	theme := gfx.LightTheme()
	return &theme
}

func drawPage(page *ui.Page, canvas *gfx.Canvas, view func(ui.C)) {
	canvas.Reset()
	canvas.BeginFrame()
	page.Draw(canvas, gfx.Rect{Width: 640, Height: 420}, view)
	canvas.EndFrame()
}

// hitLabelled returns the front-most hit with the label: the control's own hit
// or, later in the hit order, the text region the component declared over it.
func hitLabelled(canvas *gfx.Canvas, label string) (gfx.Hit, bool) {
	hits := canvas.Hits()
	for i := len(hits) - 1; i >= 0; i-- {
		if hits[i].Label == label && !hits[i].ID.IsZero() {
			return hits[i], true
		}
	}
	return gfx.Hit{}, false
}

// A page declares the editor View, the editor paints, and a pointer, key and
// text call reach the callbacks the model stored.
func TestEditorViewPaintsAndRoutesInput(t *testing.T) {
	m := new(model)
	m.wire()
	m.editor.Snapshot = components.CodeEditorSnapshot{
		Value: source, LineNumbers: true, CurrentLine: true, MatchBrackets: true,
	}
	page := ui.NewPage(ui.PageConfig{View: m.editorPage, Theme: testTheme()})
	canvas := new(gfx.Canvas)
	drawPage(page, canvas, m.editorPage)

	hit, ok := hitLabelled(canvas, "Source")
	if !ok {
		t.Fatal("the editor view painted no control named Source")
	}
	if hit.Role != gfx.RoleTextField || !hit.Multiline {
		t.Fatalf("editor hit = role %v multiline %v, want a multiline text field", hit.Role, hit.Multiline)
	}

	if result := page.Pointer(gfx.PointerEvent{
		Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, Target: hit.ID,
		X: hit.Bounds.X + 4, Y: hit.Bounds.Y + 4,
	}); !result.Handled || m.pointers != 1 {
		t.Fatalf("pointer result = %+v after %d callbacks", result, m.pointers)
	}
	if result := page.Key(gfx.KeyEvent{Key: gfx.KeyUp, Target: hit.ID}); !result.Handled || m.keys != 1 {
		t.Fatalf("key result = %+v after %d callbacks", result, m.keys)
	}
	if page.TextInput(window.TextInputContext{ID: hit.ID}) == nil || m.texts != 1 {
		t.Fatalf("text service got no editor; callbacks = %d", m.texts)
	}
}

func TestDiffViewPaintsAndRoutesInput(t *testing.T) {
	m := new(model)
	m.wire()
	left, right := "one\ntwo\nthree\n", "one\nTWO\nthree\n"
	m.diff.Snapshot = components.CodeDiffSnapshot{
		Result: components.DiffCodeLines(left, right),
		Value:  right,
	}
	page := ui.NewPage(ui.PageConfig{View: m.diffPage, Theme: testTheme()})
	canvas := new(gfx.Canvas)
	drawPage(page, canvas, m.diffPage)

	hit, ok := hitLabelled(canvas, "Changes")
	if !ok {
		t.Fatal("the diff view painted no control named Changes")
	}
	if hit.Role != gfx.RoleTextField || !hit.Multiline {
		t.Fatalf("diff hit = role %v multiline %v, want a multiline text field", hit.Role, hit.Multiline)
	}
	if result := page.Pointer(gfx.PointerEvent{
		Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, Target: hit.ID,
		X: hit.Bounds.X + 4, Y: hit.Bounds.Y + 4,
	}); !result.Handled || m.pointers != 1 {
		t.Fatalf("pointer result = %+v after %d callbacks", result, m.pointers)
	}
	if result := page.Key(gfx.KeyEvent{Key: gfx.KeyDown, Target: hit.ID}); !result.Handled || m.keys != 1 {
		t.Fatalf("key result = %+v after %d callbacks", result, m.keys)
	}
	if page.TextInput(window.TextInputContext{ID: hit.ID}) == nil || m.texts != 1 {
		t.Fatalf("text service got no editor; callbacks = %d", m.texts)
	}
}

func TestMergeViewPaintsAndRoutesInput(t *testing.T) {
	m := new(model)
	m.wire()
	result := "one\ntwo\nthree"
	m.merge.Snapshot = components.CodeMergeSnapshot{
		Current:  []string{"one", "two", "three"},
		Incoming: []string{"one", "TWO", "three"},
		Rows: []components.CodeMergeRow{
			{CurrentLine: 0, ResultLine: 0, IncomingLine: 0},
			{CurrentLine: 1, ResultLine: 1, IncomingLine: 1, CurrentChanged: true, IncomingChanged: true},
			{CurrentLine: 2, ResultLine: 2, IncomingLine: 2},
		},
		Editor: components.CodeDiffSnapshot{Value: result},
	}
	page := ui.NewPage(ui.PageConfig{View: m.mergePage, Theme: testTheme()})
	canvas := new(gfx.Canvas)
	drawPage(page, canvas, m.mergePage)

	hit, ok := hitLabelled(canvas, "Merge")
	if !ok {
		t.Fatal("the merge view painted no control named Merge")
	}
	if hit.Role != gfx.RoleTextField || !hit.Multiline {
		t.Fatalf("merge hit = role %v multiline %v, want a multiline text field", hit.Role, hit.Multiline)
	}
	if result := page.Pointer(gfx.PointerEvent{
		Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, Target: hit.ID,
		X: hit.Bounds.X + 4, Y: hit.Bounds.Y + 4,
	}); !result.Handled || m.pointers != 1 {
		t.Fatalf("pointer result = %+v after %d callbacks", result, m.pointers)
	}
	if result := page.Key(gfx.KeyEvent{Key: gfx.KeyDown, Target: hit.ID}); !result.Handled || m.keys != 1 {
		t.Fatalf("key result = %+v after %d callbacks", result, m.keys)
	}
	if page.TextInput(window.TextInputContext{ID: hit.ID}) == nil || m.texts != 1 {
		t.Fatalf("text service got no editor; callbacks = %d", m.texts)
	}
}

func testFindBarSnapshot() components.FindBarSnapshot {
	return components.FindBarSnapshot{
		Find:        components.InputSnapshot{ID: codeeditor.Shown, Label: "Query", Value: "name", Placeholder: "Find"},
		Replacement: components.InputSnapshot{ID: codeeditor.Shown, Label: "Replace with", Value: "title", Placeholder: "Replace"},
		Replacing:   true,
		PreviousID:  codeeditor.Shown, NextID: codeeditor.Shown, CloseID: codeeditor.Shown,
		MatchCaseID: codeeditor.Shown, WholeWordID: codeeditor.Shown, RegexpID: codeeditor.Shown,
		ToggleReplaceID: codeeditor.Shown, ReplaceOneID: codeeditor.Shown, ReplaceAllID: codeeditor.Shown,
		MatchCase: true, Status: "2 of 4",
		Labels: components.FindBarLabels{
			Previous: "Previous match", Next: "Next match", Close: "Close",
			MatchCase: "Match case", WholeWord: "Whole word", Regexp: "Regular expression",
			ToggleReplace: "Toggle replace", ReplaceOne: "Replace", ReplaceAll: "Replace all",
		},
	}
}

func TestFindBarViewPaintsAndRoutesInput(t *testing.T) {
	m := new(model)
	m.wire()
	m.find.Snapshot = testFindBarSnapshot()
	page := ui.NewPage(ui.PageConfig{View: m.findPage, Theme: testTheme()})
	canvas := new(gfx.Canvas)
	drawPage(page, canvas, m.findPage)

	hit, ok := hitLabelled(canvas, "Find")
	if !ok {
		t.Fatal("the find bar view painted no control named Find")
	}
	if want := components.FindBarHeight(*testTheme(), true); hit.Bounds.Height != want {
		t.Fatalf("find bar height = %g, want %g", hit.Bounds.Height, want)
	}
	if field, ok := hitLabelled(canvas, "Query"); !ok || field.Role != gfx.RoleTextField {
		t.Fatalf("the find input painted no text field: hit %v, found %v", field, ok)
	}
	if _, ok := hitLabelled(canvas, "Next match"); !ok {
		t.Fatal("the find bar painted no next-match button")
	}
	parts := m.find.Parts()
	if parts.IsZero() || parts.At(codeeditor.PartReplaceAll).IsZero() {
		t.Fatal("the find bar declared no parts")
	}
	labels := [...]string{
		"Query", "Replace with", "Previous match", "Next match", "Close",
		"Match case", "Whole word", "Regular expression",
		"Toggle replace", "Replace", "Replace all",
	}
	for index, label := range labels {
		control, ok := hitLabelled(canvas, label)
		if !ok {
			t.Fatalf("%q painted no hit", label)
		}
		if want := parts.At(index); control.ID != want {
			t.Fatalf("%q hit = %v, want part %v", label, control.ID, want)
		}
	}

	if result := page.Pointer(gfx.PointerEvent{
		Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, Target: parts.At(codeeditor.PartNext),
		X: hit.Bounds.X + 4, Y: hit.Bounds.Y + 4,
	}); !result.Handled || m.pointers != 1 {
		t.Fatalf("pointer result = %+v after %d callbacks", result, m.pointers)
	}
	if result := page.Key(gfx.KeyEvent{Key: gfx.KeyEnter, Target: parts.At(codeeditor.PartFind)}); !result.Handled || m.keys != 1 {
		t.Fatalf("key result = %+v after %d callbacks", result, m.keys)
	}
	if result := page.Wheel(gfx.ScrollEvent{Target: parts.At(codeeditor.PartFind)}); !result.Handled || m.scrolls != 1 {
		t.Fatalf("scroll result = %+v after %d callbacks", result, m.scrolls)
	}
	if page.TextInput(window.TextInputContext{ID: hit.ID}) == nil || m.texts != 1 {
		t.Fatalf("text service got no editor for the control; callbacks = %d", m.texts)
	}
	if page.TextInput(window.TextInputContext{ID: parts.At(codeeditor.PartFind)}) == nil || m.texts != 2 {
		t.Fatalf("text service got no editor for the query field; callbacks = %d", m.texts)
	}
}

func TestCanvasViewPaintsAndRoutesInput(t *testing.T) {
	m := new(model)
	m.wire()
	m.canvas.Role = gfx.RoleTerminal
	page := ui.NewPage(ui.PageConfig{View: m.canvasPage, Theme: testTheme()})
	canvas := new(gfx.Canvas)
	drawPage(page, canvas, m.canvasPage)

	hit, ok := hitLabelled(canvas, "Canvas")
	if !ok || m.paints == 0 {
		t.Fatalf("the canvas view painted no control: hit %v, draw calls %d", ok, m.paints)
	}
	if hit.Role != gfx.RoleTerminal {
		t.Fatalf("canvas hit role = %v, want terminal", hit.Role)
	}
	if result := page.Pointer(gfx.PointerEvent{Kind: gfx.PointerDown, Target: hit.ID}); !result.Handled || m.pointers != 1 {
		t.Fatalf("pointer result = %+v after %d callbacks", result, m.pointers)
	}
	if result := page.Key(gfx.KeyEvent{Key: gfx.KeyEnter, Target: hit.ID}); !result.Handled || m.keys != 1 {
		t.Fatalf("key result = %+v after %d callbacks", result, m.keys)
	}
	if page.TextInput(window.TextInputContext{ID: hit.ID}) == nil || m.texts != 1 {
		t.Fatalf("text service got no editor; callbacks = %d", m.texts)
	}
}

func TestNewBindsTheTextDocument(t *testing.T) {
	document := text.NewEditor(source)
	document.SetSelection(text.Range{Start: 2, End: 5})
	editor := codeeditor.New(document)
	if editor.Document != document {
		t.Fatal("New did not keep the document")
	}
	if !editor.Snapshot.LineNumbers || !editor.Snapshot.CurrentLine || !editor.Snapshot.MatchBrackets {
		t.Fatalf("New defaults = %+v, want line numbers, current line and brackets", editor.Snapshot)
	}
}

// A warm editor paint, including the page's declaration and the component's
// layout caches, allocates nothing.
func TestEditorViewWarmPaintAllocatesNothing(t *testing.T) {
	m := new(model)
	m.wire()
	m.editor.Snapshot = components.CodeEditorSnapshot{
		Value: source, LineNumbers: true, CurrentLine: true, MatchBrackets: true,
	}
	page := ui.NewPage(ui.PageConfig{View: m.editorPage, Theme: testTheme()})
	canvas := new(gfx.Canvas)
	for range 4 {
		drawPage(page, canvas, m.editorPage)
	}
	if allocs := testing.AllocsPerRun(50, func() { drawPage(page, canvas, m.editorPage) }); allocs != 0 {
		t.Fatalf("a warm editor paint allocated %g times", allocs)
	}
}

func BenchmarkEditorViewWarmPaint(b *testing.B) {
	m := new(model)
	m.wire()
	m.editor.Snapshot = components.CodeEditorSnapshot{
		Value: source, LineNumbers: true, CurrentLine: true, MatchBrackets: true,
	}
	page := ui.NewPage(ui.PageConfig{View: m.editorPage, Theme: testTheme()})
	canvas := new(gfx.Canvas)
	for range 4 {
		drawPage(page, canvas, m.editorPage)
	}
	b.ReportAllocs()
	for b.Loop() {
		drawPage(page, canvas, m.editorPage)
	}
}

// A warm find bar paint, including the page's declaration and the component's
// text caches, allocates nothing.
func TestFindBarViewWarmPaintAllocatesNothing(t *testing.T) {
	m := new(model)
	m.wire()
	m.find.Snapshot = testFindBarSnapshot()
	page := ui.NewPage(ui.PageConfig{View: m.findPage, Theme: testTheme()})
	canvas := new(gfx.Canvas)
	for range 4 {
		drawPage(page, canvas, m.findPage)
	}
	if allocs := testing.AllocsPerRun(50, func() { drawPage(page, canvas, m.findPage) }); allocs != 0 {
		t.Fatalf("a warm find bar paint allocated %g times", allocs)
	}
}

func BenchmarkFindBarViewWarmPaint(b *testing.B) {
	m := new(model)
	m.wire()
	m.find.Snapshot = testFindBarSnapshot()
	page := ui.NewPage(ui.PageConfig{View: m.findPage, Theme: testTheme()})
	canvas := new(gfx.Canvas)
	for range 4 {
		drawPage(page, canvas, m.findPage)
	}
	b.ReportAllocs()
	for b.Loop() {
		drawPage(page, canvas, m.findPage)
	}
}
