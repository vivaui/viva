package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/vivaui/viva/internal/bsdiff"
)

// PatchSchema is the only patch format this package reads.
const PatchSchema = 1

// patch limits, matching the archive limits in the feed: a patch is not a
// reason to trust a larger document or to fill the disk.
const (
	maxPatchManifest = 4 << 20
	maxPatchFile     = 512 << 20
	maxPatchTotal    = 2 << 30
)

// PatchFile is one file of a patch. Path is relative to the payload root and
// slash-separated. Op is "same" (the file is copied from the old payload),
// "diff" (the blob is a bsdiff against the old file) or "new" (the blob is the
// whole new file).
type PatchFile struct {
	Path   string `json:"path"`
	Mode   uint32 `json:"mode"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
	Op     string `json:"op"`
	Blob   string `json:"blob,omitempty"`
}

// PatchManifest describes a patch: what it applies to, from where, and every
// file the new payload holds.
type PatchManifest struct {
	Schema      int    `json:"schema"`
	App         string `json:"app"`
	FromVersion string `json:"from_version"`
	ToVersion   string `json:"to_version"`
	OS          string `json:"os"`
	Arch        string `json:"arch"`
	// Payload is "app" for a macOS bundle and "file" for a Linux executable.
	Payload string      `json:"payload"`
	Files   []PatchFile `json:"files"`
}

// Patch is an open patch file. Its manifest is read; blobs are applied by
// Apply, which streams them, so a large patch is never held in memory whole.
type PatchReader struct {
	manifest PatchManifest
	source   *tar.Reader
	closer   io.Closer
	byBlob   map[string]*PatchFile
	done     map[string]bool
}

// ReadPatch opens path and reads its manifest. The caller must Close it.
func ReadPatch(path string) (*PatchReader, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("update: opening the patch: %w", err)
	}
	patch, err := newPatch(file)
	if err != nil {
		file.Close()
		return nil, err
	}
	patch.closer = file
	return patch, nil
}

func newPatch(reader io.Reader) (*PatchReader, error) {
	gz, err := gzip.NewReader(reader)
	if err != nil {
		return nil, fmt.Errorf("%w: the patch is not a gzip stream: %v", ErrManifest, err)
	}
	tarReader := tar.NewReader(gz)
	header, err := tarReader.Next()
	if err != nil {
		return nil, fmt.Errorf("%w: the patch is empty: %v", ErrManifest, err)
	}
	if header.Name != "patch.json" || header.Typeflag != tar.TypeReg || header.Size > maxPatchManifest {
		return nil, fmt.Errorf("%w: the patch does not start with patch.json", ErrManifest)
	}
	encoded, err := io.ReadAll(io.LimitReader(tarReader, maxPatchManifest+1))
	if err != nil {
		return nil, fmt.Errorf("%w: reading patch.json: %v", ErrManifest, err)
	}
	var manifest PatchManifest
	if err := json.Unmarshal(encoded, &manifest); err != nil {
		return nil, fmt.Errorf("%w: patch.json: %v", ErrManifest, err)
	}
	if err := manifest.validate(); err != nil {
		return nil, err
	}
	byBlob := make(map[string]*PatchFile, len(manifest.Files))
	for i := range manifest.Files {
		file := &manifest.Files[i]
		if file.Blob != "" {
			byBlob[file.Blob] = file
		}
	}
	return &PatchReader{manifest: manifest, source: tarReader, closer: gz, byBlob: byBlob, done: map[string]bool{}}, nil
}

// Manifest returns the patch's description.
func (patch *PatchReader) Manifest() PatchManifest { return patch.manifest }

// Close closes the patch file.
func (patch *PatchReader) Close() error {
	if patch.closer != nil {
		return patch.closer.Close()
	}
	return nil
}

// Apply writes the new payload into staging, taking unchanged files from
// oldRoot. staging must not exist or must be empty; it is created here. The
// result is verified file by file; on error staging is removed, so a failed
// patch leaves nothing behind.
func (patch *PatchReader) Apply(oldRoot, staging string) error {
	if info, statErr := os.Stat(staging); statErr == nil && (!info.IsDir() || !emptyDir(staging)) {
		return fmt.Errorf("update: the staging directory %s is not empty", staging)
	}
	if err := os.MkdirAll(staging, 0o755); err != nil {
		return fmt.Errorf("update: creating the staging directory: %w", err)
	}
	complete := false
	defer func() {
		if !complete {
			_ = os.RemoveAll(staging)
		}
	}()

	// The files that need no blob are copied first; the tar reader waits at
	// the next entry while this happens.
	for i := range patch.manifest.Files {
		file := &patch.manifest.Files[i]
		if file.Op == "same" {
			if err := patch.copySame(oldRoot, staging, file); err != nil {
				return err
			}
		}
	}
	for {
		header, err := patch.source.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("%w: reading the patch: %v", ErrManifest, err)
		}
		if !strings.HasPrefix(header.Name, "files/") {
			return fmt.Errorf("%w: unexpected entry %q", ErrManifest, header.Name)
		}
		file, ok := patch.byBlob[header.Name]
		if !ok {
			return fmt.Errorf("%w: blob %q is not in the manifest", ErrManifest, header.Name)
		}
		if header.Typeflag != tar.TypeReg || header.Size < 0 || header.Size > maxPatchFile {
			return fmt.Errorf("%w: blob %q has an invalid size", ErrManifest, header.Name)
		}
		if patch.done[header.Name] {
			return fmt.Errorf("%w: blob %q appears twice", ErrManifest, header.Name)
		}
		if err := patch.applyBlob(oldRoot, staging, file, io.LimitReader(patch.source, header.Size)); err != nil {
			return err
		}
		patch.done[header.Name] = true
	}
	for i := range patch.manifest.Files {
		file := &patch.manifest.Files[i]
		if file.Op != "same" && !patch.done[file.Blob] {
			return fmt.Errorf("%w: %s has no blob %q", ErrManifest, file.Path, file.Blob)
		}
	}
	complete = true
	return nil
}

func (patch *PatchReader) copySame(oldRoot, staging string, file *PatchFile) error {
	source := filepath.Join(oldRoot, filepath.FromSlash(file.Path))
	info, err := os.Lstat(source)
	if err != nil {
		return fmt.Errorf("%w: reading the old %s: %v", ErrManifest, file.Path, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%w: the installed %s is not a regular file", ErrManifest, file.Path)
	}
	return copyPatchFile(source, patchDestination(staging, file.Path), file, nil)
}

func (patch *PatchReader) applyBlob(oldRoot, staging string, file *PatchFile, blob io.Reader) error {
	destination := patchDestination(staging, file.Path)
	switch file.Op {
	case "new":
		return copyPatchFile("", destination, file, blob)
	case "diff":
		encoded, err := io.ReadAll(io.LimitReader(blob, maxPatchFile))
		if err != nil {
			return fmt.Errorf("%w: reading the patch for %s: %v", ErrManifest, file.Path, err)
		}
		oldPath := filepath.Join(oldRoot, filepath.FromSlash(file.Path))
		oldData, err := readRegularFile(oldPath, file.Path)
		if err != nil {
			return err
		}
		newData, err := bsdiff.Patch(oldData, encoded)
		if err != nil {
			return fmt.Errorf("%w: %s: %v", ErrManifest, file.Path, err)
		}
		if int64(len(newData)) != file.Size {
			return fmt.Errorf("%w: %s is %d bytes, want %d", ErrChecksum, file.Path, len(newData), file.Size)
		}
		sum := sha256.Sum256(newData)
		if hex.EncodeToString(sum[:]) != file.SHA256 {
			return fmt.Errorf("%w: %s does not match its digest", ErrChecksum, file.Path)
		}
		if err := writeData(destination, newData, os.FileMode(file.Mode&0o777)); err != nil {
			return fmt.Errorf("update: writing %s: %w", file.Path, err)
		}
		return nil
	}
	return fmt.Errorf("%w: %s has unknown op %q", ErrManifest, file.Path, file.Op)
}

func readRegularFile(name, label string) ([]byte, error) {
	info, err := os.Lstat(name)
	if err != nil {
		return nil, fmt.Errorf("%w: reading the old %s: %v", ErrManifest, label, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%w: the installed %s is not a regular file", ErrManifest, label)
	}
	if info.Size() > maxPatchFile {
		return nil, fmt.Errorf("%w: the installed %s is too large", ErrManifest, label)
	}
	data, err := os.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("%w: reading the old %s: %v", ErrManifest, label, err)
	}
	return data, nil
}

func patchDestination(staging, relative string) string {
	return filepath.Join(staging, filepath.FromSlash(relative))
}

// copyPatchFile writes source (or blob when source is empty) to destination,
// hashing as it goes, and checks the result against the manifest.
func copyPatchFile(source, destination string, file *PatchFile, blob io.Reader) error {
	var reader io.Reader = blob
	var handle *os.File
	if source != "" {
		var err error
		handle, err = os.Open(source)
		if err != nil {
			return fmt.Errorf("%w: reading the old %s: %v", ErrManifest, file.Path, err)
		}
		defer handle.Close()
		reader = handle
	}
	digest := sha256.New()
	written, err := writeStream(destination, io.TeeReader(reader, digest), os.FileMode(file.Mode&0o777))
	if err != nil {
		return fmt.Errorf("update: writing %s: %w", file.Path, err)
	}
	if written != file.Size {
		return fmt.Errorf("%w: %s is %d bytes, want %d", ErrChecksum, file.Path, written, file.Size)
	}
	if hex.EncodeToString(digest.Sum(nil)) != file.SHA256 {
		return fmt.Errorf("%w: %s does not match its digest", ErrChecksum, file.Path)
	}
	return nil
}

// writeStream writes reader to a file that is named only when it is complete.
func writeStream(name string, reader io.Reader, mode os.FileMode) (int64, error) {
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		return 0, err
	}
	temporary, err := os.CreateTemp(filepath.Dir(name), "."+filepath.Base(name)+".viva-patch-*")
	if err != nil {
		return 0, err
	}
	path := temporary.Name()
	defer os.Remove(path)
	written, err := io.Copy(temporary, reader)
	if err != nil {
		temporary.Close()
		return 0, err
	}
	if err := temporary.Chmod(mode); err != nil {
		temporary.Close()
		return 0, err
	}
	if err := temporary.Close(); err != nil {
		return 0, err
	}
	if err := os.Rename(path, name); err != nil {
		return 0, err
	}
	return written, nil
}

func writeData(name string, data []byte, mode os.FileMode) error {
	_, err := writeStream(name, bytes.NewReader(data), mode)
	return err
}

func (manifest PatchManifest) validate() error {
	if manifest.Schema != PatchSchema {
		return fmt.Errorf("%w: patch schema is %d, want %d", ErrManifest, manifest.Schema, PatchSchema)
	}
	if manifest.App == "" || manifest.FromVersion == "" || manifest.ToVersion == "" {
		return fmt.Errorf("%w: the patch does not name the application and both versions", ErrManifest)
	}
	if _, err := Compare(manifest.FromVersion, manifest.FromVersion); err != nil {
		return fmt.Errorf("%w: from_version: %v", ErrManifest, err)
	}
	if _, err := Compare(manifest.ToVersion, manifest.ToVersion); err != nil {
		return fmt.Errorf("%w: to_version: %v", ErrManifest, err)
	}
	if manifest.OS != "darwin" && manifest.OS != "linux" {
		return fmt.Errorf("%w: os %q has no patch support", ErrManifest, manifest.OS)
	}
	if manifest.Arch != "amd64" && manifest.Arch != "arm64" {
		return fmt.Errorf("%w: arch %q is not supported", ErrManifest, manifest.Arch)
	}
	if manifest.Payload != "app" && manifest.Payload != "file" {
		return fmt.Errorf("%w: payload %q is not app or file", ErrManifest, manifest.Payload)
	}
	if len(manifest.Files) == 0 {
		return fmt.Errorf("%w: the patch holds no files", ErrManifest)
	}
	if len(manifest.Files) > 100000 {
		return fmt.Errorf("%w: the patch holds too many files", ErrManifest)
	}
	seen := make(map[string]bool, len(manifest.Files))
	var total int64
	for i := range manifest.Files {
		file := &manifest.Files[i]
		clean, err := patchPath(file.Path)
		if err != nil {
			return err
		}
		if seen[clean] {
			return fmt.Errorf("%w: %s appears twice", ErrManifest, clean)
		}
		seen[clean] = true
		if file.Mode&^0o777 != 0 || file.Mode == 0 {
			return fmt.Errorf("%w: %s has mode %o", ErrManifest, clean, file.Mode)
		}
		if file.Size < 0 || file.Size > maxPatchFile {
			return fmt.Errorf("%w: %s has size %d", ErrManifest, clean, file.Size)
		}
		if len(file.SHA256) != 64 || strings.ToLower(file.SHA256) != file.SHA256 {
			return fmt.Errorf("%w: %s has no lowercase SHA-256", ErrManifest, clean)
		}
		total += file.Size
		if total > maxPatchTotal {
			return fmt.Errorf("%w: the patched payload is larger than %d bytes", ErrManifest, int64(maxPatchTotal))
		}
		switch file.Op {
		case "same":
			if file.Blob != "" {
				return fmt.Errorf("%w: %s is same and has a blob", ErrManifest, clean)
			}
		case "diff", "new":
			if file.Blob == "" || !strings.HasPrefix(file.Blob, "files/") || strings.Contains(file.Blob, "..") {
				return fmt.Errorf("%w: %s has an invalid blob name", ErrManifest, clean)
			}
		default:
			return fmt.Errorf("%w: %s has op %q", ErrManifest, clean, file.Op)
		}
	}
	return nil
}

// patchPath accepts a clean relative slash path inside the payload.
func patchPath(relative string) (string, error) {
	if relative == "" || strings.HasPrefix(relative, "/") || strings.Contains(relative, "\\") ||
		strings.ContainsRune(relative, 0) || strings.Contains(relative, ":") {
		return "", fmt.Errorf("%w: %q is not a relative path", ErrManifest, relative)
	}
	clean := path.Clean(relative)
	if clean != relative || clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("%w: %q is not a clean relative path", ErrManifest, relative)
	}
	return clean, nil
}

func emptyDir(dir string) bool {
	entries, err := os.ReadDir(dir)
	return err == nil && len(entries) == 0
}

// locateInstalledPayload is installedPayload, replaceable by tests.
var locateInstalledPayload = installedPayload

// ApplyPatch applies a downloaded patch to the running application and returns
// a directory holding the new payload, ready for InstallOptions.Payload. app
// may be empty to skip the identity check; the versions, the operating system
// and the architecture the patch names must match.
//
// The installation is not touched: a staging directory is made, and the caller
// removes it when Install fails or is not called.
func ApplyPatch(patchPath, app, fromVersion, toVersion string) (string, error) {
	patch, err := ReadPatch(patchPath)
	if err != nil {
		return "", err
	}
	defer patch.Close()
	manifest := patch.Manifest()
	if app != "" && manifest.App != app {
		return "", fmt.Errorf("%w: the patch is for %q, not %q", ErrManifest, manifest.App, app)
	}
	if manifest.FromVersion != fromVersion {
		return "", fmt.Errorf("%w: the patch updates from %s, not %s", ErrManifest, manifest.FromVersion, fromVersion)
	}
	if manifest.ToVersion != toVersion {
		return "", fmt.Errorf("%w: the patch updates to %s, not %s", ErrManifest, manifest.ToVersion, toVersion)
	}
	if manifest.OS != runtime.GOOS || manifest.Arch != runtime.GOARCH {
		return "", fmt.Errorf("%w: the patch is for %s/%s, not %s/%s", ErrManifest, manifest.OS, manifest.Arch, runtime.GOOS, runtime.GOARCH)
	}
	root, kind, err := locateInstalledPayload()
	if err != nil {
		return "", err
	}
	if kind != manifest.Payload {
		return "", fmt.Errorf("%w: the patch carries a %s payload, but this installation is a %s", ErrManifest, manifest.Payload, kind)
	}
	staging, err := os.MkdirTemp("", "viva-payload-")
	if err != nil {
		return "", fmt.Errorf("update: creating the staging directory: %w", err)
	}
	if err := patch.Apply(root, staging); err != nil {
		_ = os.RemoveAll(staging)
		return "", err
	}
	return staging, nil
}
