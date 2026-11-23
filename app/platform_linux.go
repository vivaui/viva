//go:build linux && !android

package app

func currentPlatform() Platform { return PlatformLinux }
