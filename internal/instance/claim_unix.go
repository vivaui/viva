//go:build darwin || linux

package instance

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
)

// maxSocketPath is below the 104 bytes a Unix socket path may have on macOS.
const maxSocketPath = 100

// claimDirectory is the directory a claim lives in. It is a variable so tests
// can use a directory of their own; the program never changes it.
var claimDirectory = defaultClaimDirectory

// claimAddress is where the id's socket lives: in a directory only this user can
// enter, short enough for the 104-byte limit of a Unix socket path.
func claimAddress(id string) (string, error) {
	dir, err := claimDirectory()
	if err != nil {
		return "", err
	}
	return socketPathIn(dir, id)
}

// defaultClaimDirectory is the per-user directory claims live in. It is derived
// from the user, not from the environment: a copy started from a desktop and one
// started from a terminal must find each other, and their TMPDIR and
// XDG_RUNTIME_DIR can differ (an SSH session, a service or a shell profile can
// leave the first unset or set it to another place). If the claim followed the
// environment, both copies would bind a socket and both would believe they are
// the one running copy, writing the same saved data from two processes.
func defaultClaimDirectory() (string, error) {
	if runtime.GOOS == "linux" {
		// The session runtime directory, derived from the user rather than read
		// from XDG_RUNTIME_DIR, so a launch without the variable agrees with the
		// desktop's own.
		session := fmt.Sprintf("/run/user/%d", os.Getuid())
		if info, err := os.Stat(session); err == nil && info.IsDir() && ownedByUser(info) {
			return filepath.Join(session, "viva"), nil
		}
		return filepath.Join("/tmp", fmt.Sprintf("viva-%d", os.Getuid())), nil
	}
	// ~/Library/Application Support on macOS. The cache directory can be purged
	// under disk pressure, which would remove the socket of a running copy and
	// let a second one claim the application.
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "viva"), nil
}

// socketPathIn is claimAddress for a claim directory. The split lets a test
// reach the length fallback without a machine whose home directory is that deep.
func socketPathIn(dir, id string) (string, error) {
	name := claimName(id) + ".sock"
	if len(dir)+len(name)+1 > maxSocketPath {
		// A very long home directory: /tmp is always short, and every copy
		// derives the same fallback from the user, not from the environment.
		dir = filepath.Join("/tmp", fmt.Sprintf("viva-%d", os.Getuid()))
	}
	socket := filepath.Join(dir, name)
	if len(socket) > maxSocketPath {
		return "", fmt.Errorf("instance: the claim path %s is %d bytes, over the %d-byte limit; the user's home directory is too deep", socket, len(socket), maxSocketPath)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("instance: %s is not a directory", dir)
	}
	if info.Mode().Perm()&0o077 != 0 {
		if err := os.Chmod(dir, 0o700); err != nil {
			return "", fmt.Errorf("instance: %s is open to other users: %w", dir, err)
		}
	}
	if !ownedByUser(info) {
		return "", fmt.Errorf("instance: %s belongs to another user", dir)
	}
	return socket, nil
}

// listen takes the address. It fails while another copy's socket is there, which
// is what makes the first process the one that runs.
func listen(address string) (net.Listener, error) {
	listener, err := net.Listen("unix", address)
	if err != nil {
		return nil, err
	}
	os.Chmod(address, 0o600)
	return listener, nil
}

// send hands args to the copy listening on the socket. A socket that nobody
// listens on is the stale file of a process that died, and the caller replaces
// it after errNobody.
func send(address string, args []string) error {
	conn, err := net.DialTimeout("unix", address, ioDeadline)
	if err != nil {
		return fmt.Errorf("%w: %v", errNobody, err)
	}
	return sendArgs(conn, args)
}

// discard removes a socket nobody listens on.
func discard(address string) { os.Remove(address) }
