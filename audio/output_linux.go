//go:build linux && (amd64 || arm64)

package audio

import (
	"fmt"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
)

// ALSA constants (alsa/pcm.h).
const (
	alsaPlayback       = 0
	alsaCapture        = 1
	alsaNoWait         = 1 // SND_PCM_NONBLOCK, deliberately not used
	alsaAccessRWInter  = 3 // SND_PCM_ACCESS_RW_INTERLEAVED
	alsaFormatS16LE    = 2 // SND_PCM_FORMAT_S16_LE
	alsaRecoverRetries = 4
)

// alsaDevice plays through ALSA's "default" PCM, which on a desktop is the
// sound server (PipeWire or PulseAudio) or dmix/plug: the plugin converts the
// stream's format and rate to what the hardware takes.
type alsaDevice struct {
	library uintptr
	pcm     uintptr
	frame   int // bytes per frame of the stream
	api     *alsaAPI
}

type alsaAPI struct {
	open         func(pcm *uintptr, name *byte, stream int32, mode int32) int32
	close        func(pcm uintptr) int32
	drop         func(pcm uintptr) int32
	prepare      func(pcm uintptr) int32
	recover      func(pcm uintptr, err int32, silent int32) int32
	wait         func(pcm uintptr, timeout int32) int32
	writei       func(pcm uintptr, buf unsafe.Pointer, frames uintptr) int64
	readi        func(pcm uintptr, buf unsafe.Pointer, frames uintptr) int64
	strerror     func(err int32) *byte
	paramsMalloc func(params *uintptr) int32
	paramsFree   func(params uintptr)
	paramsAny    func(pcm uintptr, params uintptr) int32
	setAccess    func(pcm uintptr, params uintptr, access int32) int32
	setFormat    func(pcm uintptr, params uintptr, format int32) int32
	setChannels  func(pcm uintptr, params uintptr, channels uint32) int32
	setRate      func(pcm uintptr, params uintptr, rate uint32, dir int32) int32
	setRateNear  func(pcm uintptr, params uintptr, rate *uint32, dir *int32) int32
	params       func(pcm uintptr, params uintptr) int32
}

func openALSA() (*alsaAPI, uintptr, error) {
	var library uintptr
	var err error
	for _, name := range []string{"libasound.so.2", "libasound.so"} {
		library = 0
		library, err = purego.Dlopen(name, purego.RTLD_NOW|purego.RTLD_LOCAL)
		if err == nil {
			break
		}
	}
	if library == 0 {
		return nil, 0, fmt.Errorf("audio: load libasound: %w", ErrUnsupported)
	}
	api := &alsaAPI{}
	purego.RegisterLibFunc(&api.open, library, "snd_pcm_open")
	purego.RegisterLibFunc(&api.close, library, "snd_pcm_close")
	purego.RegisterLibFunc(&api.drop, library, "snd_pcm_drop")
	purego.RegisterLibFunc(&api.prepare, library, "snd_pcm_prepare")
	purego.RegisterLibFunc(&api.recover, library, "snd_pcm_recover")
	purego.RegisterLibFunc(&api.wait, library, "snd_pcm_wait")
	purego.RegisterLibFunc(&api.writei, library, "snd_pcm_writei")
	purego.RegisterLibFunc(&api.readi, library, "snd_pcm_readi")
	purego.RegisterLibFunc(&api.strerror, library, "snd_strerror")
	purego.RegisterLibFunc(&api.paramsMalloc, library, "snd_pcm_hw_params_malloc")
	purego.RegisterLibFunc(&api.paramsFree, library, "snd_pcm_hw_params_free")
	purego.RegisterLibFunc(&api.paramsAny, library, "snd_pcm_hw_params_any")
	purego.RegisterLibFunc(&api.setAccess, library, "snd_pcm_hw_params_set_access")
	purego.RegisterLibFunc(&api.setFormat, library, "snd_pcm_hw_params_set_format")
	purego.RegisterLibFunc(&api.setChannels, library, "snd_pcm_hw_params_set_channels")
	purego.RegisterLibFunc(&api.setRate, library, "snd_pcm_hw_params_set_rate")
	purego.RegisterLibFunc(&api.setRateNear, library, "snd_pcm_hw_params_set_rate_near")
	purego.RegisterLibFunc(&api.params, library, "snd_pcm_hw_params")
	return api, library, nil
}

func platformDevice(format Format) (device, error) {
	api, library, err := openALSA()
	if err != nil {
		return nil, err
	}
	name := []byte("default\x00")
	var pcm uintptr
	if status := api.open(&pcm, &name[0], alsaPlayback, 0); status < 0 {
		message := alsaError(api, status)
		purego.Dlclose(library)
		return nil, fmt.Errorf("audio: open the ALSA default device: %s: %w", message, ErrUnsupported)
	}
	if err := configureALSA(api, pcm, format); err != nil {
		api.close(pcm)
		purego.Dlclose(library)
		return nil, err
	}
	return &alsaDevice{library: library, pcm: pcm, frame: format.Channels * 2, api: api}, nil
}

func configureALSA(api *alsaAPI, pcm uintptr, format Format) error {
	var params uintptr
	if status := api.paramsMalloc(&params); status < 0 {
		return fmt.Errorf("audio: allocate ALSA parameters: %s", alsaError(api, status))
	}
	defer api.paramsFree(params)
	fail := func(step string, status int32) error {
		return fmt.Errorf("audio: %s: %s", step, alsaError(api, status))
	}
	if status := api.paramsAny(pcm, params); status < 0 {
		return fail("read the ALSA device parameters", status)
	}
	if status := api.setAccess(pcm, params, alsaAccessRWInter); status < 0 {
		return fail("set interleaved access", status)
	}
	if status := api.setFormat(pcm, params, alsaFormatS16LE); status < 0 {
		return fail("set 16-bit PCM", status)
	}
	if status := api.setChannels(pcm, params, uint32(format.Channels)); status < 0 {
		return fail("set the channel count", status)
	}
	if status := api.setRate(pcm, params, uint32(format.SampleRate), 0); status < 0 {
		// A device without the plug plugin may still have a nearby rate; take
		// it only if it is the one we asked for, since this package does not
		// resample.
		rate := uint32(format.SampleRate)
		var dir int32
		if near := api.setRateNear(pcm, params, &rate, &dir); near < 0 || rate != uint32(format.SampleRate) {
			return fail("set the sample rate", status)
		}
	}
	if status := api.params(pcm, params); status < 0 {
		return fail("apply the ALSA parameters", status)
	}
	if status := api.prepare(pcm); status < 0 {
		return fail("prepare the ALSA device", status)
	}
	return nil
}

func (d *alsaDevice) write(pcm []byte) error {
	frame := d.frame
	frames := len(pcm) / frame
	offset := 0
	for offset < frames {
		written := d.api.writei(d.pcm, unsafe.Pointer(&pcm[offset*frame]), uintptr(frames-offset))
		if written > 0 {
			offset += int(written)
			continue
		}
		if written == 0 {
			// Blocking mode should not accept nothing; do not spin if it does.
			time.Sleep(time.Millisecond)
			continue
		}
		recovered := false
		for range alsaRecoverRetries {
			if status := d.api.recover(d.pcm, int32(written), 1); status < 0 {
				return fmt.Errorf("audio: play on the ALSA device: %s", alsaError(d.api, status))
			}
			written = d.api.writei(d.pcm, unsafe.Pointer(&pcm[offset*frame]), uintptr(frames-offset))
			if written >= 0 {
				offset += int(written)
				recovered = true
				break
			}
		}
		if !recovered {
			return fmt.Errorf("audio: the ALSA device stopped accepting sound: %s", alsaError(d.api, int32(written)))
		}
	}
	return nil
}

func (d *alsaDevice) close() {
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

func alsaError(api *alsaAPI, status int32) string {
	if message := api.strerror(status); message != nil {
		return goString(message)
	}
	return fmt.Sprintf("status %d", status)
}

func goString(text *byte) string {
	if text == nil {
		return ""
	}
	var length int
	for ptr := unsafe.Pointer(text); *(*byte)(unsafe.Add(ptr, length)) != 0; length++ {
	}
	return string(unsafe.Slice(text, length))
}
