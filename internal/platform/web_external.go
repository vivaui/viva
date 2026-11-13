package platform

import "strings"

// OpenableURL reports whether a NewWindow answer may hand address to the system's browser. It
// applies app.OpenURL's scheme policy (http, https or mailto, compared without case) at the web
// boundary: a page must not be able to launch an arbitrary protocol handler through a popup. The
// check lives here rather than importing app, which the platform packages cannot do.
func OpenableURL(address string) bool {
	lower := strings.ToLower(address)
	return strings.HasPrefix(lower, "https://") ||
		strings.HasPrefix(lower, "http://") ||
		strings.HasPrefix(lower, "mailto:")
}
