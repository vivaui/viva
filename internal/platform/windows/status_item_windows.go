package windows

import (
	"errors"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/ui"
)

const (
	// wmStatusNotify carries the shell's report about one status item; its
	// wParam is the anchor point and its lParam holds the event in the low word
	// and the item ID in the high word (notification icon version 4).
	wmStatusNotify = 0x8010
	// wmStatusRemove asks the status window to remove the item named in wParam.
	wmStatusRemove = 0x8011
	// wmQuitApplication is posted to the event-loop thread, not to a window.
	wmQuitApplication = 0x8012
	// wmHideWindow and wmShowWindow are posted to a window.
	wmHideWindow = 32772
	wmShowWindow = 32773

	wmActivate    = 6
	waInactive    = 0
	wmContextMenu = 0x007b
	ninSelect     = 0x0400
	ninKeySelect  = 0x0401

	notifySetVersion = 4
	notifyVersion4   = 4
	notifyMessageTip = notifyMessage | notifyIcon | notifyTip
	// notifyShowTip asks version 4 shells to show the tip as a standard tooltip.
	notifyShowTip = 0x80

	hwndTopmost        = ^uintptr(0) // HWND_TOPMOST, -1
	wsPopup            = 0x80000000
	wsExToolWindow     = 0x00000080
	wsExTopmost        = 0x00000008
	swHide             = 0
	swShow             = 5
	swRestore          = 9
	smSmallIconWidth   = 49
	monitorNearest     = 2
	dwmCornerPreferred = 33
	dwmCornerRound     = 2
)

type iconInfo struct {
	Icon     int32
	XHotspot uint32
	YHotspot uint32
	_        uint32
	Mask     uintptr
	Color    uintptr
}

type bitmapV4Header struct {
	Size          uint32
	Width, Height int32
	Planes, Bits  uint16
	Compression   uint32
	ImageSize     uint32
	XPixels       int32
	YPixels       int32
	Used          uint32
	Important     uint32
	RedMask       uint32
	GreenMask     uint32
	BlueMask      uint32
	AlphaMask     uint32
	ColorSpace    uint32
	Endpoints     [36]byte
	GammaRed      uint32
	GammaGreen    uint32
	GammaBlue     uint32
}

type monitorInfo struct {
	Size    uint32
	Monitor rect
	Work    rect
	Flags   uint32
}

type notifyIconIdentifier struct {
	Size   uint32
	_      uint32
	Window uintptr
	ID     uint32
	Item   guid
	_      uint32
}

var createBitmap = graphics.NewProc("CreateBitmap")
var createIconIndirect = user.NewProc("CreateIconIndirect")
var destroyIcon = user.NewProc("DestroyIcon")
var getDC = user.NewProc("GetDC")
var releaseDC = user.NewProc("ReleaseDC")
var postThreadMessage = user.NewProc("PostThreadMessageW")
var getCurrentThread = kernel.NewProc("GetCurrentThreadId")
var registerWindowMessage = user.NewProc("RegisterWindowMessageW")
var setForegroundWindow = user.NewProc("SetForegroundWindow")
var monitorFromPoint = user.NewProc("MonitorFromPoint")
var getMonitorInfo = user.NewProc("GetMonitorInfoW")
var getWindowRect = user.NewProc("GetWindowRect")
var isIconic = user.NewProc("IsIconic")
var notifyIconGetRect = shell.NewProc("Shell_NotifyIconGetRect")
var dwm = systemDLL("dwmapi.dll")
var dwmSetWindowAttribute = dwm.NewProc("DwmSetWindowAttribute")

// panelDismissedAt is when a panel last closed itself for losing the focus, in
// nanoseconds on the monotonic clock of the process. The press on a status item
// that makes a panel lose the focus is followed by the item's click, which must
// not open the panel it has just dismissed.
var panelDismissedAt atomic.Int64
var processStart = time.Now()

func monotonicNow() int64 { return int64(time.Since(processStart)) }

const panelDismissWindow = 350 * time.Millisecond

// statusState is what the application keeps for its status items: one hidden
// window that receives their messages.
type statusState struct {
	class          *uint16
	window         uintptr
	taskbarCreated uint32
	nextID         uint32
	items          map[uint32]*statusItem
}

type statusItem struct {
	application *windowsApplication
	id          uint32
	icon        uintptr
	tooltip     string
	onClick     func(platform.StatusClick)
	menu        func() []platform.MenuItem
	removed     atomic.Bool
}

var statusProcCallback uintptr
var statusProcOnce sync.Once

// AddStatusItem puts an icon in the notification area. Its messages reach a
// hidden window of this application, so a click is handled on the event-loop
// thread like any other event.
func (application *windowsApplication) AddStatusItem(config platform.StatusItemConfig) (platform.StatusItem, error) {
	if !config.Icon.Valid() {
		return nil, errors.New("windows: status icon pixels do not match its size")
	}
	if err := application.ensureStatusWindow(); err != nil {
		return nil, err
	}
	status := application.status
	item := &statusItem{application: application, id: status.nextID, tooltip: config.Tooltip, onClick: config.OnClick, menu: config.Menu}
	status.nextID++
	icon, err := newStatusHandle(config.Icon)
	if err != nil {
		return nil, err
	}
	item.icon = icon
	if err := item.notify(notifyAdd); err != nil {
		destroyIcon.Call(icon)
		return nil, err
	}
	version := item.data()
	version.Version = notifyVersion4
	shellNotifyIcon.Call(notifySetVersion, uintptr(unsafe.Pointer(&version)))
	status.items[item.id] = item
	return item, nil
}

func (application *windowsApplication) ensureStatusWindow() error {
	if application.status != nil {
		return nil
	}
	statusProcOnce.Do(func() { statusProcCallback = syscall.NewCallback(statusProc) })
	name, _ := syscall.UTF16PtrFromString("VivaStatusWindow")
	class := windowClass{Size: uint32(unsafe.Sizeof(windowClass{})), Procedure: statusProcCallback, Instance: application.instance, Name: name}
	if registered, _, callErr := registerClass.Call(uintptr(unsafe.Pointer(&class))); registered == 0 {
		return windowsCallError("register status window class", callErr)
	}
	// A hidden top-level window and not a message-only one: the shell sends
	// TaskbarCreated as a broadcast, which message-only windows never receive, so
	// the icon would stay gone after Explorer restarts.
	window, _, callErr := createWindow.Call(0, uintptr(unsafe.Pointer(name)), 0, 0, 0, 0, 0, 0, 0, 0, application.instance, 0)
	if window == 0 {
		unregisterClass.Call(uintptr(unsafe.Pointer(name)), application.instance)
		return windowsCallError("create status window", callErr)
	}
	created, _ := syscall.UTF16PtrFromString("TaskbarCreated")
	message, _, _ := registerWindowMessage.Call(uintptr(unsafe.Pointer(created)))
	// ID 1 belongs to the notification of SendNotification.
	application.status = &statusState{class: name, window: window, taskbarCreated: uint32(message), nextID: 2, items: make(map[uint32]*statusItem)}
	statusStates.Store(window, application.status)
	statusWindowHandle.Store(window)
	return nil
}

var statusStates sync.Map

// statusWindowHandle is the hidden window of the status items, for a notification
// to hang on when the application has no other window.
var statusWindowHandle atomic.Uintptr

// closeStatus removes every icon and the window that received their messages.
func (application *windowsApplication) closeStatus() {
	status := application.status
	if status == nil {
		return
	}
	for _, item := range status.items {
		item.delete()
	}
	statusStates.Delete(status.window)
	statusWindowHandle.CompareAndSwap(status.window, 0)
	destroyWindow.Call(status.window)
	unregisterClass.Call(uintptr(unsafe.Pointer(status.class)), application.instance)
	application.status = nil
}

func (item *statusItem) data() notifyIconData {
	data := notifyIconData{
		Size:            uint32(unsafe.Sizeof(notifyIconData{})),
		Window:          item.application.status.window,
		ID:              item.id,
		Flags:           notifyMessageTip,
		CallbackMessage: wmStatusNotify,
		Icon:            item.icon,
	}
	copyUTF16(data.Tip[:], item.tooltip)
	return data
}

func (item *statusItem) notify(operation uintptr) error {
	data := item.data()
	if ok, _, callErr := shellNotifyIcon.Call(operation, uintptr(unsafe.Pointer(&data))); ok == 0 {
		return windowsCallError("update the status item", callErr)
	}
	return nil
}

func (item *statusItem) delete() {
	data := item.data()
	shellNotifyIcon.Call(notifyDelete, uintptr(unsafe.Pointer(&data)))
	if item.icon != 0 {
		destroyIcon.Call(item.icon)
		item.icon = 0
	}
}

// SetIcon replaces the icon. It must run on the event-loop thread.
func (item *statusItem) SetIcon(icon platform.StatusIcon) error {
	if !icon.Valid() {
		return errors.New("windows: status icon pixels do not match its size")
	}
	if item.removed.Load() {
		return errors.New("windows: status item is removed")
	}
	handle, err := newStatusHandle(icon)
	if err != nil {
		return err
	}
	previous := item.icon
	item.icon = handle
	if err := item.notify(notifyModify); err != nil {
		item.icon = previous
		destroyIcon.Call(handle)
		return err
	}
	destroyIcon.Call(previous)
	return nil
}

// SetTooltip replaces the tooltip. It must run on the event-loop thread.
func (item *statusItem) SetTooltip(tooltip string) {
	if item.removed.Load() {
		return
	}
	item.tooltip = tooltip
	item.notify(notifyModify)
}

// Remove posts the removal to the event loop, so it may be called anywhere.
func (item *statusItem) Remove() {
	if item.removed.Swap(true) {
		return
	}
	if status := item.application.status; status != nil {
		postMessage.Call(status.window, wmStatusRemove, uintptr(item.id), 0)
	}
}

// newStatusHandle makes an HICON at the size the notification area draws.
func newStatusHandle(icon platform.StatusIcon) (uintptr, error) {
	size, _, _ := getSystemMetrics.Call(smSmallIconWidth)
	if size < 16 {
		size = 16
	}
	return newIconHandle(icon.Scaled(int(size)))
}

// newIconHandle makes an HICON of the icon at its own size.
func newIconHandle(scaled platform.StatusIcon) (uintptr, error) {
	size := uintptr(scaled.Size)
	header := bitmapV4Header{
		Size: uint32(unsafe.Sizeof(bitmapV4Header{})), Width: int32(size), Height: -int32(size),
		Planes: 1, Bits: 32, Compression: 3, // BI_BITFIELDS
		RedMask: 0x00ff0000, GreenMask: 0x0000ff00, BlueMask: 0x000000ff, AlphaMask: 0xff000000,
	}
	screen, _, _ := getDC.Call(0)
	defer releaseDC.Call(0, screen)
	var bits unsafe.Pointer
	color, _, callErr := createDIBSection.Call(screen, uintptr(unsafe.Pointer(&header)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if color == 0 || bits == nil {
		return 0, windowsCallError("create the icon bitmap", callErr)
	}
	defer deleteObject.Call(color)
	pixels := unsafe.Slice((*byte)(bits), int(size*size*4))
	for index := 0; index < len(pixels); index += 4 {
		pixels[index], pixels[index+1], pixels[index+2], pixels[index+3] = scaled.RGBA[index+2], scaled.RGBA[index+1], scaled.RGBA[index], scaled.RGBA[index+3]
	}
	mask, _, callErr := createBitmap.Call(size, size, 1, 1, 0)
	if mask == 0 {
		return 0, windowsCallError("create the icon mask", callErr)
	}
	defer deleteObject.Call(mask)
	info := iconInfo{Icon: 1, Mask: mask, Color: color}
	handle, _, callErr := createIconIndirect.Call(uintptr(unsafe.Pointer(&info)))
	if handle == 0 {
		return 0, windowsCallError("create the icon", callErr)
	}
	return handle, nil
}

func statusProc(window, kind, first, second uintptr) (result uintptr) {
	defer func() {
		if recovered := recover(); recovered != nil {
			platform.ReportFault("status icon window procedure", recovered)
			result = 0
		}
	}()
	value, ok := statusStates.Load(window)
	if !ok {
		result, _, _ = defaultWindowProc.Call(window, kind, first, second)
		return result
	}
	status := value.(*statusState)
	switch {
	case kind == wmStatusNotify:
		status.notified(window, first, second)
		return 0
	case kind == wmStatusRemove:
		if item := status.items[uint32(first)]; item != nil {
			item.delete()
			delete(status.items, item.id)
		}
		return 0
	case status.taskbarCreated != 0 && uint32(kind) == status.taskbarCreated:
		// The shell started again: its notification area is empty.
		for _, item := range status.items {
			if !item.removed.Load() {
				item.notify(notifyAdd)
				version := item.data()
				version.Version = notifyVersion4
				shellNotifyIcon.Call(notifySetVersion, uintptr(unsafe.Pointer(&version)))
			}
		}
		return 0
	}
	result, _, _ = defaultWindowProc.Call(window, kind, first, second)
	return result
}

// notified turns the shell's report into a click.
func (status *statusState) notified(window, anchor, packed uintptr) {
	event := uint32(packed) & 0xffff
	id := uint32(packed>>16) & 0xffff
	item := status.items[id]
	if item == nil || item.removed.Load() || item.onClick == nil && item.menu == nil {
		return
	}
	button, ok := statusButton(event)
	if !ok {
		return
	}
	// A primary activation belongs to the application when it supplied a
	// callback (for example, to restore its window). Keep menu-only icons
	// clickable, and reserve secondary activation for the context menu.
	if item.menu != nil && (button == ui.PointerButtonSecondary || item.onClick == nil) {
		item.popMenu(window, item.anchor(window, anchor))
		return
	}
	dismissed := time.Duration(monotonicNow()-panelDismissedAt.Load()) < panelDismissWindow
	item.onClick(platform.StatusClick{Button: button, Anchor: item.anchor(window, anchor), PanelDismissed: dismissed})
}

// statusButton is the click a notification-icon message stands for, if it is one.
// Version 4 reports one press twice: a left click is both WM_LBUTTONUP and
// NIN_SELECT, a right click both WM_RBUTTONUP and WM_CONTEXTMENU. Only the second
// of each is taken, because it also covers the keyboard (Enter or Space on the
// icon, the context menu key), which sends no button message.
func statusButton(event uint32) (ui.PointerButton, bool) {
	switch event {
	case ninSelect, ninKeySelect:
		return ui.PointerButtonPrimary, true
	case wmContextMenu:
		return ui.PointerButtonSecondary, true
	}
	return ui.PointerButtonNone, false
}

// anchor is the icon's rectangle on the screen, in pixels. When the shell cannot
// say, the click position it reported stands in for it.
func (item *statusItem) anchor(window, click uintptr) platform.ScreenRect {
	identifier := notifyIconIdentifier{Size: uint32(unsafe.Sizeof(notifyIconIdentifier{})), Window: window, ID: item.id}
	var bounds rect
	if status, _, _ := notifyIconGetRect.Call(uintptr(unsafe.Pointer(&identifier)), uintptr(unsafe.Pointer(&bounds))); int32(uint32(status)) >= 0 && bounds.Right > bounds.Left {
		return platform.ScreenRect{X: float32(bounds.Left), Y: float32(bounds.Top), Width: float32(bounds.Right - bounds.Left), Height: float32(bounds.Bottom - bounds.Top)}
	}
	return platform.ScreenRect{X: float32(int16(click)), Y: float32(int16(click >> 16))}
}

// Quit closes every window, close requests notwithstanding, and ends the loop.
func (application *windowsApplication) Quit() {
	if thread := application.thread; thread != 0 {
		postThreadMessage.Call(uintptr(thread), wmQuitApplication, 0, 0)
	}
}

// Hide and Show post to the window, so they run on the event-loop thread.
func (window *nativeWindow) Hide() {
	if window != nil && !window.closed.Load() {
		if handle := window.state.window; handle != 0 {
			postMessage.Call(handle, wmHideWindow, 0, 0)
		}
	}
}

func (window *nativeWindow) Show() {
	if window != nil && !window.closed.Load() {
		if handle := window.state.window; handle != 0 {
			postMessage.Call(handle, wmShowWindow, 0, 0)
		}
	}
}

// showOrRaise puts a hidden or minimized window back and brings it forward.
func showOrRaise(window uintptr) {
	if iconic, _, _ := isIconic.Call(window); iconic != 0 {
		showWindow.Call(window, swRestore)
	} else {
		showWindow.Call(window, swShow)
	}
	setForegroundWindow.Call(window)
}

// placePanel moves a panel beside its anchor, or to the middle of the screen
// without one, and gives it the rounded corners of the system's popups where
// the system has them.
func (state *windowState) placePanel() {
	preference := uint32(dwmCornerRound)
	if dwmSetWindowAttribute.Find() == nil {
		dwmSetWindowAttribute.Call(state.window, dwmCornerPreferred, uintptr(unsafe.Pointer(&preference)), unsafe.Sizeof(preference))
	}
	var bounds rect
	getWindowRect.Call(state.window, uintptr(unsafe.Pointer(&bounds)))
	width, height := float32(bounds.Right-bounds.Left), float32(bounds.Bottom-bounds.Top)
	config := state.config
	if !config.HasAnchor {
		// Centered on the monitor the cursor is on.
		var info monitorInfo
		info.Size = uint32(unsafe.Sizeof(info))
		monitor := cursorMonitor()
		getMonitorInfo.Call(monitor, uintptr(unsafe.Pointer(&info)))
		x := float32(info.Work.Left) + (float32(info.Work.Right-info.Work.Left)-width)/2
		y := float32(info.Work.Top) + (float32(info.Work.Bottom-info.Work.Top)-height)/2
		setWindowPosition.Call(state.window, hwndTopmost, uintptr(int32(x)), uintptr(int32(y)), 0, 0, 0x1)
		return
	}
	anchor := config.Anchor
	var info monitorInfo
	info.Size = uint32(unsafe.Sizeof(info))
	centerX, centerY := int32(anchor.X+anchor.Width/2), int32(anchor.Y+anchor.Height/2)
	monitor, _, _ := monitorFromPoint.Call(uintptr(uint32(centerX))|uintptr(uint32(centerY))<<32, monitorNearest)
	area := platform.ScreenRect{Width: 1 << 20, Height: 1 << 20}
	if ok, _, _ := getMonitorInfo.Call(monitor, uintptr(unsafe.Pointer(&info))); ok != 0 {
		area = platform.ScreenRect{X: float32(info.Work.Left), Y: float32(info.Work.Top), Width: float32(info.Work.Right - info.Work.Left), Height: float32(info.Work.Bottom - info.Work.Top)}
	}
	x, y := platform.PlacePanel(anchor, width, height, area)
	// SWP_NOSIZE: only the position changes.
	setWindowPosition.Call(state.window, hwndTopmost, uintptr(int32(x)), uintptr(int32(y)), 0, 0, 0x1)
}

// cursorMonitor is the monitor the pointer is on, where the user is looking.
func cursorMonitor() uintptr {
	var cursor point
	getCursorPos.Call(uintptr(unsafe.Pointer(&cursor)))
	monitor, _, _ := monitorFromPoint.Call(uintptr(uint32(cursor.X))|uintptr(uint32(cursor.Y))<<32, monitorNearest)
	return monitor
}

// placeOnScreen puts a new window in the middle of the part of the screen
// windows may use, and makes one that is larger than that fit it. Left to the
// system's default position a big window starts partway down the screen and runs
// off the bottom of it, behind the taskbar.
func (state *windowState) placeOnScreen() {
	var bounds rect
	getWindowRect.Call(state.window, uintptr(unsafe.Pointer(&bounds)))
	var info monitorInfo
	info.Size = uint32(unsafe.Sizeof(info))
	monitor := cursorMonitor()
	if ok, _, _ := getMonitorInfo.Call(monitor, uintptr(unsafe.Pointer(&info))); ok == 0 {
		return
	}
	width, height := bounds.Right-bounds.Left, bounds.Bottom-bounds.Top
	areaWidth, areaHeight := info.Work.Right-info.Work.Left, info.Work.Bottom-info.Work.Top
	width, height = min(width, areaWidth), min(height, areaHeight)
	x := info.Work.Left + (areaWidth-width)/2
	y := info.Work.Top + (areaHeight-height)/2
	// SWP_NOZORDER | SWP_NOACTIVATE: the size too, for a window that did not fit.
	setWindowPosition.Call(state.window, 0, uintptr(x), uintptr(y), uintptr(width), uintptr(height), 0x14)
}

// placeAtOrigin puts the content area's top-left at a screen point given in
// logical pixels, kept where the user can reach it on the monitor it lands on.
func (state *windowState) placeAtOrigin(origin ui.Point) {
	var bounds rect
	getWindowRect.Call(state.window, uintptr(unsafe.Pointer(&bounds)))
	var client point
	clientToScreen.Call(state.window, uintptr(unsafe.Pointer(&client)))
	width, height := bounds.Right-bounds.Left, bounds.Bottom-bounds.Top
	// The frame around the content area, which the requested point leaves out.
	x := int32(origin.X*state.scale) - (client.X - bounds.Left)
	y := int32(origin.Y*state.scale) - (client.Y - bounds.Top)
	var info monitorInfo
	info.Size = uint32(unsafe.Sizeof(info))
	monitor, _, _ := monitorFromPoint.Call(uintptr(uint32(x+width/2))|uintptr(uint32(y+height/2))<<32, monitorNearest)
	if ok, _, _ := getMonitorInfo.Call(monitor, uintptr(unsafe.Pointer(&info))); ok != 0 {
		x, y, width, height = placedFrame(info.Work, state.scale, x, y, width, height)
	}
	// SWP_NOZORDER | SWP_NOACTIVATE: the size too, for a window that did not fit.
	setWindowPosition.Call(state.window, 0, uintptr(x), uintptr(y), uintptr(width), uintptr(height), 0x14)
}

// Frame is the content area in screen coordinates, in the logical pixels of the
// window's own display. It reads the system's state, so any goroutine may call it.
func (window *nativeWindow) Frame() (ui.Rect, bool) {
	if window == nil || window.closed.Load() || window.state == nil || window.state.window == 0 {
		return ui.Rect{}, false
	}
	handle := window.state.window
	var client rect
	if ok, _, _ := getClientRect.Call(handle, uintptr(unsafe.Pointer(&client))); ok == 0 {
		return ui.Rect{}, false
	}
	var origin point
	if ok, _, _ := clientToScreen.Call(handle, uintptr(unsafe.Pointer(&origin))); ok == 0 {
		return ui.Rect{}, false
	}
	scale := float32(1)
	if getDPI.Find() == nil {
		if dpi, _, _ := getDPI.Call(handle); dpi > 0 {
			scale = float32(dpi) / 96
		}
	}
	return ui.Rect{
		X: float32(origin.X) / scale, Y: float32(origin.Y) / scale,
		Width: float32(client.Right-client.Left) / scale, Height: float32(client.Bottom-client.Top) / scale,
	}, true
}

// panelDeactivated closes a panel that lost the activation, as a popup menu
// does. The close is posted, since the window is changing activation.
func (state *windowState) panelDeactivated() {
	if state.config.WindowMode != platform.WindowPanel || state.window == 0 {
		return
	}
	panelDismissedAt.Store(monotonicNow())
	postMessage.Call(state.window, wmAllowClose, 0, 0)
}
