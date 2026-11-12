//go:build !darwin && !windows && !linux && !(js && wasm)

package app

func nativeSchemeHandlerAvailable(string) (bool, error) { return false, ErrUnsupported }

func driverSchemeHandler(string) (available, handled bool) { return false, false }
