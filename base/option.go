package base

import "github.com/vivaui/viva/gfx"

// Option is a setting a control takes after its fixed arguments, written at the
// call. It is a small value, not a function, so passing one costs no allocation.
type Option struct {
	id gfx.ID
}

// WithID names the control with an identity the application already holds, so the
// page's input routing, an initial focus or a test can address it. A control given
// no WithID takes the name its bound value or its place gives it.
//
//	ui.TextField(c, &m.query, "Search", "", ui.WithID(m.searchID))
func WithID(id gfx.ID) Option { return Option{id: id} }

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
