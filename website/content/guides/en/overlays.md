---
title: Dialogs, menus and popups
summary: Layers, focus scopes and dismissal, with a dropdown menu that closes on an outside press and a modal dialog.
order: 9
group: guides
---

Overlays are painted last, on a higher layer, and they change where input can go. In `flex` an overlay is declared in the page, in the place where it is open, and takes no room there. Viva gives every overlay three pieces.

- **A layer.** A popover, a dialog and `f.Popup()` are painted over the page, and hits declared on them win over hits beneath.
- **A focus scope.** A dialog traps Tab inside itself while it is open, so nothing behind it can take keyboard focus; a popover keeps Tab among its own controls first. Both give the focus back to where it was as they close.
- **A dismiss identity.** The identity that reports a click when the user presses Escape or presses outside the overlay. A page (`flex.Popover`, `flex.Modal`) makes it and closes the overlay for you; with a `flex.Frame` you name it and read it where the overlay is declared, like any other click.

As everywhere in Viva, whether an overlay is open is your state. You declare it, or you do not.

<!-- example: overlay/main.go -->
```go
package main

import (
	"log"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/flex"
)

type model struct {
	menuOpen   bool
	dialogOpen bool
	status     string
}

func (m *model) page(c flex.C) {
	flex.Column(c).Fill().Pad(24).Gap(12).Items(flex.Start).Children(func() {
		actions := flex.Button(c, "Actions").Outline()
		if actions.Clicked() {
			m.menuOpen = !m.menuOpen
		}
		flex.Text(c, m.status).Muted()

		// A popover hangs off its anchor while menuOpen is true. A press outside it
		// or Escape closes it, and the press goes on to what is under the pointer;
		// the focus goes back to the anchor.
		flex.PopoverWith(c, actions, flex.PopoverSpec{Width: 180, Align: flex.Start}, &m.menuOpen, func() {
			if flex.Button(c, "New file").Ghost().Clicked() {
				m.menuOpen, m.status = false, "Created"
			}
			if flex.Button(c, "Delete…").Ghost().Destructive().Clicked() {
				m.menuOpen, m.dialogOpen = false, true
			}
		})

		// A modal dims the page and keeps the keyboard while dialogOpen is true:
		// Escape, a press on the dim and the close button set it to false.
		flex.ModalWith(c, &m.dialogOpen, "Delete file?", flex.ModalSpec{Description: "This cannot be undone.", Width: 400}, nil, func() {
			if flex.Button(c, "Cancel").Outline().Clicked() {
				m.dialogOpen = false
			}
			if flex.Button(c, "Delete").Destructive().Clicked() {
				m.dialogOpen, m.status = false, "Deleted"
			}
		})
	})
}

func main() {
	m := &model{}
	if err := flex.Run(app.Config{Title: "Overlays", Width: 640, Height: 360}, m.page); err != nil {
		log.Fatal(err)
	}
}
```

## Popovers

`flex.Popover(c, anchor, &open, body)` shows the panel `body` declares, hanging off `anchor`, while `open` is true, with shadcn's popover look. `flex.PopoverWith` takes a `flex.PopoverSpec` for the side, the alignment and the width. It opens below the anchor and flips above it when there is no room. Keep the trigger's `Box` (`actions := flex.Button(c, ...)`) to anchor to it.

A press outside the panel or Escape sets `open` to false. The press then goes on to what is under the pointer, as a web popover's does, so one click on another button both closes the menu and presses that button. A press on the anchor only closes the panel, so a trigger that toggles `open` does not open it again in the same click. The focus goes back to where it was when the panel opened. `Popover` reports the paint that closed it; a panel that must stay open sets `open` back to true there. With a `flex.Frame`, `f.Popover(anchor, flex.PopoverSpec{Scope: ..., DismissID: ...})` is the same panel with identities of yours, and `Dismissed()` reports the closing press.

A popover shows in its window: one that must reach beyond the window's edges is a [window of its own](/en/docs/windows/).

The example's test checks all of it: a press at the far corner closes the menu, Escape closes it, and choosing Delete… closes it and opens the dialog.

## Popups

`f.Popup()` is the piece popovers, selects and menus are built from: it takes no room in its container, is as big as its content, is kept inside the window and is painted over everything, and does nothing else. Place it with `BelowBox`, `AboveBox` or `BesideBox` for an element of the page, or with `At(x, y)`, `Below(rect, ...)` and `Above(rect, ...)` for a point or rectangle; it uses the side you ask for and flips to the other when there is no room. `Box.Rect()` is zero until the frame ends, which is why the `Box` variants exist. A context menu is a popup placed `At` the pointer, opened from `OnPointer` on a `PointerButtonSecondary` down; closing it is yours, with a window-sized `Hit` under it and `canvas.SetEscapeTarget`.

## Dialogs

`flex.Modal(c, &open, title, body, footer)` dims the window, paints a centered surface with the title and a close button, and keeps the keyboard while `open` is true. The title is required: it is the dialog's name for assistive technology. `body` declares the content and `footer` the buttons, which sit against the right edge in the order given; either may be nil, and the surface is as tall as the body needs. Escape, a press on the dim and the close button set `open` to false. The first control in the body takes the focus as the dialog opens (`ModalSpec.InitialFocus` names another), and the focus goes back to the control that opened it as it closes. `flex.ModalWith` takes a `flex.ModalSpec` for a description, a size or no close button.

With a `flex.Frame`, `f.Dialog(flex.DialogSpec{...})` is the same dialog with identities of yours: `Scope` is the focus scope, `DismissID` the identity the dim and Escape report, `CloseID` the corner button's, and `InitialFocusID` the control that takes the focus. `Dismissed()` is true in the paint that delivers one of those clicks.

## Selects and the rest

`f.Select` is a drop-down list: a `flex.SelectState` and a scope of its own in your model, and the list opens under the trigger, or above it when there is no room, painted over everything. Inside a dialog it opens above the dialog. Tooltips and other non-modal panels are popups that declare no dismiss identity, so they neither trap focus nor swallow outside presses. Combobox, command palette, menubar and navigation menu follow the same layer, scope and dismiss conventions; the [component catalog](/en/ui/dialog/) has a runnable example of each, and the [API reference](/en/api/components/) has every option.

## Entrance and exit

A dialog, a sheet, a drawer and the menus, popovers, tooltips and toasts of the component recipes fade and move in when you start painting them and out when you stop, as the shadcn classes `animate-in` and `animate-out` do. You keep nothing for it: stop declaring the dialog and the canvas draws its last frame out over the next 100 ms, without hit regions, a focus trap or an Escape target, so it never takes a press or the keyboard from what is under it. Painting it again while it leaves turns it around from where it is. It needs a stable identity (the dialog's `DismissID`, a menu's `DismissID` or `Scope`), a menu, select or combobox needs its `End()` call, and a theme whose `Transition` has no duration, reduced motion, or a runtime without a frame clock shows the final state at once.
