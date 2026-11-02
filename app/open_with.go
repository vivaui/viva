package app

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode"
)

// OpenWith starts executable with one absolute path as its only argument. It
// does not use a shell. The executable and the path must already exist and be
// absolute; path may name a file or a directory, because editors commonly
// accept either. A directly started program is reaped in the background after
// it exits; on macOS an application bundle launches through the system's open,
// and a failed handoff is this call's error.
//
// Driver-backed runs record the request and start nothing.
func OpenWith(executable, path string) error {
	executable, err := validateOpenWithTarget("executable", executable)
	if err != nil {
		return err
	}
	path, err = validateOpenWithTarget("path", path)
	if err != nil {
		return err
	}
	info, err := os.Stat(executable)
	if err != nil {
		return fmt.Errorf("app: inspect executable: %w", err)
	}
	if err := checkOpenWithExecutable(runtime.GOOS, executable, info.Mode()); err != nil {
		return err
	}
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("app: inspect path: %w", err)
	}
	if err, handled := driverOpenWith(executable, path); handled {
		return err
	}
	return nativeOpenWith(executable, path)
}

func checkOpenWithExecutable(goos, path string, mode os.FileMode) error {
	if goos == "darwin" && mode.IsDir() && strings.EqualFold(filepath.Ext(path), ".app") {
		return nil
	}
	if !mode.IsRegular() {
		return fmt.Errorf("app: %q is not an executable file", path)
	}
	switch goos {
	case "windows":
		switch strings.ToLower(filepath.Ext(path)) {
		case ".exe", ".com":
			return nil
		default:
			return fmt.Errorf("app: %q is not a Windows executable", path)
		}
	default:
		if mode.Perm()&0o111 == 0 {
			return fmt.Errorf("app: %q is not executable", path)
		}
	}
	return nil
}

func validateOpenWithTarget(kind, value string) (string, error) {
	if value == "" {
		return "", fmt.Errorf("app: %s is empty", kind)
	}
	if strings.IndexByte(value, 0) >= 0 {
		return "", fmt.Errorf("app: %s contains NUL", kind)
	}
	for _, char := range value {
		if char < ' ' || char == '\x7f' || unicode.IsControl(char) {
			return "", fmt.Errorf("app: %s contains a control character", kind)
		}
	}
	if !filepath.IsAbs(value) {
		return "", fmt.Errorf("app: %q is not an absolute %s", value, kind)
	}
	return filepath.Clean(value), nil
}
