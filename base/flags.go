package base

// Flags are the element state bits the engine and the controls share.
const (
	FlagClip uint32 = 1 << iota
	FlagMono
	FlagTextCenter
	FlagDisabled
	FlagChecked
	FlagSmall
	FlagGrowSet
	FlagShrinkSet
	FlagHasColor
	FlagHasHit
	FlagWrap
	FlagScroll
	FlagBasis
	FlagOutline
	FlagDialog      // a Dialog's root: painted by components.Dialog around its children
	FlagAlertDialog // an AlertDialog's root: painted by components.AlertDialog
	FlagChildren    // Children was called on the element, even if it declared nothing
	FlagHasRadius   // Radius was called: on a control, its corners are its own, not the theme's
	FlagThemed      // a control with the theme's look: what the caller did not set, the recipe does
	FlagSetBackground
	FlagSetBorder
	FlagSetShadow
	FlagSetPad
	FlagSelected // the element is the chosen one of several (State.Selected)
	FlagOpen     // the element shows what it controls (State.Open)
	FlagVeto     // a click on the element is not delivered: its identity moved to it under a held press or focus
	FlagPopover  // a Popover's root: painted with components.Popover's dismiss, focus scope and Escape around its children
	FlagDrawer   // a Drawer's root: painted by components.Drawer around its children
	FlagLabelFor // the element is a label of the control its identity names (LabelFor)
	FlagMixed    // a Checkbox that is neither on nor off: some of what it stands for is chosen
	FlagInvalid  // a control whose value failed validation: the destructive border and ring
)

// flagRequired marks a page Label that appends the field title's required
// marker. It is internal: only the page Label reads it.
const flagRequired uint32 = 1 << 31
