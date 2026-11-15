//go:build !windows

package document

import "os"

// syncDirectory makes a rename durable. A directory that cannot be opened or
// synced is not an error for the save: the rename already happened, and some
// filesystems refuse to sync a directory.
func syncDirectory(path string) {
	dir, err := os.Open(path)
	if err != nil {
		return
	}
	_ = dir.Sync()
	_ = dir.Close()
}
