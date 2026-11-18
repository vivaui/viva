package base

import (
	"github.com/vivaui/viva/gfx"
)

// SplitButtonSpec declares a split button: a primary action joined to a fixed-width
// trigger that opens a menu of related actions, painted by components.SplitButton.
// The two halves share one surface and variant, with their own hit identities,
// hover, focus rings, names and hints.
//
//	d := ui.SplitButton(c, ui.SplitButtonSpec{
//		PrimaryID: saveID, MenuID: saveMenuID, Label: "Save",
//		MenuLabel: "Save as", MenuHint: "Compare and save",
//	})
//	if id, ok := d.Chosen(); ok && id == saveID { save() }
type SplitButtonSpec struct {
	PrimaryID, MenuID gfx.ID
	// Label names the primary action on screen and to assistive technology.
	Label string
	// MenuLabel names the trigger, "More options" when empty; Hint and MenuHint
	// are the descriptions read after each half's name.
	MenuLabel, Hint, MenuHint string
	// Icon is the trigger's icon, a downward chevron when zero.
	Icon gfx.Icon
	// TriggerWidth is the width of the menu half, 32 when zero.
	TriggerWidth float32
	// Open marks the menu open: the trigger stays highlighted and announces
	// itself expanded.
	Open bool
}

type SplitButtonDecl struct {
	primary, menu gfx.ID
	menuLabel     string
	hint          string
	menuHint      string
	icon          gfx.Icon
	triggerWidth  float32
	open          bool
}

// splitButtonTrigger is the width of a split button's menu half when nothing else
// was said, as components.SplitButton's default.
const SplitButtonTrigger = float32(32)

// SplitButton adds a split button. Chosen reports which half a click is on.
func (f *Frame) SplitButton(spec SplitButtonSpec) Box {
	b := f.Box(kindSplitButton)
	n := &f.Nodes[b.i]
	n.ID, n.Label = spec.PrimaryID, spec.Label
	n.Flags |= flagHasHit
	n.Aux = int32(len(WidgetsOf(f).splitButtons))
	WidgetsOf(f).splitButtons = append(WidgetsOf(f).splitButtons, SplitButtonDecl{
		primary: spec.PrimaryID, menu: spec.MenuID,
		menuLabel: spec.MenuLabel, hint: spec.Hint, menuHint: spec.MenuHint,
		icon: spec.Icon, triggerWidth: spec.TriggerWidth, open: spec.Open,
	})
	return b
}

// SplitButton adds a split button to a page. An identity left zero is named by the
// page, as a Button's is; give both to keep the application's own names.
func SplitButton(c C, spec SplitButtonSpec) Box {
	if c == nil || c.page == nil || c.f == nil {
		panic("ui: SplitButton is for a page")
	}
	if spec.PrimaryID.IsZero() {
		spec.PrimaryID = c.page.ControlID(pkButton)
	}
	if spec.MenuID.IsZero() {
		spec.MenuID = c.page.ControlID(pkButton)
	}
	b := c.f.SplitButton(spec)
	c.page.Guard(b, spec.PrimaryID, spec.Label)
	return b
}

// Open marks a SplitButton's menu open or closed: the trigger keeps its highlight
// while open and tells assistive technology it is expanded.
func (b Box) Open(open bool) Box {
	if n := b.N(); n.Kind == kindSplitButton {
		WidgetsOf(b.f).splitButtons[n.Aux].open = open
	}
	return b
}

// MenuLabel names a SplitButton's trigger to assistive technology.
func (b Box) MenuLabel(Label string) Box {
	if n := b.N(); n.Kind == kindSplitButton {
		WidgetsOf(b.f).splitButtons[n.Aux].menuLabel = Label
	}
	return b
}

// MenuHint is the description read after a SplitButton trigger's name.
func (b Box) MenuHint(Text string) Box {
	if n := b.N(); n.Kind == kindSplitButton {
		WidgetsOf(b.f).splitButtons[n.Aux].menuHint = Text
	}
	return b
}

// MenuIcon sets the icon a SplitButton's trigger draws: a downward chevron when none.
func (b Box) MenuIcon(icon gfx.Icon) Box {
	if n := b.N(); n.Kind == kindSplitButton {
		WidgetsOf(b.f).splitButtons[n.Aux].icon = icon
	}
	return b
}

// MenuTriggerWidth sets the width of a SplitButton's trigger half, 32 when zero.
func (b Box) MenuTriggerWidth(width float32) Box {
	if n := b.N(); n.Kind == kindSplitButton {
		WidgetsOf(b.f).splitButtons[n.Aux].triggerWidth = width
	}
	return b
}
