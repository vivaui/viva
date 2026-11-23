//go:build darwin && !ios

package darwin

/*
#cgo LDFLAGS: -framework CFNetwork -framework CoreFoundation
#include <stdlib.h>
#include "proxy_darwin.h"
#include "system_darwin.h"
*/
import "C"

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"
	"unsafe"
)

// ErrProxyUnavailable reports that the system's proxy policy could not be
// evaluated: CFNetwork failed, or a PAC script failed or timed out.
var ErrProxyUnavailable = errors.New("darwin: the system proxy settings cannot be read")

// SystemProxy asks CFNetwork which proxy the system selects for target.
func SystemProxy(target string) (*url.URL, error) {
	return SystemProxyContext(context.Background(), target)
}

// SystemProxyContext also stops a pending PAC download or evaluation when ctx
// ends. Native PAC work has a ten-second ceiling even without a deadline.
func SystemProxyContext(ctx context.Context, target string) (*url.URL, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	cTarget := C.CString(target)
	defer C.free(unsafe.Pointer(cTarget))
	var value, nativeError *C.char
	status := C.viva_system_proxy(cTarget, &value, &nativeError)
	if status == 2 || status == 3 {
		defer C.viva_system_free(unsafe.Pointer(value))
		return proxyFromPAC(ctx, target, C.GoString(value), status == 2, 10*time.Second)
	}
	return proxyResult(status, value, nativeError)
}

func proxyFromPAC(ctx context.Context, target, configuration string, fromURL bool, timeout time.Duration) (*url.URL, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	request := C.viva_proxy_request_create()
	if request == nil {
		return nil, fmt.Errorf("%w: the PAC request could not be allocated", ErrProxyUnavailable)
	}
	// Cancellation only writes the C request's atomic flag. Wait for an already
	// running AfterFunc before dropping our reference to its C allocation.
	cancelled := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		C.viva_proxy_request_cancel(request)
		close(cancelled)
	})
	defer func() {
		if !stop() {
			<-cancelled
		}
		C.viva_proxy_request_release(request)
	}()
	cTarget, cConfiguration := C.CString(target), C.CString(configuration)
	defer C.free(unsafe.Pointer(cTarget))
	defer C.free(unsafe.Pointer(cConfiguration))
	var value, nativeError *C.char
	urlMode := C.int(0)
	if fromURL {
		urlMode = 1
	}
	status := C.viva_proxy_execute_pac(request, cTarget, cConfiguration, urlMode, C.double(timeout.Seconds()), &value, &nativeError)
	proxy, err := proxyResult(status, value, nativeError)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return proxy, err
}

func proxyResult(status C.int, value, nativeError *C.char) (*url.URL, error) {
	if value != nil {
		defer C.viva_system_free(unsafe.Pointer(value))
	}
	if nativeError != nil {
		defer C.viva_system_free(unsafe.Pointer(nativeError))
	}
	switch status {
	case 0:
		return nil, nil
	case 1:
		proxy, err := url.Parse(C.GoString(value))
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrProxyUnavailable, err)
		}
		return proxy, nil
	default:
		message := "the system proxy settings could not be read"
		if nativeError != nil {
			message = C.GoString(nativeError)
		}
		return nil, fmt.Errorf("%w: %s", ErrProxyUnavailable, message)
	}
}
