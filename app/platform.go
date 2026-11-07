package app

// Platform identifies the runtime Viva is running on.
type Platform uint8

const (
	PlatformUnknown Platform = iota
	PlatformDarwin
	PlatformWindows
	PlatformLinux
	PlatformWeb
)

func (platform Platform) String() string {
	switch platform {
	case PlatformDarwin:
		return "darwin"
	case PlatformWindows:
		return "windows"
	case PlatformLinux:
		return "linux"
	case PlatformWeb:
		return "web"
	default:
		return "unknown"
	}
}

// CurrentPlatform reports the platform this program was built for.
func CurrentPlatform() Platform { return currentPlatform() }
