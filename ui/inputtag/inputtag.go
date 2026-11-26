// Package inputtag is the opt-in page control for entering several short values
// as tags: the values entered so far are tags in a row, each with a close button,
// followed by a text field to type the next. A page keeps the values in its model
// and an InputTag beside them:
//
//	m.hosts = inputtag.New()
//	// in the page:
//	m.hosts.View(c, "Allowed hosts", m.allowed, inputtag.Spec{Placeholder: "Add a host"}).FillWidth()
//	if value, ok := m.hosts.Added(); ok {
//		m.allowed = append(m.allowed, value)
//	}
//	if i, ok := m.hosts.Removed(); ok {
//		m.allowed = slices.Delete(m.allowed, i, i+1)
//	}
//
// Enter, or a typed Separator (a comma by default), adds what is in the field as
// a tag; Backspace in the empty field removes the last tag, and the close button
// of a tag removes that one. Empty or white-space-only entries are ignored, and a
// value already in the list is not added twice unless AllowDuplicates is set. The
// control reports what the user did and the page changes its list: the same
// answer as for an alert's close button, so the list stays the page's.
//
// MaxTags stops adding when the list is that long; Validate can refuse a value,
// and the field then keeps its text and shows the invalid look. Rejected reports
// the refusal.
//
// It is a separate package so an application that does not import it pays
// nothing for it. Keep an InputTag in the model. Do not copy one after it has been
// declared.
package inputtag

import (
	"strings"
	"unicode/utf8"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/tag"
)

// Spec declares the control.
type Spec struct {
	// Placeholder is shown in the field while it and the list are empty.
	Placeholder string
	// Separator is the rune that adds the typed value; ',' when zero. NoSeparator
	// leaves only Enter.
	Separator   rune
	NoSeparator bool
	// MaxTags is how many tags the list may have; zero is no limit.
	MaxTags int
	// AllowDuplicates lets a value be added that is already in the list.
	AllowDuplicates bool
	// Validate returns whether a value may be added; nil accepts every value.
	Validate func(value string) bool
	// Disabled leaves the field and the close buttons inert.
	Disabled bool
	// RemoveLabel is the accessible name of a close button, before the tag's text:
	// "Remove" when empty.
	RemoveLabel string
}

// InputTag is one control's state: the text being typed and what was done.
type InputTag struct {
	field    ui.Field
	added    string
	hasAdded bool
	removed  int // 1 + the index removed since the paint before, or 0
	rejected bool
	invalid  bool
	// pending is what a key or a press did since the last paint, reported by the
	// paint after it.
	pending struct {
		added      string
		hasAdded   bool
		removeLast bool
		rejected   bool
	}
	key          func(gfx.KeyEvent) gfx.EventResult
	deleteAtEdge func(forward bool)
	spec         Spec
	tags         []string
	labels       []name // "Remove <tag>", kept while the tag stays
	id           gfx.ID
	fieldID      gfx.ID
}

// New returns an empty control.
func New() *InputTag {
	t := &InputTag{}
	t.key = t.onKey
	t.deleteAtEdge = t.onDeleteAtEdge
	return t
}

// FieldID names the field with an identity the application already holds, and
// Field is the text field behind the control, for a runner that binds the editor
// the platform edits to it with Field.Bind. A page needs neither.
func (t *InputTag) FieldID(id gfx.ID) *InputTag { t.fieldID = id; return t }

// Field is the text field behind the control.
func (t *InputTag) Field() *ui.Field { return &t.field }

// Added returns the value the user added since the paint before this one.
func (t *InputTag) Added() (string, bool) { return t.added, t.hasAdded }

// Removed returns the index of the tag the user removed since the paint before
// this one.
func (t *InputTag) Removed() (int, bool) { return t.removed - 1, t.removed > 0 }

// Rejected reports whether a value was refused (by Validate, MaxTags or as a
// duplicate) since the paint before this one.
func (t *InputTag) Rejected() bool { return t.rejected }

func (s *Spec) separator() rune {
	if s.NoSeparator {
		return -1
	}
	if s.Separator == 0 {
		return ','
	}
	return s.Separator
}

// accepts says whether value may join the list, and records a refusal.
func (t *InputTag) accepts(value string) bool {
	ok := value != ""
	if ok && !t.spec.AllowDuplicates {
		for _, existing := range t.tags {
			if existing == value {
				ok = false
				break
			}
		}
	}
	if ok && t.spec.MaxTags > 0 && len(t.tags) >= t.spec.MaxTags {
		ok = false
	}
	if ok && t.spec.Validate != nil && !t.spec.Validate(value) {
		ok = false
	}
	return ok
}

// commit takes the typed text as a tag, or refuses it.
func (t *InputTag) commit() {
	editor := t.field.GetEditor()
	if editor == nil {
		return
	}
	value := strings.TrimSpace(editor.String())
	if value == "" {
		return
	}
	if !t.accepts(value) {
		t.pending.rejected, t.invalid = true, true
		return
	}
	t.pending.added, t.pending.hasAdded, t.invalid = value, true, false
	clear(editor)
}

// clear empties the editor.
func clear(editor *text.Editor) {
	editor.SetSelection(text.Range{Start: 0, End: editor.RuneCount()})
	editor.DeleteBackward()
}

func (t *InputTag) onKey(event gfx.KeyEvent) gfx.EventResult {
	if event.Release || t.spec.Disabled {
		return gfx.EventResult{}
	}
	editor := t.field.GetEditor()
	switch {
	case event.Key == gfx.KeyEnter:
		if editor != nil && strings.TrimSpace(editor.String()) != "" {
			t.commit()
			return gfx.EventResult{Handled: true, Repaint: true}
		}
	}
	return gfx.EventResult{}
}

// takeSeparated commits the text typed or pasted before a separator, and keeps
// what follows it in the field.
func (t *InputTag) takeSeparated() {
	editor := t.field.GetEditor()
	sep := t.spec.separator()
	if editor == nil || sep < 0 {
		return
	}
	value := editor.String()
	i := strings.IndexRune(value, sep)
	if i < 0 {
		return
	}
	editor.SetSelection(text.Range{Start: 0, End: utf8.RuneCountInString(value[:i+utf8.RuneLen(sep)])})
	editor.DeleteBackward()
	if value = strings.TrimSpace(value[:i]); value != "" {
		if t.accepts(value) {
			t.added, t.hasAdded, t.invalid = value, true, false
		} else {
			t.rejected, t.invalid = true, true
		}
	}
}

// onDeleteAtEdge is Backspace in the empty field: it removes the last tag.
func (t *InputTag) onDeleteAtEdge(forward bool) {
	if !forward && !t.spec.Disabled && len(t.tags) > 0 && t.field.GetEditor().RuneCount() == 0 {
		t.pending.removeLast = true
	}
}

// View declares the tags and the field at c. tags are the values in the list, in
// order; keep the slice in the model. label names the field for assistive
// technology. Declare the view on its own, not inside a Memo.
func (t *InputTag) View(c ui.C, label string, tags []string, spec Spec) ui.Box {
	if t == nil {
		panic("inputtag: View needs its InputTag")
	}
	t.added, t.hasAdded, t.rejected, t.removed = t.pending.added, t.pending.hasAdded, t.pending.rejected, 0
	t.pending.added, t.pending.hasAdded, t.pending.rejected = "", false, false
	if t.pending.removeLast {
		t.pending.removeLast, t.removed = false, len(tags)
	}
	t.spec, t.tags = spec, tags
	if editor := t.field.GetEditor(); editor != nil && editor.OnDeleteAtEdge == nil {
		editor.OnDeleteAtEdge = t.deleteAtEdge
	}
	t.takeSeparated()
	theme := c.Theme()
	metrics := theme.Metrics
	remove := spec.RemoveLabel
	if remove == "" {
		remove = "Remove"
	}
	for len(t.labels) < len(tags) {
		t.labels = append(t.labels, name{})
	}
	t.id = ui.ID(c)
	if t.fieldID.IsZero() {
		t.fieldID = ui.ID(c)
	}
	rim := theme.Input
	if c.Canvas().Focused() == t.fieldID {
		rim = theme.Primary
	}
	if t.invalid {
		rim = theme.Danger
	}
	root := ui.Flow(c).Items(ui.Center).Gap(metrics.GapItem * 0.5).LineGap(metrics.GapItem * 0.5).Pad(metrics.GapItem)
	root = root.Radius(metrics.RadiusControl).Border(1, rim)
	root.Children(func() {
		for i, value := range tags {
			if n := &t.labels[i]; n.of != value || n.verb != remove {
				n.of, n.verb, n.text = value, remove, remove+" "+value
			}
			key := c.Key(value)
			if i < len(tags) && spec.AllowDuplicates {
				key = c.KeyInt(i)
			}
			result := tag.View(key, value, tag.Spec{Closable: !spec.Disabled, CloseLabel: t.labels[i].text})
			if result.Closed {
				t.removed = i + 1
			}
		}
		placeholder := ""
		if len(tags) == 0 {
			placeholder = spec.Placeholder
		}
		field := ui.TextFieldBase(c, &t.field, label, placeholder, ui.WithID(t.fieldID)).OnKey(t.key).Width(120).Grow(1)
		if spec.Disabled {
			field.Disabled(true)
		}
	})
	return root
}

// name is the accessible name of one close button and what it was made from.
type name struct{ of, verb, text string }
