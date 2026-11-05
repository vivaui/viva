//go:build !darwin && !windows && !linux

package update

// install has nothing to replace this application with on a target without a
// native executable.
func install(InstallOptions, func()) error {
	return ErrUnsupported
}
