package ui

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
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
	Side   components.PopoverSide
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
	// ConsumeOutsidePress keeps the press that closes the panel from also reaching
	// the control under it, which it does by default, as a web popover's does.
	ConsumeOutsidePress bool
	// KeepOpenOnAnchor lets a press on the anchor through to it instead of closing
	// the panel, for an anchor that is not the panel's toggle.
	KeepOpenOnAnchor bool
}

type popoverDecl struct {
	spec   PopoverSpec
	Anchor int32
	layer  uint16 // the layer the Popover was declared in
}
