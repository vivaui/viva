//go:build linux && vivagtk

package linux

import (
	"os"
	"path/filepath"
	"testing"
)

// A partition's store goes under the user's data and cache directories, in a folder of the
// application's own and then of the partition's: two applications that name a partition alike keep
// two stores, and two partitions of one application keep two more.
func TestAPartitionKeepsItsStoreInTheUsersDirectories(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_CACHE_HOME", "")
	data, cache, err := partitionDirectories("app", "workspace-1")
	if err != nil {
		t.Fatal(err)
	}
	wantData := filepath.Join(home, ".local", "share", "app", "webkit", "workspace-1")
	wantCache := filepath.Join(home, ".cache", "app", "webkit", "workspace-1")
	if data != wantData || cache != wantCache {
		t.Fatalf("partition store = %q, %q, want %q, %q", data, cache, wantData, wantCache)
	}
	for _, directory := range []string{data, cache} {
		if info, err := os.Stat(directory); err != nil || !info.IsDir() {
			t.Fatalf("%q was not made: %v", directory, err)
		}
	}
	if other, _, err := partitionDirectories("app", "workspace-2"); err != nil || other == data {
		t.Fatalf("two partitions share %q: %v", other, err)
	}
}

func TestAPartitionFollowsTheXdgDirectories(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	data, cache, err := partitionDirectories("app", "workspace-1")
	if err != nil {
		t.Fatal(err)
	}
	if data != filepath.Join(root, "data", "app", "webkit", "workspace-1") ||
		cache != filepath.Join(root, "cache", "app", "webkit", "workspace-1") {
		t.Fatalf("partition store = %q, %q", data, cache)
	}
}

// Without a name to put in the path the application's own executable names its folder, so that two
// applications do not share a partition by accident.
func TestAPartitionIsNamedAfterTheApplicationWithoutAClass(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_CACHE_HOME", "")
	data, _, err := partitionDirectories("", "workspace-1")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(filepath.Dir(filepath.Dir(data))) == "" || filepath.Base(filepath.Dir(filepath.Dir(data))) == "webkit" {
		t.Fatalf("partition %q carries no application folder of its own", data)
	}
	if filepath.Base(data) != "workspace-1" {
		t.Fatalf("partition %q does not end in its name", data)
	}
}
