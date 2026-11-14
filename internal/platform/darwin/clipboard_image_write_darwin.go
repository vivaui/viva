//go:build darwin

package darwin

/*
#include "system_darwin.h"
*/
import "C"

import (
	"fmt"
	"runtime"
	"unsafe"
)

// SetClipboardImagePNG writes validated PNG data on the general pasteboard.
func SetClipboardImagePNG(data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("darwin: clipboard PNG is empty")
	}
	status := C.viva_system_set_clipboard_image_png((*C.uint8_t)(unsafe.Pointer(&data[0])), C.size_t(len(data)))
	runtime.KeepAlive(data)
	if status != 0 {
		return fmt.Errorf("darwin: write clipboard PNG failed")
	}
	return nil
}
