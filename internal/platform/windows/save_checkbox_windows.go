package windows

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"
)

const saveCheckboxControlID = 0x5649

var saveDialogCustomizeInterface = guid{
	Data1: 0xe6fdd21a, Data2: 0x163f, Data3: 0x4975,
	Data4: [8]byte{0x9c, 0x8c, 0xa6, 0x9f, 0x1b, 0xa3, 0x70, 0x34},
}

func addSaveDialogCheckbox(dialog uintptr, label string, checked bool) (uintptr, error) {
	var custom uintptr
	if err := hresult(comCall(dialog, 0, uintptr(unsafe.Pointer(&saveDialogCustomizeInterface)), uintptr(unsafe.Pointer(&custom)))); err != nil {
		return 0, fmt.Errorf("windows: customize save file dialog: %w", err)
	}
	name, err := syscall.UTF16PtrFromString(strings.ReplaceAll(label, "&", "&&")) // a lone & would become a mnemonic
	if err != nil {
		release(&custom)
		return 0, err
	}
	initial := uintptr(0)
	if checked {
		initial = 1
	}
	if err := hresult(comCall(custom, 8, saveCheckboxControlID, uintptr(unsafe.Pointer(name)), initial)); err != nil {
		release(&custom)
		return 0, fmt.Errorf("windows: add save checkbox: %w", err)
	}
	return custom, nil
}

func saveDialogCheckboxState(custom uintptr) (bool, error) {
	var checked uint32
	if err := hresult(comCall(custom, 17, saveCheckboxControlID, uintptr(unsafe.Pointer(&checked)))); err != nil {
		return false, fmt.Errorf("windows: read save checkbox: %w", err)
	}
	return checked != 0, nil
}
