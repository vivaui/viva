//go:build darwin || linux

package instance

import (
	"os"
	"syscall"
)

func ownedByUser(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && int(stat.Uid) == os.Getuid()
}
