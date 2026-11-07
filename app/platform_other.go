//go:build !darwin && !windows && !linux && !(js && wasm)

package app

func currentPlatform() Platform { return PlatformUnknown }
