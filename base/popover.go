package base

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/style"
)

// PopoverSpec declares a Popover: its identities, which are the application's, and
// where it goes.
type PopoverSpec struct {
	// Scope is the focus scope of the panel: Tab moves among its controls, and the
	// focus goes back to where it was when the panel closes. DismissID is the target
	// of a press outside the panel, of Escape and, unless KeepOpenOnAnchor, of a press
	// on the anchor: Dismissed reports it.
	Scope, DismissID gfx.ID
	// InitialFocusID, when set, takes the keyboard as the panel opens. It is asked
	// once, as AutoFocus is, and not again while the panel stays open, so the user
	// can move the focus inside it without the next paint pulling it back.
	InitialFocusID gfx.ID
	// Side is the edge of the anchor the panel opens on (below it unless set); it
	// flips to the opposite edge when there is no room. Align lines the panel up with
	// the anchor along that edge (Auto is Center). Offset is the gap to the anchor,
	// 4 when zero.
	Side   style.PopoverSide
	Align  Align
	Offset float32
	// AlignOffset moves the panel along the anchor from where Align put it, as
	// components.HoverCardOptions.AlignOffset does: positive toward the end.
	AlignOffset float32
	// Title and Description, when set, head the panel as shadcn's PopoverHeader
	// does: the title in the popover foreground, medium weight, and under it the
	// description in the muted color, wrapped to the width. The body follows.
	Title, Description string
	// TitleWeight is the title's font weight, medium when zero, as
	// components.PopoverOptions.TitleWeight is.
	TitleWeight float32
	// Width is the panel's width, 288 when zero; its height fits what is in it
	// (give the body a Height to fix it).
	Width float32
	// Padding insets the panel's content from its surface, the theme's PadSurface
	// when zero. A menu-like panel gives components.DropdownMenuPadding so its rows
	// line up with the menus around it.
	Padding float32
	// Margin is the space the panel keeps from the edges of the window as it
	// flips and shifts to stay inside it, 8 when zero. A panel that must stay
	// clear of a title strip the page does not own gives a larger margin.
	Margin float32
	// Instant opens the panel without the entrance motion, as a list that
	// appears under the cursor as the user types has no travel of its own. The
	// layout is the animated panel's.
	Instant bool
	// Radius overrides the panel's corner radius; zero keeps the popover recipe's.
	// A hover card, painted with components.PopupRecipe, is rounder than a popover.
	Radius float32
	// NoDismiss leaves the panel without a press-outside target and without an
	// Escape target: a hover preview the pointer keeps open. Dismissed then never
	// reports it.
	NoDismiss bool
	// ConsumeOutsidePress keeps the press that closes the panel from also reaching
	// the control under it, which it does by default, as a web popover's does.
	ConsumeOutsidePress bool
	// KeepOpenOnAnchor lets a press on the anchor through to it instead of closing
	// the panel, for an anchor that is not the panel's toggle.
	KeepOpenOnAnchor bool
}

type PopoverDecl struct {
	spec   PopoverSpec
	Anchor int32
	layer  uint16 // the layer the Popover was declared in
}
