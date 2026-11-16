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
	handle, _, _ := clipboardData.Call(borland)
	if handle == 0 {
		// The format is advertised but carries no data: its owner is gone, or
		// it renders on request and did not. The text itself was read fine, so
		// paste it as stream text instead of failing the whole paste, the rule
		// Scintilla's reader applies to a missing marker.
		return value, false, nil
	}
	return value, borlandClipboardMarker(handle), nil
}

// borlandClipboardMarker reports whether the Borland block declares a column
// selection. A block that cannot be sized or locked is not a column marker;
// the text is pasted as stream text, as Scintilla does.
func borlandClipboardMarker(handle uintptr) bool {
	if size, _, _ := globalSize.Call(handle); size != 1 {
		return false
	}
	memory, _, _ := globalLock.Call(handle)
	if memory == 0 {
		return false
	}
	defer globalUnlock.Call(handle)
	return *(*byte)(pointer(memory)) == 0x02
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
