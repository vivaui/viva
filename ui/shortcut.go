package ui

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/keymap"
)

// The modifiers of a Shortcut. Primary is Command on macOS and Control elsewhere,
// the modifier of a platform's own shortcuts (Primary+S saves); Ctrl is Control on
// every platform (Ctrl+Tab).
const (
	Primary = keymap.Primary
	Shift   = gfx.ModifierShift
	Alt     = gfx.ModifierAlt
	Ctrl    = gfx.ModifierControl
)

// shortcutEntry is a Shortcut declared in the last paint: its chord, the identity its
// click is delivered to, and where the focus must be for it (an element's identity,
// or a container's scope; neither for a page-wide one).
type shortcutEntry struct {
	// The chord's fields, not a keymap.Chord: the Page's type would keep that
	// type's methods in every program, Shortcut or not.
	modifiers gfx.Modifiers
	char      byte
	key       gfx.Key
	id        gfx.ID
	focus     gfx.ID
	scope     gfx.Scope
}

// Shortcut reports whether this paint delivers the chord modifiers+key pressed
// anywhere in the page: a window's Save or Find, declared next to what it does.
// key is a character ("s", "/") or a key's name ("enter", "escape", "delete",
// "f2", "up"; see keymap.Parse).
//
//	if ui.Shortcut(c, ui.Primary, "s") {
//		m.save()
//	}
//
// A Shortcut acts as a click does, in the paint that delivers it, and is matched
// before the keys reach the control with the focus, except that a chord without
// Primary, Control or Alt is left to a text field, Combobox or terminal being typed
// in. While an overlay is open (Page.Overlay), a page-wide Shortcut does not act
// (the overlay has the keyboard); Box.Shortcut on an element in it does. Not inside
// a Memo.
func Shortcut(c C, modifiers gfx.Modifiers, key string) bool {
	return c.page.shortcut(c, modifiers, key, gfx.ID{}, 0)
}

// Shortcut is ui.Shortcut for while the keyboard focus is on the element or in it: a
// list's Delete acts on the list being worked in, a panel's Primary+F searches it.
// Where Shortcuts of one chord nest, the innermost one with the focus acts. The
// element is a control, or a container of the page.
//
//	list := ui.Column(c).Children(rows)
//	if list.Shortcut(0, "delete") {
//		m.removeSelected()
//	}
func (b Box) Shortcut(modifiers gfx.Modifiers, key string) bool {
	p := b.f.page
	if p == nil {
		panic("ui: Box.Shortcut needs a page: declare it with a ui.C")
	}
	if int(b.i) < len(p.nodes) && p.nodes[b.i].named {
		return p.shortcut(&p.ctx, modifiers, key, gfx.ID{}, p.resolve(b.i))
	}
	if id := b.n().id; !id.IsZero() {
		return p.shortcut(&p.ctx, modifiers, key, id, 0)
	}
	panic("ui: Box.Shortcut is for a control or a container: the element must be able to hold the focus or have it inside")
}

func (p *Page) shortcut(c C, modifiers gfx.Modifiers, key string, focus gfx.ID, scope gfx.Scope) bool {
	p.forbidInMemo("Shortcut")
	chord, ok := keymap.Parse(modifiers, key)
	if !ok {
		panic("ui: Shortcut " + key + ": not a key keymap.Parse names, nor one character")
	}
	id := p.controlID(pkShortcut)
	p.shortcuts = append(p.shortcuts, shortcutEntry{modifiers: chord.Modifiers, char: chord.Char, key: chord.Key, id: id, focus: focus, scope: scope})
	if p.shortcutRoute == nil {
		p.keyRoute, p.shortcutRoute = (*Page).routeKey, (*Page).matchShortcut
	}
	return c.f.canvas != nil && c.f.canvas.Clicked(id)
}

// matchShortcut finds the Shortcut the key event presses where the focus is, and
// returns the result that delivers it.
func (p *Page) matchShortcut(event gfx.KeyEvent) (gfx.EventResult, bool) {
	if event.Release || len(p.shortcuts) == 0 {
		return gfx.EventResult{}, false
	}
	chord, ok := keymap.FromEvent(event)
	if !ok {
		return gfx.EventResult{}, false
	}
	if chord.Modifiers&(keymap.Primary|gfx.ModifierControl|gfx.ModifierAlt) == 0 && p.typing(event.Target) {
		return gfx.EventResult{}, false // the key types, or edits, in the field
	}
	target := event.Target
	// A page-wide Shortcut acts with the focus in the page, or with none in a window
	// the page fills; not while an overlay has the keyboard.
	pageWide := !p.overlay && (p.Owns(target) || target.IsZero() && p.window)
	best := -1
	for i := range p.shortcuts {
		entry := &p.shortcuts[i]
		if entry.modifiers != chord.Modifiers || entry.char != chord.Char || entry.key != chord.Key {
			continue
		}
		switch {
		case !entry.focus.IsZero():
			if entry.focus != target {
				continue
			}
		case entry.scope != 0:
			if target.IsZero() || !p.tree.Within(target.Scope(), entry.scope) {
				continue
			}
		case !pageWide:
			continue
		}
		if best < 0 || p.narrower(entry, &p.shortcuts[best]) {
			best = i
		}
	}
	if best < 0 {
		return gfx.EventResult{}, false
	}
	return gfx.EventResult{Handled: true, Activate: p.shortcuts[best].id}, true
}

// narrower reports whether a is nearer the focus than b, both matching it: an
// element's own beats a container's, an inner container's an outer's, and any a
// page-wide one. Of two as near, the one declared first stays.
func (p *Page) narrower(a, b *shortcutEntry) bool {
	switch {
	case !b.focus.IsZero():
		return false
	case !a.focus.IsZero():
		return true
	case b.scope == 0:
		return a.scope != 0
	case a.scope == 0:
		return false
	}
	return a.scope != b.scope && p.tree.Within(a.scope, b.scope)
}

// typing reports whether the control named id takes typing: a text field, a
// Combobox, or a Control that is a text field or a terminal.
func (p *Page) typing(id gfx.ID) bool {
	if id.IsZero() {
		return false
	}
	for _, entry := range p.fields {
		if entry.id == id {
			return true
		}
	}
	for _, s := range p.combos {
		if s.ids.input == id {
			return true
		}
	}
	h := p.control(id)
	return h != nil && (h.Role == gfx.RoleTextField || h.Role == gfx.RoleTerminal)
}

// onShortcut is the Config's OnShortcut, where a window offers the chords of
// printable keys (Primary+S), and a printable key itself when nothing edits text:
// an open Menu's typeahead, then a Shortcut's, then base's.
func (p *Page) onShortcut(event gfx.KeyEvent) gfx.EventResult {
	if event.Modifiers&^gfx.ModifierShift == 0 {
		// A printable key, when nothing edits text: an open Menu's or a Tree's typeahead.
		if p.menuRoute != nil {
			if result, ok := p.menuRoute(p, event); ok {
				return result
			}
		}
		if p.typeRoute != nil {
			if result, ok := p.typeRoute(p, event); ok {
				return result
			}
		}
	}
	if p.shortcutRoute != nil {
		if result, ok := p.shortcutRoute(p, event); ok {
			return result
		}
	}
	if p.baseShortcut != nil {
		return p.baseShortcut(event)
	}
	return gfx.EventResult{}
}
