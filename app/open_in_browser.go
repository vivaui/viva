package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/vivaui/viva/internal/platform"
)

// ErrNoBrowser is returned by OpenPathInBrowser when the system has no default
// web browser to give the file to.
var ErrNoBrowser = errors.New("app: no default web browser")

// OpenPathInBrowser opens a local file in the user's default web browser, the
// application the system opens http and https links with, whatever application
// the file's own type is associated with. It is for pages an application wrote
// or previews: an .html file is often associated with an editor, where OpenPath
// would open it. The browser is given the file's URL, so it shows the file and
// does not run it. The path must be absolute and name a regular file, not a
// link or a directory. A system with no default browser returns ErrNoBrowser;
// one where a browser cannot be started, as on the Web target, ErrUnsupported.
// Driver-backed runs record the request without starting a browser. A native
// call can wait for the browser to take the file, so callers on an interactive
// window thread should run it away from that thread.
func OpenPathInBrowser(path string) error {
	if strings.IndexByte(path, 0) >= 0 {
		return fmt.Errorf("app: path contains NUL")
	}
	if !filepath.IsAbs(path) {
		return fmt.Errorf("app: %q is not an absolute path", path)
	}
	path = filepath.Clean(path)
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("app: %q is not a regular file", path)
	}
	if err, handled := driverOpenInBrowser(path); handled {
		return err
	}
	if err := nativeOpenInBrowser(path); err != nil {
		if errors.Is(err, platform.ErrNoBrowser) {
			return ErrNoBrowser
		}
		return err
	}
	return nil
}
