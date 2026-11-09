package ui

import (
	"unsafe"

	"github.com/vivaui/viva/gfx"
)

// TextValue is what a TextField or TextArea edits: a *string of the model, which
// the field shows in every paint and writes when the user edits it, or a *Field,
// for a field whose caret and selection the program reads or moves too (a command
// line, a find bar).
//
//	ui.TextField(c, &m.name, "Name", "")    // m.name is a string
//	ui.TextField(c, &m.query, "Search", "") // m.query is a ui.Field
type TextValue interface{ *string | *Field }

// boundText is the Field a page keeps for a text field bound to a string: the
// editor, its caret and its input method live here, and synced is the text the
// field and the string last agreed on.
type boundText struct {
	field  Field
	synced string
}

// fieldOf names the text field bound to value and returns the Field that edits it.
// A string is kept in step with a Field of the page's: what the user typed since
// the last paint is written to *value, and a value the program changed is put in
// the field with the caret at its end. When both changed, what the user typed is
// kept, as the newer edit.
func fieldOf[T TextValue](p *Page, kind uint8, value T) (*Field, gfx.ID) {
	switch v := any(value).(type) {
	case *Field:
		return v, p.boundID(kind, unsafe.Pointer(v))
	case *string:
		id := p.boundID(kind, unsafe.Pointer(v))
		bound := p.texts[id]
		if bound == nil {
			bound = &boundText{}
			bound.field.SetText(*v)
			bound.synced = *v
			p.texts[id] = bound
		}
		switch typed := bound.field.Text(); {
		case typed != bound.synced:
			*v, bound.synced = typed, typed
		case *v != bound.synced:
			bound.field.SetText(*v)
			bound.synced = *v
		}
		return &bound.field, id
	}
	panic("unreachable: TextValue is *string or *Field")
}

// sweepTexts forgets the fields of strings whose names the page no longer keeps:
// a field a Memo copies keeps its name, and so its editor.
func (p *Page) sweepTexts() {
	for id := range p.texts {
		if _, ok := p.names[id]; !ok {
			delete(p.texts, id)
		}
	}
}

// Secure shows the field's text as bullets and keeps it out of accessibility, the
// clipboard and the platform's suggestions: a password. It is Field.Secure for a
// field bound to a string.
//
//	ui.TextField(c, &m.password, "Password", "").Secure()
func (b Box) Secure() Box {
	n := b.n()
	if n.kind != kindInput {
		panic("ui: Secure is for a TextField")
	}
	b.f.inputs[n.aux].Secure = true // what the field paints and the platform is told
	return b
}
