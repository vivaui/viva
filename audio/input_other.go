//go:build (!darwin || ios) && !windows && !linux

package audio

func platformInputDevice(Format) (inputDevice, error) { return nil, ErrInputUnsupported }
