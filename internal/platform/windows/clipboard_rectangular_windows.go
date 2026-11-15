package windows

import (
	"syscall"
	"unsafe"
)

// These IDs are initialized only by a clipboard operation, under clipboardMu.
var columnSelectFormat, borlandBlockFormat uintptr

func rectangularClipboardFormats() (uintptr, uintptr, error) {
	if columnSelectFormat == 0 {
		name, err := syscall.UTF16PtrFromString("MSDEVColumnSelect")
		if err != nil {
			return 0, 0, err
		}
		format, _, err := registerClipboardFormat.Call(uintptr(unsafe.Pointer(name)))
		if format == 0 {
			return 0, 0, windowsCallError("register column clipboard format", err)
		}
		columnSelectFormat = format
	}
	if borlandBlockFormat == 0 {
		name, err := syscall.UTF16PtrFromString("Borland IDE Block Type")
		if err != nil {
			return 0, 0, err
		}
		format, _, err := registerClipboardFormat.Call(uintptr(unsafe.Pointer(name)))
		if format == 0 {
			return 0, 0, windowsCallError("register Borland clipboard format", err)
		}
		borlandBlockFormat = format
	}
	return columnSelectFormat, borlandBlockFormat, nil
}

// SetClipboardRectangularText writes Unicode text and both Scintilla column markers.
func SetClipboardRectangularText(value string) error {
	return setClipboardText(value, true)
}

// ClipboardTextWithFormat reads text and its markers while the clipboard is open.
func ClipboardTextWithFormat() (string, bool, error) {
	clipboardMu.Lock()
	defer clipboardMu.Unlock()
	if opened, _, err := openClipboard.Call(0); opened == 0 {
		return "", false, windowsCallError("open clipboard", err)
	}
	defer closeClipboard.Call()
	if available, _, _ := clipboardFormatAvailable.Call(clipboardUnicodeText); available == 0 {
		return "", false, nil
	}
	value, err := clipboardTextOpen()
	if err != nil {
		return "", false, err
	}
	column, borland, err := rectangularClipboardFormats()
	if err != nil {
		return "", false, err
	}
	if available, _, _ := clipboardFormatAvailable.Call(column); available != 0 {
		return value, true, nil
	}
	if available, _, _ := clipboardFormatAvailable.Call(borland); available == 0 {
		return value, false, nil
	}
	handle, _, err := clipboardData.Call(borland)
	if handle == 0 {
		return "", false, windowsCallError("read Borland clipboard marker", err)
	}
	rectangular, err := borlandClipboardMarker(handle)
	if err != nil {
		return "", false, err
	}
	return value, rectangular, nil
}

func borlandClipboardMarker(handle uintptr) (bool, error) {
	if size, _, _ := globalSize.Call(handle); size != 1 {
		return false, nil
	}
	memory, _, err := globalLock.Call(handle)
	if memory == 0 {
		return false, windowsCallError("lock Borland clipboard marker", err)
	}
	defer globalUnlock.Call(handle)
	return *(*byte)(pointer(memory)) == 0x02, nil
}

func allocateColumnClipboardMarker(value byte) (uintptr, error) {
	handle, _, err := globalAlloc.Call(globalMoveable|globalZeroInit, 1)
	if handle == 0 {
		return 0, windowsCallError("allocate column clipboard marker", err)
	}
	if value != 0 {
		memory, _, err := globalLock.Call(handle)
		if memory == 0 {
			globalFree.Call(handle)
			return 0, windowsCallError("lock column clipboard marker", err)
		}
		*(*byte)(pointer(memory)) = value
		globalUnlock.Call(handle)
	}
	return handle, nil
}
