//go:build darwin

package darwin

/*
#cgo LDFLAGS: -framework CFNetwork -framework CoreFoundation
#include <stdlib.h>
#include "proxy_darwin.h"
#include "system_darwin.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"net/url"
	"unsafe"
)

// ErrProxyUnavailable reports that the system's proxy policy could not be
// evaluated: it is a PAC script, or CFNetwork failed.
var ErrProxyUnavailable = errors.New("darwin: the system proxy settings cannot be read")

// SystemProxy asks CFNetwork which proxy the system selects for target.
func SystemProxy(target string) (*url.URL, error) {
	cTarget := C.CString(target)
	defer C.free(unsafe.Pointer(cTarget))
	var value, nativeError *C.char
	status := C.viva_system_proxy(cTarget, &value, &nativeError)
	switch status {
	case 0:
		return nil, nil
	case 1:
		defer C.viva_system_free(unsafe.Pointer(value))
		proxy, err := url.Parse(C.GoString(value))
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrProxyUnavailable, err)
		}
		return proxy, nil
	case 2:
		return nil, fmt.Errorf("%w: the system uses a proxy auto-configuration script", ErrProxyUnavailable)
	default:
		message := "the system proxy settings could not be read"
		if nativeError != nil {
			message = C.GoString(nativeError)
			C.viva_system_free(unsafe.Pointer(nativeError))
		}
		return nil, fmt.Errorf("%w: %s", ErrProxyUnavailable, message)
	}
}
