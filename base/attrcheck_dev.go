//go:build dev || vivadev

package base

import (
	"os"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/style"
)

// attrCheckWarn makes the rules report and continue instead of stopping the
// first declaration error: a sweep of a whole page is how the existing pages are
// cleaned when a rule is added.
var attrCheckWarn = os.Getenv("VIVA_ATTR_CHECK") == "warn"

// checkAttributes reports the generic attributes a declared element cannot use,
// so a page learns at declaration time that the attribute would have had no
// effect instead of finding out later. It runs only in dev builds; the release
// build has an empty implementation, so the rules cost nothing shipped.
func (f *Frame) checkAttributes() {
	for i := range f.Nodes {
		f.checkAttributesOf(&f.Nodes[i])
	}
}

// markAttr records that the caller set a generic attribute whose value the
// element cannot be asked to tell apart from its default.
func (n *Node) markAttr(a uint8) { n.attrs |= a }

// checkAttributesOf reports the generic attributes n cannot use.
func (f *Frame) checkAttributesOf(n *Node) {
	name := kindAttributeName(n)
	if n.Flags&FlagChildren != 0 && !f.attributeChildContainer(n) {
		f.badAttribute(n, name, "Children", "only containers and the controls with a content slot take Children; use Label, Caption or the control's own content")
	}
	// Gap, Items and Justify place the children of a layout container. A control
	// with a content slot is a Row while it holds children, so it takes them too.
	layouts := n.Kind == kindContainer || n.isContainer() || n.Kind == kindTabs || n.Kind == kindSelect
	// A Button with an icon spaces the icon from its label with Gap; with
	// Children it is a Row, which spans the gap between its children.
	takesGap := layouts
	if n.Kind == kindButton && hasIcon(f, n) {
		takesGap = true
	}
	switch n.Kind {
	case kindSwitch, kindCheckbox:
		// Their own rules below say why Gap does nothing.
	default:
		if n.attrs&attrGap != 0 && (!takesGap || !gapDirection(n.Dir)) {
			f.badAttribute(n, name, "Gap", "Gap spaces the children of a Row, Column, Flow or Grid; this element lays no children out that way")
		}
	}
	if n.attrs&attrItems != 0 && (!layouts || !itemsDirection(n.Dir)) {
		f.badAttribute(n, name, "Items", "Items places the children across a Row, Column, Stack or Flow; this element lays no children out that way")
	}
	if n.attrs&attrJustify != 0 && (!layouts || !justifyDirection(n.Dir)) {
		f.badAttribute(n, name, "Justify", "Justify places the children along a Row, Column or Flow; this element lays no children out that way")
	}
	if n.attrs&attrLineGap != 0 && n.Dir != flow {
		f.badAttribute(n, name, "LineGap", "LineGap is the space between the lines of a Flow; it has no effect on any other container")
	}
	if n.Ext != 0 {
		if extra := &f.Exts[n.Ext]; extra.States != 0 && n.ID.IsZero() {
			if looks := &f.States[extra.States-1]; looks.follow == 0 {
				f.badAttribute(n, name, "Hover/Pressed/Focus", "a state look needs an identity to report the state; give the element an ID or a StateFrom")
			}
		}
	}
	if n.Flags&FlagMixed != 0 && n.Kind != kindCheckbox {
		f.badAttribute(n, name, "Mixed", "Mixed is a Checkbox's neither-on-nor-off state")
	}
	extra := (*Extra)(nil)
	if n.Ext != 0 {
		extra = &f.Exts[n.Ext]
	}
	if n.attrs&attrScroll != 0 && (extra == nil || extra.scrollState == nil) {
		f.badAttribute(n, name, "Scroll*", "Follow, Sideways, Scrollbars and Scrollbar* are a ScrollArea's (Scroll, List, Tree); on any other element they do nothing")
	}
	if extra != nil {
		extra := extra
		if extra.hitExact && n.Kind != kindCheckbox && n.Kind != kindSwitch {
			f.badAttribute(n, name, "HitExact", "only a Checkbox and a Switch target their laid-out rectangle exactly")
		}
		if extra.hitOut != [4]float32{} && !engineHitKind(n.Kind) {
			f.badAttribute(n, name, "HitOutset", "the pointer region grows beyond the rectangle only for the engine's own hits; this element declares its hit itself")
		}
		if extra.hit != (gfx.HitOptions{}) && !engineHitKind(n.Kind) && n.Kind != kindSplitter {
			f.badAttribute(n, name, "HitWith", "this element declares its own hit; HitWith's options do not replace it")
		}
	}
	if n.attrs&attrHit != 0 {
		switch n.Kind {
		case kindSelect, kindTabs, kindInput, kindTextarea:
			f.badAttribute(n, name, "Hit", "this control declares its own hit under its own identity (the select's scope, the field's snapshot ID); Hit's ID does not replace it. Name it with Label")
		}
	}
	if n.Flags&flagLabelFor != 0 && !engineHitKind(n.Kind) {
		f.badAttribute(n, name, "LabelFor", "this element declares no hit of its own to be a label of; put LabelFor on an engine element that has one")
	}
	if n.Place != placeNone && n.Parent != -1 {
		f.badAttribute(n, name, "At/Below/Above/Beside/Within", "only a Popup is placed against an anchor or kept inside the viewport")
	}
	if (n.ColSpan > 1 || n.RowSpan > 1) && !f.parentIsGrid(n) {
		f.badAttribute(n, name, "ColSpan/RowSpan", "spanning cells only means something in a Grid")
	}
	if n.Ext != 0 {
		if extra := &f.Exts[n.Ext]; extra.States != 0 {
			if looks := &f.States[extra.States-1]; looks.follow != 0 && f.Nodes[looks.follow-1].ID.IsZero() {
				f.badAttribute(n, name, "StateFrom", "the element it follows has no identity, so it reports no state; give that element an ID or a Hit")
			}
		}
	}
	switch n.Kind {
	case kindIcon, kindImage, kindSpacer:
		if n.Flags&flagSetPad != 0 || n.PadSet != 0 {
			f.badAttribute(n, name, "Pad", "this element's measurement is its own; Pad does not move what it paints")
		}
	}
	switch n.Kind {
	case kindImage:
		if n.Flags&flagHasColor != 0 {
			f.badAttribute(n, name, "Color", "an Image paints its own colors")
		}
	case kindText:
		// A Text is where the element-level type lives: every text attribute is its own.
	case kindSlot:
		// A Slot's Label is its DrawContext's; element-level type styles nothing.
		f.checkTextAttributes(n, name, false, false, false, false, false)
	case kindContainer:
		f.checkTextAttributes(n, name, false, false, false, false, false)
	}
	if f.attributeWidget(n) {
		f.checkWidgetAttributes(n, name)
	}
	if n.Flags&FlagThemed != 0 {
		f.checkThemedAttributes(n, name)
	}
}

// attributeWidget reports whether n is a widget that paints its whole surface
// itself: the engine draws the element's box first and the widget covers it, so
// most generic attributes never reach what the page sees.
func (f *Frame) attributeWidget(n *Node) bool {
	switch n.Kind {
	case kindSlider, kindSegmented, kindTable, kindInputGroup, kindProgress,
		kindSpinner, kindSplitter, kindSplitButton, kindPagination:
		return true
	}
	return false
}

// hasIcon reports whether the element carries an icon of its own.
func hasIcon(f *Frame, n *Node) bool {
	if n.Ext == 0 {
		return false
	}
	icon := f.Exts[n.Ext].icon
	return len(icon.Commands) > 0 || len(icon.Layers) > 0
}

func gapDirection(dir direction) bool {
	return dir == row || dir == column || dir == flow || dir == grid
}

func itemsDirection(dir direction) bool {
	return dir == row || dir == column || dir == StackDir || dir == flow
}

func justifyDirection(dir direction) bool {
	return dir == row || dir == column || dir == flow
}

// attributeChildContainer reports whether an element takes Children that the
// layout places: a container, or a themed control whose content slot makes it a
// container. A Slot is where a caller puts the content it hands a host, and its
// Children are painted in its rectangle.
func (f *Frame) attributeChildContainer(n *Node) bool {
	return n.isContainer() || n.Kind == kindSlot
}

// checkTextAttributes reports the element-level type attributes n ignores.
func (f *Frame) checkTextAttributes(n *Node, name string, mono, face, line, align bool, wrap bool) {
	if !mono && n.Flags&flagMono != 0 {
		f.badAttribute(n, name, "Mono", "Mono styles a Text or a single-line text field; this element paints no monospace text")
	}
	if !face && n.Face != 0 {
		f.badAttribute(n, name, "MonoFace", "MonoFace styles a Text or a single-line text field; this element paints no monospace text")
	}
	if !line && n.LineH > 0 {
		f.badAttribute(n, name, "LineHeight", "LineHeight styles the text this element lays out; this element lays out no text")
	}
	if !align {
		if n.Flags&FlagTextCenter != 0 {
			f.badAttribute(n, name, "TextCenter", "TextCenter aligns a Text or a text field's value; this element aligns no text")
		}
		if n.TextEnd {
			f.badAttribute(n, name, "TextEnd", "TextEnd aligns a Text or a text field's value to its direction's end")
		}
		if n.TextDirection != 0 {
			f.badAttribute(n, name, "TextDirection", "TextDirection sets the paragraph direction of a Text or a text field's value")
		}
	}
	if !wrap && n.MaxLines > 0 {
		f.badAttribute(n, name, "MaxLines", "MaxLines wraps a Text or autoresizes a Textarea; this element does not wrap")
	}
}

// checkThemedAttributes reports the attributes a themed control does not use.
func (f *Frame) checkThemedAttributes(n *Node, name string) {
	switch n.Kind {
	case kindButton:
		f.checkTextAttributes(n, name, false, false, true, false, false)
		f.checkButtonSize(n, name, true)
	case kindBadge:
		f.checkTextAttributes(n, name, false, false, true, false, false)
		f.checkButtonSize(n, name, false)
	case kindToggle:
		f.checkTextAttributes(n, name, false, false, true, false, false)
		f.checkButtonSize(n, name, true)
	case kindTabs:
		f.checkTextAttributes(n, name, false, false, false, false, false)
		f.checkButtonSize(n, name, false)
		if n.Variant != 0 {
			f.badAttribute(n, name, "Variant", "a Tabs strip has its own variants: use Line() or Vertical()")
		}
	case kindSelect:
		f.checkTextAttributes(n, name, false, false, false, false, false)
		f.checkButtonSize(n, name, true)
		if n.ButtonSize == style.SizeLarge || n.ButtonSize == style.SizeIcon {
			f.badAttribute(n, name, "Size", "a Select trigger has the default, Small and XSmall sizes")
		}
		if n.Variant != 0 {
			f.badAttribute(n, name, "Variant", "a Select trigger has no variants")
		}
	case kindInput:
		f.checkTextAttributes(n, name, true, true, true, true, false)
		f.checkButtonSize(n, name, false)
		if n.Flags&flagSetPad != 0 || n.PadSet != 0 {
			// A field's Pad is the room inside its border: it works.
		}
		if n.Variant != 0 {
			f.badAttribute(n, name, "Variant", "an Input has no variants")
		}
	case kindTextarea:
		f.checkTextAttributes(n, name, false, false, true, false, true)
		f.checkButtonSize(n, name, false)
		if n.Variant != 0 {
			f.badAttribute(n, name, "Variant", "a Textarea has no variants")
		}
	case kindCheckbox:
		if n.Flags&flagSetPad != 0 || n.PadSet != 0 {
			f.badAttribute(n, name, "Pad", "a checkbox is its box and caption; Pad moves nothing")
		}
		if n.attrs&attrGap != 0 {
			f.badAttribute(n, name, "Gap", "a checkbox is its box and caption; Gap spaces no children")
		}
		f.checkTextAttributes(n, name, false, false, false, false, false)
		f.checkButtonSize(n, name, false)
		if n.Variant != 0 {
			f.badAttribute(n, name, "Variant", "a Checkbox has no variants")
		}
		f.checkCaptionType(n, name)
	case kindSwitch:
		if n.Flags&flagSetPad != 0 || n.PadSet != 0 {
			f.badAttribute(n, name, "Pad", "a switch is its track and caption; Pad moves nothing")
		}
		if n.attrs&attrGap != 0 {
			f.badAttribute(n, name, "Gap", "a switch is its track and caption; Gap spaces no children")
		}
		f.checkTextAttributes(n, name, false, false, false, false, false)
		f.checkButtonSize(n, name, true)
		if n.ButtonSize != 0 && n.ButtonSize != style.SizeSmall {
			f.badAttribute(n, name, "Size", "a Switch has the default and Small sizes")
		}
		if n.Variant != 0 {
			f.badAttribute(n, name, "Variant", "a Switch has no variants")
		}
		f.checkCaptionType(n, name)
	}
}

// checkButtonSize reports a size method the control does not read; some controls
// take no Size call at all, and some take only a subset (checkThemedAttributes
// adds the subsets).
func (f *Frame) checkButtonSize(n *Node, name string, any bool) {
	if !any && n.ButtonSize != 0 {
		f.badAttribute(n, name, "Size", "this control does not read the size methods (XSmall, Small, Large, IconOnly)")
	}
}

// checkCaptionType reports type styling on a Checkbox or Switch with no caption,
// where there is no text it could style.
func (f *Frame) checkCaptionType(n *Node, name string) {
	if n.Text != "" {
		return
	}
	if n.FontSet || n.WeightSet || n.Flags&flagHasColor != 0 {
		f.badAttribute(n, name, "FontSize/Weight/Color", "this control's type styles its Caption; give it one with Caption(text)")
	}
}

func (f *Frame) badAttributeIfChildren(n *Node, name string) {
	if n.Flags&FlagChildren != 0 {
		f.badAttribute(n, name, "Children", "this control has no content slot: its default content is its label")
	}
}

// engineHitKind reports whether the engine declares the element's own hit, the
// kinds Hit and LabelFor reach.
func engineHitKind(k Kind) bool {
	switch k {
	case kindButton, kindBadge, kindToggle, kindContainer, kindText, kindIcon, kindSlot:
		return true
	}
	return false
}

// parentIsGrid reports whether the element sits in a Grid.
func (f *Frame) parentIsGrid(n *Node) bool {
	return n.Parent >= 0 && f.Nodes[n.Parent].Kind == kindContainer && f.Nodes[n.Parent].Dir == grid
}

// checkWidgetAttributes reports the generic attributes a widget that paints its
// own surface does not use.
func (f *Frame) checkWidgetAttributes(n *Node, name string) {
	extra := &Extra{}
	if n.Ext != 0 {
		extra = &f.Exts[n.Ext]
	}
	rejectPad := func() {
		if n.Flags&flagSetPad != 0 || n.PadSet != 0 {
			f.badAttribute(n, name, "Pad", "this widget paints its whole surface; Pad moves nothing it paints")
		}
		if n.attrs&attrGap != 0 || n.attrs&attrItems != 0 || n.attrs&attrJustify != 0 {
			f.badAttribute(n, name, "Gap/Items/Justify", "this widget lays out its own parts, not children")
		}
	}
	rejectSurface := func() {
		if n.Flags&flagSetBackground != 0 {
			f.badAttribute(n, name, "Background", "this widget paints its own surface over the element's background")
		}
		if n.Flags&flagSetBorder != 0 {
			f.badAttribute(n, name, "Border", "this widget paints its own border")
		}
		if n.Flags&flagHasRadius != 0 {
			f.badAttribute(n, name, "Radius", "this widget paints its own corners")
		}
		if n.Flags&flagSetShadow != 0 {
			f.badAttribute(n, name, "Shadow", "this widget paints its own elevation")
		}
		if extra.gradient != 0 {
			f.badAttribute(n, name, "Gradient", "this widget paints its own surface over the element's gradient")
		}
	}
	rejectType := func() {
		f.checkTextAttributes(n, name, false, false, false, false, false)
		if n.FontSet {
			f.badAttribute(n, name, "FontSize", "this widget measures and styles its own text")
		}
		if n.WeightSet {
			f.badAttribute(n, name, "Weight", "this widget styles its own text")
		}
		if n.Flags&flagHasColor != 0 {
			f.badAttribute(n, name, "Color", "this widget paints its own colors")
		}
	}
	rejectChildren := func() {
		if n.Flags&FlagChildren != 0 {
			f.badAttribute(n, name, "Children", "this widget paints its own parts and has no content slot")
		}
	}
	rejectVariant := func() {
		if n.Variant != 0 {
			f.badAttribute(n, name, "Variant", "this widget has no theme variants")
		}
	}
	rejectSize := func() {
		if n.ButtonSize != 0 {
			f.badAttribute(n, name, "Size", "this widget does not read the size methods (XSmall, Small, Large, IconOnly)")
		}
	}
	rejectLabel := func() {
		if n.Label != "" {
			f.badAttribute(n, name, "Label", "this widget does not name a hit with Label; give the name where it declares its hit")
		}
	}
	rejectDisabled := func() {
		if n.Flags&FlagDisabled != 0 {
			f.badAttribute(n, name, "Disabled", "this widget does not check the disabled state; disable its own state or leave it out")
		}
	}
	switch n.Kind {
	case kindSlider:
		rejectPad()
		rejectSurface()
		rejectType()
		rejectChildren()
		rejectVariant()
		rejectSize()
	case kindSegmented:
		rejectPad()
		rejectSurface()
		rejectType()
		rejectChildren()
		rejectVariant()
		rejectSize()
		rejectLabel()
	case kindTable:
		rejectPad()
		rejectSurface()
		// FontSize and Color style the table's text; its own weight rules keep
		// headers medium and rows regular.
		f.checkTextAttributes(n, name, false, false, false, false, false)
		if n.WeightSet {
			f.badAttribute(n, name, "Weight", "a Table's header is medium and its rows regular; change the font size or color instead")
		}
		rejectChildren()
		rejectVariant()
		rejectSize()
		rejectLabel()
		rejectDisabled()
	case kindInputGroup:
		rejectPad()
		rejectSurface()
		rejectType()
		rejectChildren()
		rejectVariant()
		rejectSize()
		rejectLabel()
		rejectDisabled()
	case kindProgress:
		rejectPad()
		rejectSurface()
		rejectType()
		rejectChildren()
		rejectVariant()
		rejectSize()
		// Label names the progress bar's accessible node; it declares no hit of
		// its own to name otherwise.
		rejectDisabled()
	case kindSpinner:
		rejectPad()
		rejectSurface()
		rejectType()
		rejectChildren()
		rejectVariant()
		rejectSize()
		rejectLabel()
		rejectDisabled()
	case kindSplitter:
		rejectPad()
		rejectType()
		rejectChildren()
		rejectVariant()
		rejectSize()
	case kindSplitButton:
		rejectPad()
		rejectSurface()
		rejectType()
		rejectChildren()
	case kindPagination:
		rejectPad()
		rejectSurface()
		rejectType()
		rejectChildren()
		rejectVariant()
		rejectSize()
		rejectLabel()
		rejectDisabled()
	}
}

// badAttribute panics with the element's name, the attribute and what to do
// instead. It is the dev build's report: a page that set an attribute the
// element cannot use is a declaration error, not something to ignore.
func (f *Frame) badAttribute(n *Node, kind, attribute, hint string) {
	message := "ui: " + kind + " does not support " + attribute + ": " + hint
	if attrCheckWarn {
		println("VIVA_ATTR_CHECK: " + message)
		return
	}
	panic(message)
}

// kindAttributeName is the element's name in an attribute message: a container
// keeps the name its layout gives it.
func kindAttributeName(n *Node) string {
	if n.Kind == kindContainer {
		switch n.Dir {
		case row:
			return "Row"
		case column:
			return "Column"
		case StackDir:
			return "Stack"
		case flow:
			return "Flow"
		case grid:
			return "Grid"
		}
	}
	if n.Kind == kindSplitButton {
		return "SplitButton"
	}
	return controlName(n.Kind)
}
