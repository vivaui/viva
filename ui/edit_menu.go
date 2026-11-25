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

var installEditMenu sync.Once

// useEditMenu installs the menu painter and the system clipboard the edit menu of
// a TextField or TextArea needs.
func useEditMenu() {
	useMenu()
	installEditMenu.Do(func() {
		base.ClipboardText, base.SetClipboardText = app.ClipboardText, app.SetClipboardText
	})
}
