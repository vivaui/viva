package ui

import (
	"strconv"

	"github.com/vivaui/viva/components"
)

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
	n := &s.f.Nodes[s.node]
	ids := &s.ids
	options := components.SingleSelectOptions{
		InputID: ids.input, TriggerID: ids.trigger, DismissID: ids.dismiss, Scope: ids.focus,
		ScrollID: ids.Scroll, OptionIDs: ids.options,
		Label: s.Label, Placeholder: s.placeholder, EmptyLabel: s.empty, CreatePrefix: s.createPrefix,
		Viewport: s.f.bounds, Layer: max(s.layer, s.f.popupLayer), Disabled: n.Flags&flagDisabled != 0,
	}
	if s.clearable {
		if s.clearLabelOf != s.Label {
			s.clearLabel, s.clearLabelOf = "Clear "+s.Label, s.Label
		}
		options.ClearID, options.ClearLabel = ids.clear, s.clearLabel
	}
	components.PaintSingleSelect(d.Canvas, *d.Theme, d.Rect, &s.list, options)
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
