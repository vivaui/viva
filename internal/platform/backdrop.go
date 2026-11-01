package platform

// BackdropKind selects the system material beneath a window's content.
type BackdropKind uint8

const (
	BackdropNone BackdropKind = iota
	BackdropMica
	BackdropAcrylic
	BackdropMicaAlt
)

// BackdropStyle is read on the window thread after a frame is painted.
type BackdropStyle struct {
	Kind BackdropKind
	Dark bool
}
