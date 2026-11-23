//go:build android

package android

/*
#cgo LDFLAGS: -llog
#include <stdlib.h>
#include "jni_android.h"
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// Priorities of the system log.
const (
	logInfo  = 4
	logError = 6
)

// logf writes a line to the system log (adb logcat -s viva).
func logf(priority int, format string, args ...any) {
	line := C.CString(fmt.Sprintf(format, args...))
	defer C.free(unsafe.Pointer(line))
	C.viva_log(C.int(priority), line)
}
