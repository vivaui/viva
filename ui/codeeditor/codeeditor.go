// Package codeeditor is the opt-in page control for the source-code canvases
// of package components: a code editor, a line-aligned diff, a three-way merge
// and a find and replace bar. Each type keeps the caller-owned component, the
// snapshot painted at every paint and the callbacks that take its input. View
// declares the control; the wrapper owns the control's identity and input
// routing, so a call site never makes a components.Begin* control itself.
//
//	editor := &codeeditor.Editor{}
//	editor.Snapshot.Value = source
//	...
//	codeeditor.View(c, "Source", editor).Fill()
//
// The text, undo history and selection stay in the application's text.Editor;
// Snapshot is the presentation of them. Update Snapshot outside the paint. The
// wrapper copies it at paint time and sets ID and Label to the control's own,
// so a caller does not assign them.
//
// The input callbacks are the application's: Key handles editing commands and
// movements, Pointer handles what the component's scrollbars did not take, and
// Text supplies the platform's text service with the application's editor.
// Each may be nil. Declare a View on its own, not inside a ui.Memo.
package codeeditor

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/window"
)

// New is an Editor that paints document: its value, caret, selection and
// composition are copied from the text model at every paint, so the page does
// not assign them itself. Line numbers, the current-line tint and bracket
// matching are on; turn them off in Snapshot, or set Language and Highlight
// there.
func New(document *text.Editor) *Editor {
	return &Editor{
		Document: document,
		Snapshot: components.CodeEditorSnapshot{
			LineNumbers: true, CurrentLine: true, MatchBrackets: true,
		},
	}
}

// Editor is one code editor's page control. Keep it in the model beside the
// application's text.Editor. The zero value is ready; one Editor serves one
// place on a page and one text.Editor. Keep it where View finds it: its stable
// handlers and draw function point at this value, so do not copy an Editor
// after it has been declared.
//
// Code keeps the component's layout, scroll and caret caches across paints.
// Snapshot is the presentation painted next; the wrapper sets its ID and Label.
type Editor struct {
	// Document, when set, is the application's text model. The wrapper copies
	// its value, caret, selection and composition into Snapshot at every paint;
	// the application still owns the text, undo and editing commands.
	Document *text.Editor
	// Code is the caller-owned code editor view.
	Code components.CodeEditor
	// Snapshot is painted at every paint. Update it when the text, selection or
	// other presentation state changes.
	Snapshot components.CodeEditorSnapshot
	// Key receives the keys pressed while the editor has focus. See
	// components.CodeEditorCommandForKey and components.CodeEditor.Move.
	Key func(gfx.KeyEvent) gfx.EventResult
	// Pointer receives the pointer events the editor's scrollbars did not handle.
	Pointer func(gfx.PointerEvent) gfx.EventResult
	// Scroll receives the wheel the editor did not scroll. A nil callback leaves
	// what the editor cannot scroll to the page.
	Scroll func(gfx.ScrollEvent) gfx.EventResult
	// Text gives the platform's text service the application's editor for the
	// focused control. Nil disables native editing and input methods.
	Text func(window.TextInputContext) *window.TextInput

	handlers ui.Handlers
	drawFn   func(*ui.DrawContext)
	wired    bool
}

// View declares the editor at c. Give it a size; it is a ui.Control with a
// text-field role, painted through the component. The component paints in a box
// at least 64 pixels tall, so give it a height a code editor can use. The
// control's identity and labels are the wrapper's, so the snapshot's ID and
// Label are ignored.
func View(c ui.C, label string, editor *Editor) ui.Box {
	if editor == nil {
		panic("codeeditor: View needs its Editor")
	}
	editor.prepare()
	return ui.Control(c, label, &editor.handlers).Draw(editor.drawFn)
}

// Parts is the scope of this control's own hit regions. Caller-owned hits
// painted by a snapshot (gutter markers and fold markers) may name themselves
// in it so this control receives their pointer events; the scrollbars use At(0)
// and At(1) when the snapshot enables them. It is empty until View has declared
// the control at least once.
func (e *Editor) Parts() gfx.Scope { return e.handlers.Parts() }

func (e *Editor) prepare() {
	if e.wired {
		return
	}
	e.wired = true
	e.handlers.Role = gfx.RoleTextField
	e.handlers.Key = e.key
	e.handlers.Pointer = e.pointer
	e.handlers.Scroll = e.scroll
	e.handlers.Text = e.text
	e.drawFn = e.draw
}

func (e *Editor) key(event gfx.KeyEvent) gfx.EventResult {
	if e.Key == nil {
		return gfx.EventResult{}
	}
	return e.Key(event)
}

func (e *Editor) pointer(event gfx.PointerEvent) gfx.EventResult {
	if result := e.Code.Pointer(event); result.Handled {
		return result
	}
	if e.Pointer == nil {
		return gfx.EventResult{}
	}
	return e.Pointer(event)
}

func (e *Editor) scroll(event gfx.ScrollEvent) gfx.EventResult {
	if result := e.Code.Scroll(event); result.Handled {
		return result
	}
	if e.Scroll == nil {
		return gfx.EventResult{}
	}
	return e.Scroll(event)
}

func (e *Editor) text(context window.TextInputContext) *window.TextInput {
	if e.Text == nil {
		return nil
	}
	return e.Text(context)
}

func (e *Editor) draw(d *ui.DrawContext) {
	snapshot := e.Snapshot
	if e.Document != nil {
		snapshot.Value = e.Document.String()
		snapshot.Caret = e.Document.ByteOffset(e.Document.Caret())
		selection := e.Document.Selection()
		snapshot.SelectionStart = e.Document.ByteOffset(selection.Start)
		snapshot.SelectionEnd = e.Document.ByteOffset(selection.End)
		snapshot.CompositionStart, snapshot.CompositionEnd = 0, 0
		if composition, ok := e.Document.Composition(); ok {
			snapshot.CompositionStart = e.Document.ByteOffset(composition.Start)
			snapshot.CompositionEnd = e.Document.ByteOffset(composition.End)
		}
	}
	snapshot.ID = d.Node.ID
	snapshot.Label = d.Label
	parts := e.handlers.Parts()
	if !snapshot.ScrollbarID.IsZero() {
		snapshot.ScrollbarID = parts.At(0)
	}
	if !snapshot.HorizontalScrollbarID.IsZero() {
		snapshot.HorizontalScrollbarID = parts.At(1)
	}
	e.Code.Paint(d.Canvas, *d.Theme, d.Rect, snapshot)
}
