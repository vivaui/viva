package base

import "github.com/vivaui/viva/gfx"

// Radio adds one option of a radio group, shadcn's RadioGroupItem with its label: a
// 16 pixel circle that is filled with the primary color and holds a dot while *value is
// option, and the label after it. A click sets *value to option (RadioBase's rules), and
// the focus moving onto it with the arrow keys of a RadioGroup chooses it, as a native
// radio group does. Name it with c.Key when the options are reordered; Hit gives it the
// application's identity.
//
//	ui.Radio(c, &m.plan, Monthly, "Monthly")
func Radio[T comparable](c C, Value *T, option T, Label string) Box {
	return radio(c, c.page.ControlID(pkRadio), Value, option, Label)
}

// RadioID is Radio for an option the application already names, as TextFieldID is for
// TextField: id is the option's identity in the page's Tree, so an initial focus, the
// application's own routing or a test can address it.
//
//	ui.RadioID(c, hitKeepParent, &m.ungroup, false, "Keep hosts in the parent")
func RadioID[T comparable](c C, ID gfx.ID, Value *T, option T, Label string) Box {
	return radio(c, ID, Value, option, Label)
}

func radio[T comparable](c C, ID gfx.ID, Value *T, option T, Label string) Box {
	th := c.f.Palette
	m := th.Metrics
	b := c.f.RadioBase(ID, Label, *Value == option)
	choose(c, b, Value, option)
	if b.State().FocusVisible && *Value != option {
		*Value = option
	}
	chosen := *Value == option
	border, fill := th.Input, th.Background
	fill.Alpha = 0
	if chosen {
		border, fill = th.Primary, th.Primary
	}
	ring := th.Ring
	ring.Alpha *= 0.5
	b.Height(m.ControlHeight).Gap(m.GapItem).Children(func() {
		Row(c).Width(16).Height(16).Shrink(0).Radius(8).Border(1, border).Background(fill).
			Items(Center).Justify(Center).StateFrom(b).Focus(Look{}.Border(th.Ring).Ring(3, ring)).
			Children(func() {
				if chosen {
					Row(c).Width(8).Height(8).Radius(4).Background(th.PrimaryForeground)
				}
			})
		Text(c, Label).FontSize(m.FontSizeM).Weight(gfx.WeightMedium).Ellipsis()
	})
	return b
}

// RadioGroup adds a column for the Radio options inside it: Tab reaches the chosen one
// (else the first) and goes on out of the group, and the arrow keys move the focus,
// and with it the choice, among them.
//
//	ui.RadioGroup(c).Children(func() {
//		ui.Radio(c.Key("m"), &m.plan, Monthly, "Monthly")
//		ui.Radio(c.Key("y"), &m.plan, Yearly, "Yearly")
//	})
func RadioGroup(c C) Box {
	return Column(c).Gap(12).FocusGroup(FocusBoth)
}
