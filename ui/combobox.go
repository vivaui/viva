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
// Not inside a Memo.
func Combobox[T comparable](c C, value *T, choices []T, nameOf func(T) string, label string) Box {
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
		default:
			if at, ok := canvas.ClickedIn(ids.options); ok {
				state.list.Choose(at)
			}
		}
	}
	changed := false
	switch chosen := state.list.Value(); {
	case chosen != state.synced: // the user chose
		if i, err := strconv.Atoi(chosen); err == nil && i < len(choices) {
			changed = *value != choices[i]
			*value, key = choices[i], chosen
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
	state.layer = f.widgets.layer + 1
	// The field declares its own hits as it paints; the element only places it, as
	// wide as its container, a control tall.
	b := f.Slot().Height(f.theme.Metrics.ControlHeight)
	b.n().id = ids.input // what Tooltip, Shortcut and AutoFocus name
	state.node = b.i
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
	list   components.SingleSelectState
	input  app.TextInput
	ids    comboIDs
	labels []string
	keys   []string // keys[i] is the Value of option i: its index
	opts   []components.SingleSelectOption
	synced string // the chosen key as *value and the list last agreed on it
	paint  func(*DrawContext)

	f                  *Frame
	node               int32
	label, placeholder string
	layer              uint16
}

// comboIDs are a Combobox's identities, in the scope its value names.
type comboIDs struct {
	input, trigger, dismiss, focus, scroll gfx.ID
	options                                gfx.Scope
}

func newComboIDs(tree *gfx.Tree, scope gfx.Scope) comboIDs {
	return comboIDs{
		input: scope.At(0), trigger: scope.At(1), dismiss: scope.At(2), focus: scope.At(3), scroll: scope.At(4),
		options: tree.Sub(scope, gfx.Index(5)),
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
		s.keys = append(s.keys, strconv.Itoa(len(s.keys)))
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
	components.PaintSingleSelect(d.Canvas, *d.Theme, d.Rect, &s.list, components.SingleSelectOptions{
		InputID: ids.input, TriggerID: ids.trigger, DismissID: ids.dismiss, Scope: ids.focus,
		ScrollID: ids.scroll, OptionIDs: ids.options,
		Label: s.label, Placeholder: s.placeholder, Viewport: s.f.bounds,
		Layer: max(s.layer, s.f.popupLayer), Disabled: n.flags&flagDisabled != 0,
	})
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
