package windows

import (
	"syscall"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

var getWindowPropertyStore = shell.NewProc("SHGetPropertyStoreForWindow")
var iidPropertyStore = guid{0x886d8eeb, 0x8cf2, 0x4446, [8]byte{0x8d, 0x02, 0xcd, 0xba, 0x1d, 0xbd, 0xcf, 0x99}}
var appModelKey = guid{0x9f4c2855, 0x9f79, 0x4b39, [8]byte{0xa8, 0xd0, 0xe1, 0xd4, 0x2d, 0xe1, 0xd5, 0xf3}}

type propertyKey struct {
	Format guid
	ID     uint32
}

// PROPVARIANT's union is 16 bytes on Win64, 8 on Win32. The string pointer is
// borrowed only for SetValue, which copies it; it must not be PropVariantClear'd.
type stringProperty struct {
	Kind     uint16
	Reserved [3]uint16
	Text     *uint16
	Tail     uintptr
}

func setStringProperty(store uintptr, id uint32, text string) error {
	encoded, err := syscall.UTF16PtrFromString(text)
	if err != nil {
		return err
	}
	key := propertyKey{Format: appModelKey, ID: id}
	value := stringProperty{Kind: 31 /* VT_LPWSTR */, Text: encoded}
	return hresult(comCall(store, 6 /* SetValue */, uintptr(unsafe.Pointer(&key)), uintptr(unsafe.Pointer(&value))))
}

// WM_SETICON supplies an individual window. Explorer reads the group's picture
// from its shortcut or these properties instead. Set the icon before the
// window AppID: assigning the AppID tells the shell to refresh the button.
// Called only on the window thread, including before its first ShowWindow.
func (state *windowState) setTaskbarIcon(path string) error {
	id := platform.AppUserModelID(state.config.Class)
	if id == "" {
		return nil
	}
	var store uintptr
	status, _, _ := getWindowPropertyStore.Call(state.window, uintptr(unsafe.Pointer(&iidPropertyStore)), uintptr(unsafe.Pointer(&store)))
	if err := hresult(status); err != nil {
		return err
	}
	defer release(&store)
	// Also clear a partially applied update if a later SetValue fails.
	state.taskbarIconSet = true
	for _, property := range []struct {
		id   uint32
		text string
	}{
		{3, path + ",0"},
		{5, id},
	} {
		if err := setStringProperty(store, property.id, property.text); err != nil {
			return err
		}
	}
	return nil // HWND property stores apply SetValue immediately; no Commit.
}

func (state *windowState) clearTaskbarIcon() {
	if !state.taskbarIconSet {
		return
	}
	var store uintptr
	status, _, _ := getWindowPropertyStore.Call(state.window, uintptr(unsafe.Pointer(&iidPropertyStore)), uintptr(unsafe.Pointer(&store)))
	if hresult(status) != nil {
		return
	}
	defer release(&store)
	for _, id := range []uint32{3, 5} {
		key := propertyKey{Format: appModelKey, ID: id}
		var empty stringProperty // VT_EMPTY releases the window's stored value.
		comCall(store, 6, uintptr(unsafe.Pointer(&key)), uintptr(unsafe.Pointer(&empty)))
	}
	state.taskbarIconSet = false
}
