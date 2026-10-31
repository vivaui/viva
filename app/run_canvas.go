//go:build darwin || windows || linux

package app

import (
	"fmt"
	"runtime"
	"sync"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/ui"
)

// RunCanvas presents a retained canvas and blocks until the native window
// closes. When config.Paint is set the canvas is repainted on window resizes
// and pointer interactions; otherwise the initial canvas stays unchanged.
func RunCanvas(config Config, canvas *ui.Canvas) error {
	normalized, err := config.normalize()
	if err != nil {
		return err
	}
	if normalized.TextInput != nil && !nativeTextInputSupported {
		return fmt.Errorf("%w: native text input", ErrUnsupported)
	}
	if runner != nil {
		return runner(normalized, canvas)
	}
	if handled, err := runReloadWorkerIfEnabled(normalized, canvas); handled {
		return err
	}
	if canvas == nil {
		canvas = &ui.Canvas{}
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	canvas.SetReduceMotion(nativeReduceMotion())
	canvas.SetTextEngine(nativeTextEngine())
	painter, err := newPainter(canvas, &normalized)
	if err != nil {
		return err
	}
	painter.setClipboardText = func(value string) { _ = nativeSetClipboardText(value) }
	painter.setCursor = nativeSetCursor
	// Transitions wake the event loop through the adapter's thread-safe post
	// primitive: no reader goroutine, and no timer until something animates.
	var waker *platform.Waker
	if normalized.Invalidator != nil {
		waker = &normalized.Invalidator.waker
	}
	if normalized.Paint != nil {
		if waker == nil {
			waker = &platform.Waker{}
		}
		frames := newFrameTimer(waker.Wake)
		defer frames.stop()
		painter.scheduleFrame = frames.request
		painter.scheduleIfAnimating()
	}
	var resizeHandler platform.ResizeHandler
	var pointerHandler platform.PointerHandler
	var keyHandler platform.KeyHandler
	var scrollHandler platform.ScrollHandler
	var redrawHandler platform.RedrawHandler
	if normalized.Paint != nil {
		resizeHandler = painter.resize
	}
	if normalized.Paint != nil || normalized.OnPointer != nil || normalized.TextInput != nil {
		pointerHandler = painter.pointer
	}
	if normalized.Paint != nil || normalized.OnKey != nil || normalized.TextInput != nil {
		keyHandler = painter.key
	}
	if normalized.Paint != nil || normalized.OnScroll != nil {
		scrollHandler = painter.scroll
	}
	var dropHandler platform.FileDropHandler
	if normalized.OnFileDrop != nil {
		dropHandler = painter.fileDrop
	}
	if waker != nil {
		redrawHandler = wakeHandler(normalized.OnWake, painter.repaint)
	}
	defer watchDesktopSettings(canvas, waker)()
	var closeHandler platform.CloseRequestHandler
	if normalized.OnCloseRequest != nil {
		closeHandler = platform.CloseRequestHandler(normalized.OnCloseRequest)
	}

	return runNative(platform.Config{
		Actions:   painter.windowActions(),
		TextInput: painter.initialTextInput(),
		Title:     normalized.Title, Class: normalized.Class, Icon: normalized.Icon.platform(), Width: normalized.Width, Height: normalized.Height,
		MinWidth: normalized.MinWidth, MinHeight: normalized.MinHeight,
		Origin: normalized.originPoint(), HasOrigin: normalized.Origin != nil,
		WindowMode:      platform.WindowMode(normalized.WindowMode),
		WindowMaterials: normalized.WindowMaterials,
		TitlebarHeight:  normalized.TitlebarHeight,
		Anchor:          normalized.Anchor.rect, HasAnchor: normalized.Anchor.set,
		ClearColor: platform.Color{
			Red: normalized.ClearColor.Red, Green: normalized.ClearColor.Green,
			Blue: normalized.ClearColor.Blue, Alpha: normalized.ClearColor.Alpha,
		},
		Rectangles:       painter.renderer.Rectangles(),
		TextLabels:       painter.labels,
		Accessibility:    painter.accessibility,
		OnResize:         resizeHandler,
		OnFrameChange:    frameChangeHandler(normalized.OnFrameChange),
		OnMinimize:       minimizeHandler(normalized.OnMinimize),
		OnMaximize:       maximizeHandler(normalized.OnMaximize),
		OnPointer:        pointerHandler,
		PressAt:          painter.pressAt,
		OnKey:            keyHandler,
		OnShortcut:       painter.shortcutHandler(&normalized),
		OnAccessibility:  painter.accessibilityHandler(&normalized),
		OnFocusLost:      painter.focusLostHandler(),
		OnScroll:         scrollHandler,
		OnFileDrop:       dropHandler,
		OnRedraw:         redrawHandler,
		Waker:            waker,
		OnCloseRequest:   closeHandler,
		OnClosed:         normalized.OnClosed,
		DynamicTextInput: normalized.TextInput != nil,
	})
}

// frameChangeHandler adapts the application's frame callback to the platform's.
func frameChangeHandler(callback func(ui.Rect)) platform.FrameChangeHandler {
	if callback == nil {
		return nil
	}
	return platform.FrameChangeHandler(callback)
}

// minimizeHandler adapts the application's minimize callback to the platform's.
// OpenWindow always installs one, so the runtime's own state stays current.
func minimizeHandler(callback func(bool)) platform.MinimizeHandler {
	if callback == nil {
		return nil
	}
	return platform.MinimizeHandler(callback)
}

// maximizeHandler adapts the application's maximize callback to the platform's.
// OpenWindow always installs one, so the runtime's own state stays current.
func maximizeHandler(callback func(bool)) platform.MaximizeHandler {
	if callback == nil {
		return nil
	}
	return platform.MaximizeHandler(callback)
}

func prepareNativeWindow(config Config, canvas *ui.Canvas) (platform.Config, func(), error) {
	normalized, err := config.normalize()
	if err != nil {
		return platform.Config{}, nil, err
	}
	if normalized.TextInput != nil && !nativeTextInputSupported {
		return platform.Config{}, nil, fmt.Errorf("%w: native text input", ErrUnsupported)
	}
	if canvas == nil {
		canvas = &ui.Canvas{}
	}
	var cleanupOnce sync.Once
	var frames *frameTimer
	cleanup := func() {
		cleanupOnce.Do(func() {
			if frames != nil {
				frames.stop()
			}
		})
	}
	canvas.SetReduceMotion(nativeReduceMotion())
	canvas.SetTextEngine(nativeTextEngine())
	painter, err := newPainter(canvas, &normalized)
	if err != nil {
		cleanup()
		return platform.Config{}, nil, err
	}
	painter.setClipboardText = func(value string) { _ = nativeSetClipboardText(value) }
	painter.setCursor = nativeSetCursor
	var waker *platform.Waker
	if normalized.Invalidator != nil {
		waker = &normalized.Invalidator.waker
	}
	if normalized.Paint != nil {
		if waker == nil {
			waker = &platform.Waker{}
		}
		frames = newFrameTimer(waker.Wake)
		painter.scheduleFrame = frames.request
		painter.scheduleIfAnimating()
	}
	var resizeHandler platform.ResizeHandler
	var pointerHandler platform.PointerHandler
	var keyHandler platform.KeyHandler
	var scrollHandler platform.ScrollHandler
	var redrawHandler platform.RedrawHandler
	if normalized.Paint != nil {
		resizeHandler = painter.resize
	}
	if normalized.Paint != nil || normalized.OnPointer != nil || normalized.TextInput != nil {
		pointerHandler = painter.pointer
	}
	if normalized.Paint != nil || normalized.OnKey != nil || normalized.TextInput != nil {
		keyHandler = painter.key
	}
	if normalized.Paint != nil || normalized.OnScroll != nil {
		scrollHandler = painter.scroll
	}
	var dropHandler platform.FileDropHandler
	if normalized.OnFileDrop != nil {
		dropHandler = painter.fileDrop
	}
	if waker != nil {
		redrawHandler = wakeHandler(normalized.OnWake, painter.repaint)
	}
	stopWatchingDesktop := watchDesktopSettings(canvas, waker)
	var closeHandler platform.CloseRequestHandler
	if normalized.OnCloseRequest != nil {
		closeHandler = platform.CloseRequestHandler(normalized.OnCloseRequest)
	}
	actions := painter.windowActions()
	return platform.Config{
		Actions:   actions,
		TextInput: painter.initialTextInput(),
		Title:     normalized.Title, Class: normalized.Class, Icon: normalized.Icon.platform(), Width: normalized.Width, Height: normalized.Height,
		MinWidth: normalized.MinWidth, MinHeight: normalized.MinHeight,
		Origin: normalized.originPoint(), HasOrigin: normalized.Origin != nil,
		WindowMode:      platform.WindowMode(normalized.WindowMode),
		WindowMaterials: normalized.WindowMaterials,
		TitlebarHeight:  normalized.TitlebarHeight,
		Anchor:          normalized.Anchor.rect, HasAnchor: normalized.Anchor.set,
		ClearColor: platform.Color{
			Red: normalized.ClearColor.Red, Green: normalized.ClearColor.Green,
			Blue: normalized.ClearColor.Blue, Alpha: normalized.ClearColor.Alpha,
		},
		Rectangles: painter.renderer.Rectangles(), TextLabels: painter.labels,
		Accessibility: painter.accessibility, OnResize: resizeHandler,
		OnFrameChange: frameChangeHandler(normalized.OnFrameChange),
		OnMinimize:    minimizeHandler(normalized.OnMinimize),
		OnMaximize:    maximizeHandler(normalized.OnMaximize),
		OnReopen:      normalized.onReopen,
		OnPointer:     pointerHandler, PressAt: painter.pressAt,
		OnKey: keyHandler, OnShortcut: painter.shortcutHandler(&normalized), OnAccessibility: painter.accessibilityHandler(&normalized), OnFocusLost: painter.focusLostHandler(), OnScroll: scrollHandler, OnFileDrop: dropHandler, OnRedraw: redrawHandler,
		Waker: waker, OnCloseRequest: closeHandler,
		OnClosed:         normalized.OnClosed,
		DynamicTextInput: normalized.TextInput != nil,
	}, func() { stopWatchingDesktop(); cleanup() }, nil
}
