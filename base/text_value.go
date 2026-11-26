package base

import (
	"unicode/utf8"
	"unsafe"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
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
// the field as the edit that turns the field's text into it (see Field.follow): the
// caret and selection keep to the text they were on. When both changed, what the user
// typed is kept, as the newer edit.
func fieldOf[T TextValue](p *Page, Kind uint8, Value T) (*Field, gfx.ID) {
	switch v := any(Value).(type) {
	case *Field:
		return v, p.boundID(Kind, unsafe.Pointer(v))
	case *string:
		id := p.boundID(Kind, unsafe.Pointer(v))
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
			bound.field.follow(*v)
			bound.synced = *v
		}
		return &bound.field, id
	}
	panic("unreachable: TextValue is *string or *Field")
}

// follow puts value in the field as the program's edit of the text it holds: only the
// runes between the start and the end the two share are replaced, so a long text that
// grows at its end, as a log does, costs what was added, and the caret and selection keep
// to the text they were on. A caret at the end of the text stays at its end as text is
// added there, and one inside replaced text goes to the end of what replaced it. Like
// SetText, the change is not the user's edit. A field being composed by an input method,
// or one that restricts what it takes, is set as SetText sets it.
func (field *Field) follow(value string) {
	editor := field.ensure()
	old := editor.String()
	if _, composing := editor.Composition(); composing || old == value {
		if composing {
			field.SetText(value)
		}
		return
	}
	prefix := commonPrefix(old, value)
	for prefix > 0 && (prefix < len(old) && !utf8.RuneStart(old[prefix]) || prefix < len(value) && !utf8.RuneStart(value[prefix])) {
		prefix--
	}
	suffix := commonSuffix(old[prefix:], value[prefix:])
	for suffix > 0 && !utf8.RuneStart(old[len(old)-suffix]) {
		suffix--
	}
	// Both ends are rune boundaries, which the editor's index turns into rune indexes
	// without counting the runes before them.
	start, end := editor.RuneIndex(prefix), editor.RuneIndex(len(old)-suffix)
	inserted := value[prefix : len(value)-suffix]
	shift := utf8.RuneCountInString(inserted) - (end - start)
	last := editor.RuneCount()
	place := func(at int) int {
		switch {
		case at < start || at == start && (at != end || at != last):
			return at
		case at >= end:
			return at + shift
		default:
			return end + shift
		}
	}
	selection, caret := editor.Selection(), editor.Caret()
	anchor := selection.Start
	if caret == selection.Start {
		anchor = selection.End
	}
	after := text.Range{Start: place(anchor), End: place(caret)}
	edits := [1]text.Replacement{{Range: text.Range{Start: start, End: end}, Text: inserted}}
	if editor.ReplaceRangesWith(edits[:], text.ReplaceRangesOptions{AfterSelection: &after}) != nil {
		field.SetText(value)
		return
	}
	field.seen = editor.String() // the program's own change is not an edit
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

// NavigationKeys sends Up, Down, PageUp and PageDown to the application while the
// platform edits the field, instead of moving its caret: for a search box whose
// arrows move a list's selection. Box.OnKey is the handler form, and turns this on
// for the field.
//
//	ui.TextField(c, &m.query, "Search", "").NavigationKeys()
func (b Box) NavigationKeys() Box {
	n := b.N()
	if n.Kind != kindInput {
		panic("ui: NavigationKeys is for a TextField")
	}
	WidgetsOf(b.f).inputs[n.Aux].NavigationKeys = true
	return b
}

// Secure shows the field's text as bullets and keeps it out of accessibility, the
// clipboard and the platform's suggestions: a password. It is Field.Secure for a
// field bound to a string, for a TextField as for the rows of a TextArea.
//
//	ui.TextField(c, &m.password, "Password", "").Secure()
//	ui.TextArea(c, &m.privateKey, "Private key", "").Secure()
func (b Box) Secure() Box {
	n := b.N()
	switch n.Kind {
	case kindInput:
		WidgetsOf(b.f).inputs[n.Aux].Secure = true // what the field paints and the platform is told
	case kindTextarea:
		WidgetsOf(b.f).textareas[n.Aux].Secure = true
	default:
		panic("ui: Secure is for a TextField or TextArea")
	}
	return b
}
