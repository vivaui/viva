//go:build (!darwin || ios) && (!linux || android) && !windows

package autostart

func enable(Entry) error           { return ErrUnsupported }
func disable(string) error         { return ErrUnsupported }
func enabled(string) (bool, error) { return false, ErrUnsupported }
