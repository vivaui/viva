//go:build ios

package app

// What a phone has no use for, and Android answers with its common fallbacks:
// an iPhone has no file icons for the application to ask for, and no reopening.

func nativeFileIcon(string, int) ([]byte, error) { return nil, ErrUnsupported }
func setReopenHandler(*Config, *Window)          {}
