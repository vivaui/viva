//go:build darwin && !ios

package darwin

/*
#include <stdlib.h>
#include "watermark_darwin.h"
*/
import "C"
import (
	"fmt"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

func (window *nativeWindow) SetWatermark(value platform.Watermark) error {
	if window == nil || window.closed.Load() {
		return fmt.Errorf("darwin: window is closed")
	}
	value, err := platform.NormalizeWatermark(value)
	if err != nil {
		return err
	}
	text := C.CString(value.Text)
	defer C.free(unsafe.Pointer(text))
	family := C.CString(value.FontFamily)
	defer C.free(unsafe.Pointer(family))
	if C.viva_application_set_watermark(window.token, nativeWatermark(value, text, family)) == 0 {
		return fmt.Errorf("darwin: window is unavailable")
	}
	return nil
}
