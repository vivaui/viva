package app

import "github.com/vivaui/viva/internal/platform"

// BackdropKind selects a native window background material.
type BackdropKind = platform.BackdropKind

const (
	BackdropNone    = platform.BackdropNone
	BackdropMica    = platform.BackdropMica
	BackdropAcrylic = platform.BackdropAcrylic
	// BackdropMicaAlt is the alternate Mica material for tabbed windows.
	// It requires Windows 11 22H2 or later.
	BackdropMicaAlt = platform.BackdropMicaAlt
)

// BackdropStyle selects a material and its light or dark appearance.
type BackdropStyle = platform.BackdropStyle
