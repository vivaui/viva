//go:build !android

package linux

import (
	"os"
	"path/filepath"
	"strings"
)

// wmClass is the WM_CLASS property of a window: the instance name and the class
// name, each ending in a NUL (ICCCM 4.1.2.5). A desktop environment matches the
// class against a launcher entry's StartupWMClass, which is how a taskbar finds
// the application's icon. Both names are the same one: the configured class,
// else the executable's file name. nil means there is no usable name.
func wmClass(configured string) []byte {
	name := printable(strings.TrimSpace(configured))
	if name == "" {
		executable, err := os.Executable()
		if err != nil {
			return nil
		}
		name = printable(strings.TrimSuffix(filepath.Base(executable), ".exe"))
	}
	if name == "" || len(name) > 255 {
		return nil
	}
	value := make([]byte, 0, 2*len(name)+2)
	value = append(value, name...)
	value = append(value, 0)
	value = append(value, name...)
	return append(value, 0)
}

// printable drops control characters, which cannot be part of a property name.
func printable(text string) string {
	return strings.Map(func(r rune) rune {
		if r < ' ' || r == 0x7f {
			return -1
		}
		return r
	}, text)
}
