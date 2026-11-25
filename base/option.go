package base

import "github.com/vivaui/viva/gfx"

// Option is a setting a control takes after its fixed arguments, written at the
// call. It is a small value, not a function, so passing one costs no allocation.
type Option struct {
	id       gfx.ID
	maxRunes int
	count    bool
	plain    bool // WithoutEditMenu
}

// WithID names the control with an identity the application already holds, so the
// page's input routing, an initial focus or a test can address it. A control given
// no WithID takes the name its bound value or its place gives it.
//
//	ui.TextField(c, &m.query, "Search", "", ui.WithID(m.searchID))
func WithID(id gfx.ID) Option { return Option{id: id} }

// MaxLength caps a TextField or TextArea at n runes: what is typed, pasted or
// composed beyond it is cut off the end, and text already longer is left alone,
// as text.Editor.Restrict does. Zero or less is no cap.
//
//	ui.TextField(c, &m.name, "Name", "", ui.MaxLength(32))
func MaxLength(n int) Option { return Option{maxRunes: n} }

// ShowCount shows how many runes a TextField or TextArea holds, and of how many
// when MaxLength caps it ("12/32"), at the end of the field. It takes the place of
// an Action.
//
//	ui.TextField(c, &m.title, "Title", "", ui.MaxLength(80), ui.ShowCount())
func ShowCount() Option { return Option{count: true} }

// limitsOf is the cap and the count opts ask a text control for.
func limitsOf(opts []Option) (maxRunes int, count bool) {
	for _, o := range opts {
		if o.maxRunes != 0 {
			maxRunes = o.maxRunes
		}
		count = count || o.count
	}
	return max(maxRunes, 0), count
}

// namedBy is the identity opts give a control, zero when they give none. A control
// has one name, so a second WithID is a mistake in the call.
func namedBy(control string, opts []Option) gfx.ID {
	var id gfx.ID
	for _, o := range opts {
		if o.id.IsZero() {
			continue
		}
		if !id.IsZero() {
			panic("ui: " + control + " takes one WithID")
		}
		id = o.id
	}
	return id
}

// namedFieldOf is fieldOf for a field that may carry the application's name. A
// named field keeps its editor under the name its value is given, as fieldOf keeps
// it; only the element takes the application's identity. A *Field is the editor
// itself, so naming it declares nothing for it in the page.
func namedFieldOf[T TextValue](p *Page, Kind uint8, Value T, named gfx.ID) (*Field, gfx.ID) {
	if named.IsZero() {
		return fieldOf(p, Kind, Value)
	}
	if v, ok := any(Value).(*Field); ok {
		return v, named
	}
	field, _ := fieldOf(p, Kind, Value)
	return field, named
}
