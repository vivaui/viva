package windows

import (
	"fmt"
	"syscall"
	"unsafe"
)

type guid struct {
	Data1        uint32
	Data2, Data3 uint16
	Data4        [8]byte
}
type point struct{ X, Y int32 }
type rect struct{ Left, Top, Right, Bottom int32 }
type message struct {
	Window        uintptr
	Kind          uint32
	First, Second uintptr
	Time          uint32
	Point         point
	Private       uint32
}
type paintStruct struct {
	DC                   uintptr
	Erase                int32
	Bounds               rect
	Restore, Incremental int32
	Reserved             [32]byte
}
type windowClass struct {
	Size, Style                        uint32
	Procedure                          uintptr
	ClassExtra, WindowExtra            int32
	Instance, Icon, Cursor, Background uintptr
	Menu, Name                         *uint16
	SmallIcon                          uintptr
}
type createStruct struct {
	Parameter, Instance, Menu, Parent uintptr
	Height, Width, Y, X               int32
	Style                             int32
	Name, Class                       *uint16
	ExtendedStyle                     uint32
}
type trackMouse struct {
	Size, Flags uint32
	Window      uintptr
	HoverTime   uint32
}

func pointer(address uintptr) unsafe.Pointer { return *(*unsafe.Pointer)(unsafe.Pointer(&address)) }
func vtable(object uintptr, index int) uintptr {
	return *(*uintptr)(unsafe.Add(pointer(*(*uintptr)(pointer(object))), index*int(unsafe.Sizeof(uintptr(0)))))
}

//go:uintptrescapes
func comCall(object uintptr, index int, arguments ...uintptr) uintptr {
	var values [16]uintptr
	if len(arguments) >= len(values) {
		panic("windows: COM call exceeds argument limit")
	}
	values[0] = object
	copy(values[1:], arguments)
	result, _, _ := syscall.SyscallN(vtable(object, index), values[:len(arguments)+1]...)
	return result
}
func release(object *uintptr) {
	if *object != 0 {
		comCall(*object, 2)
		*object = 0
	}
}
func hresult(status uintptr) error {
	if int32(uint32(status)) < 0 {
		return fmt.Errorf("windows: system call failed with HRESULT %#x", uint32(status))
	}
	return nil
}

var kernel = syscall.NewLazyDLL("kernel32.dll")
var systemDirectory = kernel.NewProc("GetSystemDirectoryW")

func systemDLL(name string) *syscall.LazyDLL {
	var path [32768]uint16
	length, _, _ := systemDirectory.Call(uintptr(unsafe.Pointer(&path[0])), uintptr(len(path)))
	if length == 0 || length >= uintptr(len(path)) {
		panic("windows: cannot resolve system directory")
	}
	return syscall.NewLazyDLL(syscall.UTF16ToString(path[:length]) + `\` + name)
}

var user = systemDLL("user32.dll")
var registerClass = user.NewProc("RegisterClassExW")
var unregisterClass = user.NewProc("UnregisterClassW")
var createWindow = user.NewProc("CreateWindowExW")
var destroyWindow = user.NewProc("DestroyWindow")
var defaultWindowProc = user.NewProc("DefWindowProcW")
var getMessage = user.NewProc("GetMessageW")
var translateMessage = user.NewProc("TranslateMessage")
var dispatchMessage = user.NewProc("DispatchMessageW")
var postMessage = user.NewProc("PostMessageW")
var postQuitMessage = user.NewProc("PostQuitMessage")
var getClientRect = user.NewProc("GetClientRect")
var screenToClient = user.NewProc("ScreenToClient")
var beginPaint = user.NewProc("BeginPaint")
var endPaint = user.NewProc("EndPaint")
var invalidateRect = user.NewProc("InvalidateRect")
var validateRect = user.NewProc("ValidateRect")
var setTimer = user.NewProc("SetTimer")
var killTimer = user.NewProc("KillTimer")
var getCaretBlinkTime = user.NewProc("GetCaretBlinkTime")
var showWindow = user.NewProc("ShowWindow")
var updateWindow = user.NewProc("UpdateWindow")
var getDPI = user.NewProc("GetDpiForWindow")
var getDoubleClickTime = user.NewProc("GetDoubleClickTime")
var getSystemMetrics = user.NewProc("GetSystemMetrics")
var setDPI = user.NewProc("SetProcessDpiAwarenessContext")
var adjustWindow = user.NewProc("AdjustWindowRectExForDpi")
var adjustWindowLegacy = user.NewProc("AdjustWindowRectEx")
var setWindowPosition = user.NewProc("SetWindowPos")
var setFocus = user.NewProc("SetFocus")
var setCapture = user.NewProc("SetCapture")
var releaseCapture = user.NewProc("ReleaseCapture")
var trackMouseEvent = user.NewProc("TrackMouseEvent")
var getKeyState = user.NewProc("GetKeyState")
var loadCursor = user.NewProc("LoadCursorW")
var getModule = kernel.NewProc("GetModuleHandleW")
var ole = systemDLL("ole32.dll")
var initializeCOM = ole.NewProc("CoInitializeEx")
var uninitializeCOM = ole.NewProc("CoUninitialize")
var createCOM = ole.NewProc("CoCreateInstance")
