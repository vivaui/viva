//go:build js && wasm

package app

func nativeSchemeHandlerAvailable(string) (bool, error) { return false, ErrUnsupported }
