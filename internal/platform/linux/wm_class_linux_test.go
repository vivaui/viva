//go:build !android

package linux

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWMClassIsTheInstanceAndTheClassEachEndingInNUL(t *testing.T) {
	if got := wmClass("my-app"); !bytes.Equal(got, []byte("my-app\x00my-app\x00")) {
		t.Fatalf("wmClass = %q", got)
	}
}

func TestWMClassDefaultsToTheExecutableName(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Skip(err)
	}
	name := strings.TrimSuffix(filepath.Base(executable), ".exe")
	if got := wmClass(""); !bytes.Equal(got, []byte(name+"\x00"+name+"\x00")) {
		t.Fatalf("wmClass(\"\") = %q, want the executable %q", got, name)
	}
	if got := wmClass("   "); !bytes.Equal(got, []byte(name+"\x00"+name+"\x00")) {
		t.Fatalf("a blank class = %q", got)
	}
}

func TestWMClassDropsControlCharactersAndRefusesWhatIsLeftEmptyOrTooLong(t *testing.T) {
	if got := wmClass("a\x00b\nc"); !bytes.Equal(got, []byte("abc\x00abc\x00")) {
		t.Fatalf("control characters kept: %q", got)
	}
	if got := wmClass("\x01\x02"); got == nil {
		t.Fatal("a name of only control characters fell back to nothing instead of the executable's")
	}
	if got := wmClass(strings.Repeat("x", 256)); got != nil {
		t.Fatalf("an over-long name was accepted: %d bytes", len(got))
	}
}
