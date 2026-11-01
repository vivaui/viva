//go:build js && wasm

package app

import (
	"github.com/vivaui/viva/internal/platform/web"
	"github.com/vivaui/viva/ui"
)

// RunCanvas mounts the application into the browser page and blocks until the
// page unloads, so the program that runs natively also builds for
// GOOS=js GOARCH=wasm. It paints through the Canvas2D backend, a preview next to
// the native renderers.
//
// The page is the window: WindowMode, MinWidth, MinHeight, Origin, Icon,
// TitlebarHeight, WindowControls and the other native window options are
// ignored, as are OnClosed, OnFileDrop and the frame and focus hooks the page
// has no counterpart for. Text input works through Config.TextInput.
func RunCanvas(config Config, canvas *ui.Canvas) error {
	normalized, err := config.normalize()
	if err != nil {
		return err
	}
	if canvas == nil {
		canvas = &ui.Canvas{}
	}
	if normalized.Tree != nil {
		canvas.SetTree(normalized.Tree)
	}
	page := web.Config{
		Title: normalized.Title, Width: normalized.Width, Height: normalized.Height,
		Paint: normalized.Paint, OnPointer: normalized.OnPointer, OnKey: normalized.OnKey,
		OnShortcut: normalized.OnShortcut, OnScroll: normalized.OnScroll,
		TextInput: webTextInput(normalized.TextInput),
		Canvas:    canvas,
		OnWake:    normalized.OnWake,
	}
	if normalized.Invalidator != nil {
		page.Waker = &normalized.Invalidator.waker
	}
	return runWeb(page)
}
