# Audio

`audio` plays PCM on the system's default output device. It exists for a
stream an application receives from elsewhere — a remote desktop's sound, a
synthesized prompt — not for the system's own beeps, which `app.PlaySystemSound`
already covers.

```go
out, err := audio.Open(audio.Format{SampleRate: 48000, Channels: 2})
if err != nil {
    // No device on this machine: carry on without sound.
}
defer out.Close()
for block := range blocks {
    out.Write(block) // interleaved signed 16-bit little-endian frames
}
```

`Output` implements `io.Writer`. `Write` blocks while the device holds enough
sound, so a producer reading from a socket needs no buffer of its own. `Close`
stops the stream and releases the device; sound the device has not taken yet is
discarded. It is safe to call more than once.

## What it is not

- It does not decode. It accepts signed 16-bit little-endian PCM, one or two
  channels.
- It has no volume control. The stream follows the system's output volume and
  mute, which is what a remote desktop wants: the machine's own controls stay
  authoritative.
- It does not exist on WebAssembly. `Open` returns `ErrUnsupported`.

## Microphone capture

`audio.OpenInput` captures the system's default input device as the same
16-bit little-endian PCM. `Input` implements `io.ReadCloser`; nothing is opened
until it is called, and `Close` joins the capture goroutine and releases the
device. If the reader stalls, the oldest quarter-second is dropped rather than
blocking the device. Targets without a backend return `ErrInputUnsupported`.

Opening the microphone is a privacy decision the operating system gates, so an
application declares it:

- **macOS** asks the user once per application. The bundle must carry
  `NSMicrophoneUsageDescription`, and a hardened-runtime build must be signed
  with `com.apple.security.device.audio-input`. Both are project settings, not
  code: put the usage string under `[mac.info]` and the entitlement in the
  property list that `[mac] entitlements` points to. Without them macOS can
  terminate the process or deliver silence, and `OpenInput` reports success
  either way, so a capture of all zeros means the permission is missing, not
  that the microphone is quiet.
- **Windows** gates desktop applications with Settings > Privacy > Microphone;
  when access is off the device delivers silence.
- **Linux** has no per-application prompt; a sandbox such as Flatpak grants
  access through its portal.

An application should say why it listens before it calls `OpenInput`, and
should not open the device before the user asks for sound.

## Backends

| Target | Backend | Format handling |
| --- | --- | --- |
| macOS | `AudioUnit` (`kAudioUnitSubType_DefaultOutput`), cgo | The unit's input scope is set to the stream's format; CoreAudio resamples and remixes to the device. |
| Windows | WASAPI shared mode, COM through `syscall`/`purego`, no cgo | The client is initialized with `AUDCLNT_STREAMFLAGS_AUTOCONVERTPCM` and the stream's format, so the audio engine converts to the device mix format. Requires Windows 7 or newer. |
| Linux | ALSA `default` PCM through `dlopen("libasound.so.2")`, `purego`, no cgo | The `plug`/sound-server path converts the format and rate. Only `amd64` and `arm64` have the purego ABI path; other architectures return `ErrUnsupported`. |

The device is opened when `Open` is called and stays open until `Close`.
While an `Output` is open it owns one goroutine (whose OS thread is fixed for
WASAPI's COM apartment), a 200 ms ring buffer, and the platform player. An
application that never imports `audio` links none of it; one that imports it
but never opens an `Output` starts no goroutine and opens no device.

`Write` copies into the ring and returns; the device drains it on the output
goroutine. The ring holds a fifth of a second, which absorbs a burst from a
network without adding a fixed delay in steady state, where the device takes
each block as it arrives.

## Verification

- `go test ./audio` covers format validation, ordering across the ring, a full
  ring blocking and `Close` unblocking it, device errors, partial frames, and
  the no-write path. The ring path is allocation-free.
- `VIVA_AUDIO_TEST=1 go test ./audio -run TestOutputDevice` opens the real
  default device on macOS and plays 200 ms of silence at 48 kHz/44.1 kHz,
  stereo and mono. Audio has no headless substitute, so this is opt-in.
- The Windows backend was run on a real Windows 11 machine from inside an RDP
  session, which gives the session a "Remote Audio" endpoint: `Open` at
  44.1 kHz stereo, a 200 ms write, and `Close` all succeeded. The same build
  outside a session, on a machine with no audio device, returned
  `ErrUnsupported` from `GetDefaultAudioEndpoint`.
- Linux was run on a Ubuntu 22.04.3 machine: the test binary opened ALSA's
  `default` PCM (PulseAudio's `auto_null` sink, 44.1 kHz stereo), wrote 200 ms
  at 48 kHz/44.1 kHz stereo and mono, and closed, all successfully. That is the
  full ALSA path through the sound server, not a physical device:

  ```sh
  GOOS=linux GOARCH=amd64 go test -c -o /tmp/audio-linux.test ./audio
  tools/testenv/upload.sh linux /tmp/audio-linux.test /home/user/audio.test
  tools/testenv/exec.sh linux 'VIVA_AUDIO_TEST=1 XDG_RUNTIME_DIR=/run/user/1000 \
    /home/user/audio.test -test.run TestOutputDevice -test.v'
  ```
- `go vet` cross-compiles the package for `windows/amd64`, `linux/amd64`,
  `darwin/arm64` and `js/wasm`.

Measured on an Apple M2 Pro, macOS 24.5.0, Go 1.26.0:

```
BenchmarkWrite-10  1179094  1070 ns/op  598.06 MB/s  0 B/op  0 allocs/op
```

Stripped binary size, `-trimpath -ldflags="-s -w"`, compared with a minimal
Viva application that imports `app` and does not import `audio` (the marginal
cost an application pays only if it uses this package):

| Target | Without `audio` | With `audio` | Delta |
| --- | ---: | ---: | ---: |
| darwin/arm64 | 2,588,594 B | 2,639,330 B | +50,736 B |
| linux/amd64 | 2,093,321 B | 2,121,993 B | +28,672 B |
| windows/amd64 | 3,065,856 B | 3,185,152 B | +119,296 B |

The Windows delta includes the WASAPI COM call path that a minimal `app`
build does not otherwise reference. A standalone program's delta is larger
(about 0.5–0.7 MB) because it then also links `purego` and the parts of
`golang.org/x/sys/windows` the backend uses; that is not the cost a Viva
application pays.

Windows playback has been exercised through a real RDP session's audio
endpoint, and Linux playback through PulseAudio's null sink. Neither has been
run on a machine with a physical output device; cross-compilation and `go vet`
cover the builds until one is.
