//go:build !(darwin || windows || (linux && !android && !vivagtk))

package app

// capabilityTransparentWindow is absent on a target that cannot composite a
// window over the desktop; see capability_transparent_native.go.
const capabilityTransparentWindow Capability = 0
