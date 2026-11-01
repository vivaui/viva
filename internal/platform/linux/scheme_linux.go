//go:build linux

package linux

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// schemeLookupTimeout bounds the xdg-mime process; it normally answers at once.
const schemeLookupTimeout = 5 * time.Second

// SchemeHandlerAvailable reports whether an application is registered for a URL
// scheme such as "vscode": the desktop entry xdg-mime names for the scheme's
// x-scheme-handler MIME type. Without xdg-mime the answer is unknown, reported as
// ErrUnsupported.
func SchemeHandlerAvailable(scheme string) (bool, error) {
	path, err := exec.LookPath("xdg-mime")
	if err != nil {
		return false, unsupported("looking up a URL scheme handler requires xdg-mime")
	}
	ctx, cancel := context.WithTimeout(context.Background(), schemeLookupTimeout)
	defer cancel()
	output, err := exec.CommandContext(ctx, path, "query", "default", "x-scheme-handler/"+scheme).Output()
	if err != nil {
		return false, fmt.Errorf("linux: look up URL scheme handler with xdg-mime: %w", err)
	}
	return strings.TrimSpace(string(output)) != "", nil
}
