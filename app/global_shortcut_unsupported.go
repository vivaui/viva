//go:build !windows && !linux && !darwin

package app

func nativeRegisterGlobalShortcut(GlobalShortcut, func()) (func(), error) { return nil, ErrUnsupported }
