package windows

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

const (
	wmDestroy          = 2
	wmMove             = 3
	wmSize             = 5
	wmKillFocus        = 8
	wmSetFocus         = 7
	wmPaint            = 15
	wmClose            = 16
	wmErase            = 20
	wmSetCursor        = 32
	wmGetObject        = 61
	wmNCCreate         = 129
	wmNCHitTest        = 132
	wmKeyDown          = 256
	wmChar             = 258
	wmSysKeyDown       = 260
	wmIMEStart         = 269
	wmIMEEnd           = 270
	wmIMEComposition   = 271
	wmIMERequest       = 0x0288
	wmTimer            = 275
	mkLButton          = 1
	mkAnyButton        = 0x73 // MK_LBUTTON, MK_RBUTTON, MK_MBUTTON, MK_XBUTTON1 and MK_XBUTTON2
	wmMouseMove        = 512
	wmLeftDown         = 513
	wmLeftUp           = 514
	wmRightDown        = 516
	wmRightUp          = 517
	wmMiddleDown       = 519
	wmMiddleUp         = 520
	wmXButtonDown      = 523
	wmXButtonUp        = 524
	wmWheel            = 522
	wmHorizontalWheel  = 526
	wmCaptureChanged   = 533
	wmSettingChange    = 26
	wmFontChange       = 29
	wmMouseLeave       = 675
	wmDPIChanged       = 736
	wmAllowClose       = 32769
	wmApplyCursor      = 32770
	wmRedraw           = 32771
	wsOverlappedWindow = 0x00cf0000
)

type windowState struct {
	application                            *windowsApplication
	config                                 platform.Config
	renderer                               renderer
	window                                 uintptr
	ready, tracking, sizing                bool
	minimized                              bool
	maximized                              bool
	lastFrame                              ui.Rect
	scale                                  float32
	scaleFixed                             bool // VIVA_SCALE set it: the display's own scale is ignored
	err                                    error
	pointerEvents, resizeEvents, keyEvents uint64
	started                                time.Time
	clicks                                 input.ClickCounter
	cursor                                 atomic.Uintptr
	redraws                                redrawQueue
	textState                              platform.TextState
	textScroll                             float32
	textMask                               string
	textDragging                           bool
	textDragAnchor                         int
	textInput                              platform.TextInput
	textEnabled, caretVisible, focused     bool
	caretTimer                             uintptr
	baseRectangleCount, baseTextLabelCount int
	textLabels                             []platform.TextLabel
	textRectangles                         []render.Rectangle
	textEngine                             ui.TextEngine
	textSpans                              []ui.TextSpan
	textRuns                               []ui.TextRun
	captionPress                           captionPress
	imeComposing                           bool
	pendingHighSurrogate                   uint16
	accessibility                          uiaTree
	pendingKey                             pendingKey
	deadKey                                bool
	icons                                  [2]uintptr
	drop                                   *dropTarget
	wheel                                  wheelSettings
	devices                                platform.DeviceRecovery
	ncTracking                             bool
	imeTarget                              text.Range
	imeHasTarget                           bool
	titlebarThemed, titlebarDark           bool
	backdropStyle                          platform.BackdropStyle
	backdropApplied, backdropActive        bool
	backdropNotified, backdropPolicyKnown  bool
	backdropPolicy                         backdropPolicy
	backdropPowerNotification              uintptr
}

var active atomic.Pointer[windowState]
var pending atomic.Pointer[windowState]
var windowStates sync.Map
var callbackOnce sync.Once
var callback uintptr

func Run(config platform.Config) error {
	return RunApplication(func(application platform.Application) error {
		_, err := application.OpenWindow(config)
		return err
	})
}

type windowsApplication struct {
	className *uint16
	instance  uintptr
	windows   int
	firstErr  error
	// thread is the event-loop thread, which Quit posts to; status holds the
	// application's status items, once it has any.
	thread uint32
	status *statusState
	// oleReady is set once OLE is initialized on the event-loop thread, which
	// a window needs to take files by drag and drop.
	oleReady bool
}

type nativeWindow struct {
	state  *windowState
	closed atomic.Bool
}

// reportFrame tells the application where its content area is now, in logical
// pixels of the window's display. It reports only changes.
func (state *windowState) reportFrame() {
	if !state.ready || state.config.OnFrameChange == nil || state.window == 0 || state.minimized || state.zoomed() {
		return
	}
	var client rect
	if ok, _, _ := getClientRect.Call(state.window, uintptr(unsafe.Pointer(&client))); ok == 0 {
		return
	}
	var origin point
	if ok, _, _ := clientToScreen.Call(state.window, uintptr(unsafe.Pointer(&origin))); ok == 0 {
		return
	}
	scale := state.scale
	if scale <= 0 {
		scale = 1
	}
	frame := ui.Rect{
		X: float32(origin.X) / scale, Y: float32(origin.Y) / scale,
		Width:  float32(client.Right-client.Left) / scale,
		Height: float32(client.Bottom-client.Top) / scale,
	}
	if frame == state.lastFrame {
		return
	}
	state.lastFrame = frame
	state.config.OnFrameChange(frame)
}

func (window *nativeWindow) Close() {
	if window == nil || window.closed.Swap(true) {
		return
	}
	if handle := window.state.window; handle != 0 {
		postMessage.Call(handle, wmAllowClose, 0, 0)
	}
}

func RunApplication(start platform.ApplicationStart) error {
	status, _, _ := initializeCOM.Call(0, 2)
	if err := hresult(status); err != nil {
		return err
	}
	defer uninitializeCOM.Call()
	if setDPI.Find() == nil {
		setDPI.Call(^uintptr(3))
	}
	callbackOnce.Do(func() { callback = syscall.NewCallback(windowProc) })
	name, _ := syscall.UTF16PtrFromString("VivaNativeWindow")
	instance, _, _ := getModule.Call(0)
	cursor, _, _ := loadCursor.Call(0, 32512)
	class := windowClass{Size: uint32(unsafe.Sizeof(windowClass{})), Procedure: callback, Instance: instance, Cursor: cursor, Name: name}
	registered, _, classErr := registerClass.Call(uintptr(unsafe.Pointer(&class)))
	if registered == 0 {
		return fmt.Errorf("windows: register class: %w", classErr)
	}
	defer unregisterClass.Call(uintptr(unsafe.Pointer(name)), instance)
	application := &windowsApplication{className: name, instance: instance}
	thread, _, _ := getCurrentThread.Call()
	application.thread = uint32(thread)
	defer application.closeStatus()
	defer application.closeOLE()
	if err := start(application); err != nil {
		application.closeAll()
		return err
	}
	var event message
	for application.windows > 0 {
		received, _, messageErr := getMessage.Call(uintptr(unsafe.Pointer(&event)), 0, 0, 0)
		if int32(received) == -1 {
			application.closeAll()
			return fmt.Errorf("windows: event loop: %w", messageErr)
		}
		if received == 0 {
			break
		}
		if event.Window == 0 && event.Kind == wmQuitApplication {
			application.closeAll()
			continue
		}
		translateMessage.Call(uintptr(unsafe.Pointer(&event)))
		dispatchMessage.Call(uintptr(unsafe.Pointer(&event)))
	}
	return application.firstErr
}

func (application *windowsApplication) OpenWindow(config platform.Config) (platform.Window, error) {
	if err := validateTextInput(config.TextInput); err != nil {
		return nil, err
	}
	state := &windowState{application: application, config: config, scale: 1, started: time.Now()}
	if err := state.initializeText(); err != nil {
		return nil, err
	}
	title, err := syscall.UTF16PtrFromString(config.Title)
	if err != nil {
		state.closeText()
		return nil, err
	}
	cursor, _, _ := loadCursor.Call(0, 32512)
	state.cursor.Store(cursor)
	style := uintptr(wsOverlappedWindow)
	var extendedStyle uintptr
	switch config.WindowMode {
	case platform.WindowFrameless:
		style = wsPopup
	case platform.WindowPanel:
		// A popup that has no taskbar button and stays above other windows.
		style = wsPopup
		extendedStyle = wsExToolWindow | wsExTopmost
	}
	// DirectComposition owns the client pixels; an opaque redirection bitmap
	// underneath it would hide the native material.
	if config.WindowBackdrop != nil && nativeBackdropAPI() != backdropUnavailable {
		extendedStyle |= wsExNoRedirectionBitmap
	}
	bounds := rect{Right: int32(config.Width), Bottom: int32(config.Height)}
	if !state.customFrame() {
		adjustWindowLegacy.Call(uintptr(unsafe.Pointer(&bounds)), style, 0, 0)
	}
	setApplicationIdentity(config.Class)
	pending.Store(state)
	state.window, _, err = createWindow.Call(extendedStyle, uintptr(unsafe.Pointer(application.className)), uintptr(unsafe.Pointer(title)), style, 0x80000000, 0x80000000, uintptr(bounds.Right-bounds.Left), uintptr(bounds.Bottom-bounds.Top), 0, 0, application.instance, 0)
	pending.CompareAndSwap(state, nil)
	if state.window == 0 {
		state.closeText()
		return nil, fmt.Errorf("windows: create window: %w", err)
	}
	application.windows++
	if config.Waker != nil && config.OnRedraw != nil {
		config.Waker.Bind(func() {
			state.redraws.post(func() bool {
				if state.window == 0 {
					return false
				}
				posted, _, _ := postMessage.Call(state.window, wmRedraw, 0, 0)
				return posted != 0
			})
		})
	}
	if getDPI.Find() == nil {
		dpi, _, _ := getDPI.Call(state.window)
		if dpi > 0 {
			state.scale = float32(dpi) / 96
		}
	}
	if override, ok := scaleOverride(os.Getenv("VIVA_SCALE")); ok {
		state.scale, state.scaleFixed = override, true
	}
	state.configureClicks()
	state.wheel = readWheelSettings()
	state.applyTitlebarTheme()
	state.acceptFiles()
	state.setIcons(config.Icon)
	if state.scale != 1 {
		bounds = rect{Right: int32(float32(config.Width) * state.scale), Bottom: int32(float32(config.Height) * state.scale)}
		if state.customFrame() {
			// The client area is the whole window: there is no frame to add.
		} else if adjustWindow.Find() == nil {
			adjustWindow.Call(uintptr(unsafe.Pointer(&bounds)), style, 0, 0, uintptr(state.scale*96))
		} else {
			adjustWindowLegacy.Call(uintptr(unsafe.Pointer(&bounds)), style, 0, 0)
		}
		setWindowPosition.Call(state.window, 0, 0, 0, uintptr(bounds.Right-bounds.Left), uintptr(bounds.Bottom-bounds.Top), 0x16)
	}
	if config.WindowMode != platform.WindowPanel {
		if config.HasOrigin {
			state.placeAtOrigin(config.Origin)
		} else {
			state.placeOnScreen()
		}
	}
	state.renderer.composition = config.WindowBackdrop != nil && nativeBackdropAPI() != backdropUnavailable
	if err := state.renderer.initialize(state.window, config.ClearColor); err != nil {
		destroyWindow.Call(state.window)
		return nil, err
	}
	if err := state.renderer.apply(platform.Frame{Update: initialUpdate(len(state.textRectangles)), Rectangles: state.textRectangles, TextLabels: state.textLabels}); err != nil {
		destroyWindow.Call(state.window)
		return nil, err
	}
	if err := state.accessibility.initialize(state, config.Accessibility); err != nil {
		destroyWindow.Call(state.window)
		return nil, err
	}
	state.installFrame()
	state.watchBackdropPower()
	state.applyBackdrop()
	state.ready = true
	if state.backdropActive {
		postMessage.Call(state.window, wmRedraw, 0, 0)
	}
	if config.WindowMode == platform.WindowPanel {
		state.placePanel()
		showWindow.Call(state.window, swShow)
		setForegroundWindow.Call(state.window)
	} else {
		showWindow.Call(state.window, 1)
	}
	updateWindow.Call(state.window)
	state.testEvents()
	return &nativeWindow{state: state}, nil
}

func (application *windowsApplication) closeAll() {
	windowStates.Range(func(key, value any) bool {
		state := value.(*windowState)
		if state.application == application && state.window != 0 {
			destroyWindow.Call(state.window)
		}
		return true
	})
}

func windowProc(window, kind, first, second uintptr) (result uintptr) {
	var state *windowState
	if kind == wmNCCreate {
		state = pending.Load()
		if state != nil {
			state.window = window
			windowStates.Store(window, state)
		}
	} else if value, ok := windowStates.Load(window); ok {
		state = value.(*windowState)
	}
	if state == nil {
		result, _, _ = defaultWindowProc.Call(window, kind, first, second)
		return
	}
	active.Store(state)
	defer func() {
		if recovered := recover(); recovered != nil {
			platform.ReportFault("window procedure", recovered)
			state.fail(fmt.Errorf("windows: event handler panicked: %v", recovered))
			result = 0
		}
	}()
	switch kind {
	case wmGetMinMaxInfo:
		defaultWindowProc.Call(window, kind, first, second)
		state.constrainTracking((*minMaxInfo)(pointer(second)))
		return 0
	case wmDestroy:
		state.destroyed(window)
		return 0
	case wmAllowClose:
		destroyWindow.Call(window)
		return 0
	case wmApplyAppearance:
		state.settingsChanged()
		return 0
	case wmHideWindow:
		showWindow.Call(window, swHide)
		return 0
	case wmShowWindow:
		showOrRaise(window)
		return 0
	case wmClose:
		if state.config.OnCloseRequest != nil && state.config.WindowMode != platform.WindowPanel {
			state.config.OnCloseRequest(func() { postMessage.Call(window, wmAllowClose, 0, 0) })
		} else {
			destroyWindow.Call(window)
		}
		return 0
	case wmErase:
		return 1
	case wmApplyCursor:
		if cursor := state.cursor.Load(); cursor != 0 {
			setCursorNative.Call(cursor)
		}
		return 0
	case wmRedraw:
		state.redraws.begin()
		if state.config.OnRedraw != nil {
			frame, redrawErr := state.config.OnRedraw()
			state.frame(frame, redrawErr)
		}
		return 0
	case wmGetObject:
		if result, handled := state.accessibility.returnRawProvider(window, first, second); handled {
			return result
		}
	case wmAccessibility:
		state.runAccessibility()
		return 0
	case wmNCCalcSize:
		if state.customFrame() {
			return state.calcSize(first, second)
		}
	}
	if !state.ready {
		result, _, _ = defaultWindowProc.Call(window, kind, first, second)
		return
	}
	switch kind {
	case wmFontChange:
		// A font was installed or removed: no shaped line is known to be right.
		if engine, err := theTextEngine(); err == nil {
			engine.forget()
		}
	case wmBackdropPower:
		state.settingsChanged()
		return 1
	case wmBackdropTheme, wmBackdropComposition:
		state.settingsChanged()
	case wmSettingChange:
		// The user changed some system setting, perhaps the light or dark mode
		// ("ImmersiveColorSet"). The observer reads the mode and ignores the rest.
		state.wheel = readWheelSettings()
		state.settingsChanged()
		platform.NotifyAppearanceChanged()
	case wmActivate:
		if uint16(first) == waInactive {
			state.panelDeactivated()
		}
	case wmSetFocus:
		state.focused = true
		state.setCaretVisible(true)
		if err := state.restartCaretTimer(); err != nil {
			state.fail(err)
		}
		return 0
	case wmCaptureChanged:
		state.textDragging = false
		return 0
	case wmSetCursor:
		// Every native frame owns its non-client cursor, including resize
		// edges on standard titled windows. Only override the client area.
		if uintptr(uint16(second)) != htClient {
			break
		}
		if cursor := state.cursor.Load(); cursor != 0 {
			setCursorNative.Call(cursor)
			return 1
		}
	case wmIMEStart:
		if state.textEnabled {
			state.imeComposing = true
			state.positionCompositionWindow()
			return 0
		}
	case wmIMEComposition:
		if state.textEnabled {
			state.positionCompositionWindow()
			state.handleComposition(second)
			return 0
		}
	case wmIMERequest:
		if result, handled := state.handleIMERequest(first, second); handled {
			return result
		}
	case wmIMEEnd:
		if state.textEnabled && state.imeComposing {
			state.imeComposing = false
			state.imeHasTarget = false
			state.handleTextEvent(platform.TextEvent{Kind: platform.TextUnmark})
			return 0
		}
	case wmDeadChar:
		state.deadKey = true
	case wmChar:
		if state.wantsKeyReleases() && state.character(first, second) {
			return 0
		}
		if state.textEnabled && !state.imeComposing {
			state.handleUTF16Character(uint16(first))
			return 0
		}
	case wmPaint:
		var paint paintStruct
		beginPaint.Call(window, uintptr(unsafe.Pointer(&paint)))
		var bounds rect
		getClientRect.Call(window, uintptr(unsafe.Pointer(&bounds)))
		err := state.present(int(bounds.Right), int(bounds.Bottom))
		endPaint.Call(window, uintptr(unsafe.Pointer(&paint)))
		if err != nil {
			state.fail(err)
		}
		return 0
	case wmMove:
		state.reportFrame()
		return 0
	case wmSize:
		if first == 1 { // SIZE_MINIMIZED
			if !state.minimized {
				state.minimized = true
				if state.ready && state.config.OnMinimize != nil {
					state.config.OnMinimize(true)
				}
			}
			return 0
		}
		if state.minimized {
			state.minimized = false
			if state.ready && state.config.OnMinimize != nil {
				state.config.OnMinimize(false)
			}
		}
		zoomed := first == 2 // SIZE_MAXIMIZED
		if state.maximized != zoomed && first != 1 {
			state.maximized = zoomed
			if state.ready && state.config.OnMaximize != nil {
				state.config.OnMaximize(zoomed)
			}
		}
		if state.sizing {
			return 0
		}
		state.sizing = true
		defer func() { state.sizing = false }()
		if state.config.OnResize != nil {
			state.resizeEvents++
			frame, err := state.config.OnResize(state.logicalSize(int(uint16(second)), int(uint16(second>>16))))
			state.frame(frame, err)
		}
		state.reportFrame()
		invalidateRect.Call(window, 0, 0)
		return 0
	case wmDPIChanged:
		if !state.scaleFixed {
			state.scale = float32(uint16(first>>16)) / 96
		}
		state.configureClicks()
		state.accessibility.setScale(state.scale)
		bounds := (*rect)(pointer(second))
		resizes := state.resizeEvents
		setWindowPosition.Call(window, 0, uintptr(bounds.Left), uintptr(bounds.Top), uintptr(bounds.Right-bounds.Left), uintptr(bounds.Bottom-bounds.Top), 0x14)
		state.reportFrame()
		if state.resizeEvents == resizes && state.config.OnResize != nil && state.ready {
			// The window kept its pixel size, so no WM_SIZE told the application the scale
			// changed: icons and strokes are rounded for it, and must be painted again.
			var client rect
			getClientRect.Call(window, uintptr(unsafe.Pointer(&client)))
			state.resizeEvents++
			frame, err := state.config.OnResize(state.logicalSize(int(client.Right), int(client.Bottom)))
			state.frame(frame, err)
		}
		invalidateRect.Call(window, 0, 0)
		return 0
	case wmLeftDown, wmLeftUp, wmRightDown, wmRightUp, wmMiddleDown, wmMiddleUp, wmXButtonDown, wmXButtonUp, wmMouseMove:
		x16, y16 := int32(int16(second)), int32(int16(second>>16))
		switch kind {
		case wmLeftDown:
			if state.beginsCaptionPress(x16, y16) {
				state.captionPress.begin(x16, y16)
			}
		case wmLeftUp:
			state.captionPress.end()
		case wmMouseMove:
			if state.captionPress.dragged(x16, y16, first&mkLButton != 0) {
				state.dragCaption(x16, y16)
				return 0
			}
		}
		if kind == wmLeftDown || kind == wmRightDown || kind == wmMiddleDown || kind == wmXButtonDown {
			setFocus.Call(window)
			setCapture.Call(window)
		}
		if !state.tracking {
			track := trackMouse{Size: uint32(unsafe.Sizeof(trackMouse{})), Flags: 2, Window: window}
			trackMouseEvent.Call(uintptr(unsafe.Pointer(&track)))
			state.tracking = true
		}
		eventKind := platform.PointerMove
		button := ui.PointerButtonNone
		switch kind {
		case wmLeftDown, wmLeftUp:
			button = ui.PointerButtonPrimary
		case wmRightDown, wmRightUp:
			button = ui.PointerButtonSecondary
		case wmMiddleDown, wmMiddleUp:
			button = ui.PointerButtonMiddle
		case wmXButtonDown, wmXButtonUp:
			var known bool
			if button, known = sideButton(first); !known {
				return 1
			}
		}
		if kind == wmLeftDown || kind == wmRightDown || kind == wmMiddleDown || kind == wmXButtonDown {
			eventKind = platform.PointerDown
		} else if kind == wmLeftUp || kind == wmRightUp || kind == wmMiddleUp || kind == wmXButtonUp {
			eventKind = platform.PointerUp
		}
		x, y := float32(int16(second))/state.scale, float32(int16(second>>16))/state.scale
		count := 0
		if eventKind == platform.PointerDown {
			count = state.clicks.Press(time.Since(state.started), uint8(button), x, y)
		} else if eventKind == platform.PointerUp {
			count = state.clicks.Current()
		}
		screen := point{X: int32(int16(second)), Y: int32(int16(second >> 16))}
		clientToScreen.Call(window, uintptr(unsafe.Pointer(&screen)))
		pointerEvent := platform.PointerEvent{
			Kind: eventKind, Button: button, Modifiers: modifiers(), X: x, Y: y,
			ScreenX: float32(screen.X) / state.scale, ScreenY: float32(screen.Y) / state.scale, ClickCount: count,
		}
		if state.config.OnPointer != nil {
			state.pointerEvents++
			frame, err := state.config.OnPointer(pointerEvent)
			state.frame(frame, err)
		}
		if textEvent, handled, err := state.textPointerEvent(pointerEvent); err != nil {
			state.fail(err)
		} else if handled {
			state.handleTextEvent(textEvent)
		}
		if (kind == wmLeftUp || kind == wmRightUp || kind == wmMiddleUp || kind == wmXButtonUp) && first&mkAnyButton == 0 {
			// The capture is held until the last button is up, so a drag that
			// started with one button keeps going when another is released.
			releaseCapture.Call()
		}
		if kind == wmXButtonDown || kind == wmXButtonUp {
			return 1 // the side-button messages ask for TRUE, unlike the others
		}
		return 0
	case wmMouseLeave, wmKillFocus:
		state.tracking = false
		state.captionPress.end()
		if kind == wmKillFocus {
			state.focused = false
			state.textDragging = false
			state.stopCaretTimer()
			state.focusLost()
		}
		if state.config.OnPointer != nil {
			frame, err := state.config.OnPointer(platform.PointerEvent{Kind: platform.PointerExit})
			state.frame(frame, err)
		}
		return 0
	case wmKeyUp, wmSysKeyUp:
		if state.wantsKeyReleases() && state.keyUp(first, second, modifiers()) {
			return 0
		}
	case wmKeyDown, wmSysKeyDown:
		if state.textEnabled && state.imeComposing {
			break // IME candidate keys must not submit or dismiss the surrounding form.
		}
		modifiers := modifiers()
		wanted := state.wantsKeyReleases()
		if wanted && !state.imeComposing {
			// Control and Command chords are offered to the application first,
			// as without releases.
			if state.offerShortcut(first, second, modifiers) {
				return 0
			}
			if state.keyDown(first, second, modifiers) {
				return 0
			}
		}
		if !(wanted && !state.imeComposing) && state.offerShortcut(first, second, modifiers) {
			return 0
		}
		if state.textEnabled && !state.imeComposing && !wanted && state.textKey(first, second, modifiers) {
			return 0
		}
		key := normalizedKey(first)
		// A plain space with the text system focused is the character WM_CHAR
		// inserts; sending VK_SPACE as well would type it twice. Space stays a
		// key for a control that takes no text, so it still activates one.
		if state.config.OnKey != nil && key != input.KeyUnknown && !(spaceIsText(first, modifiers) && state.textEnabled) {
			state.keyEvents++
			frame, err := state.config.OnKey(platform.KeyEvent{Key: key, Modifiers: modifiers, Repeat: second&(1<<30) != 0})
			state.frame(frame, err)
			return 0
		}
	case wmWheel, wmHorizontalWheel:
		if state.config.OnScroll != nil {
			frame, err := state.config.OnScroll(state.wheelEvent(kind == wmHorizontalWheel, first, second))
			state.frame(frame, err)
		}
		return 0
	case wmDropFiles:
		state.handleDropFiles(first)
		return 0
	case wmNCMouseMove, wmNCLButtonDown, wmNCLButtonUp, wmNCLButtonDblClk, wmNCMouseLeave:
		// Over the maximize button the system sends the mouse as non-client, so that it
		// can show its snap layouts there; the application still hears it.
		if state.customFrame() && state.maximizeButtonMessage(kind, first, second) {
			return 0
		}
	case wmTimer:
		state.handleCaretTimer(first)
		return 0
	case wmNCHitTest:
		if state.customFrame() {
			if code := state.hitTest(int32(int16(second)), int32(int16(second>>16))); code != 0 {
				return code
			}
		}
		result, _, _ = defaultWindowProc.Call(window, kind, first, second)
		return result
	}
	result, _, _ = defaultWindowProc.Call(window, kind, first, second)
	return result
}

// logicalSize is a client size of width by height device pixels in logical pixels, with
// the display scale it was divided by.
func (state *windowState) logicalSize(width, height int) platform.Size {
	return platform.Size{Width: float32(width) / state.scale, Height: float32(height) / state.scale, Scale: state.scale}
}

// scaleOverride reads VIVA_SCALE, a display scale for Viva alone, as on Linux: it lets a
// window be drawn and tested at 125% or 150% on a display that is at 100%. It is never below
// 1 or above 8 and snaps to quarters, like the scales Windows offers.
func scaleOverride(value string) (float32, bool) {
	scale, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || math.IsNaN(scale) || scale <= 0 {
		return 0, false
	}
	return float32(math.Round(min(max(scale, 1), 8)*4) / 4), true
}

func (state *windowState) frame(frame platform.Frame, err error) {
	state.frameWithText(frame, err, false)
}

func (state *windowState) frameWithText(frame platform.Frame, err error, nativeVisualChanged bool) {
	if err == nil {
		state.applyBackdrop()
	}
	if err == nil {
		var dirty bool
		frame, dirty, err = state.prepareFrame(frame, nativeVisualChanged)
		if err == nil {
			err = state.applyFrame(frame)
		}
		if err == nil && dirty {
			invalidateRect.Call(state.window, 0, 0)
		}
	}
	if err != nil {
		state.fail(err)
		return
	}
	if frame.Accessibility != nil {
		if err := state.accessibility.update(frame.Accessibility); err != nil {
			state.fail(err)
			return
		}
	}
}

// destroyed releases what the window held when the system destroys it. The
// window count goes down however a step fails: a count left standing keeps the
// message loop running with no window to show, and the single-instance claim held
// by a process nobody can see.
func (state *windowState) destroyed(window uintptr) {
	released := false
	release := func() {
		if released {
			return
		}
		released = true
		windowStates.Delete(window)
		state.window = 0
		state.application.windows--
	}
	defer func() {
		release()
		if state.application.windows == 0 {
			active.CompareAndSwap(state, nil)
		}
	}()
	state.stopCaretTimer()
	state.closeBackdropPower()
	state.closeDrop()
	state.accessibility.close()
	state.renderer.close()
	state.closeText()
	if state.config.Waker != nil && state.config.OnRedraw != nil {
		state.config.Waker.Unbind()
	}
	state.destroyIcons()
	removeNotificationIcon(window)
	release()
	if state.application.firstErr == nil && state.err != nil {
		state.application.firstErr = state.err
	}
	if state.config.OnClosed != nil {
		state.config.OnClosed()
	}
}

func (state *windowState) fail(err error) {
	if state.err == nil {
		state.err = err
	}
	if state.window != 0 {
		postMessage.Call(state.window, wmAllowClose, 0, 0)
	}
}
func normalizedKey(key uintptr) input.Key {
	switch key {
	case 9:
		return input.KeyTab
	case 13:
		return input.KeyEnter
	case 32:
		return input.KeySpace
	case 27:
		return input.KeyEscape
	case 37:
		return input.KeyLeft
	case 39:
		return input.KeyRight
	case 38:
		return input.KeyUp
	case 40:
		return input.KeyDown
	case 36:
		return input.KeyHome
	case 35:
		return input.KeyEnd
	case 8:
		return input.KeyBackspace
	case 46:
		return input.KeyDelete
	case 33:
		return input.KeyPageUp
	case 34:
		return input.KeyPageDown
	case 45:
		return input.KeyInsert
	}
	if key >= 0x70 && key <= 0x7b {
		return input.KeyF1 + input.Key(key-0x70)
	}
	return input.KeyUnknown
}

func shortcutText(key uintptr) string {
	const digits = "0123456789"
	const letters = "abcdefghijklmnopqrstuvwxyz"
	switch {
	case key >= '0' && key <= '9':
		index := int(key - '0')
		return digits[index : index+1]
	case key >= 'A' && key <= 'Z':
		index := int(key - 'A')
		return letters[index : index+1]
	default:
		return ""
	}
}
func modifiers() input.Modifiers {
	var result input.Modifiers
	for index, key := range [...]uintptr{16, 17, 18, 91} {
		if keyIsDown(key) {
			result |= input.Modifiers(1 << index)
		}
	}
	if keyIsDown(0x5c) { // the right Windows key is a second Meta
		result |= input.ModifierMeta
	}
	return result
}
func (state *windowState) testEvents() {
	if value := os.Getenv("VIVA_TEST_CLICK_POINT"); value != "" {
		parts := strings.Split(value, ",")
		if len(parts) == 2 {
			horizontal, horizontalErr := strconv.Atoi(parts[0])
			vertical, verticalErr := strconv.Atoi(parts[1])
			if horizontalErr == nil && verticalErr == nil {
				position := uintptr(uint16(float32(horizontal)*state.scale)) | uintptr(uint16(float32(vertical)*state.scale))<<16
				windowProc(state.window, wmLeftDown, 1, position)
				updateWindow.Call(state.window)
				windowProc(state.window, wmLeftUp, 0, position)
				updateWindow.Call(state.window)
			}
		}
	}
	if os.Getenv("VIVA_TEST_KEY") != "" {
		windowProc(state.window, wmKeyDown, 9, 0)
		updateWindow.Call(state.window)
	}
	if os.Getenv("VIVA_TEST_AUTOCLOSE") != "" {
		postMessage.Call(state.window, wmAllowClose, 0, 0)
	}
}

// sideButton is the button a WM_XBUTTON message is for. The high word of wParam
// says which: XBUTTON1 is the Back button of a mouse, XBUTTON2 Forward.
func sideButton(wParam uintptr) (ui.PointerButton, bool) {
	switch uint16(wParam >> 16) {
	case 1:
		return ui.PointerButtonBack, true
	case 2:
		return ui.PointerButtonForward, true
	}
	return ui.PointerButtonNone, false
}

// configureClicks sets the double-click time and distance from the system
// settings. The distance is a rectangle in device pixels centered on the first
// press, so a press may land half of it away on each axis.
func (state *windowState) configureClicks() {
	state.clicks.Interval = 500 * time.Millisecond
	state.clicks.Distance = 2
	if getDoubleClickTime.Find() == nil {
		if milliseconds, _, _ := getDoubleClickTime.Call(); milliseconds > 0 {
			state.clicks.Interval = time.Duration(milliseconds) * time.Millisecond
		}
	}
	if getSystemMetrics.Find() == nil {
		const smCxDoubleClick = 36
		if width, _, _ := getSystemMetrics.Call(smCxDoubleClick); width > 0 && state.scale > 0 {
			state.clicks.Distance = float32(width) / 2 / state.scale
		}
	}
}
