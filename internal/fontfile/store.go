package fontfile

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"sync"
)

// fontKey names a font by its content: a 128-bit FNV-1a hash and the length. It is a name, not a
// guarantee, so a file found under it is used only when it holds exactly the same bytes.
type fontKey [17]byte

func keyOf(data []byte) fontKey {
	hash := fnv.New128a()
	hash.Write(data)
	var key fontKey
	hash.Sum(key[:0])
	key[16] = byte(len(data))
	return key
}

// Registry registers font data with a platform engine that only opens files. The data is written
// once under a name made of its digest, so the same font registered again is the same file, and
// registered twice at once it is registered with the engine once and taken away with its last
// holder. The files stay on disk: an engine may read a font lazily for as long as the process
// lives, and a font is a few hundred kilobytes at most.
type Registry struct {
	mu      sync.Mutex
	entries map[fontKey]*entry
}

type entry struct {
	path   string
	family string
	holds  int
}

// Register makes data known to the engine through add, which receives the path of a file holding
// it and registers it for this process alone. remove undoes add and runs when the last holder has
// unregistered. It returns the family the font names and the function that gives this holder's
// registration up; calling that more than once is harmless.
func (r *Registry) Register(data []byte, add func(path string) error, remove func(path string)) (family string, unregister func(), err error) {
	family, err = Family(data)
	if err != nil {
		return "", nil, err
	}
	key := keyOf(data)
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing := r.entries[key]; existing != nil {
		if staged, err := os.ReadFile(existing.path); err != nil || !bytes.Equal(staged, data) {
			return "", nil, errors.New("fontfile: a different font is registered under the same name")
		}
		existing.holds++
		return existing.family, r.release(key, remove), nil
	}
	path, err := stage(data, key)
	if err != nil {
		return "", nil, err
	}
	if err := add(path); err != nil {
		return "", nil, fmt.Errorf("fontfile: register %s: %w", family, err)
	}
	if r.entries == nil {
		r.entries = map[fontKey]*entry{}
	}
	r.entries[key] = &entry{path: path, family: family, holds: 1}
	return family, r.release(key, remove), nil
}

func (r *Registry) release(key fontKey, remove func(path string)) func() {
	var once sync.Once
	return func() {
		once.Do(func() {
			r.mu.Lock()
			defer r.mu.Unlock()
			existing := r.entries[key]
			if existing == nil {
				return
			}
			if existing.holds--; existing.holds > 0 {
				return
			}
			delete(r.entries, key)
			remove(existing.path)
		})
	}
}

// stage writes data to its file, unless the file is already there with the right size.
func stage(data []byte, key fontKey) (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil || dir == "" {
		dir = os.TempDir()
	}
	dir = filepath.Join(dir, "viva", "fonts")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(dir, hex.EncodeToString(key[:])+extension(data))
	if existing, err := os.ReadFile(path); err == nil && bytes.Equal(existing, data) {
		return path, nil
	}
	file, err := os.CreateTemp(dir, "stage-*")
	if err != nil {
		return "", err
	}
	_, werr := file.Write(data)
	cerr := file.Close()
	if err := errors.Join(werr, cerr); err != nil {
		os.Remove(file.Name())
		return "", err
	}
	if err := os.Rename(file.Name(), path); err != nil {
		os.Remove(file.Name())
		return "", err
	}
	return path, nil
}

// extension is the file suffix for the font's kind. The engines read the content, but a file
// named for it is what they and a person looking in the cache expect.
func extension(data []byte) string {
	switch {
	case len(data) >= 4 && string(data[:4]) == "ttcf":
		return ".ttc"
	case len(data) >= 4 && string(data[:4]) == "OTTO":
		return ".otf"
	}
	return ".ttf"
}
