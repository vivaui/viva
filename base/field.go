package base

import (
	"unicode/utf8"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/window"
)

// Field is one single-line text field's state, kept in the application's model; the
// zero value is an empty field. It owns the editor the platform's text service
// edits, so typing, the caret, selection, clipboard and input methods work without
// any further wiring beyond giving Fields to Config.TextInput.
//
// A Field and its bound editor belong to the window thread. All access, including
// Text, Bind and Secure, must happen there once the field is attached to a window.
// Pass the string returned by Text to background work, not the Field itself;
// apply results on the window thread with EffectState.Take or Config.OnWake.
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
	// Invalid paints the field as one whose value is wrong, with the destructive
	// border and ring components.Input gives an invalid input. It is the application's
	// to set and clear, as Secure is; it changes the look only.
	Invalid bool
	// Disabled paints the field dimmed and takes it out of the Tab order and the pointer's
	// reach, as components.Input does for a disabled input. It is the application's to set
	// and clear, as Secure is.
	Disabled bool
	// ReadOnly keeps the field's ordinary look and its value visible, but takes no
	// edits: the platform is given no editor for it, so typing, pasting and input
	// methods leave it alone, and no caret is painted. It still takes the focus, as
	// components.Input's read-only field does, so the value can be read and its text
	// cursor seen. It is the application's to set and clear, as Disabled is; a field
	// bound to a string takes Box.ReadOnly instead.
	ReadOnly bool
	// Hint is the description assistive technology reads after the field's name and value, for
	// what they do not say. It is the application's to set and clear, as Secure is.
	Hint string
	// Alignment places the value inside the field, as components.Input's
	// Alignment: gfx.AlignCenter for a value centred in it, gfx.AlignRight (or
	// AlignEnd) for one against its end. The zero value starts the text.
	Alignment gfx.Alignment

	editor *text.Editor
	ID     gfx.ID
	// input is the stable TextInput the platform provider hands out. Returning
	// a fresh one on every repaint allocated once per focused frame.
	input window.TextInput
	// blink is the phase of the caret when the field paints it itself, which a fixed-pitch
	// field does (see Box.Mono): the platform draws the caret of the others.
	blink CaretBlink
	// seen is the text as the field was last declared, and changed whether that
	// declaration found it edited since the one before (Box.Changed).
	seen    string
	changed bool
	// spans is the scratch the secondary selections' byte offsets are copied
	// into each paint.
	spans []text.Span
	// area lays a field out in rows when it is declared as a FieldArea; nil until it
	// is. rows is whether the last declaration was one: a field declared as a
	// FieldArea once and as a Field since keeps its layout but moves by its line.
	area *TextareaLayout
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

// Text returns an immutable snapshot of what is in the field. Call it on the
// window thread before starting background work: it may initialize the editor
// and populate its text cache, so concurrent calls are not safe either.
func (field *Field) Text() string { return field.ensure().String() }

// SetText replaces the contents and puts the caret at the end. Like Text, it
// must run on the window thread, not in a background worker.
func (field *Field) SetText(Value string) {
	editor := field.ensure()
	editor.SetSelection(text.Range{Start: 0, End: editor.RuneCount()})
	editor.Insert(Value)
	field.seen = editor.String() // the program's own change is not an edit
}

// Select selects the runes [start, end) of the field, placing the caret at end,
// as a drag or Shift-click does. A field painted read-only shows the selection
// without offering an editor. Like SetText, it must run on the window thread.
// The page shows the selection in the next paint.
//
//	m.preview.Select(0, 6)
//	ui.TextFieldID(c, id, &m.preview, "Preview", "").ReadOnly(true)
func (field *Field) Select(start, end int) {
	field.ensure().SetSelection(text.Range{Start: start, End: end})
}

// Field adds a text field for field, named id (a stable identity from your Tree),
// with an accessible label and placeholder. It is Input with the snapshot taken from
// the field: focus, platform editing and the caret are filled in for you.
func (f *Frame) Field(field *Field, ID gfx.ID, Label, placeholder string) Box {
	return f.Input(field.Snapshot(f, ID, Label, placeholder))
}

// FieldBase is Field without a look, as InputBase is Input without one: the same editor,
// focus, caret and platform editing, with the element's own Background, Border, Pad and
// looks around them. It is what a field that does not read as a form control is built
// from, such as the editor of a table cell.
//
//	f.FieldBase(&m.cell, cellID, "Cell", "").Pad(8).Border(2, theme.Primary).Radius(0)
func (f *Frame) FieldBase(field *Field, ID gfx.ID, Label, placeholder string) Box {
	return f.InputBase(field.Snapshot(f, ID, Label, placeholder))
}

// ReadOnly makes the TextField or TextArea of the box read-only:
// ReadOnly(true) keeps its ordinary look and its value visible, but gives the
// platform no editor, paints no caret and applies no edit, as Field.ReadOnly
// does from the field's own state. It still takes the focus, so the value stays
// readable and selectable. ReadOnly(false) offers editing again.
//
//	ui.TextField(c, &m.token, "Token", "").ReadOnly(true)
//	ui.TextArea(c, &m.notes, "Notes", "").Height(120).ReadOnly(m.archived)
//
// It is the chained form for a field bound to a string; a *Field sets
// Field.ReadOnly where the model is made.
func (b Box) ReadOnly(readOnly bool) Box {
	n := b.N()
	switch n.Kind {
	case kindInput:
		snapshot := &WidgetsOf(b.f).inputs[n.Aux]
		snapshot.ReadOnly = readOnly
		if readOnly {
			snapshot.ShowCaret = false
		}
	case kindTextarea:
		snapshot := &WidgetsOf(b.f).textareas[n.Aux]
		snapshot.ReadOnly = readOnly
		if readOnly {
			snapshot.ShowCaret = false
		}
	default:
		panic("ui: ReadOnly is for a TextField or TextArea")
	}
	return b
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
	n := b.N()
	if n.Kind != kindInput || b.f.Surface == nil {
		return false
	}
	return b.f.Surface.ClickedByKey(WidgetsOf(b.f).inputs[n.Aux].ID)
}

// OnKey gives the field a handler for the keys pressed while it has the
// keyboard, before the page routes them anywhere else: a search field whose Up
// and Down move a list below it, and whose Escape closes it. The handler is a
// chain on the TextField or TextFieldID box:
//
//	search := ui.TextField(c, &m.query, "Search", "")
//	search.OnKey(func(event gfx.KeyEvent) gfx.EventResult {
//		switch event.Key {
//		case gfx.KeyDown:
//			m.next()
//			return gfx.EventResult{Handled: true, Repaint: true}
//		case gfx.KeyEnter:
//			m.open()
//			return gfx.EventResult{Handled: true}
//		}
//		return gfx.EventResult{}
//	})
//
// Returning Handled claims the key; returning anything else leaves it to the
// page's other routes, so Enter can still submit the form and Escape can still
// close an overlay. Keys that type text are never offered, so typing always
// reaches the field. The handler also switches the field to NavigationKeys:
// Up, Down, PageUp and PageDown reach the application instead of moving the
// caret, which is what a command palette needs.
//
// Not inside a Memo, whose copied fields are not declared.
func (b Box) OnKey(handler func(gfx.KeyEvent) gfx.EventResult) Box {
	n := b.N()
	if n.Kind != kindInput {
		panic("ui: OnKey is for a TextField")
	}
	if handler == nil {
		panic("ui: OnKey needs a handler")
	}
	if b.f.memoDepth > 0 {
		panic("ui: OnKey in a Memo: a copied field is not declared, so its handler would disappear; declare it outside the Memo")
	}
	p := b.f.page
	if p == nil {
		panic("ui: OnKey is for a TextField of a page")
	}
	for i := len(p.fields) - 1; i >= 0; i-- {
		if p.fields[i].ID == n.ID {
			p.fields[i].key = handler
			if p.fieldKeyRoute == nil {
				p.keyRoute, p.fieldKeyRoute = (*Page).RouteKey, (*Page).FieldKey
			}
			WidgetsOf(b.f).inputs[n.Aux].NavigationKeys = true
			return b
		}
	}
	panic("ui: OnKey must be chained on a TextField or TextFieldID")
}

// Fields answers Config.TextInput for the fields: the platform asks which editor
// belongs to the focused control, and gets the one of the field declared with that
// identity.
func Fields(fields ...*Field) func(window.TextInputContext) *window.TextInput {
	return func(context window.TextInputContext) *window.TextInput {
		for _, field := range fields {
			if field.ID == context.ID && !context.ID.IsZero() {
				field.input.Alignment = context.Alignment
				field.input.Editor = field.ensure()
				return &field.input
			}
		}
		return nil
	}
}

// snapshot is what the field is painted from: focus, platform editing and the caret are
// filled in for the frame being declared.
func (field *Field) Snapshot(f *Frame, ID gfx.ID, Label, placeholder string) InputSnapshot {
	field.rows = false // FieldArea sets it again after
	editor := field.ensure()
	field.ID = ID
	value := editor.String()
	field.changed = value != field.seen
	field.seen = value
	focused := f.Surface != nil && f.Surface.Focused() == ID
	selection := editor.Selection()
	snapshot := InputSnapshot{
		ID: ID, Label: Label, Value: value, Placeholder: placeholder, Secure: field.Secure, Invalid: field.Invalid, Disabled: field.Disabled, ReadOnly: field.ReadOnly, Hint: field.Hint,
		Alignment: field.Alignment,
		Focused:   focused,
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
	snapshot.ShowCaret = focused && selection.Start == selection.End && !field.ReadOnly
	if editor.SelectionCount() > 1 {
		// A field bound to a multi-selection editor paints every selection; the
		// platform's own editor behind it still sees only the primary one. The
		// field is the editor's owner, not a copy: a caller that adds a selection
		// sees it without another declaration.
		snapshot.SelectionRanges = editor.SelectionSpansInto(field.spans)
		field.spans = snapshot.SelectionRanges
	}
	if marked, ok := editor.Composition(); ok {
		snapshot.CompositionStart, snapshot.CompositionEnd = editor.ByteOffset(marked.Start), editor.ByteOffset(marked.End)
	}
	return snapshot
}

// FieldArea is Field in several rows, with the shadcn textarea look: the same
// editor, focus and platform editing, Enter starting a new line, long lines wrapped
// and the rows scrolled to follow the caret. Give it a Height (it is one row tall
// otherwise). It has no Submitted: Enter is a new line.
//
//	f.FieldArea(&m.notes, notesID, "Notes", "Anything to add").Height(120)
func (f *Frame) FieldArea(field *Field, ID gfx.ID, Label, placeholder string) Box {
	input := field.Snapshot(f, ID, Label, placeholder)
	if field.area == nil {
		field.area = new(TextareaLayout)
	}
	field.rows = true
	return f.Textarea(TextareaSnapshot{
		ID: input.ID, Label: input.Label, Value: input.Value, DisplayValue: input.Value, Placeholder: placeholder,
		Hint: input.Hint, Disabled: input.Disabled, ReadOnly: input.ReadOnly, Invalid: input.Invalid,
		Focused: input.Focused, HideEditorText: true, ShowCaret: input.ShowCaret, Layout: field.area,
		Caret: input.Caret, SelectionStart: input.SelectionStart, SelectionEnd: input.SelectionEnd,
		SelectionRanges:  input.SelectionRanges,
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
	if !field.rows || event.Target != field.ID || field.ID.IsZero() {
		return gfx.EventResult{}
	}
	movement, extend := TextareaMovementForKey(event.Key, event.Modifiers)
	if movement == TextareaMoveNone {
		return gfx.EventResult{}
	}
	editor := field.ensure()
	value := editor.String()
	to := field.area.Move(value, editor.ByteOffset(editor.Caret()), movement)
	editor.MoveTo(utf8.RuneCountInString(value[:to]), extend)
	return gfx.EventResult{Handled: true, Repaint: true}
}
