// Package autocomplete is the opt-in page control for a text field that suggests
// completions while the user types: a list of the options that contain what was
// typed opens under the field, and choosing one writes it into the field. Unlike a
// Combobox, the value stays free text: the user may type something that is in no
// option and keep it. A page keeps the field's value in a ui.Field and an
// Autocomplete in its model:
//
//	m.host = autocomplete.New()
//	// in the page:
//	m.host.View(c, &m.hostField, "Host", "Hostname", autocomplete.Spec{Options: m.known})
//	if option, ok := m.host.Chosen(); ok {
//		// the field already holds option
//	}
//
// The list opens while the field has the keyboard and something is typed that at
// least one option contains, without regard to case, and an option equal to the
// text is not offered. Up and Down move through the options, Enter and Tab choose
// the highlighted one, and Escape closes the list until the text changes. Without
// a highlighted option Enter is left to the page, so it still submits a form. A
// press on an option chooses it without taking the keyboard from the field.
//
// It is a separate package so an application that does not import it pays
// nothing for it. Keep an Autocomplete in the model beside its field. Do not copy
// one after it has been declared.
package autocomplete

import (
	"unicode"
	"unicode/utf8"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

// Spec declares the list.
type Spec struct {
	// Options are the completions on offer, in the order the list shows the ones
	// that match. Keep the slice in the model.
	Options []string
	// MaxRows is how many options the list shows at most; 6 when zero.
	MaxRows int
}

// Autocomplete is one field's list.
type Autocomplete struct {
	open      bool
	matches   []int
	highlight int // the row highlighted, or -1 until a key moves it
	typed     string
	dismissed string // the text Escape closed the list for
	pending   int    // 1 + the option chosen since the last paint, or 0
	chosen    int
	spec      Spec
	field     *ui.Field
	id        gfx.ID
	fieldID   gfx.ID
	key       func(gfx.KeyEvent) gfx.EventResult
}

// New returns a closed list.
func New() *Autocomplete {
	a := &Autocomplete{highlight: -1}
	a.key = a.onKey
	return a
}

// FieldID names the field with an identity the application already holds, for a
// runner that resolves the platform text service by an application identity.
func (a *Autocomplete) FieldID(id gfx.ID) *Autocomplete { a.fieldID = id; return a }

// Chosen reports the option the user chose since the paint before this one.
func (a *Autocomplete) Chosen() (string, bool) {
	if a.chosen == 0 {
		return "", false
	}
	return a.spec.Options[a.chosen-1], true
}

// containsFold reports whether s has query in it, without regard to case.
func containsFold(s, query string) bool {
	if query == "" {
		return true
	}
	for i := range s {
		rest, q := s[i:], query
		for q != "" && rest != "" {
			x, xs := utf8.DecodeRuneInString(rest)
			y, ys := utf8.DecodeRuneInString(q)
			if unicode.ToLower(x) != unicode.ToLower(y) {
				break
			}
			rest, q = rest[xs:], q[ys:]
		}
		if q == "" {
			return true
		}
	}
	return false
}

func (a *Autocomplete) refresh(c ui.C) bool {
	a.matches = a.matches[:0]
	editor := a.field.GetEditor()
	if editor == nil || c.Canvas().Focused() != a.id {
		return false
	}
	value := editor.String()
	if value != a.typed {
		a.typed, a.highlight, a.dismissed = value, -1, ""
	}
	if value == "" || value == a.dismissed {
		return false
	}
	limit := a.spec.MaxRows
	if limit <= 0 {
		limit = 6
	}
	for i, option := range a.spec.Options {
		if len(a.matches) == limit {
			break
		}
		if option != value && containsFold(option, value) {
			a.matches = append(a.matches, i)
		}
	}
	a.highlight = min(a.highlight, len(a.matches)-1)
	return len(a.matches) > 0
}

func (a *Autocomplete) onKey(event gfx.KeyEvent) gfx.EventResult {
	if !a.open || event.Release {
		return gfx.EventResult{}
	}
	switch event.Key {
	case gfx.KeyDown:
		a.highlight = (a.highlight + 1) % len(a.matches)
		return gfx.EventResult{Handled: true, Repaint: true}
	case gfx.KeyUp:
		if a.highlight <= 0 {
			a.highlight = len(a.matches) - 1
		} else {
			a.highlight--
		}
		return gfx.EventResult{Handled: true, Repaint: true}
	case gfx.KeyEnter, gfx.KeyTab:
		if a.highlight < 0 {
			return gfx.EventResult{}
		}
		a.choose(a.matches[a.highlight])
		return gfx.EventResult{Handled: true, Repaint: true}
	case gfx.KeyEscape:
		a.dismissed, a.open = a.typed, false
		return gfx.EventResult{Handled: true, Repaint: true}
	}
	return gfx.EventResult{}
}

// choose writes the option in place of the text.
func (a *Autocomplete) choose(index int) {
	editor := a.field.GetEditor()
	if editor == nil {
		return
	}
	editor.SetSelection(text.Range{Start: 0, End: editor.RuneCount()})
	editor.Insert(a.spec.Options[index])
	a.pending, a.open = index+1, false
}

// View declares the field and, while a completion is on offer, the list under it.
// label and placeholder are the field's. Declare the view on its own, not inside
// a Memo.
func (a *Autocomplete) View(c ui.C, field *ui.Field, label, placeholder string, spec Spec) ui.Box {
	if a == nil {
		panic("autocomplete: View needs its Autocomplete")
	}
	a.chosen, a.pending = a.pending, 0
	a.spec, a.field = spec, field
	if a.id = a.fieldID; a.id.IsZero() {
		a.id = ui.ID(c)
	}
	a.open = a.refresh(c)
	box := ui.TextField(c, field, label, placeholder, ui.WithID(a.id)).OnKey(a.key)
	if a.open {
		ui.Popover(c, box, &a.open, ui.PopoverSpec{
			Align: ui.Start, Padding: 4, Instant: true, NoDismiss: true, KeepOpenOnAnchor: true,
		}, func() { a.list(c) })
	}
	return box
}

func (a *Autocomplete) list(c ui.C) {
	theme := c.Theme()
	metrics := theme.Metrics
	ui.Column(c).FillWidth().Children(func() {
		for row, index := range a.matches {
			item := c.KeyInt(index)
			box := ui.Row(item).FillWidth().Items(ui.Center).Pad(metrics.GapItem).Radius(metrics.RadiusControl)
			if row == a.highlight {
				box = box.Background(theme.Accent)
			}
			box = box.HitWith(ui.ID(item), a.spec.Options[index], gfx.HitOptions{Role: gfx.RoleOption, SkipFocus: true})
			box.Hover(ui.Look{}.Background(theme.Hover))
			box.Children(func() { ui.Text(item, a.spec.Options[index]) })
			if box.Clicked() {
				a.choose(index)
			}
		}
	})
}
