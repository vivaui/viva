package ui

import (
	"unicode/utf8"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

// Field is one single-line text field's state, kept in the application's model; the
// zero value is an empty field. It owns the editor the platform's text service
// edits, so typing, the caret, selection, clipboard and input methods work without
// any further wiring beyond giving Fields to Config.TextInput.
//
//	type model struct{ name, password ui.Field }
//
//	m.password.Secure = true
//	config.TextInput = ui.Fields(&m.name, &m.password)
//	f.Field(&m.name, nameID, "Name", "Your name")
//	f.Field(&m.password, passwordID, "Password", "")
//	m.name.Text()                       // what was typed
type Field struct {
	// Secure shows bullets instead of the text and keeps it out of accessibility.
	Secure bool

	editor *text.Editor
	id     gfx.ID
	// input is the stable TextInput the platform provider hands out. Returning
	// a fresh one on every repaint allocated once per focused frame.
	input app.TextInput
	// blink is the phase of the caret when the field paints it itself, which a fixed-pitch
	// field does (see Box.Mono): the platform draws the caret of the others.
	blink components.CaretBlink
	// seen is the text as the field was last declared, and changed whether that
	// declaration found it edited since the one before (Box.Changed).
	seen    string
	changed bool
	// area lays a field out in rows when it is declared as a FieldArea; nil until it
	// is. rows is whether the last declaration was one: a field declared as a
	// FieldArea once and as a Field since keeps its layout but moves by its line.
	area *components.TextareaLayout
	rows bool
}

func (field *Field) ensure() *text.Editor {
	if field.editor == nil {
		field.editor = text.NewEditor("")
	}
	return field.editor
}

// Bind makes the field edit editor, an editor the application already has and reads
// (its caret, its selection, its undo), instead of one of its own: for a field the
// program drives itself, such as a search box that moves the caret on its own keys.
// The field is the page's handle on that editor, not a copy; what the editor holds
// when bound is not an edit, as SetText's is not. Bind once, where the model is
// made, as the field is kept there.
//
//	m.searchField.Bind(&m.search)
//	ui.TextFieldID(c, m.searchID, &m.searchField, "Search", "")
func (field *Field) Bind(editor *text.Editor) {
	if editor == nil {
		panic("ui: Field.Bind needs an editor")
	}
	field.editor = editor
	field.seen = editor.String()
}

// Text is what is in the field.
func (field *Field) Text() string { return field.ensure().String() }

// SetText replaces the contents and puts the caret at the end.
func (field *Field) SetText(value string) {
	editor := field.ensure()
	editor.SetSelection(text.Range{Start: 0, End: editor.RuneCount()})
	editor.Insert(value)
	field.seen = editor.String() // the program's own change is not an edit
}

// Field adds a text field for field, named id (a stable identity from your Tree),
// with an accessible label and placeholder. It is Input with the snapshot taken from
// the field: focus, platform editing and the caret are filled in for you.
func (f *frame) Field(field *Field, id gfx.ID, label, placeholder string) Box {
	return f.Input(field.snapshot(f, id, label, placeholder))
}

// FieldBase is Field without a look, as InputBase is Input without one: the same editor,
// focus, caret and platform editing, with the element's own Background, Border, Pad and
// looks around them. It is what a field that does not read as a form control is built
// from, such as the editor of a table cell.
//
//	f.FieldBase(&m.cell, cellID, "Cell", "").Pad(8).Border(2, theme.Primary).Radius(0)
func (f *frame) FieldBase(field *Field, id gfx.ID, label, placeholder string) Box {
	return f.InputBase(field.snapshot(f, id, label, placeholder))
}

// FieldArea is Field in several rows, with the shadcn textarea look: the same
// editor, focus and platform editing, Enter starting a new line, long lines wrapped
// and the rows scrolled to follow the caret. Give it a Height (it is one row tall
// otherwise). It has no Submitted: Enter is a new line.
//
//	f.FieldArea(&m.notes, notesID, "Notes", "Anything to add").Height(120)
func (f *frame) FieldArea(field *Field, id gfx.ID, label, placeholder string) Box {
	input := field.snapshot(f, id, label, placeholder)
	if field.area == nil {
		field.area = new(components.TextareaLayout)
	}
	field.rows = true
	return f.Textarea(components.TextareaSnapshot{
		ID: input.ID, Label: input.Label, Value: input.Value, DisplayValue: input.Value, Placeholder: placeholder,
		Focused: input.Focused, HideEditorText: true, ShowCaret: input.ShowCaret, Layout: field.area,
		Caret: input.Caret, SelectionStart: input.SelectionStart, SelectionEnd: input.SelectionEnd,
		CompositionStart: input.CompositionStart, CompositionEnd: input.CompositionEnd,
	})
}

// Key moves the caret of a FieldArea by the rows it shows: Up and Down to the row
// above and below at the same place across, Home and End to the start and end of
// the row, with Shift to select. The platform's editor knows one line, so these
// are the field's. It returns Handled for a key it acted on; give it the window's
// keys before anything else that would take them (a page does this for its
// TextAreas):
//
//	if result := m.notes.Key(event); result.Handled { return result }
func (field *Field) Key(event gfx.KeyEvent) gfx.EventResult {
	if !field.rows || event.Target != field.id || field.id.IsZero() {
		return gfx.EventResult{}
	}
	movement, extend := components.TextareaMovementForKey(event.Key, event.Modifiers)
	if movement == components.TextareaMoveNone {
		return gfx.EventResult{}
	}
	editor := field.ensure()
	value := editor.String()
	to := field.area.Move(value, editor.ByteOffset(editor.Caret()), movement)
	editor.MoveTo(utf8.RuneCountInString(value[:to]), extend)
	return gfx.EventResult{Handled: true, Repaint: true}
}

// snapshot is what the field is painted from: focus, platform editing and the caret are
// filled in for the frame being declared.
func (field *Field) snapshot(f *frame, id gfx.ID, label, placeholder string) components.InputSnapshot {
	field.rows = false // FieldArea sets it again after
	editor := field.ensure()
	field.id = id
	value := editor.String()
	field.changed = value != field.seen
	field.seen = value
	focused := f.canvas != nil && f.canvas.Focused() == id
	selection := editor.Selection()
	snapshot := components.InputSnapshot{
		ID: id, Label: label, Value: value, Placeholder: placeholder, Secure: field.Secure,
		Focused: focused,
		// The field paints its own text, caret, selection and composition, and the
		// platform's editor is only where input arrives. A platform that lays text out
		// with its own metrics can put the line a fraction of a pixel away from the
		// canvas line, which moves the value when the field gains the keyboard.
		HideEditorText: true,
		Blink:          &field.blink,
	}
	// Where the caret, the selection and the composition are: what a field that paints them
	// itself needs, and what the platform's own presentation of them does without.
	snapshot.Caret = editor.ByteOffset(editor.Caret())
	snapshot.SelectionStart, snapshot.SelectionEnd = editor.ByteOffset(selection.Start), editor.ByteOffset(selection.End)
	snapshot.ShowCaret = focused && selection.Start == selection.End
	if marked, ok := editor.Composition(); ok {
		snapshot.CompositionStart, snapshot.CompositionEnd = editor.ByteOffset(marked.Start), editor.ByteOffset(marked.End)
	}
	return snapshot
}

// Submitted reports whether this paint is delivering Enter pressed in the text field the
// box declares (f.Field or f.Input), and false in every other paint: a click, a typed
// character or a held key does not count. It is how a chat box sends and a search box
// runs, where the field is declared, like Clicked:
//
//	if f.Field(&m.draft, draftID, "Message", "Type a message").Submitted() { m.send(); m.draft.SetText("") }
//
// A field is one line, so Enter is free to mean "done"; a Textarea keeps Enter for new
// lines and has no Submitted.
func (b Box) Submitted() bool {
	n := b.n()
	if n.kind != kindInput || b.f.canvas == nil {
		return false
	}
	return b.f.canvas.ClickedByKey(b.f.inputs[n.aux].ID)
}

// Fields answers Config.TextInput for the fields: the platform asks which editor
// belongs to the focused control, and gets the one of the field declared with that
// identity.
func Fields(fields ...*Field) func(app.TextInputContext) *app.TextInput {
	return func(context app.TextInputContext) *app.TextInput {
		for _, field := range fields {
			if field.id == context.ID && !context.ID.IsZero() {
				field.input.Editor = field.ensure()
				return &field.input
			}
		}
		return nil
	}
}
