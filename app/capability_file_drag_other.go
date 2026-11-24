//go:build !((darwin && !ios) || windows || (linux && !android && !vivagtk))

package app

// capabilityFileDrag is absent on a target that cannot take files out of a
// window; see capability_file_drag_native.go.
const capabilityFileDrag Capability = 0
