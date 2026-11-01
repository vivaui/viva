package app

import "github.com/vivaui/viva/internal/platform"

// BackdropKind selects a native window background material.
type BackdropKind uint8

const (
	// BackdropNone disables the native material.
	BackdropNone BackdropKind = iota
	// BackdropMica requires Windows 11 22H2 or later.
	BackdropMica
	// BackdropAcrylic uses the system backdrop on Windows 11 22H2 and later.
	// Windows 10 1809 through Windows 11 21H2 use an undocumented accent policy.
	BackdropAcrylic
	// BackdropMicaAlt is the alternate Mica material for tabbed windows.
	// It requires Windows 11 22H2 or later.
	BackdropMicaAlt
)

// BackdropStyle selects a material and its light or dark appearance.
type BackdropStyle struct {
	Kind BackdropKind
	Dark bool
}

func (config Config) windowMaterials() platform.WindowMaterials {
	materials := platform.WindowMaterials{OnBackdropChange: config.OnBackdropChange}
	if config.WindowBackdrop != nil {
		backdrop := config.WindowBackdrop
		materials.WindowBackdrop = func() platform.BackdropStyle {
			style := backdrop()
			return platform.BackdropStyle{Kind: platform.BackdropKind(style.Kind), Dark: style.Dark}
		}
	}
	return materials
}
