// Package hostsrc holds the Java sources of the Android host: the Activity and the
// SurfaceView that give the Go program a window, and what it asks of the system's
// text drawing. viva build compiles them with the application, so the host needs
// no Gradle project.
package hostsrc

import "embed"

// Java is the host's source tree, rooted at java/.
//
//go:embed java
var Java embed.FS
