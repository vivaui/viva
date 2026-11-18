package ui

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/style"
)

// draw paints the field, and its list while it is open.
func comboStateDraw(s *ComboState, d *DrawContext) {
	n := &s.GetF().Nodes[s.Node]
	ids := s.IDsPtr()
	options := style.SingleSelectOptions{
		InputID: ids.GetInput(), TriggerID: ids.GetTrigger(), DismissID: ids.GetDismiss(), Scope: ids.GetFocus(),
		ScrollID: ids.Scroll, OptionIDs: ids.GetOptions(),
		Label: s.Label, Placeholder: s.GetPlaceholder(), EmptyLabel: s.GetEmpty(), CreatePrefix: s.GetCreatePrefix(),
		Viewport: s.GetF().GetBounds(), Layer: max(s.GetLayer(), s.GetF().GetPopupLayer()), Disabled: n.Flags&FlagDisabled != 0,
	}
	if s.GetClearable() {
		if s.GetClearLabelOf() != s.Label {
			s.SetClearLabel("Clear " + s.Label)
			s.SetClearLabelOf(s.Label)
		}
		options.ClearID, options.ClearLabel = ids.GetClear(), s.GetClearLabel()
	}
	components.PaintSingleSelect(d.Canvas, *d.Theme, d.Rect, s.ListPtr(), options)
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
func boxDescribe(b Box, describe func(i int) ComboboxOption) Box {
	s := b.ComboState()
	if s == nil || describe == nil {
		return b
	}
	n := len(s.GetLabels())
	same := n == len(s.GetDescs())
	s.SetScratch(s.GetScratch()[:0])
	for i := 0; i < n; i++ {
		d := describe(i)
		if d.Label == "" {
			d.Label = s.GetLabels()[i]
		}
		s.SetScratch(append(s.GetScratch(), d))
		same = same && s.GetDescs()[i].SameAs(d)
	}
	if same {
		return b
	}
	s.SetDescs(append(s.GetDescs()[:0], s.GetScratch()...))
	s.SetOpts(s.GetOpts()[:0])
	for i, d := range s.GetDescs() {
		s.SetOpts(append(s.GetOpts(), style.SingleSelectOption{Value: s.GetKeys()[i], Label: d.Label, Detail: d.Detail, Search: d.Search, Disabled: d.Disabled, Leading: d.Leading, Trailing: d.Trailing, Content: d.Content}))
	}
	s.ListPtr().SetOptions(s.GetOpts())
	return b
}
