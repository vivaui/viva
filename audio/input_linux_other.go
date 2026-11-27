//go:build linux && !amd64 && !arm64 && !android

package audio

func platformInputDevice(Format) (inputDevice, error) { return nil, ErrInputUnsupported }
