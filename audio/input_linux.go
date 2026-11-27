//go:build linux && (amd64 || arm64)

package audio

import (
	"errors"
	"fmt"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
)

var errInputStopped = errors.New("audio: input stopped")

type alsaInputDevice struct {
	library     uintptr
	pcm         uintptr
	frame       int
	chunkFrames int
	api         *alsaAPI
	stopped     atomic.Bool
}

func platformInputDevice(format Format) (inputDevice, error) {
	api, library, err := openALSA()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInputUnsupported, err)
	}
	name := []byte("default\x00")
	var pcm uintptr
	if status := api.open(&pcm, &name[0], alsaCapture, 0); status < 0 {
		message := alsaError(api, status)
		purego.Dlclose(library)
		return nil, fmt.Errorf("audio: open the ALSA default capture device: %s: %w", message, ErrInputUnsupported)
	}
	if err := configureALSA(api, pcm, format); err != nil {
		api.close(pcm)
		purego.Dlclose(library)
		return nil, err
	}
	return &alsaInputDevice{
		library:     library,
		pcm:         pcm,
		frame:       format.Channels * 2,
		chunkFrames: max(format.SampleRate/100, 1),
		api:         api,
	}, nil
}

func (d *alsaInputDevice) read(pcm []byte) (int, error) {
	wantFrames := min(len(pcm)/d.frame, d.chunkFrames)
	if wantFrames < 1 {
		wantFrames = 1
	}
	for {
		if d.stopped.Load() {
			return 0, errInputStopped
		}
		ready := d.api.wait(d.pcm, 50)
		if d.stopped.Load() {
			return 0, errInputStopped
		}
		if ready == 0 {
			continue
		}
		if ready < 0 {
			if status := d.api.recover(d.pcm, ready, 1); status < 0 {
				return 0, fmt.Errorf("audio: wait on the ALSA capture device: %s", alsaError(d.api, status))
			}
			continue
		}
		got := d.api.readi(d.pcm, unsafe.Pointer(&pcm[0]), uintptr(wantFrames))
		if got > 0 {
			return int(got) * d.frame, nil
		}
		if got == 0 {
			time.Sleep(time.Millisecond)
			continue
		}
		recovered := false
		for range alsaRecoverRetries {
			if d.stopped.Load() {
				return 0, errInputStopped
			}
			if status := d.api.recover(d.pcm, int32(got), 1); status < 0 {
				return 0, fmt.Errorf("audio: capture on the ALSA device: %s", alsaError(d.api, status))
			}
			got = d.api.readi(d.pcm, unsafe.Pointer(&pcm[0]), uintptr(wantFrames))
			if got > 0 {
				return int(got) * d.frame, nil
			}
			if got == 0 {
				recovered = true
				break
			}
		}
		if !recovered {
			return 0, fmt.Errorf("audio: the ALSA capture device stopped: %s", alsaError(d.api, int32(got)))
		}
	}
}

func (d *alsaInputDevice) stop() {
	d.stopped.Store(true)
}

func (d *alsaInputDevice) close() {
	if d.pcm != 0 {
		d.api.drop(d.pcm)
		d.api.close(d.pcm)
		d.pcm = 0
	}
	if d.library != 0 {
		purego.Dlclose(d.library)
		d.library = 0
	}
}
