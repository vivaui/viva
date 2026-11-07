//go:build js && wasm

package app

import (
	"github.com/vivaui/viva/internal/platform/web"
	"github.com/vivaui/viva/ui"
)

// RunCanvas mounts the application into the browser page and blocks until the
// page unloads, so a program that runs natively can also be deployed for
// GOOS=js GOARCH=wasm. The Canvas2D backend is a release target with the same
// component, input, focus, text and accessibility semantics as the native
// renderers, not a visual preview.
//
// The page is the window: WindowMode, MinWidth, MinHeight, Origin, Icon,
// TitlebarHeight, WindowControls and the other native window options are
// ignored. OnResize reports viewport changes; OnMinimize reports the page
// becoming hidden or visible; OnFullscreen reports the browser's fullscreen
// mode, which RequestFullscreen asks for; OnCloseRequest gates unloading, but
// allow must be called synchronously;
// OnClosed runs on pagehide. OnFileDrop reports drops from the page's outside,
// with FileRef values instead of paths; OnFrameChange, OnMaximize and the
// native frame hooks have no browser counterpart and are ignored. Text input
// works through Config.TextInput.
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
		ClearColor: ui.Color{
			Red: float32(normalized.ClearColor.Red), Green: float32(normalized.ClearColor.Green),
			Blue: float32(normalized.ClearColor.Blue), Alpha: float32(normalized.ClearColor.Alpha),
		},
		Paint: normalized.Paint, OnPointer: normalized.OnPointer, OnKey: normalized.OnKey,
		OnShortcut: normalized.OnShortcut, OnScroll: normalized.OnScroll, OnModifierTap: normalized.OnModifierTap,
		OnFileDrop: normalized.OnFileDrop,
		OnResize:   normalized.OnResize, OnMinimize: normalized.OnMinimize,
		OnFullscreen: normalized.OnFullscreen, OnCloseRequest: normalized.OnCloseRequest,
		OnClosed:  normalized.OnClosed,
		TextInput: webTextInput(normalized.TextInput),
		Canvas:    canvas,
		OnWake:    normalized.OnWake,
	}
	if normalized.Invalidator != nil {
		page.Waker = &normalized.Invalidator.waker
	}
	return runWeb(page)
}
