//go:build windows

package windows

import (
	"fmt"
	"syscall"
	"unsafe"
)

var assocQueryString = systemDLL("shlwapi.dll").NewProc("AssocQueryStringW")

const (
	assocfIsProtocol = 0x00001000 // pszAssoc names a URL protocol, not a file extension
	assocStrCommand  = 1          // ASSOCSTR_COMMAND: the open verb's command line

	hresultNoAssociation = 0x80070483 // HRESULT_FROM_WIN32(ERROR_NO_ASSOCIATION)
	hresultMoreData      = 0x80070ea4 // HRESULT_FROM_WIN32(ERROR_MORE_DATA)
	hresultNoBuffer      = 0x8007007a // HRESULT_FROM_WIN32(ERROR_INSUFFICIENT_BUFFER)
)

// SchemeHandlerAvailable reports whether an application is registered for a URL
// scheme such as "vscode", as ShellExecute would route it.
func SchemeHandlerAvailable(scheme string) (bool, error) {
	name, err := syscall.UTF16PtrFromString(scheme)
	if err != nil {
		return false, err
	}
	var command [1024]uint16
	length := uint32(len(command))
	result, _, _ := assocQueryString.Call(assocfIsProtocol, assocStrCommand,
		uintptr(unsafe.Pointer(name)), 0, uintptr(unsafe.Pointer(&command[0])), uintptr(unsafe.Pointer(&length)))
	switch uint32(result) {
	case 0, 1, hresultMoreData, hresultNoBuffer: // S_OK, S_FALSE, or a command too long for the buffer
		return true, nil
	case hresultNoAssociation:
		return false, nil
	}
	return false, fmt.Errorf("windows: look up URL scheme handler: HRESULT 0x%08x", uint32(result))
}
