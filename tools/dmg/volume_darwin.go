//go:build darwin

package dmg

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// Describe reads what the alias of name, a file at the top of the volume
// mounted at mount, needs from the mounted volume.
func Describe(mount, volume, name string) (File, error) {
	root, err := os.Stat(mount)
	if err != nil {
		return File{}, err
	}
	file, err := os.Stat(filepath.Join(mount, name))
	if err != nil {
		return File{}, err
	}
	rootStat, ok1 := root.Sys().(*syscall.Stat_t)
	fileStat, ok2 := file.Sys().(*syscall.Stat_t)
	if !ok1 || !ok2 {
		return File{}, fmt.Errorf("dmg: no file system details for %s", mount)
	}
	return File{
		Volume: volume, VolumeCreated: time.Unix(rootStat.Birthtimespec.Sec, 0).UTC(),
		Name: name, Created: time.Unix(fileStat.Birthtimespec.Sec, 0).UTC(),
		ID: uint32(fileStat.Ino), ParentID: uint32(rootStat.Ino),
	}, nil
}

func volumeMounted(mount string) (bool, error) {
	root, err := os.Stat(mount)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	parent, err := os.Stat(filepath.Dir(mount))
	if err != nil {
		return false, err
	}
	rootStat, rootOK := root.Sys().(*syscall.Stat_t)
	parentStat, parentOK := parent.Sys().(*syscall.Stat_t)
	if !rootOK || !parentOK {
		return false, fmt.Errorf("dmg: no file system details for %s", mount)
	}
	return rootStat.Dev != parentStat.Dev, nil
}
