package windows

import (
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

// A WindowCustomTitlebar window keeps the system's frame style, so it still
// resizes, snaps, minimizes and animates, but gives its whole area to the
// application: the caption is removed by answering WM_NCCALCSIZE, the edges
// that would have been the frame are found by WM_NCHITTEST, and the minimize,
// maximize and close buttons are drawn by Viva (app/window_controls.go) and
// reach the window through platform.WindowActions.

const (
	wmNCCalcSize = 131
	wmSysCommand = 274

	scMinimize = 0xf020
	scMaximize = 0xf030
	scRestore  = 0xf120

	smCXSizeFrame    = 32
	smCYSizeFrame    = 33
	smCXPaddedBorder = 92

	wmNCMouseMove     = 0x00a0
	wmNCLButtonDown   = 161
	wmNCLButtonUp     = 162
	wmNCLButtonDblClk = 163
	wmNCMouseLeave    = 0x02a2

	tmeLeave     = 0x00000002
	tmeNonClient = 0x00000010

	htClient      = 1
	htCaption     = 2
	htMaxButton   = 9
	htLeft        = 10
	htRight       = 11
	htTop         = 12
	htTopLeft     = 13
	htTopRight    = 14
	htBottom      = 15
	htBottomLeft  = 16
	htBottomRight = 17
)

var isZoomed = user.NewProc("IsZoomed")
var monitorFromWindow = user.NewProc("MonitorFromWindow")
var shellAppBarMessage = shell.NewProc("SHAppBarMessage")
var getCursorPos = user.NewProc("GetCursorPos")
var getSystemMetricsForDpi = user.NewProc("GetSystemMetricsForDpi")
var dwmExtendFrame = dwm.NewProc("DwmExtendFrameIntoClientArea")

// margins is MARGINS.
type margins struct{ Left, Right, Top, Bottom int32 }

// customFrame reports whether the application draws the whole window.
func (state *windowState) customFrame() bool {
	return state.config.WindowMode == platform.WindowCustomTitlebar
}

func (state *windowState) zoomed() bool {
	result, _, _ := isZoomed.Call(state.window)
	return result != 0
}

// frameThickness is the size of the invisible frame a maximized window
// overhangs its monitor by, in pixels.
func (state *windowState) frameThickness() (horizontal, vertical int32) {
	metric := func(index uintptr) int32 {
		if getSystemMetricsForDpi.Find() == nil {
			value, _, _ := getSystemMetricsForDpi.Call(index, uintptr(state.scale*96))
			return int32(value)
		}
		value, _, _ := getSystemMetrics.Call(index)
		return int32(value)
	}
	padded := metric(smCXPaddedBorder)
	return metric(smCXSizeFrame) + padded, metric(smCYSizeFrame) + padded
}

// calcSize answers WM_NCCALCSIZE: the client area is the whole window, except
// that a maximized window is pulled back inside its monitor, and short of an edge
// where the taskbar hides itself.
func (state *windowState) calcSize(validate, parameters uintptr) uintptr {
	if validate == 0 || parameters == 0 {
		return 0
	}
	proposed := (*rect)(pointer(parameters))
	if state.zoomed() {
		horizontal, vertical := state.frameThickness()
		proposed.Left, proposed.Top, proposed.Right, proposed.Bottom = platform.MaximizedClient(
			proposed.Left, proposed.Top, proposed.Right, proposed.Bottom, horizontal, vertical, state.autoHideEdges())
	}
	return 0
}

// appBarData is APPBARDATA.
type appBarData struct {
	Size     uint32
	Window   uintptr
	Callback uint32
	Edge     uint32
	Bounds   rect
	Param    uintptr
}

const abmGetAutoHideBarEx = 0x0000000b

// autoHideEdges reports on which edges of the window's monitor an auto-hiding
// taskbar or other bar waits for the pointer, in the order left, top, right, bottom.
func (state *windowState) autoHideEdges() (edges [4]bool) {
	monitor, _, _ := monitorFromWindow.Call(state.window, monitorNearest)
	var info monitorInfo
	info.Size = uint32(unsafe.Sizeof(info))
	if ok, _, _ := getMonitorInfo.Call(monitor, uintptr(unsafe.Pointer(&info))); ok == 0 {
		return edges
	}
	for edge := range edges {
		data := appBarData{Size: uint32(unsafe.Sizeof(appBarData{})), Window: state.window, Edge: uint32(edge), Bounds: info.Monitor}
		bar, _, _ := shellAppBarMessage.Call(abmGetAutoHideBarEx, uintptr(unsafe.Pointer(&data)))
		edges[edge] = bar != 0
	}
	return edges
}

// hitTest answers WM_NCHITTEST for a point given in screen coordinates. The point
// is taken against the client area, which is the whole window except while it is
// maximized and its frame hangs off the monitor.
func (state *windowState) hitTest(screenX, screenY int32) uintptr {
	var client rect
	getClientRect.Call(state.window, uintptr(unsafe.Pointer(&client)))
	var origin point
	clientToScreen.Call(state.window, uintptr(unsafe.Pointer(&origin)))
	x, y := screenX-origin.X, screenY-origin.Y
	width, height := client.Right, client.Bottom
	if x < 0 || y < 0 || x >= width || y >= height {
		return 0
	}
	scale := state.scale
	if state.overMaximizeButton(float32(x)/scale, float32(y)/scale, float32(width)/scale) {
		return htMaxButton
	}
	return classifyPoint(x, y, width, height, scale, state.zoomed(), func() platform.PressKind {
		return state.pressAt(float32(x)/scale, float32(y)/scale)
	})
}

// overMaximizeButton reports whether a window point, in logical pixels, is on the
// maximize button Viva draws and nothing of the application covers it. Windows 11
// shows its snap layouts when the pointer rests on what the window calls its
// maximize button.
func (state *windowState) overMaximizeButton(x, y, width float32) bool {
	return !state.fullscreen.Load() && state.config.Actions != nil &&
		platform.MaximizeButtonAt(x, y, width, state.config.TitlebarStrip()) &&
		state.pressAt(x, y) == platform.PressControl
}

// maximizeButtonMessage handles a mouse message the system sent as non-client
// because the pointer is on the maximize button (see hitTest), by handing the
// application the pointer event it would have had over a client button. It
// reports whether it did. A press is taken from the system, which would maximize
// the window itself, and the application's button does it from its own click; the
// window captures the pointer so the release reaches the button wherever it comes.
// The messages reach the application as client mouse messages, but the pointer
// never counts as being in the client area (see forwardClientMouse).
func (state *windowState) maximizeButtonMessage(kind, hit, screen uintptr) bool {
	switch kind {
	case wmNCMouseMove, wmNCLButtonDown, wmNCLButtonUp, wmNCLButtonDblClk:
	case wmNCMouseLeave:
		state.ncTracking = false
		var cursor point
		getCursorPos.Call(uintptr(unsafe.Pointer(&cursor)))
		var client rect
		getClientRect.Call(state.window, uintptr(unsafe.Pointer(&client)))
		screenToClient.Call(state.window, uintptr(unsafe.Pointer(&cursor)))
		if cursor.X >= 0 && cursor.Y >= 0 && cursor.X < client.Right && cursor.Y < client.Bottom {
			return true // it moved into the client area, which tells the application itself
		}
		if state.config.OnPointer != nil {
			frame, err := state.config.OnPointer(platform.PointerEvent{Kind: platform.PointerExit})
			state.frame(frame, err)
		}
		return true
	default:
		return false
	}
	if hit != htMaxButton {
		return false
	}
	position := point{X: int32(int16(screen)), Y: int32(int16(screen >> 16))}
	screenToClient.Call(state.window, uintptr(unsafe.Pointer(&position)))
	client := uintptr(uint16(position.X)) | uintptr(uint16(position.Y))<<16
	switch kind {
	case wmNCMouseMove:
		if !state.ncTracking {
			track := trackMouse{Size: uint32(unsafe.Sizeof(trackMouse{})), Flags: tmeLeave | tmeNonClient, Window: state.window}
			trackMouseEvent.Call(uintptr(unsafe.Pointer(&track)))
			state.ncTracking = true
		}
		state.forwardClientMouse(wmMouseMove, 0, client)
	case wmNCLButtonDown, wmNCLButtonDblClk:
		state.forwardClientMouse(wmLeftDown, mkLButton, client)
	case wmNCLButtonUp:
		state.forwardClientMouse(wmLeftUp, 0, client)
	}
	return true
}

// clientMessage forwards a client mouse message to the window: windowProc. It
// is a variable so that a test can see what is forwarded and with which
// tracking state; it is assigned in init because windowProc itself calls
// forwardClientMouse through maximizeButtonMessage.
var clientMessage func(window, kind, first, second uintptr) uintptr

func init() { clientMessage = windowProc }

// forwardClientMouse hands the application the client mouse message that a
// non-client message over the maximize button stands for. The client's own
// leave tracking stays off while it runs: the point is one the window answers as
// non-client, so a client TrackMouseEvent registered there reports the pointer
// gone at once, and that leave would cancel the application's press before its
// button could see the release. The window's non-client tracking, armed by the
// first move over the button, reports the pointer leaving the button instead.
func (state *windowState) forwardClientMouse(kind, first, second uintptr) {
	armed := state.tracking
	state.tracking = true
	clientMessage(state.window, kind, first, second)
	state.tracking = armed
}

// pressAt is what the application does with a press at a window point.
func (state *windowState) pressAt(x, y float32) platform.PressKind {
	if state.config.PressAt == nil {
		return platform.PressClient
	}
	return state.config.PressAt(x, y)
}

// classifyPoint is the hit-test code of a point inside a window of the given
// size in pixels: a resize edge or corner, the caption, or the client area.
// press says what a press at the point does. A move region is the caption; one
// under a backdrop is the client's too, so that the application hears the press,
// and the window moves from it once the pointer drags (see
// windowState.dragCaption).
func classifyPoint(x, y, width, height int32, scale float32, zoomed bool, press func() platform.PressKind) uintptr {
	kind := press()
	if !zoomed {
		if edge := platform.EdgeAt(float32(x)/scale, float32(y)/scale, float32(width)/scale, float32(height)/scale, kind == platform.PressControl); edge != platform.EdgeNone {
			return edgeHitCode[edge]
		}
	}
	if kind == platform.PressMove {
		return htCaption
	}
	return htClient
}

// edgeHitCode is the WM_NCHITTEST code of each window edge.
var edgeHitCode = [...]uintptr{
	platform.EdgeTopLeft: htTopLeft, platform.EdgeTop: htTop, platform.EdgeTopRight: htTopRight, platform.EdgeRight: htRight,
	platform.EdgeBottomRight: htBottomRight, platform.EdgeBottom: htBottom, platform.EdgeBottomLeft: htBottomLeft, platform.EdgeLeft: htLeft,
}

// installFrame prepares a custom-titlebar window: a shadow, and the actions
// behind Viva's buttons.
func (state *windowState) installFrame() {
	if !state.customFrame() {
		return
	}
	if dwmExtendFrame.Find() == nil {
		// One pixel of frame is what keeps the system's shadow around a window
		// that has no caption.
		frame := margins{Bottom: 1}
		dwmExtendFrame.Call(state.window, uintptr(unsafe.Pointer(&frame)))
	}
	if state.config.Actions == nil {
		return
	}
	// The actions are posted rather than performed, since they run inside the
	// pointer handler and a size change would re-enter it.
	*state.config.Actions = platform.WindowActions{
		Minimize: func() { postMessage.Call(state.window, wmSysCommand, scMinimize, 0) },
		ToggleMaximize: func() {
			command := uintptr(scMaximize)
			if state.zoomed() {
				command = scRestore
			}
			postMessage.Call(state.window, wmSysCommand, command, 0)
		},
		RequestClose: func() { postMessage.Call(state.window, wmClose, 0, 0) },
		Maximized:    state.zoomed,
	}
}

// captionDragSlop is how far, in device pixels, the pointer moves with the
// button down before a press on a backdrop over a move region becomes a
// window move.
const captionDragSlop = 4

// captionPress is a primary press on a backdrop over a move region that has
// not dragged yet. The application has the press already, to dismiss its
// overlay on a click; the window moves only if the pointer drags first.
type captionPress struct {
	pending bool
	x, y    int32
}

// begin records a press at a client point in device pixels.
func (press *captionPress) begin(x, y int32) { *press = captionPress{pending: true, x: x, y: y} }

// end forgets the press: the button came up, or the window lost the pointer.
func (press *captionPress) end() { press.pending = false }

// dragged reports whether the pointer has moved far enough, with the button
// still down, for the press to become a window move. It then forgets the press.
func (press *captionPress) dragged(x, y int32, down bool) bool {
	if !press.pending {
		return false
	}
	if !down {
		press.pending = false
		return false
	}
	dx, dy := x-press.x, y-press.y
	if dx > -captionDragSlop && dx < captionDragSlop && dy > -captionDragSlop && dy < captionDragSlop {
		return false
	}
	press.pending = false
	return true
}

// beginsCaptionPress reports whether a primary press at a client point is one
// that waits to become a window move: a backdrop over a move region.
func (state *windowState) beginsCaptionPress(x, y int32) bool {
	return state.customFrame() && state.pressAt(float32(x)/state.scale, float32(y)/state.scale) == platform.PressMoveOnDrag
}

// dragCaption hands a press the application has already seen to the system's
// window move. The application is told the pointer left, which cancels its
// press, because the system's loop swallows the button release.
func (state *windowState) dragCaption(x, y int32) {
	if state.config.OnPointer != nil {
		frame, err := state.config.OnPointer(platform.PointerEvent{Kind: platform.PointerExit})
		state.frame(frame, err)
	}
	releaseCapture.Call()
	screen := point{X: x, Y: y}
	clientToScreen.Call(state.window, uintptr(unsafe.Pointer(&screen)))
	sendMessage.Call(state.window, wmNCLButtonDown, htCaption, uintptr(uint16(screen.X))|uintptr(uint16(screen.Y))<<16)
}
