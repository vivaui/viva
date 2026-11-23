//go:build (!darwin || ios) && !windows && !linux

package audio

// platformDevice is the build targets Viva does not give an audio backend:
// WebAssembly, and anything else Go can target. The package still compiles so
// that an application shared with a supported target does not need its own
// build tags.
func platformDevice(Format) (device, error) { return nil, ErrUnsupported }
