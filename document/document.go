// Package document reads and writes one file as a document: the bytes on disk,
// a revision that names them, and an atomic save that refuses to replace a file
// another program changed since it was read.
//
// It is the storage half of an editor, not an editor: the application owns the
// file format, the undo history and the autosave policy. A revision is the
// SHA-256 of the bytes, so a caller can compare what it read with what is on
// disk without keeping the whole file in memory.
package document

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

var (
	// ErrChanged means the file is not the one the caller read: another program
	// wrote, replaced or removed it. The caller decides whether to reload,
	// overwrite, or ask the user.
	ErrChanged = errors.New("document: the file changed since it was read")
	// ErrNotRegular means the path is not a regular file: a directory, a device,
	// or a symbolic link. A document follows none of them.
	ErrNotRegular = errors.New("document: not a regular file")
)

// Revision names one version of a file's bytes: the SHA-256 of the content.
type Revision [sha256.Size]byte

// Read returns the file's bytes and their revision. A symbolic link or a path
// that is not a regular file is ErrNotRegular.
func Read(path string) ([]byte, Revision, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, Revision{}, err
	}
	if !info.Mode().IsRegular() {
		return nil, Revision{}, fmt.Errorf("%w: %s", ErrNotRegular, path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, Revision{}, err
	}
	return data, Revision(sha256.Sum256(data)), nil
}

// Save writes data to path atomically, refusing when the file on disk is not
// expected. The bytes go to a temporary file in the same directory, are synced,
// then renamed over the target: a crash leaves either the old file or the new
// one, never a half-written one. The mode of the existing file is kept; a new
// file (expected is the zero Revision) is created 0644.
//
// A file that is missing returns ErrChanged unless expected is zero. A symbolic
// link or another non-regular file is ErrNotRegular. The returned Revision is
// the one the saved bytes have, for the next Save or Read.
func Save(path string, data []byte, expected Revision) (Revision, error) {
	info, err := os.Lstat(path)
	mode := fs.FileMode(0o644)
	switch {
	case err == nil:
		if !info.Mode().IsRegular() {
			return Revision{}, fmt.Errorf("%w: %s", ErrNotRegular, path)
		}
		mode = info.Mode().Perm()
	case errors.Is(err, os.ErrNotExist):
		if expected != (Revision{}) {
			return Revision{}, ErrChanged
		}
	default:
		return Revision{}, err
	}
	if err == nil {
		current, err := os.ReadFile(path)
		if err != nil {
			return Revision{}, err
		}
		if Revision(sha256.Sum256(current)) != expected {
			return Revision{}, ErrChanged
		}
	}
	revision := Revision(sha256.Sum256(data))
	temporary, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return Revision{}, err
	}
	name := temporary.Name()
	cleanup := func() { _ = os.Remove(name) }
	if err := temporary.Chmod(mode); err != nil {
		temporary.Close()
		cleanup()
		return Revision{}, err
	}
	_, err = temporary.Write(data)
	if err == nil {
		err = temporary.Sync()
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		cleanup()
		return Revision{}, err
	}
	// The file may have changed while the temporary was written: look again
	// before replacing it, as an editor does before its final rename.
	current, err := os.ReadFile(path)
	if err == nil && info == nil {
		// A file appeared where there was none: it is not the empty document
		// the caller meant to create.
		cleanup()
		return Revision{}, ErrChanged
	}
	if err == nil && info != nil {
		if Revision(sha256.Sum256(current)) != expected {
			cleanup()
			return Revision{}, ErrChanged
		}
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		cleanup()
		return Revision{}, err
	} else if err != nil && expected != (Revision{}) {
		cleanup()
		return Revision{}, ErrChanged
	}
	if err := os.Rename(name, path); err != nil {
		cleanup()
		return Revision{}, err
	}
	syncDirectory(filepath.Dir(path))
	return revision, nil
}

// Changed reports whether the file on disk is not revision. A missing file is
// changed; the error says why it could not be read.
func Changed(path string, revision Revision) (bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return Revision(sha256.Sum256(data)) != revision, nil
}
