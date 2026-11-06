//go:build !windows

package app

func nativeRegisterGlobalShortcut(GlobalShortcut, func()) (func(), error) { return nil, ErrUnsupported }
