//go:build darwin

package app

import (
	"reflect"
	"testing"
)

func TestOpenWithCommandUsesOpenForApplicationBundles(t *testing.T) {
	command := openWithCommand("/Applications/Code.app", "/tmp/server config.yml")
	want := []string{"/usr/bin/open", "-a", "/Applications/Code.app", "/tmp/server config.yml"}
	if !reflect.DeepEqual(command.Args, want) {
		t.Fatalf("application bundle args = %q, want %q", command.Args, want)
	}

	command = openWithCommand("/usr/local/bin/editor", "/tmp/server config.yml")
	want = []string{"/usr/local/bin/editor", "/tmp/server config.yml"}
	if !reflect.DeepEqual(command.Args, want) {
		t.Fatalf("executable args = %q, want %q", command.Args, want)
	}
}
