package instance

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestSecondLaunchHandsItsArgumentsToTheFirst(t *testing.T) {
	isolate(t)
	got := make(chan []string, 4)
	first, primary, err := Claim("dev.test.one", []string{"first"}, func(args []string) { got <- args })
	if err != nil || !primary {
		t.Fatalf("first claim: primary %v, err %v", primary, err)
	}
	defer first.Close()
	want := []string{"ssh://user@host:2222/", "", "ünï cödé", "with space"}
	second, primary, err := Claim("dev.test.one", want, func([]string) { t.Error("the second process must not serve") })
	if err != nil || primary || second != nil {
		t.Fatalf("second claim: primary %v, err %v", primary, err)
	}
	select {
	case args := <-got:
		if !reflect.DeepEqual(args, want) {
			t.Fatalf("handler got %q, want %q", args, want)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the arguments never arrived")
	}
}

func TestDifferentIDsDoNotMeetAndEmptyArgumentsWork(t *testing.T) {
	isolate(t)
	a, primaryA, _ := Claim("dev.test.a", nil, func([]string) {})
	b, primaryB, _ := Claim("dev.test.b", nil, func([]string) {})
	if !primaryA || !primaryB {
		t.Fatal("two applications should both be primary")
	}
	a.Close()
	b.Close()
	got := make(chan []string, 1)
	c, primary, _ := Claim("dev.test.a", nil, func(args []string) { got <- args })
	if !primary {
		t.Fatal("a closed instance left its claim behind")
	}
	defer c.Close()
	Claim("dev.test.a", nil, nil)
	if args := <-got; len(args) != 0 {
		t.Fatalf("args = %q", args)
	}
}

func TestAStaleSocketIsReplaced(t *testing.T) {
	isolate(t)
	first, primary, _ := Claim("dev.test.stale", nil, func([]string) {})
	if !primary {
		t.Fatal("not primary")
	}
	// A process that died leaves its socket file: close the listener only.
	first.listener.Close()
	<-first.done
	second, primary, err := Claim("dev.test.stale", nil, func([]string) {})
	if err != nil || !primary {
		t.Fatalf("after a crash: primary %v, err %v", primary, err)
	}
	second.Close()
}

func TestOversizeAndMalformedMessagesAreRefused(t *testing.T) {
	if err := writeArgs(&strings.Builder{}, []string{strings.Repeat("x", maxMessage)}); err == nil {
		t.Fatal("an oversize message was written")
	}
	if err := writeArgs(&strings.Builder{}, []string{"a\x00b"}); err == nil {
		t.Fatal("NUL in an argument was written")
	}
	if _, primary, err := Claim("", nil, nil); err == nil || primary {
		t.Fatal("an empty id was accepted")
	}
}

// isolate gives a test its own claim directory.
func isolate(t *testing.T) {
	t.Helper()
	dir, err := os.MkdirTemp("", "gi")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
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
		if runtime.GOOS == "windows" {
			// The fallback asks the volume for its 8.3 names, and a volume may
			// have them switched off; there is no portable shorter path to try.
			t.Skipf("the volume offers no short path: %v", err)
		}
		t.Fatalf("a deep claim directory leaves no socket path: %v", err)
	}
	if len(path) > maxSocketPath {
		t.Fatalf("socket path %q is %d bytes, over the %d-byte limit", path, len(path), maxSocketPath)
	}
	if runtime.GOOS != "windows" {
		short := filepath.Join("/tmp", fmt.Sprintf("viva-%d", os.Getuid()))
		if !strings.HasPrefix(path, short+string(os.PathSeparator)) {
			t.Fatalf("a deep directory did not fall back to %s: %q", short, path)
		}
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
		if !reflect.DeepEqual(args, []string{"handoff"}) {
			t.Fatalf("handler got %q", args)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the arguments never arrived")
	}
}
