//go:build windows

package audio

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// WASAPI constants (audioclient.h).
const (
	wasapiShareModeShared     = 0
	wasapiEventCallback       = 0x00040000
	wasapiAutoConvertPCM      = 0x80000000
	wasapiSRCDefaultQuality   = 0x08000000
	wasapiRender              = 0 // eRender
	wasapiConsole             = 0 // eConsole
	coinitMultithreaded       = 0x0
	clsctxAll                 = 0x17
	coinitSFalse              = 0x1
	hresultChangedMode        = 0x80010106
	wasapiBufferDuration100ns = 100 * 10000 // 100 ms
	waveFormatPCM             = 1
)

type guid struct {
	Data1        uint32
	Data2, Data3 uint16
	Data4        [8]byte
}

var (
	clsidMMDeviceEnumerator = guid{0xBCDE0395, 0xE52F, 0x467C, [8]byte{0x8E, 0x3D, 0xC4, 0x57, 0x92, 0x91, 0x69, 0x2E}}
	iidMMDeviceEnumerator   = guid{0xA95664D2, 0x9614, 0x4F35, [8]byte{0xA7, 0x46, 0xDE, 0x8D, 0xB6, 0x36, 0x17, 0xE6}}
	iidAudioClient          = guid{0x1CB9AD4C, 0xDBFA, 0x4C32, [8]byte{0xB1, 0x78, 0xC2, 0xF5, 0x68, 0xA7, 0x03, 0xB2}}
	iidAudioRenderClient    = guid{0xF294ACFC, 0x3146, 0x4483, [8]byte{0xA7, 0xBF, 0xAD, 0xDC, 0xA7, 0xC2, 0x60, 0xE2}}
)

// waveFormatEx is WAVEFORMATEX (mmreg.h). cbSize stays zero for PCM.
type waveFormatEx struct {
	FormatTag      uint16
	Channels       uint16
	SamplesPerSec  uint32
	AvgBytesPerSec uint32
	BlockAlign     uint16
	BitsPerSample  uint16
	Size           uint16
}

var (
	ole32            = windows.NewLazySystemDLL("ole32.dll")
	coInitializeEx   = ole32.NewProc("CoInitializeEx")
	coUninitialize   = ole32.NewProc("CoUninitialize")
	coCreateInstance = ole32.NewProc("CoCreateInstance")
)

// pointer and vtable read a COM interface the way the Windows platform package
// does: the object points at its vtable, and each method is one slot.
func pointer(address uintptr) unsafe.Pointer { return *(*unsafe.Pointer)(unsafe.Pointer(&address)) }

func vtable(object uintptr, index int) uintptr {
	return *(*uintptr)(unsafe.Add(pointer(*(*uintptr)(pointer(object))), index*int(unsafe.Sizeof(uintptr(0)))))
}

//go:uintptrescapes
func comCall(object uintptr, index int, arguments ...uintptr) uintptr {
	var values [16]uintptr
	if len(arguments) >= len(values) {
		panic("audio: COM call exceeds argument limit")
	}
	values[0] = object
	copy(values[1:], arguments)
	result, _, _ := syscall.SyscallN(vtable(object, index), values[:len(arguments)+1]...)
	return result
}

func release(object *uintptr) {
	if *object != 0 {
		comCall(*object, 2) // IUnknown::Release
		*object = 0
	}
}

func hresult(step string, result uintptr) error {
	if int32(uint32(result)) < 0 {
		return fmt.Errorf("audio: %s: HRESULT %#x", step, uint32(result))
	}
	return nil
}

// wasapiDevice plays through the default render endpoint in shared mode. The
// stream is initialized with its own PCM format and AUTOCONVERTPCM, so the
// audio engine resamples and remixes it to the device's mix format.
type wasapiDevice struct {
	enumerator uintptr
	endpoint   uintptr
	client     uintptr
	render     uintptr
	event      windows.Handle
	frames     uint32
	started    bool
	format     waveFormatEx
}

func platformDevice(format Format) (device, error) {
	result, _, _ := coInitializeEx.Call(0, coinitMultithreaded)
	switch uint32(result) {
	case 0, coinitSFalse:
	case hresultChangedMode:
		return nil, fmt.Errorf("audio: the audio thread is already in a COM apartment of another mode: %w", ErrUnsupported)
	default:
		return nil, hresult("initialize COM", result)
	}
	d := &wasapiDevice{}
	if err := d.open(format); err != nil {
		d.close()
		return nil, err
	}
	return d, nil
}

func (d *wasapiDevice) open(format Format) error {
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
		return fmt.Errorf("%w: %v", ErrUnsupported, err)
	}
	// IMMDeviceEnumerator::GetDefaultAudioEndpoint.
	if r := comCall(d.enumerator, 4, wasapiRender, wasapiConsole, uintptr(unsafe.Pointer(&d.endpoint))); int32(uint32(r)) < 0 {
		return fmt.Errorf("audio: no default output device: %w", ErrUnsupported)
	}
	// IMMDevice::Activate(IID_IAudioClient, ...).
	if r := comCall(d.endpoint, 3, uintptr(unsafe.Pointer(&iidAudioClient)), clsctxAll, 0, uintptr(unsafe.Pointer(&d.client))); int32(uint32(r)) < 0 {
		return hresult("activate the audio client", r)
	}

	d.format = waveFormatEx{
		FormatTag:     waveFormatPCM,
		Channels:      uint16(format.Channels),
		SamplesPerSec: uint32(format.SampleRate),
		BlockAlign:    uint16(format.Channels * 2),
		BitsPerSample: 16,
	}
	d.format.AvgBytesPerSec = d.format.SamplesPerSec * uint32(d.format.BlockAlign)
	// IAudioClient::Initialize in shared mode. AUTOCONVERTPCM lets the engine
	// accept this format whatever the device mixes at.
	const flags = wasapiEventCallback | wasapiAutoConvertPCM | wasapiSRCDefaultQuality
	if r := comCall(d.client, 3,
		wasapiShareModeShared,
		flags,
		wasapiBufferDuration100ns,
		0,
		uintptr(unsafe.Pointer(&d.format)),
		0,
	); int32(uint32(r)) < 0 {
		return hresult("initialize the audio client", r)
	}
	runtime.KeepAlive(&d.format)

	if r := comCall(d.client, 4, uintptr(unsafe.Pointer(&d.frames))); int32(uint32(r)) < 0 {
		return hresult("read the audio buffer size", r)
	}
	if d.frames == 0 {
		return fmt.Errorf("audio: the audio client reported an empty buffer")
	}
	// IAudioClient::GetService(IID_IAudioRenderClient).
	if r := comCall(d.client, 14, uintptr(unsafe.Pointer(&iidAudioRenderClient)), uintptr(unsafe.Pointer(&d.render))); int32(uint32(r)) < 0 {
		return hresult("get the audio render client", r)
	}
	event, err := windows.CreateEvent(nil, 0, 0, nil)
	if err != nil {
		return fmt.Errorf("audio: create the audio event: %w", err)
	}
	d.event = event
	// IAudioClient::SetEventHandle.
	if r := comCall(d.client, 13, uintptr(event)); int32(uint32(r)) < 0 {
		return hresult("set the audio event", r)
	}
	// IAudioClient::Start.
	if r := comCall(d.client, 10); int32(uint32(r)) < 0 {
		return hresult("start the audio client", r)
	}
	d.started = true
	return nil
}

func (d *wasapiDevice) write(pcm []byte) error {
	frameBytes := int(d.format.BlockAlign)
	if frameBytes == 0 {
		frameBytes = 4
	}
	for len(pcm) > 0 {
		var padding uint32
		if r := comCall(d.client, 6, uintptr(unsafe.Pointer(&padding))); int32(uint32(r)) < 0 {
			return hresult("read the audio buffer padding", r)
		}
		free := d.frames
		if padding < free {
			free -= padding
		} else {
			free = 0
		}
		if free == 0 {
			windows.WaitForSingleObject(d.event, 200)
			continue
		}
		frames := free
		if want := uint32(len(pcm) / frameBytes); want < frames {
			frames = want
		}
		if frames == 0 {
			return nil
		}
		var data unsafe.Pointer
		// IAudioRenderClient::GetBuffer.
		if r := comCall(d.render, 3, uintptr(frames), uintptr(unsafe.Pointer(&data))); int32(uint32(r)) < 0 {
			return hresult("get an audio buffer", r)
		}
		bytes := int(frames) * frameBytes
		copy(unsafe.Slice((*byte)(data), bytes), pcm[:bytes])
		// IAudioRenderClient::ReleaseBuffer.
		if r := comCall(d.render, 4, uintptr(frames), 0); int32(uint32(r)) < 0 {
			return hresult("release an audio buffer", r)
		}
		pcm = pcm[bytes:]
	}
	return nil
}

func (d *wasapiDevice) close() {
	if d.client != 0 && d.started {
		comCall(d.client, 11) // Stop
		d.started = false
	}
	release(&d.render)
	release(&d.client)
	release(&d.endpoint)
	release(&d.enumerator)
	if d.event != 0 {
		windows.CloseHandle(d.event)
		d.event = 0
	}
	coUninitialize.Call()
}
