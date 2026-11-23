//go:build darwin && !ios

package app

func currentPlatform() Platform { return PlatformDarwin }
