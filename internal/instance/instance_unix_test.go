//go:build darwin || linux

package instance

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// isolate gives a test its own claim directory.
func isolate(t testing.TB) {
	t.Helper()
	dir := t.TempDir()
	previous := claimDirectory
	claimDirectory = func() (string, error) { return dir, nil }
	t.Cleanup(func() { claimDirectory = previous })
}

// A copy started from a desktop and one started from a terminal can carry
// different temporary directories. The claim must not move with the environment:
// if it did, each copy would bind its own socket, both would become primary, and
// both would hold the same saved data.
func TestTheClaimDirectoryDoesNotFollowTheEnvironment(t *testing.T) {
	before, err := defaultClaimDirectory()
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(before) {
		t.Fatalf("the claim directory %q is not absolute", before)
	}
	other := filepath.Join(t.TempDir(), strings.Repeat("deep-directory-", 6))
	t.Setenv("XDG_RUNTIME_DIR", other)
	t.Setenv("TMPDIR", other)
	t.Setenv("TEMP", other)
	t.Setenv("TMP", other)
	after, err := defaultClaimDirectory()
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatalf("the claim directory moved from %q to %q with the environment", before, after)
	}
}

// A claim directory too deep for a socket path must still claim: the path falls
// back to a short one instead of failing to bind, and both launches agree on it.
func TestALongClaimDirectoryKeepsTheSocketPathShort(t *testing.T) {
	base := t.TempDir()
	deep := filepath.Join(base, strings.Repeat("deep-directory-", 6), strings.Repeat("x", 40))
	if err := os.MkdirAll(deep, 0o700); err != nil {
		t.Fatal(err)
	}
	if len(deep) <= maxSocketPath {
		t.Fatalf("the test needs a directory over %d bytes, got %d", maxSocketPath, len(deep))
	}

	path, err := socketPathIn(deep, "dev.test.deep")
	if err != nil {
		t.Fatalf("a deep claim directory leaves no socket path: %v", err)
	}
	if len(path) > maxSocketPath {
		t.Fatalf("socket path %q is %d bytes, over the %d-byte limit", path, len(path), maxSocketPath)
	}
	short := filepath.Join("/tmp", fmt.Sprintf("viva-%d", os.Getuid()))
	if !strings.HasPrefix(path, short+string(os.PathSeparator)) {
		t.Fatalf("a deep directory did not fall back to %s: %q", short, path)
	}
	if again, err := socketPathIn(deep, "dev.test.deep"); err != nil || again != path {
		t.Fatalf("the socket path is not stable: %q then %q (%v)", path, again, err)
	}

	// Claim goes through the same fallback: the directory the platform would
	// give is replaced by the deep one, so both launches below must still agree.
	previous := claimDirectory
	claimDirectory = func() (string, error) { return deep, nil }
	t.Cleanup(func() { claimDirectory = previous })

	got := make(chan []string, 1)
	first, primary, err := Claim("dev.test.deep", nil, func(args []string) { got <- args })
	if err != nil || !primary {
		t.Fatalf("claim with a deep directory: primary %v, err %v", primary, err)
	}
	defer first.Close()
	if second, primary, err := Claim("dev.test.deep", []string{"handoff"}, func([]string) {
		t.Error("the second process must not serve")
	}); err != nil || primary || second != nil {
		t.Fatalf("second claim: primary %v, err %v", primary, err)
	}
	select {
	case args := <-got:
		if len(args) != 1 || args[0] != "handoff" {
			t.Fatalf("handler got %q", args)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the arguments never arrived")
	}
}
