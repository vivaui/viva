//go:build windows

package document

// syncDirectory does nothing: Windows has no directory handle to sync, and the
// rename is durable once it returns.
func syncDirectory(string) {}
