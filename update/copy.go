package update

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// copyTree copies the regular files and directories under source into
// destination, which is created. Links, devices and other entries are refused:
// a payload is made of files a patch wrote.
func copyTree(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		switch {
		case entry.IsDir():
			return os.MkdirAll(target, info.Mode().Perm()|0o700)
		case info.Mode().IsRegular():
			return copyRegularFile(path, target, info.Mode().Perm())
		default:
			return fmt.Errorf("%w: %s is not a regular file or directory", ErrUnsafeArchive, path)
		}
	})
}

// copyRegularFile copies source to destination, which is created with mode.
func copyRegularFile(source, destination string, mode os.FileMode) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Chmod(destination, mode)
}
