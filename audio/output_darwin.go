//go:build darwin && !ios

package audio

/*
#cgo darwin LDFLAGS: -framework AudioToolbox -framework CoreAudio
#include <stdint.h>

typedef struct viva_audio viva_audio;

viva_audio *viva_audio_open(double rate, uint32_t channels, int32_t *status);
int64_t viva_audio_write(viva_audio *a, const void *pcm, uintptr_t len);
void viva_audio_close(viva_audio *a);
*/
import "C"

import (
	"fmt"
	"runtime"
	"unsafe"
)

// darwinDevice plays through the default output AudioUnit. The unit's input
// scope takes the stream's own format, so CoreAudio resamples and remixes it
// to the device's when they differ.
type darwinDevice struct {
	handle *C.viva_audio
}

func platformDevice(format Format) (device, error) {
	var status C.int32_t
	handle := C.viva_audio_open(C.double(format.SampleRate), C.uint32_t(format.Channels), &status)
	if handle == nil {
		if status == -1 {
			return nil, fmt.Errorf("audio: the default output unit is missing: %w", ErrUnsupported)
		}
		return nil, fmt.Errorf("audio: open the default output unit: OSStatus %d", int32(status))
	}
	return &darwinDevice{handle: handle}, nil
}

func (d *darwinDevice) write(pcm []byte) error {
	if len(pcm) == 0 {
		return nil
	}
	written := C.viva_audio_write(d.handle, unsafe.Pointer(&pcm[0]), C.uintptr_t(len(pcm)))
	runtime.KeepAlive(pcm)
	if written < 0 {
		return fmt.Errorf("audio: play on the output unit: OSStatus %d", int64(written))
	}
	if int(written) != len(pcm) {
		return fmt.Errorf("audio: play on the output unit: %d of %d bytes", int64(written), len(pcm))
	}
	return nil
}

func (d *darwinDevice) close() {
	if d.handle != nil {
		C.viva_audio_close(d.handle)
		d.handle = nil
	}
}
