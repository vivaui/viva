//go:build darwin

package darwin

/*
#cgo LDFLAGS: -framework IOKit
#include <stdlib.h>
#include "sleep_darwin.h"
#include "system_darwin.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"sync"
	"unsafe"
)

// ErrSleepUnavailable reports that IOKit refused the assertion.
var ErrSleepUnavailable = errors.New("darwin: sleep inhibition is unavailable")

// PreventSleep asks IOKit to keep the machine out of idle sleep until release
// runs. The assertion is what pmset -g assertions lists beside the reason.
func PreventSleep(reason string) (release func(), err error) {
	cReason := C.CString(reason)
	defer C.free(unsafe.Pointer(cReason))
	var assertion C.uint32_t
	var nativeError *C.char
	if status := C.viva_system_keep_awake(cReason, &assertion, &nativeError); status != 0 {
		text := "IOKit refused the assertion"
		if nativeError != nil {
			text = C.GoString(nativeError)
			C.viva_system_free(unsafe.Pointer(nativeError))
		}
		return nil, fmt.Errorf("%w: %s", ErrSleepUnavailable, text)
	}
	var once sync.Once
	return func() {
		once.Do(func() { C.viva_system_release_awake(assertion) })
	}, nil
}
