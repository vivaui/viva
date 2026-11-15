//go:build windows

package windows

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

// ErrSleepUnavailable reports that the system refused a power request.
var ErrSleepUnavailable = errors.New("windows: sleep inhibition is unavailable")

const (
	// POWER_REQUEST_CONTEXT_VERSION is 0, the only version PowerCreateRequest
	// accepts.
	powerRequestContextVersion = 0
	// POWER_REQUEST_CONTEXT_SIMPLE_STRING makes the context carry one string.
	powerRequestContextSimpleString = 1
	// PowerRequestSystemRequired keeps the system out of idle sleep without
	// keeping the display on.
	powerRequestSystemRequired = 1
)

// reasonContext is REASON_CONTEXT. The union is laid out as its detailed form,
// which is at least as large as the simple string pointer it also holds.
type reasonContext struct {
	version uint32
	flags   uint32
	reason  reasonDetailed
}

type reasonDetailed struct {
	module  uintptr
	id      uint32
	count   uint32
	strings uintptr
}

var (
	powerCreateRequest = kernel.NewProc("PowerCreateRequest")
	powerSetRequest    = kernel.NewProc("PowerSetRequest")
	powerClearRequest  = kernel.NewProc("PowerClearRequest")
)

// PreventSleep asks the system to stay out of idle sleep until release runs.
// The reason is what powercfg /requests shows beside the request.
func PreventSleep(reason string) (release func(), err error) {
	text, err := syscall.UTF16FromString(reason)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSleepUnavailable, err)
	}
	context := reasonContext{version: powerRequestContextVersion, flags: powerRequestContextSimpleString}
	context.reason.module = uintptr(unsafe.Pointer(&text[0]))
	request, _, callErr := powerCreateRequest.Call(uintptr(unsafe.Pointer(&context)))
	runtime.KeepAlive(text)
	if request == 0 {
		return nil, fmt.Errorf("%w: PowerCreateRequest: %v", ErrSleepUnavailable, callErr)
	}
	if ok, _, callErr := powerSetRequest.Call(request, powerRequestSystemRequired); ok == 0 {
		closeHandle.Call(request)
		return nil, fmt.Errorf("%w: PowerSetRequest: %v", ErrSleepUnavailable, callErr)
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			powerClearRequest.Call(request, powerRequestSystemRequired)
			closeHandle.Call(request)
			// The request copies the reason, but keeping the string live until
			// the request ends costs nothing and does not rely on that.
			runtime.KeepAlive(text)
		})
	}, nil
}
