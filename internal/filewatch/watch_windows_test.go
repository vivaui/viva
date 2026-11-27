package filewatch

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWindowsReplacingWatchedDirectoryAsksForResync(t *testing.T) {
	for _, tree := range []bool{false, true} {
		name := "directory"
		if tree {
			name = "tree"
		}
		t.Run(name, func(t *testing.T) {
			parent := t.TempDir()
			root := filepath.Join(parent, "workspace")
			moved := filepath.Join(t.TempDir(), "moved")
			if err := os.Mkdir(root, 0o700); err != nil {
				t.Fatal(err)
			}
			woken := make(chan struct{}, 1)
			watch, err := openWatch(root, tree, nil, func() {
				select {
				case woken <- struct{}{}:
				default:
				}
			})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(watch.Close)

			if err := os.Rename(root, moved); err != nil {
				t.Fatalf("move watched root: %v", err)
			}
			if err := os.Mkdir(root, 0o700); err != nil {
				t.Fatalf("recreate watched root: %v", err)
			}
			select {
			case <-woken:
				if p := watch.Drain(nil); !p.Resync || len(p.Names) != 0 {
					t.Fatalf("root replacement reported %+v, want only Resync", p)
				}
			case <-time.After(4 * time.Second):
				t.Fatal("root replacement did not wake the watch")
			}
			watch.Close()
			if err := os.Remove(moved); err != nil {
				t.Fatalf("remove the old root after Close: %v", err)
			}
			if err := os.RemoveAll(parent); err != nil {
				t.Fatalf("remove the watched parent after Close: %v", err)
			}
		})
	}
}
