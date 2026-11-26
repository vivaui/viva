package ui

import (
	"sync"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/base"
)

// EditMenuLabels are the words of the Cut, Copy, Paste and Select All menu every
// TextField and TextArea opens on a secondary press; see PageConfig.EditMenu.
type EditMenuLabels = base.EditMenuLabels

// WithoutEditMenu keeps a TextField or TextArea from that menu, for a field the
// application gives a ContextMenu of its own.
func WithoutEditMenu() Option { return base.WithoutEditMenu() }

var installEditMenu, installPrimarySelection sync.Once

// useEditMenu installs the menu painter and the system clipboard the edit menu of
// a TextField or TextArea needs.
func useEditMenu() {
	useMenu()
	installEditMenu.Do(func() {
		base.ClipboardText, base.SetClipboardText = app.ClipboardText, app.SetClipboardText
	})
}

// SetClipboardText puts text on the system clipboard, for a button that copies a
// value (a fingerprint, a command): call it where the press is read. It returns the
// platform's error when the clipboard cannot be written.
//
//	if ui.Button(c, "Copy").Clicked() { _ = ui.SetClipboardText(m.fingerprint) }
func SetClipboardText(text string) error { return app.SetClipboardText(text) }
