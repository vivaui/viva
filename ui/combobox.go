package ui

import (
	"strconv"
	"unsafe"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

// Combobox is a field over choices that filters them as the user types, *value the
// chosen one: the arrows move through what matches, Enter or a click chooses, and
// Escape, Tab or a press outside closes the list and drops what was typed. It is
// Select for a list too long to scroll through, typed into with the platform's
// text service and input methods. label names it, for assistive technology and as
// its placeholder until Placeholder says otherwise; nameOf names the choices, or is
// nil for strings.
//
//	ui.Combobox(c, &m.timezone, zones, Zone.String, "Time zone").Width(240)
//
// Clearable adds a button that clears the choice, Empty says what the list shows
// when nothing matches, and Creatable offers what was typed as a new choice, which
// Created reports for the application to add. Describe gives choices a detail, text to
// search by and a disabled state.
//
// Not inside a Memo.
func Combobox[T comparable](c C, value *T, choices []T, nameOf func(T) string, label string) Box {
	return combobox(c, value, choices, nameOf, label)
}

// ComboboxOption describes one choice of a Combobox, as Describe gives it: Label is its
// name; Detail is shown after it in a muted face (an address, an account) and, with Search
// (extra text that is never shown, such as a group or an alias), is matched by what the
// user types; Disabled keeps the choice in the list, dimmed and not chosen by a click or
// the keys, so a search can still explain why it cannot be had.
//
// Leading and Trailing reserve room before the label and at the end of the row for what
// their Paint draws (a flag, a status dot, a badge), and Content paints the row's text
// itself in place of the label and detail. They draw only: a press on them chooses the
// option, and the field shows the chosen option's label and detail.
type ComboboxOption struct {
	Label, Detail, Search      string
	Disabled                   bool
	Leading, Trailing, Content ComboboxPaint
}

// ComboboxPaint is room in an option's row and what paints it: see ComboboxOption.
type ComboboxPaint = components.SingleSelectPaint

func combobox[T comparable](c C, value *T, choices []T, nameOf func(T) string, label string) Box {
	p := c.page
	p.forbidInMemo("Combobox")
	if label == "" {
		panic("ui: Combobox needs a label: it is the field's name for assistive technology")
	}
	scope := p.boundScope(pkCombobox, unsafe.Pointer(value))
	state := stateIn[comboState](p, scope, "combobox")
	if state.paint == nil {
		state.paint = state.draw // made once: a method value made in each paint allocates
		state.list.LabelsOnly, state.list.MatchCaseInsensitive = true, true
	}
	f := c.f
	labels, selected := options(p, value, choices, nameOf)
	state.setOptions(labels)
	key := ""
	if selected >= 0 {
		key = state.keys[selected]
	}
	if state.ids.options == 0 {
		state.ids = newComboIDs(p.tree, scope)
	}
	ids := &state.ids
	// What the user did since the last paint: a choice (made here or by Enter in
	// Key) is written to *value; a value the program changed is shown.
	if canvas := f.canvas; canvas != nil {
		switch id, delivering := canvas.ClickTarget(); {
		case !delivering:
		case id == ids.trigger:
			state.list.Open = !state.list.Open
			canvas.RequestFocus(ids.input)
		case id == ids.input:
			state.list.Open = true
		case id == ids.dismiss:
			state.list.Dismiss()
		case id == ids.clear:
			state.list.Clear()
		default:
			if at, ok := canvas.ClickedIn(ids.options); ok {
				state.list.Choose(at)
			}
		}
	}
	changed := false
	state.created = ""
	switch chosen := state.list.Value(); {
	case chosen != state.synced: // the user chose
		var zero T
		switch i, isKey := comboKey(chosen); {
		case chosen == "": // cleared
			changed = *value != zero
			*value, key = zero, ""
		case isKey && i < len(choices):
			changed = *value != choices[i]
			*value, key = choices[i], chosen
		case !isKey: // typed and created: the application takes it into its choices
			state.created = chosen
			state.list.ForgetCreated()
			state.list.SetValue(key)
			chosen = key
		}
		state.synced = chosen
	case key != chosen: // the program set *value
		state.list.SetValue(key)
		state.synced = key
	}
	if state.list.Open {
		p.overlay = true
	}
	state.f, state.label, state.placeholder = f, label, label
	// Clearable, Empty and Creatable say these again for this paint, after the clicks
	// were taken with what the last paint showed.
	state.clearable, state.empty, state.list.AllowCreate, state.createPrefix = false, "", false, ""
	state.layer = f.widgetLayer() + 1
	// The field declares its own hits as it paints; the element only places it, as
	// wide as its container, a control tall.
	b := f.Slot().Height(f.theme.Metrics.ControlHeight)
	b.n().id = ids.input // what Tooltip, Shortcut and AutoFocus name
	state.node, state.gen = b.i, b.gen
	b.Draw(state.paint)
	f.bindDone(b, changed)
	p.combos = append(p.combos, state)
	if p.comboRoute == nil {
		p.keyRoute, p.comboRoute = (*Page).routeKey, (*Page).comboKey
		p.wheelRoute, p.textRoute = (*Page).overlayWheel, (*Page).comboText
	}
	return b
}

// comboState is what a Combobox keeps between paints: the list's state, the search
// editor's platform input, and its options keyed by their index.
type comboState struct {
	list    components.SingleSelectState
	input   app.TextInput
	ids     comboIDs
	labels  []string
	descs   []ComboboxOption // what Describe last gave, to tell a change
	scratch []ComboboxOption
	keys    []string // keys[i] is the Value of option i: its index
	opts    []components.SingleSelectOption
	synced  string // the chosen key as *value and the list last agreed on it
	created string // what the user created in this paint, for Created
	paint   func(*DrawContext)

	f                         *frame
	node                      int32
	gen                       uint32
	label, placeholder, empty string
	clearLabel, clearLabelOf  string // "Clear <label>", made once per label
	createPrefix              string
	clearable                 bool
	layer                     uint16
}

// comboKeyPrefix starts the Value of each option: a byte no one types, so an option
// is told from a value the user created by typing it.
const comboKeyPrefix = "\x00"

// comboKey reads an option's index from a Value; isKey is false for a value the
// user created.
func comboKey(value string) (index int, isKey bool) {
	if len(value) < 2 || value[0] != 0 {
		return 0, false
	}
	i, err := strconv.Atoi(value[1:])
	return i, err == nil
}

// comboIDs are a Combobox's identities, in the scope its value names.
type comboIDs struct {
	input, trigger, dismiss, focus, scroll, clear gfx.ID
	options                                       gfx.Scope
}

func newComboIDs(tree *gfx.Tree, scope gfx.Scope) comboIDs {
	return comboIDs{
		input: scope.At(0), trigger: scope.At(1), dismiss: scope.At(2), focus: scope.At(3), scroll: scope.At(4),
		clear: scope.At(6), options: tree.Sub(scope, gfx.Index(5)),
	}
}

// setOptions gives the list the labels when they are not the ones it has.
func (s *comboState) setOptions(labels []string) {
	same := len(labels) == len(s.labels)
	for i := 0; same && i < len(labels); i++ {
		same = labels[i] == s.labels[i]
	}
	if same {
		return
	}
	s.labels = append(s.labels[:0], labels...)
	for len(s.keys) < len(labels) {
		s.keys = append(s.keys, comboKeyPrefix+strconv.Itoa(len(s.keys)))
	}
	s.opts = s.opts[:0]
	for i, label := range labels {
		s.opts = append(s.opts, components.SingleSelectOption{Value: s.keys[i], Label: label})
	}
	s.list.SetOptions(s.opts)
}

// draw paints the field, and its list while it is open.
func (s *comboState) draw(d *DrawContext) {
	n := &s.f.nodes[s.node]
	ids := &s.ids
	options := components.SingleSelectOptions{
		InputID: ids.input, TriggerID: ids.trigger, DismissID: ids.dismiss, Scope: ids.focus,
		ScrollID: ids.scroll, OptionIDs: ids.options,
		Label: s.label, Placeholder: s.placeholder, EmptyLabel: s.empty, CreatePrefix: s.createPrefix,
		Viewport: s.f.bounds, Layer: max(s.layer, s.f.popupLayer), Disabled: n.Flags&flagDisabled != 0,
	}
	if s.clearable {
		if s.clearLabelOf != s.label {
			s.clearLabel, s.clearLabelOf = "Clear "+s.label, s.label
		}
		options.ClearID, options.ClearLabel = ids.clear, s.clearLabel
	}
	components.PaintSingleSelect(d.Canvas, *d.Theme, d.Rect, &s.list, options)
}

// combo is the Combobox the box declares, or nil.
func (b Box) combo() *comboState {
	b.n()
	if p := b.f.page; p != nil {
		for _, s := range p.combos {
			if s.f == b.f && s.node == b.i && s.gen == b.gen {
				return s
			}
		}
	}
	return nil
}

// Clearable gives a Combobox a button that clears its choice, setting the value to
// its type's zero value.
func (b Box) Clearable() Box {
	if s := b.combo(); s != nil {
		s.clearable = true
	}
	return b
}

// Describe gives a Combobox's choices more than a name: describe(i) is choice i's
// ComboboxOption, whose Label, when set, replaces the name nameOf gave. It is called for
// every choice in every paint, so keep it to reading fields. It goes by position, since a
// method cannot take the choice type.
//
//	ui.Combobox(c, &m.host, hosts, Host.String, "Host").Describe(func(i int) ui.ComboboxOption {
//		h := hosts[i]
//		return ui.ComboboxOption{Detail: h.Address, Search: h.Group, Disabled: !h.Online}
//	})
func (b Box) Describe(describe func(i int) ComboboxOption) Box {
	s := b.combo()
	if s == nil || describe == nil {
		return b
	}
	n := len(s.labels)
	same := n == len(s.descs)
	s.scratch = s.scratch[:0]
	for i := 0; i < n; i++ {
		d := describe(i)
		if d.Label == "" {
			d.Label = s.labels[i]
		}
		s.scratch = append(s.scratch, d)
		same = same && s.descs[i].sameAs(d)
	}
	if same {
		return b
	}
	s.descs = append(s.descs[:0], s.scratch...)
	s.opts = s.opts[:0]
	for i, d := range s.descs {
		s.opts = append(s.opts, components.SingleSelectOption{Value: s.keys[i], Label: d.Label, Detail: d.Detail, Search: d.Search, Disabled: d.Disabled, Leading: d.Leading, Trailing: d.Trailing, Content: d.Content})
	}
	s.list.SetOptions(s.opts)
	return b
}

// Empty is what a Combobox's list says when nothing matches what was typed.
func (b Box) Empty(text string) Box {
	if s := b.combo(); s != nil {
		s.empty = text
	}
	return b
}

// Creatable lets the user choose what they typed when it is no choice yet, in a
// row reading prefix and the text ("Create " when empty); Created reports it.
func (b Box) Creatable(prefix string) Box {
	if s := b.combo(); s != nil {
		if prefix == "" {
			prefix = "Create "
		}
		s.list.AllowCreate, s.createPrefix = true, prefix
	}
	return b
}

// Created reports what the user typed and chose as a new choice in this paint. The
// value is not changed: add it to the choices and set the value to it.
//
//	field := ui.Combobox(c, &m.group, m.groups, nil, "Group").Creatable("")
//	if name, ok := field.Created(); ok {
//		m.groups = append(m.groups, name)
//		m.group = name
//	}
func (b Box) Created() (string, bool) {
	if s := b.combo(); s != nil && s.created != "" {
		return s.created, true
	}
	return "", false
}

// comboKey gives the keys of a Combobox's field to its list: the arrows, Enter,
// Escape, Tab and Backspace on an empty search.
func (p *Page) comboKey(event gfx.KeyEvent) (gfx.EventResult, bool) {
	for _, s := range p.combos {
		if event.Target != s.ids.input {
			continue
		}
		if result := s.list.Key(event); result.Handled || result.Repaint {
			return result, true
		}
	}
	return gfx.EventResult{}, false
}

// comboText is the platform's editor of a Combobox's search, for TextInput.
func (p *Page) comboText(id gfx.ID) *app.TextInput {
	for _, s := range p.combos {
		if s.ids.input == id {
			s.input.Editor = &s.list.Query
			return &s.input
		}
	}
	return nil
}

// comboWheel scrolls the open list of a Combobox the wheel is over.
func (p *Page) comboWheel(event gfx.ScrollEvent) (gfx.EventResult, bool) {
	for _, s := range p.combos {
		if _, inside := event.Target.In(s.ids.options); inside && s.list.Open {
			return gfx.EventResult{Handled: true, Repaint: s.list.Wheel(event)}, true
		}
	}
	return gfx.EventResult{}, false
}

// overlayWheel scrolls the open Menu or Combobox list the wheel is over.
func (p *Page) overlayWheel(event gfx.ScrollEvent) (gfx.EventResult, bool) {
	if len(p.menus) > 0 {
		if result, ok := p.menuWheel(event); ok {
			return result, true
		}
	}
	if len(p.combos) > 0 {
		return p.comboWheel(event)
	}
	return gfx.EventResult{}, false
}

// sameAs reports whether two descriptions give the same row. Painters are functions and
// cannot be compared, so a row that has one is taken as changed in every paint: its
// list is rebuilt, which costs the options' slice and nothing else.
func (o ComboboxOption) sameAs(p ComboboxOption) bool {
	if o.Leading.Paint != nil || o.Trailing.Paint != nil || o.Content.Paint != nil ||
		p.Leading.Paint != nil || p.Trailing.Paint != nil || p.Content.Paint != nil {
		return false
	}
	return o.Label == p.Label && o.Detail == p.Detail && o.Search == p.Search && o.Disabled == p.Disabled &&
		o.Leading.Width == p.Leading.Width && o.Trailing.Width == p.Trailing.Width && o.Content.Width == p.Content.Width
}
