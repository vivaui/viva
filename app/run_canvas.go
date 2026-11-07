//go:build darwin || windows || linux

package app

import (
	"fmt"
	"runtime"
	"slices"
	"sync"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/ui"
)

// RunCanvas presents a retained canvas and blocks until the native window
// closes. When config.Paint is set the canvas is repainted on window resizes
// and pointer interactions; otherwise the initial canvas stays unchanged.
func RunCanvas(config Config, canvas *ui.Canvas) error {
	attendFaults()
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
	painter.setCursorImage = nativeSetCursorImage
	native, frames := nativeWindowConfig(&normalized, painter)
	if frames != nil {
		defer frames.stop()
	}
	defer watchDesktopSettings(canvas, native.Waker)()
	return runNative(native)
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

// nativeWindowConfig is the window a platform adapter opens: the painter's
// initial scene, input and text state, plus the callbacks that keep it current.
// It reads no native API, so the wiring can be assembled and driven without the
// window loop's main thread. Callers have already given the canvas the
// platform's own preferences. Frames, when not nil, runs the window's
// animations and is stopped when the window closes.
func nativeWindowConfig(normalized *Config, painter *painter) (platform.Config, *frameTimer) {
	// Transitions wake the event loop through the adapter's thread-safe post
	// primitive: no reader goroutine, and no timer until something animates.
	var waker *platform.Waker
	if normalized.Invalidator != nil {
		waker = &normalized.Invalidator.waker
	}
	var frames *frameTimer
	if normalized.Paint != nil {
		if waker == nil {
			waker = &platform.Waker{}
		}
		frames = newFrameTimer(waker.Wake)
		painter.scheduleFrame = frames.request
		painter.scheduleIfAnimating()
	}
	normalized.Web.repaintWith(painter)
	var resizeHandler platform.ResizeHandler
	var pointerHandler platform.PointerHandler
	var keyHandler platform.KeyHandler
	var scrollHandler platform.ScrollHandler
	var redrawHandler platform.RedrawHandler = painter.repaint
	var gate *visibilityGate
	if normalized.Paint != nil || painter.controls != nil || normalized.OnResize != nil || normalized.OnFullscreen != nil {
		resizeHandler = painter.resize
	}
	if normalized.Paint != nil || normalized.OnPointer != nil || normalized.TextInput != nil {
		pointerHandler = painter.pointerEvent
	}
	if normalized.Paint != nil || normalized.OnKey != nil || normalized.OnModifierTap != nil || normalized.TextInput != nil {
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
		waker.SetSpacing(frameInterval)
		gate = &visibilityGate{waker: waker}
		redrawHandler = wakeHandler(waker, gate, normalized.OnWake, painter.repaint)
	}
	var closeHandler platform.CloseRequestHandler
	if normalized.OnCloseRequest != nil {
		closeHandler = platform.CloseRequestHandler(normalized.OnCloseRequest)
	}
	return platform.Config{
		Actions:   painter.windowActions(),
		TextInput: painter.initialTextInput(),
		Title:     normalized.Title, Class: normalized.Class, Icon: normalized.Icon.platform(), Width: normalized.Width, Height: normalized.Height,
		MinWidth: normalized.MinWidth, MinHeight: normalized.MinHeight,
		Origin: normalized.originPoint(), HasOrigin: normalized.Origin != nil,
		WindowMode:      platform.WindowMode(normalized.WindowMode),
		WindowMaterials: normalized.windowMaterials(),
		TitlebarHeight:  normalized.TitlebarHeight,
		Anchor:          normalized.Anchor.rect, HasAnchor: normalized.Anchor.set,
		ClearColor: platform.Color{
			Red: normalized.ClearColor.Red, Green: normalized.ClearColor.Green,
			Blue: normalized.ClearColor.Blue, Alpha: normalized.ClearColor.Alpha,
		},
		Web:              normalized.Web.bind(),
		NativeViews:      slices.Clone(painter.lastViews),
		Rectangles:       painter.renderer.Rectangles(),
		TextLabels:       painter.labels,
		Accessibility:    painter.accessibility,
		OnResize:         resizeHandler,
		OnFrameChange:    frameChangeHandler(normalized.OnFrameChange),
		OnMinimize:       minimizeHandler(normalized.OnMinimize),
		OnVisibility:     gate.handler(),
		OnMaximize:       maximizeHandler(normalized.OnMaximize),
		OnFullscreen:     painter.fullscreenHandler(normalized.OnFullscreen),
		OnReopen:         normalized.onReopen,
		OnPointer:        pointerHandler,
		PressAt:          painter.pressAt,
		OnKey:            keyHandler,
		ModifierTaps:     normalized.OnModifierTap != nil,
		OnShortcut:       painter.shortcutHandler(normalized),
		OnAccessibility:  painter.accessibilityHandler(normalized),
		OnFocusLost:      painter.focusLostHandler(),
		OnScroll:         scrollHandler,
		OnFileDrop:       dropHandler,
		OnRedraw:         redrawHandler,
		Waker:            waker,
		OnCloseRequest:   closeHandler,
		OnClosed:         normalized.OnClosed,
		DynamicTextInput: normalized.TextInput != nil,
	}, frames
}

// prepareNativeWindow builds the native window for one application window. It
// runs on the window loop's main thread, which the platform preferences it
// reads require; a test owns the wiring through nativeWindowConfig instead.
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
	painter.setCursorImage = nativeSetCursorImage
	native, timer := nativeWindowConfig(&normalized, painter)
	frames = timer
	stopWatchingDesktop := watchDesktopSettings(canvas, native.Waker)
	return native, func() { stopWatchingDesktop(); cleanup() }, nil
}
