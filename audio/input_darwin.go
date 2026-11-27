//go:build darwin && !ios

package audio

/*
#cgo darwin LDFLAGS: -framework AudioToolbox -framework CoreAudio
#include <stdint.h>

typedef struct viva_audio_input viva_audio_input;

viva_audio_input *viva_audio_input_open(double rate, uint32_t channels, int32_t *status);
int64_t viva_audio_input_read(viva_audio_input *a, void *pcm, uintptr_t len);
void viva_audio_input_stop(viva_audio_input *a);
void viva_audio_input_close(viva_audio_input *a);
*/
import "C"

import (
	"errors"
	"fmt"
	"runtime"
	"unsafe"
)

var errInputStopped = errors.New("audio: input stopped")

type darwinInputDevice struct {
	handle *C.viva_audio_input
}

func platformInputDevice(format Format) (inputDevice, error) {
	var status C.int32_t
	handle := C.viva_audio_input_open(C.double(format.SampleRate), C.uint32_t(format.Channels), &status)
	if handle == nil {
		if status == -1 {
			return nil, fmt.Errorf("audio: the default input device is missing: %w", ErrInputUnsupported)
		}
		return nil, fmt.Errorf("audio: open the default input unit: OSStatus %d", int32(status))
	}
	return &darwinInputDevice{handle: handle}, nil
}

func (d *darwinInputDevice) read(pcm []byte) (int, error) {
	if len(pcm) == 0 {
		return 0, nil
	}
	n := C.viva_audio_input_read(d.handle, unsafe.Pointer(&pcm[0]), C.uintptr_t(len(pcm)))
	runtime.KeepAlive(pcm)
	if n == 0 {
		return 0, errInputStopped
	}
	if n < 0 {
		return 0, fmt.Errorf("audio: capture on the input unit: OSStatus %d", int64(n))
	}
	return int(n), nil
}

func (d *darwinInputDevice) stop() {
	if d.handle != nil {
		C.viva_audio_input_stop(d.handle)
	}
}

func (d *darwinInputDevice) close() {
	if d.handle != nil {
		C.viva_audio_input_close(d.handle)
		d.handle = nil
	}
}
