//go:build linux && !amd64 && !arm64

package audio

// platformDevice on Linux architectures without the purego ABI support the
// rest of Viva uses.
func platformDevice(Format) (device, error) { return nil, ErrUnsupported }
