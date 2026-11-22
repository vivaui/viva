package window

// Appearance is the look the system is set to: light or dark.
type Appearance uint8

const (
	// AppearanceUnknown means the system does not say: a desktop without a
	// preference, or a platform that cannot be asked.
	AppearanceUnknown Appearance = iota
	AppearanceLight
	AppearanceDark
)

func (appearance Appearance) String() string {
	switch appearance {
	case AppearanceLight:
		return "light"
	case AppearanceDark:
		return "dark"
	}
	return "unknown"
}
