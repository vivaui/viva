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

// comCall0 to comCall7 call method index of a COM object with that many
// arguments after the object itself. They have a fixed arity because a variadic
// wrapper allocates its argument slice on every call; uintptrescapes keeps what
// an argument points at alive through the call, which a pointer converted at the
// call site needs.

//go:uintptrescapes
func comCall0(object uintptr, index int) uintptr {
	result, _, _ := syscall.SyscallN(vtable(object, index), object)
	return result
}

//go:uintptrescapes
func comCall1(object uintptr, index int, a0 uintptr) uintptr {
	result, _, _ := syscall.SyscallN(vtable(object, index), object, a0)
	return result
}

//go:uintptrescapes
func comCall2(object uintptr, index int, a0 uintptr, a1 uintptr) uintptr {
	result, _, _ := syscall.SyscallN(vtable(object, index), object, a0, a1)
	return result
}

//go:uintptrescapes
func comCall3(object uintptr, index int, a0 uintptr, a1 uintptr, a2 uintptr) uintptr {
	result, _, _ := syscall.SyscallN(vtable(object, index), object, a0, a1, a2)
	return result
}

//go:uintptrescapes
func comCall4(object uintptr, index int, a0 uintptr, a1 uintptr, a2 uintptr, a3 uintptr) uintptr {
	result, _, _ := syscall.SyscallN(vtable(object, index), object, a0, a1, a2, a3)
	return result
}

//go:uintptrescapes
func comCall5(object uintptr, index int, a0 uintptr, a1 uintptr, a2 uintptr, a3 uintptr, a4 uintptr) uintptr {
	result, _, _ := syscall.SyscallN(vtable(object, index), object, a0, a1, a2, a3, a4)
	return result
}

//go:uintptrescapes
func comCall6(object uintptr, index int, a0 uintptr, a1 uintptr, a2 uintptr, a3 uintptr, a4 uintptr, a5 uintptr) uintptr {
	result, _, _ := syscall.SyscallN(vtable(object, index), object, a0, a1, a2, a3, a4, a5)
	return result
}

//go:uintptrescapes
func comCall7(object uintptr, index int, a0 uintptr, a1 uintptr, a2 uintptr, a3 uintptr, a4 uintptr, a5 uintptr, a6 uintptr) uintptr {
	result, _, _ := syscall.SyscallN(vtable(object, index), object, a0, a1, a2, a3, a4, a5, a6)
	return result
}

func release(object *uintptr) {
	if *object != 0 {
		comCall0(*object, 2)
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
var winmm = systemDLL("winmm.dll")
var timeBeginPeriod = winmm.NewProc("timeBeginPeriod")
var timeEndPeriod = winmm.NewProc("timeEndPeriod")
