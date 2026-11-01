package windows

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

// WebView2 without WebView2Loader.dll: the loader's job is to find the installed runtime and call
// the one function its client library exports, which is done here, so nothing is shipped with
// the application. The runtime is the system's (Windows 11 has it; Windows 10 gets it with
// Edge); an application that opens no page never looks for it.
//
// Everything is COM. The runtime's objects are called through their vtables (comCall); the few
// objects it calls back are handlers made here (newHandler), which are never freed: one per
// event registration and one per request, a handful for the life of a page.

var (
	regOpenKey        = advapi.NewProc("RegOpenKeyExW")
	regQueryValue     = advapi.NewProc("RegQueryValueExW")
	regCloseKey       = advapi.NewProc("RegCloseKey")
	coTaskMemFree     = ole.NewProc("CoTaskMemFree")
	shlwapi           = systemDLL("shlwapi.dll")
	createMemoryBlob  = shlwapi.NewProc("SHCreateMemStream")
	webViewClientOnce sync.Once
)

const (
	hresultNoInterface = 0x80004002
	keyRead            = 0x20019
	keyWow6432         = 0x200
	regString          = 1
	hkeyLocalMachine   = 0x80000002
)

// errNoWebView2 says the system has no WebView2 runtime.
var errNoWebView2 = errors.New("the WebView2 runtime is not installed")

// registryString reads a string value of a key of the 32-bit view of the registry.
func registryString(root uintptr, path, name string) (string, bool) {
	subKey, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return "", false
	}
	var key uintptr
	if status, _, _ := regOpenKey.Call(root, uintptr(unsafe.Pointer(subKey)), 0, keyRead|keyWow6432, uintptr(unsafe.Pointer(&key))); status != 0 {
		return "", false
	}
	defer regCloseKey.Call(key)
	valueName, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return "", false
	}
	var kind, size uint32
	if status, _, _ := regQueryValue.Call(key, uintptr(unsafe.Pointer(valueName)), 0, uintptr(unsafe.Pointer(&kind)), 0, uintptr(unsafe.Pointer(&size))); status != 0 || kind != regString || size < 2 {
		return "", false
	}
	buffer := make([]uint16, size/2+1)
	if status, _, _ := regQueryValue.Call(key, uintptr(unsafe.Pointer(valueName)), 0, uintptr(unsafe.Pointer(&kind)), uintptr(unsafe.Pointer(&buffer[0])), uintptr(unsafe.Pointer(&size))); status != 0 {
		return "", false
	}
	return syscall.UTF16ToString(buffer), true
}

// webViewClientLibrary finds the runtime's client library: the stable channel first, then the
// preview channels, for all users and then for this one.
func webViewClientLibrary() (string, error) {
	arch := map[string]string{"amd64": "x64", "arm64": "arm64", "386": "x86"}[runtime.GOARCH]
	if arch == "" {
		return "", fmt.Errorf("windows: WebView2 does not run on %s", runtime.GOARCH)
	}
	for _, channel := range []string{
		"{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}", "{2CD8A007-E189-409D-A2C8-9AF4EF3C72AA}",
		"{0D50BFEC-CD6A-4F9A-964C-C7416E3ACB10}", "{65C35B14-6C1D-4122-AC46-7148CC9D6497}",
	} {
		for _, root := range []uintptr{hkeyLocalMachine, hkeyCurrentUser} {
			folder, ok := registryString(root, `Software\Microsoft\EdgeUpdate\ClientState\`+channel, "EBWebView")
			if !ok || folder == "" {
				continue
			}
			library := filepath.Join(folder, "EBWebView", arch, "EmbeddedBrowserWebView.dll")
			if _, err := os.Stat(library); err == nil {
				return library, nil
			}
		}
	}
	return "", errNoWebView2
}

// A handler is an object the runtime calls back. All of them share a vtable: IUnknown and one
// method, Invoke, whose arguments are whatever the event passes.
type comHandler struct {
	vtbl   *[4]uintptr
	invoke func(first, second uintptr) uintptr
}

var (
	handlerTable [4]uintptr
	handlerOnce  sync.Once
	handlerLive  []*comHandler // handlers are never freed
)

func newHandler(invoke func(first, second uintptr) uintptr) uintptr {
	handlerOnce.Do(func() {
		handlerTable[0] = syscall.NewCallback(func(this, iid, object uintptr) uintptr {
			// The runtime asks for the interfaces that tell it whether it may call from another
			// thread (IAgileObject, INoMarshal) and for marshaling; it gets neither, and calls
			// on this thread. Any other interface is the handler it was given.
			id := (*guid)(pointer(iid))
			switch {
			case id.Data1 == 0x94ea2b94 || id.Data1 == 0xecc8691b || id.Data1 == 0x00000003:
				*(*uintptr)(pointer(object)) = 0
				return hresultNoInterface
			}
			*(*uintptr)(pointer(object)) = this
			return 0
		})
		handlerTable[1] = syscall.NewCallback(func(this uintptr) uintptr { return 1 })
		handlerTable[2] = syscall.NewCallback(func(this uintptr) uintptr { return 1 })
		handlerTable[3] = syscall.NewCallback(func(this, first, second uintptr) uintptr {
			return (*comHandler)(pointer(this)).invoke(first, second)
		})
	})
	h := &comHandler{vtbl: &handlerTable, invoke: invoke}
	handlerLive = append(handlerLive, h)
	return uintptr(unsafe.Pointer(h))
}

// webEnvironment is the process's one environment: the runtime's browser process and its settings.
type webEnvironment struct {
	object  uintptr
	failed  error
	started bool
	waiting []func(object uintptr, err error)
}

var sharedEnvironment webEnvironment

// withEnvironment calls then once the environment exists (at once if it does), on this thread.
func withEnvironment(userData string, then func(object uintptr, err error)) {
	env := &sharedEnvironment
	switch {
	case env.object != 0 || env.failed != nil:
		then(env.object, env.failed)
		return
	case env.started:
		env.waiting = append(env.waiting, then)
		return
	}
	env.started = true
	env.waiting = append(env.waiting, then)
	finish := func(object uintptr, err error) {
		env.object, env.failed = object, err
		waiting := env.waiting
		env.waiting = nil
		for _, wait := range waiting {
			wait(object, err)
		}
	}
	library, err := webViewClientLibrary()
	if err != nil {
		finish(0, err)
		return
	}
	dll, err := syscall.LoadDLL(library)
	if err != nil {
		finish(0, fmt.Errorf("windows: load the WebView2 runtime: %w", err))
		return
	}
	create, err := dll.FindProc("CreateWebViewEnvironmentWithOptionsInternal")
	if err != nil {
		finish(0, fmt.Errorf("windows: the WebView2 runtime lacks its entry point: %w", err))
		return
	}
	folder, err := syscall.UTF16PtrFromString(userData)
	if err != nil {
		finish(0, err)
		return
	}
	handler := newHandler(func(status, object uintptr) uintptr {
		if int32(uint32(status)) < 0 || object == 0 {
			finish(0, fmt.Errorf("windows: create the WebView2 environment: HRESULT %#x", uint32(status)))
			return 0
		}
		comCall(object, 1) // the runtime releases its reference when the call returns
		finish(object, nil)
		return 0
	})
	// (checkRunningInstance, runtimeType installed, user data folder, no options, completion)
	status, _, _ := create.Call(1, 0, uintptr(unsafe.Pointer(folder)), 0, handler)
	if int32(uint32(status)) < 0 {
		finish(0, fmt.Errorf("windows: start the WebView2 environment: HRESULT %#x", uint32(status)))
	}
}

// wideString copies a string the runtime allocated and frees it.
func wideString(address uintptr) string {
	if address == 0 {
		return ""
	}
	defer coTaskMemFree.Call(address)
	var units []uint16
	for offset := uintptr(0); ; offset += 2 {
		unit := *(*uint16)(pointer(address + offset))
		if unit == 0 {
			break
		}
		units = append(units, unit)
	}
	return syscall.UTF16ToString(append(units, 0))
}

func wide(text string) *uint16 {
	pointer, err := syscall.UTF16PtrFromString(text)
	if err != nil { // the text holds a NUL
		pointer, _ = syscall.UTF16PtrFromString("")
	}
	return pointer
}

// The vtable slots used, counted from IUnknown's three.
const (
	envCreateController     = 3
	envCreateResponse       = 4
	controllerPutVisible    = 4
	controllerPutBounds     = 6
	controllerMoveFocus     = 12
	controllerClose         = 24
	controllerGetWebView    = 25
	viewNavigate            = 5
	viewNavigateToString    = 6
	viewAddNavStarting      = 7
	viewAddContentLoading   = 9
	viewAddNavCompleted     = 15
	viewAddScriptOnCreate   = 27
	viewExecuteScript       = 29
	viewReload              = 31
	viewAddMessageReceived  = 34
	viewGoBack              = 40
	viewGoForward           = 41
	viewGetSource           = 4
	viewAddWebResourceReq   = 55
	viewAddResourceFilter   = 57
	messageTryGetString     = 5
	startingGetURI          = 3
	resourceArgsGetRequest  = 3
	resourceArgsPutResponse = 5
	resourceRequestGetURI   = 3
	resourceContextAll      = 0
)
