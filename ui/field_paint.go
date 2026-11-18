package ui

import (
	"unicode/utf8"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/style"
)

// FieldArea is Field in several rows, with the shadcn textarea look: the same
// editor, focus and platform editing, Enter starting a new line, long lines wrapped
// and the rows scrolled to follow the caret. Give it a Height (it is one row tall
// otherwise). It has no Submitted: Enter is a new line.
//
//	f.FieldArea(&m.notes, notesID, "Notes", "Anything to add").Height(120)
func (f *frame) FieldArea(field *Field, ID gfx.ID, Label, placeholder string) Box {
	input := field.snapshot(f, ID, Label, placeholder)
	if field.area == nil {
		field.area = new(components.TextareaLayout)
	}
	field.rows = true
	return f.Textarea(components.TextareaSnapshot{
		ID: input.ID, Label: input.Label, Value: input.Value, DisplayValue: input.Value, Placeholder: placeholder,
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
func (field *Field) snapshot(f *frame, ID gfx.ID, Label, placeholder string) style.InputSnapshot {
	field.rows = false // FieldArea sets it again after
	editor := field.ensure()
	field.ID = ID
	value := editor.String()
	field.changed = value != field.seen
	field.seen = value
	focused := f.Surface != nil && f.Surface.Focused() == ID
	selection := editor.Selection()
	snapshot := style.InputSnapshot{
		ID: ID, Label: Label, Value: value, Placeholder: placeholder, Secure: field.Secure, Invalid: field.Invalid, Disabled: field.Disabled, Hint: field.Hint,
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
	snapshot.ShowCaret = focused && selection.Start == selection.End
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
