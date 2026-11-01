//go:build !windows

package instance

// shortPath is a Windows-only fallback; other systems report no short form.
func shortPath(string) (string, bool) { return "", false }
