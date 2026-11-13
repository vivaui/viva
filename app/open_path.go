package app

import (
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ErrExecutablePath is returned by OpenPath for a file the system would run
// instead of showing.
var ErrExecutablePath = errors.New("app: the file would be run, not opened")

// OpenPath opens a local file in the application the system has chosen for its
// type, as a double click in the file manager does. It is for documents; a file
// that the system would execute (a program, a script host's file, an installer,
// a shortcut, a bundle) is refused with ErrExecutablePath, because a path that
// came from somewhere else must never become a way to run it. The path must be
// absolute and name a regular file, not a link or a directory. Driver-backed
// runs record the request without asking the system to open anything.
func OpenPath(path string) error {
	if strings.IndexByte(path, 0) >= 0 {
		return fmt.Errorf("app: path contains NUL")
	}
	if !filepath.IsAbs(path) {
		return fmt.Errorf("app: %q is not an absolute path", path)
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if err := checkOpenPath(runtime.GOOS, path, info.Mode()); err != nil {
		return err
	}
	if err, handled := driverOpenPath(path); handled {
		return err
	}
	return nativeOpenURL(openPathTarget(runtime.GOOS, path))
}

// OpenDirectoryPath asks the user to choose one existing local directory and
// returns its path through the same integration as OpenDirectory. Cancellation
// is ErrCanceled and the Web target returns ErrUnsupported.
func OpenDirectoryPath(options OpenDirectoryOptions) (string, error) {
	return OpenDirectory(options)
}

// openPathTarget is what the platform's opener is given: a file URL where the
// opener takes URLs (macOS), the path itself elsewhere.
func openPathTarget(goos, path string) string {
	if goos == "darwin" {
		return (&url.URL{Scheme: "file", Path: path}).String()
	}
	return path
}

// executableExtensions lists, per system, the file types it runs or installs
// when opened. The names are lower case with the dot.
var executableExtensions = map[string][]string{
	"windows": {
		".exe", ".com", ".bat", ".cmd", ".scr", ".pif", ".lnk", ".url", ".msi", ".msp", ".mst", ".ps1", ".psm1", ".psd1", ".ps1xml",
		".vbs", ".vbe", ".js", ".jse", ".wsf", ".wsh", ".hta", ".cpl", ".reg", ".jar", ".inf", ".gadget", ".application", ".appref-ms",
		".library-ms", ".search-ms", ".settingcontent-ms", ".diagcab", ".chm", ".msc", ".sct", ".appx", ".msix", ".appxbundle", ".msixbundle",
	},
	"darwin": {
		".app", ".command", ".tool", ".terminal", ".workflow", ".action", ".scpt", ".scptd", ".pkg", ".mpkg", ".dmg", ".webloc", ".inetloc",
		".fileloc", ".service", ".prefpane", ".saver", ".wflow", ".jar",
	},
	"linux": {
		".desktop", ".run", ".appimage", ".bin", ".jar", ".deb", ".rpm", ".flatpakref", ".flatpak", ".snap",
	},
}

// checkOpenPath refuses what must not be opened: anything but a regular file,
// a file with an execute bit on systems that have one, and the types in
// executableExtensions.
func checkOpenPath(goos, path string, mode fs.FileMode) error {
	if !mode.IsRegular() {
		return fmt.Errorf("app: %q is not a regular file", path)
	}
	if goos != "windows" && mode.Perm()&0o111 != 0 {
		return fmt.Errorf("%w: %s is executable", ErrExecutablePath, filepath.Base(path))
	}
	name := strings.ToLower(filepath.Base(path))
	for _, extension := range executableExtensions[goos] {
		if strings.HasSuffix(name, extension) {
			return fmt.Errorf("%w: %s", ErrExecutablePath, filepath.Base(path))
		}
	}
	return nil
}
