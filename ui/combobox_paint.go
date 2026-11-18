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
