package ui

import (
	"strings"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/keymap"
)

// listType moves to the row whose name begins with the letters typed, from the row
// the keys are on, choosing it; and chooses every row on Primary+A in a list that
// chooses several.
func (p *Page) ListType(event gfx.KeyEvent) (gfx.EventResult, bool) {
	text := event.Text
	if text == "" && event.Key == gfx.KeySpace {
		text = " "
	}
	if text == "" {
		return gfx.EventResult{}, false
	}
	for _, s := range p.listStates {
		if _, inside := event.Target.In(s.list.rows); !inside || s.list.f == nil || s.list.f.Surface == nil {
			continue
		}
		epoch := p.epoch + 1
		if s.Selection != nil && event.Modifiers == keymap.Primary && strings.EqualFold(text, "a") {
			s.selectAll(epoch)
			return gfx.EventResult{Handled: true, Repaint: true}, true
		}
		if s.Label == nil || s.NoTypeahead || event.Modifiers&^gfx.ModifierShift != 0 {
			return gfx.EventResult{}, false
		}
		rows := components.MenuTypeaheadRows{
			N: s.list.n, ID: func(i int) gfx.ID { return s.list.rows.At(s.key(i)) }, Label: s.Label,
		}
		if s.Header != nil {
			rows.Disabled = s.isHeader
		}
		now := clockEpoch.Add(s.list.f.Surface.Now())
		typing := text == " " && s.list.typeahead.Typing(now)
		if id, ok := s.list.typeahead.TypeRows(text, now, rows, event.Target); ok {
			if row := s.indexOf(id); row >= 0 {
				s.chooseRow(row, epoch)
				s.revealRow(row)
			}
			return gfx.EventResult{Handled: true, Repaint: true, Focus: id, FocusVisible: true}, true
		}
		if typing {
			return gfx.EventResult{Handled: true}, true
		}
		return gfx.EventResult{}, false
	}
	return gfx.EventResult{}, false
}
