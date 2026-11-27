//go:build windows

package audio

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	wasapiCapture          = 1 // eCapture
	wasapiBufferFlagSilent = 0x2
)

var (
	iidAudioCaptureClient = guid{0xC8ADBD64, 0xE71E, 0x48A0, [8]byte{0xA4, 0xDE, 0x18, 0x5C, 0x39, 0x5C, 0xD3, 0x17}}
	errInputStopped       = errors.New("audio: input stopped")
)

type wasapiInputDevice struct {
	enumerator uintptr
	endpoint   uintptr
	client     uintptr
	capture    uintptr
	audioEvent windows.Handle
	stopMu     sync.Mutex
	stopEvent  windows.Handle
	started    bool
	frameBytes int
}

func platformInputDevice(format Format) (inputDevice, error) {
	result, _, _ := coInitializeEx.Call(0, coinitMultithreaded)
	switch uint32(result) {
	case 0, coinitSFalse:
	case hresultChangedMode:
		return nil, fmt.Errorf("audio: the audio thread is already in a COM apartment of another mode: %w", ErrInputUnsupported)
	default:
		return nil, hresult("initialize COM", result)
	}
	d := &wasapiInputDevice{frameBytes: format.Channels * 2}
	if err := d.open(format); err != nil {
		d.close()
		return nil, err
	}
	return d, nil
}

func (d *wasapiInputDevice) open(format Format) error {
	r, _, _ := coCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsidMMDeviceEnumerator)),
		0,
		clsctxAll,
		uintptr(unsafe.Pointer(&iidMMDeviceEnumerator)),
		uintptr(unsafe.Pointer(&d.enumerator)),
	)
	runtime.KeepAlive(&clsidMMDeviceEnumerator)
	runtime.KeepAlive(&iidMMDeviceEnumerator)
	if err := hresult("create the audio endpoint enumerator", r); err != nil {
		return fmt.Errorf("%w: %v", ErrInputUnsupported, err)
	}
	// IMMDeviceEnumerator::GetDefaultAudioEndpoint(eCapture, eConsole, ...).
	if r := comCall(d.enumerator, 4, wasapiCapture, wasapiConsole, uintptr(unsafe.Pointer(&d.endpoint))); int32(uint32(r)) < 0 {
		return fmt.Errorf("audio: no default input device: %w", ErrInputUnsupported)
	}
	// IMMDevice::Activate(IID_IAudioClient, ...).
	if r := comCall(d.endpoint, 3, uintptr(unsafe.Pointer(&iidAudioClient)), clsctxAll, 0, uintptr(unsafe.Pointer(&d.client))); int32(uint32(r)) < 0 {
		return hresult("activate the audio capture client", r)
	}

	wfx := waveFormatEx{
		FormatTag:     waveFormatPCM,
		Channels:      uint16(format.Channels),
		SamplesPerSec: uint32(format.SampleRate),
		BlockAlign:    uint16(format.Channels * 2),
		BitsPerSample: 16,
	}
	wfx.AvgBytesPerSec = wfx.SamplesPerSec * uint32(wfx.BlockAlign)
	const flags = wasapiEventCallback | wasapiAutoConvertPCM | wasapiSRCDefaultQuality
	if r := comCall(d.client, 3,
		wasapiShareModeShared,
		flags,
		wasapiBufferDuration100ns,
		0,
		uintptr(unsafe.Pointer(&wfx)),
		0,
	); int32(uint32(r)) < 0 {
		return hresult("initialize the audio capture client", r)
	}
	runtime.KeepAlive(&wfx)

	var frames uint32
	if r := comCall(d.client, 4, uintptr(unsafe.Pointer(&frames))); int32(uint32(r)) < 0 {
		return hresult("read the capture buffer size", r)
	}
	if frames == 0 {
		return fmt.Errorf("audio: the audio capture client reported an empty buffer")
	}
	// IAudioClient::GetService(IID_IAudioCaptureClient).
	if r := comCall(d.client, 14, uintptr(unsafe.Pointer(&iidAudioCaptureClient)), uintptr(unsafe.Pointer(&d.capture))); int32(uint32(r)) < 0 {
		return hresult("get the audio capture client", r)
	}
	audioEvent, err := windows.CreateEvent(nil, 0, 0, nil)
	if err != nil {
		return fmt.Errorf("audio: create the capture event: %w", err)
	}
	d.audioEvent = audioEvent
	stopEvent, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		return fmt.Errorf("audio: create the capture stop event: %w", err)
	}
	d.stopEvent = stopEvent
	// IAudioClient::SetEventHandle.
	if r := comCall(d.client, 13, uintptr(d.audioEvent)); int32(uint32(r)) < 0 {
		return hresult("set the capture event", r)
	}
	// IAudioClient::Start.
	if r := comCall(d.client, 10); int32(uint32(r)) < 0 {
		return hresult("start the audio capture client", r)
	}
	d.started = true
	return nil
}

func (d *wasapiInputDevice) read(pcm []byte) (int, error) {
	written := 0
	for written == 0 {
		var packetFrames uint32
		if r := comCall(d.capture, 5, uintptr(unsafe.Pointer(&packetFrames))); int32(uint32(r)) < 0 {
			return 0, hresult("read the next capture packet size", r)
		}
		if packetFrames == 0 {
			handles := [2]windows.Handle{d.audioEvent, d.stopEvent}
			ev, err := windows.WaitForMultipleObjects(handles[:], false, 200)
			if err != nil {
				return 0, fmt.Errorf("audio: wait for capture event: %w", err)
			}
			if ev == windows.WAIT_OBJECT_0+1 {
				return 0, errInputStopped
			}
			continue
		}
		for packetFrames > 0 {
			var data unsafe.Pointer
			var numFrames uint32
			var flags uint32
			if r := comCall(d.capture, 3, uintptr(unsafe.Pointer(&data)), uintptr(unsafe.Pointer(&numFrames)), uintptr(unsafe.Pointer(&flags)), 0, 0); int32(uint32(r)) < 0 {
				return written, hresult("get a capture buffer", r)
			}
			nBytes := int(numFrames) * d.frameBytes
			if nBytes > 0 && written+nBytes <= len(pcm) {
				if flags&wasapiBufferFlagSilent != 0 || data == nil {
					clear(pcm[written : written+nBytes])
				} else {
					copy(pcm[written:written+nBytes], unsafe.Slice((*byte)(data), nBytes))
				}
				written += nBytes
			}
			if r := comCall(d.capture, 4, uintptr(numFrames)); int32(uint32(r)) < 0 {
				return written, hresult("release a capture buffer", r)
			}
			if written >= len(pcm) {
				break
			}
			if r := comCall(d.capture, 5, uintptr(unsafe.Pointer(&packetFrames))); int32(uint32(r)) < 0 {
				return written, hresult("read the next capture packet size", r)
			}
		}
	}
	return written, nil
}

func (d *wasapiInputDevice) stop() {
	d.stopMu.Lock()
	if d.stopEvent != 0 {
		_ = windows.SetEvent(d.stopEvent)
	}
	d.stopMu.Unlock()
}

func (d *wasapiInputDevice) close() {
	if d.client != 0 && d.started {
		comCall(d.client, 11) // Stop
		d.started = false
	}
	release(&d.capture)
	release(&d.client)
	release(&d.endpoint)
	release(&d.enumerator)
	if d.audioEvent != 0 {
		_ = windows.CloseHandle(d.audioEvent)
		d.audioEvent = 0
	}
	d.stopMu.Lock()
	if d.stopEvent != 0 {
		_ = windows.CloseHandle(d.stopEvent)
		d.stopEvent = 0
	}
	d.stopMu.Unlock()
	coUninitialize.Call()
}
