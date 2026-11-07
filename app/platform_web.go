//go:build js && wasm

package app

func currentPlatform() Platform { return PlatformWeb }
