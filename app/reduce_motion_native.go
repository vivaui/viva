//go:build !js

package app

func platformReduceMotion() bool { return nativeReduceMotion() }
