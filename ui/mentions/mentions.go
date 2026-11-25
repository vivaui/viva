// Package mentions is the opt-in page control for a text field that offers
// people, hosts or tags as the user types a trigger: typing "@" and a few
// letters opens a list under the field, and choosing one writes it in place of
// what was typed. A page keeps the field's value in a ui.Field and a Mentions in
// its model:
//
//	m.mentions = mentions.New()
//	// in the page:
//	m.mentions.View(c, &m.message, "Message", "Write a message", mentions.Spec{Options: m.people})
//	if option, ok := m.mentions.Chosen(); ok {
//		m.notify(option) // the field already holds "@Ada "
//	}
//
// The list opens while the caret is at the end of a word that starts with the
// Trigger ("@" by default) at the start of the text or after a space, and closes
// when the word ends, the caret leaves it or the field loses the keyboard. Up
// and Down move through the options, Enter and Tab choose the highlighted one,
// and Escape closes the list until the word changes; the other keys type as
// usual. An option matches when the typed letters appear in its Label, without
// regard to case. A press on an option chooses it without taking the keyboard
// from the field.
//
// It is a separate package so an application that does not import it pays
// nothing for it. Keep a Mentions in the model beside its field: its state is
// made once, and a paint allocates nothing while no mention is being typed. Do
// not copy a Mentions after it has been declared.
package mentions

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

// Option is one thing that can be mentioned.
type Option struct {
	// Label is what is written after the trigger and what the typed letters are
	// matched against.
	Label string
	// Detail is a muted second line of the list, such as a role or an address;
	// it is not matched.
	Detail string
}

// Spec declares the list.
type Spec struct {
	// Options are everything that can be mentioned, in the order the list shows
	// the ones that match. Keep the slice in the model.
	Options []Option
	// Trigger is the rune that starts a mention; '@' when zero.
	Trigger rune
	// MaxRows is how many options the list shows at most; 6 when zero.
	MaxRows int
	// Empty is the line of the list when nothing matches; the list stays closed
	// when it is empty.
	Empty string
	// NoSuffix leaves out the space written after a chosen option.
	NoSuffix bool
}

// Mentions is one field's list.
type Mentions struct {
	open      bool
	matches   []int // indexes into Spec.Options of the rows shown
	highlight int
	wordStart int // the rune index of the trigger of the word being typed, or -1
	dismissed int // the wordStart Escape closed the list for, or -1
	chosen    int // 1 + the option chosen since the last paint, or 0
	pending   int // what a key or press chose, reported by the paint after it
	spec      Spec
	field     *ui.Field
	key       func(gfx.KeyEvent) gfx.EventResult
	id        gfx.ID
	fieldID   gfx.ID
}

// New returns a closed list.
func New() *Mentions {
	m := &Mentions{wordStart: -1, dismissed: -1}
	m.key = m.onKey
	return m
}

// FieldID names the field with an identity the application already holds, in
// place of the one its place gives it. It is for a runner that resolves the
// platform text service by an application identity, such as a component
// preview; a page does not need it.
func (m *Mentions) FieldID(id gfx.ID) *Mentions { m.fieldID = id; return m }

// Chosen reports the option the user chose since the paint before this one,
// by key or by press.
func (m *Mentions) Chosen() (Option, bool) {
	if m.chosen == 0 {
		return Option{}, false
	}
	return m.spec.Options[m.chosen-1], true
}

func (s *Spec) trigger() rune {
	if s.Trigger == 0 {
		return '@'
	}
	return s.Trigger
}

// word finds the mention being typed: the rune index of the trigger and the
// letters after it, when the caret (a rune index) is at the end of a word that
// starts with the trigger. It reads the text in place and allocates nothing.
func word(value string, caret int, trigger rune) (start int, query string, ok bool) {
	end := 0
	for n := 0; n < caret && end < len(value); n++ {
		_, size := utf8.DecodeRuneInString(value[end:])
		end += size
	}
	prefix := value[:end]
	for i := len(prefix); i > 0; {
		r, size := utf8.DecodeLastRuneInString(prefix[:i])
		i -= size
		switch {
		case r == trigger:
			if i > 0 {
				if before, _ := utf8.DecodeLastRuneInString(prefix[:i]); !unicode.IsSpace(before) {
					return 0, "", false
				}
			}
			return utf8.RuneCountInString(prefix[:i]), prefix[i+size:], true
		case unicode.IsSpace(r):
			return 0, "", false
		}
	}
	return 0, "", false
}

// containsFold reports whether s has query in it, without regard to case.
func containsFold(s, query string) bool {
	if query == "" {
		return true
	}
	for i := range s {
		rest, q := s[i:], query
		for q != "" && rest != "" {
			a, as := utf8.DecodeRuneInString(rest)
			b, bs := utf8.DecodeRuneInString(q)
			if unicode.ToLower(a) != unicode.ToLower(b) {
				break
			}
			rest, q = rest[as:], q[bs:]
		}
		if q == "" {
			return true
		}
	}
	return false
}

// refresh finds the word at the caret and the options that match it, and
// reports whether the list is to be shown.
func (m *Mentions) refresh(c ui.C, spec Spec) bool {
	m.matches = m.matches[:0]
	editor := m.field.GetEditor()
	if editor == nil || c.Canvas().Focused() != m.id {
		m.wordStart = -1
		return false
	}
	selection := editor.Selection()
	if selection.Start != selection.End {
		m.wordStart = -1
		return false
	}
	start, query, ok := word(editor.String(), editor.Caret(), spec.trigger())
	if !ok {
		m.wordStart, m.dismissed = -1, -1
		return false
	}
	if start != m.wordStart {
		m.highlight = 0
	}
	m.wordStart = start
	if m.dismissed == start {
		return false
	}
	limit := spec.MaxRows
	if limit <= 0 {
		limit = 6
	}
	for i := range spec.Options {
		if len(m.matches) == limit {
			break
		}
		if containsFold(spec.Options[i].Label, query) {
			m.matches = append(m.matches, i)
		}
	}
	m.highlight = min(m.highlight, max(len(m.matches)-1, 0))
	return len(m.matches) > 0 || spec.Empty != ""
}

func (m *Mentions) onKey(event gfx.KeyEvent) gfx.EventResult {
	if !m.open || event.Release {
		return gfx.EventResult{}
	}
	switch event.Key {
	case gfx.KeyDown:
		if len(m.matches) > 0 {
			m.highlight = (m.highlight + 1) % len(m.matches)
		}
		return gfx.EventResult{Handled: true, Repaint: true}
	case gfx.KeyUp:
		if len(m.matches) > 0 {
			m.highlight = (m.highlight + len(m.matches) - 1) % len(m.matches)
		}
		return gfx.EventResult{Handled: true, Repaint: true}
	case gfx.KeyEnter, gfx.KeyTab:
		if len(m.matches) == 0 {
			return gfx.EventResult{}
		}
		m.insert(m.matches[m.highlight])
		return gfx.EventResult{Handled: true, Repaint: true}
	case gfx.KeyEscape:
		m.dismissed = m.wordStart
		m.open = false
		return gfx.EventResult{Handled: true, Repaint: true}
	}
	return gfx.EventResult{}
}

// insert writes the option in place of the word being typed.
func (m *Mentions) insert(index int) {
	editor := m.field.GetEditor()
	if editor == nil || m.wordStart < 0 {
		return
	}
	var b strings.Builder
	b.WriteRune(m.spec.trigger())
	b.WriteString(m.spec.Options[index].Label)
	if !m.spec.NoSuffix {
		b.WriteByte(' ')
	}
	editor.SetSelection(text.Range{Start: m.wordStart, End: editor.Caret()})
	editor.Insert(b.String())
	m.pending = index + 1
	m.open = false
	m.wordStart, m.dismissed = -1, -1
}

// View declares the field and, while a mention is being typed, the list under
// it. The field is a ui.TextField on field; label and placeholder are its.
// Declare the view on its own, not inside a Memo.
func (m *Mentions) View(c ui.C, field *ui.Field, label, placeholder string, spec Spec) ui.Box {
	if m == nil {
		panic("mentions: View needs its Mentions")
	}
	m.chosen, m.pending = m.pending, 0
	m.spec, m.field = spec, field
	m.id = m.fieldID
	if m.id.IsZero() {
		m.id = ui.ID(c)
	}
	m.open = m.refresh(c, spec)
	box := ui.TextField(c, field, label, placeholder, ui.WithID(m.id)).OnKey(m.key)
	if m.open {
		ui.Popover(c, box, &m.open, ui.PopoverSpec{
			Align: ui.Start, Padding: 4, Instant: true,
			NoDismiss: true, KeepOpenOnAnchor: true,
		}, func() { m.list(c) })
	}
	return box
}

// list declares the rows of the options that match.
func (m *Mentions) list(c ui.C) {
	theme := c.Theme()
	metrics := theme.Metrics
	ui.Column(c).FillWidth().Children(func() {
		if len(m.matches) == 0 {
			ui.Text(c, m.spec.Empty).Muted().FontSize(metrics.FontSizeS).Pad(metrics.GapItem)
			return
		}
		for row, index := range m.matches {
			option := &m.spec.Options[index]
			item := c.KeyInt(index)
			box := ui.Row(item).FillWidth().Items(ui.Center).Gap(metrics.GapItem).Pad(metrics.GapItem).Radius(metrics.RadiusControl)
			if row == m.highlight {
				box = box.Background(theme.Accent)
			}
			box = box.HitWith(ui.ID(item), option.Label, gfx.HitOptions{Role: gfx.RoleOption, SkipFocus: true})
			box.Hover(ui.Look{}.Background(theme.Hover))
			box.Children(func() {
				ui.Text(item, option.Label).Shrink(0)
				if option.Detail != "" {
					ui.Text(item, option.Detail).FontSize(metrics.FontSizeS).Muted().Ellipsis()
				}
			})
			if box.Clicked() {
				m.insert(index)
			}
		}
	})
}
