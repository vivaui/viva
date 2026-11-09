//go:build darwin

package darwin

/*
#include "native.h"
*/
import "C"

import (
	"fmt"
	"unsafe"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

// MeasureText shapes and measures one UTF-8 line with the macOS system font at
// a weight (gfx.TextStyle.Weight), the font a label of that weight is drawn in.
func MeasureText(value string, fontSize, weight float32) (text.Metrics, error) {
	if fontSize <= 0 {
		return text.Metrics{}, fmt.Errorf("darwin: font size must be positive")
	}
	var data *C.uchar
	if len(value) > 0 {
		data = (*C.uchar)(unsafe.Pointer(unsafe.StringData(value)))
	}
	var metrics C.viva_text_metrics
	status := C.viva_measure_text(data, C.size_t(len(value)), C.double(fontSize), C.double(weight), &metrics)
	if status != 0 {
		return text.Metrics{}, fmt.Errorf("darwin: CoreText measurement failed with status %d", status)
	}
	return text.Metrics{
		Width:   float32(metrics.width),
		Height:  float32(metrics.height),
		Ascent:  float32(metrics.ascent),
		Descent: float32(metrics.descent),
		Leading: float32(metrics.leading),
	}, nil
}

// MeasureMono reports the system fixed-pitch font's character cell at a size:
// the advance of one character and the height of one line.
func MeasureMono(fontSize float32, face gfx.MonoFace) (width, height float32, err error) {
	if fontSize <= 0 {
		return 0, 0, fmt.Errorf("darwin: font size must be positive")
	}
	syncMonoFamilies()
	var advance, lineHeight C.double
	if status := C.viva_measure_mono(C.double(fontSize), C.int(face), &advance, &lineHeight); status != 0 {
		return 0, 0, fmt.Errorf("darwin: CoreText fixed-pitch measurement failed with status %d", status)
	}
	return float32(advance), float32(lineHeight), nil
}
