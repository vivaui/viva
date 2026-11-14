// Package patchgen writes a delta update between two released payloads: the
// installed .app bundle on macOS, or the executable a Linux tar.gz installs.
//
// It is a release tool: it imports the bsdiff encoder, which needs a suffix
// array and the whole payload in memory. The update package, which runs in the
// user's application, only applies patches.
package patchgen

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gabstv/go-bsdiff/pkg/bsdiff"
	"github.com/vivaui/viva/update"
)

// Request is one patch to write.
type Request struct {
	App         string
	FromVersion string
	ToVersion   string
	OS          string
	Arch        string
	// OldArchive and NewArchive are the released archives: the previous
	// version and the one the feed lists.
	OldArchive string
	NewArchive string
	// Executable is the program's file name in a Linux archive. Empty finds
	// the only executable in it.
	Executable string
	// Output receives the gzip tar patch.
	Output io.Writer
}

// Write makes the patch and returns its manifest.
func Write(request Request) (update.PatchManifest, error) {
	if request.App == "" || request.FromVersion == "" || request.ToVersion == "" {
		return update.PatchManifest{}, fmt.Errorf("patchgen: the application and both versions are required")
	}
	if request.OS != "darwin" && request.OS != "linux" {
		return update.PatchManifest{}, fmt.Errorf("patchgen: patches are made for darwin and linux, not %q", request.OS)
	}
	if request.Arch != "amd64" && request.Arch != "arm64" {
		return update.PatchManifest{}, fmt.Errorf("patchgen: architecture %q must be amd64 or arm64", request.Arch)
	}
	oldPayload, oldKind, cleanupOld, err := extractPayload(request.OldArchive, request.OS, request.Arch, request.Executable)
	if err != nil {
		return update.PatchManifest{}, fmt.Errorf("patchgen: old release: %w", err)
	}
	defer cleanupOld()
	newPayload, newKind, cleanupNew, err := extractPayload(request.NewArchive, request.OS, request.Arch, request.Executable)
	if err != nil {
		return update.PatchManifest{}, fmt.Errorf("patchgen: new release: %w", err)
	}
	defer cleanupNew()
	if oldKind != newKind {
		return update.PatchManifest{}, fmt.Errorf("patchgen: the two archives hold different payloads (%s and %s)", oldKind, newKind)
	}
	manifest := update.PatchManifest{
		Schema: update.PatchSchema, App: request.App,
		FromVersion: request.FromVersion, ToVersion: request.ToVersion,
		OS: request.OS, Arch: request.Arch, Payload: newKind,
	}

	oldFiles, err := readFiles(oldPayload)
	if err != nil {
		return update.PatchManifest{}, err
	}
	newFiles, err := readFiles(newPayload)
	if err != nil {
		return update.PatchManifest{}, err
	}
	names := make([]string, 0, len(newFiles))
	for name := range newFiles {
		names = append(names, name)
	}
	sort.Strings(names)

	var blobs []blob
	for _, name := range names {
		file := newFiles[name]
		old, existed := oldFiles[name]
		entry := update.PatchFile{
			Path: name, Mode: uint32(file.mode.Perm()), Size: int64(len(file.data)),
			SHA256: digest(file.data),
		}
		switch {
		case existed && bytes.Equal(old.data, file.data):
			entry.Op = "same"
		case existed:
			patch, err := bsdiff.Bytes(old.data, file.data)
			if err != nil {
				return update.PatchManifest{}, fmt.Errorf("patchgen: diffing %s: %w", name, err)
			}
			entry.Op, entry.Blob = "diff", blobName(len(blobs)+1)
			blobs = append(blobs, blob{name: entry.Blob, data: patch})
		default:
			entry.Op, entry.Blob = "new", blobName(len(blobs)+1)
			blobs = append(blobs, blob{name: entry.Blob, data: file.data})
		}
		manifest.Files = append(manifest.Files, entry)
	}
	if len(manifest.Files) == 0 {
		return update.PatchManifest{}, fmt.Errorf("patchgen: the new payload holds no files")
	}
	if err := writePatch(request.Output, manifest, blobs); err != nil {
		return update.PatchManifest{}, err
	}
	return manifest, nil
}

type fileData struct {
	data []byte
	mode os.FileMode
}

type blob struct {
	name string
	data []byte
}

func blobName(index int) string { return fmt.Sprintf("files/%04d", index) }

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func readFiles(root string) (map[string]fileData, error) {
	files := make(map[string]fileData)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("patchgen: %s is not a regular file", path)
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(relative)] = fileData{data: data, mode: info.Mode()}
		return nil
	})
	return files, err
}

func writePatch(output io.Writer, manifest update.PatchManifest, blobs []blob) error {
	encoded, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	gz := gzip.NewWriter(output)
	writer := tar.NewWriter(gz)
	now := time.Unix(0, 0).UTC()
	writeEntry := func(name string, data []byte) error {
		header := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(data)), Typeflag: tar.TypeReg, ModTime: now}
		if err := writer.WriteHeader(header); err != nil {
			return err
		}
		_, err := writer.Write(data)
		return err
	}
	if err := writeEntry("patch.json", encoded); err != nil {
		return err
	}
	for _, item := range blobs {
		if err := writeEntry(item.name, item.data); err != nil {
			return err
		}
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return gz.Close()
}

// extractPayload unpacks archive and returns the payload root: the .app
// directory for darwin, or a directory holding just the executable for linux.
// The cleanup function removes the temporary tree.
func extractPayload(archive, goos, arch, executable string) (root, kind string, cleanup func(), err error) {
	directory, err := os.MkdirTemp("", "viva-patchgen-")
	if err != nil {
		return "", "", nil, err
	}
	removeTree := func() { _ = os.RemoveAll(directory) }
	switch goos {
	case "darwin":
		if !strings.HasSuffix(strings.ToLower(archive), ".zip") {
			removeTree()
			return "", "", nil, fmt.Errorf("patchgen: macOS payloads come from a .zip, not %s", filepath.Base(archive))
		}
		if err := extractZip(archive, directory); err != nil {
			removeTree()
			return "", "", nil, err
		}
		bundles, err := filepath.Glob(filepath.Join(directory, "*.app"))
		if err != nil || len(bundles) != 1 {
			removeTree()
			return "", "", nil, fmt.Errorf("patchgen: %s does not hold exactly one .app", filepath.Base(archive))
		}
		if _, err := os.Stat(filepath.Join(bundles[0], "Contents", "MacOS")); err != nil {
			removeTree()
			return "", "", nil, fmt.Errorf("patchgen: %s is not a macOS bundle", filepath.Base(archive))
		}
		return bundles[0], "app", removeTree, nil
	default:
		if !strings.HasSuffix(strings.ToLower(archive), ".tar.gz") {
			removeTree()
			return "", "", nil, fmt.Errorf("patchgen: Linux payloads come from a .tar.gz, not %s", filepath.Base(archive))
		}
		if err := extractTarGz(archive, directory); err != nil {
			removeTree()
			return "", "", nil, err
		}
		program, err := findExecutable(directory, executable)
		if err != nil {
			removeTree()
			return "", "", nil, err
		}
		programRoot, err := os.MkdirTemp("", "viva-patchgen-payload-")
		if err != nil {
			removeTree()
			return "", "", nil, err
		}
		target := filepath.Join(programRoot, filepath.Base(program))
		if err := copyFile(program, target); err != nil {
			os.RemoveAll(programRoot)
			removeTree()
			return "", "", nil, err
		}
		return programRoot, "file", func() { os.RemoveAll(programRoot); removeTree() }, nil
	}
}

// findExecutable finds the program in an unpacked Linux archive. A named
// executable wins; otherwise the archive must hold one executable besides
// install.sh.
func findExecutable(root, name string) (string, error) {
	var named, candidates []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 || entry.Name() == "install.sh" {
			return nil
		}
		if name != "" && entry.Name() == name {
			named = append(named, path)
		}
		candidates = append(candidates, path)
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(named) == 1 {
		return named[0], nil
	}
	if name == "" && len(candidates) == 1 {
		return candidates[0], nil
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("patchgen: the archive holds no executable")
	}
	return "", fmt.Errorf("patchgen: the archive holds %d executables; name one with the manifest's executable", len(candidates))
}

func copyFile(source, destination string) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	return os.WriteFile(destination, data, 0o755)
}

// extractZip unpacks a zip, refusing absolute paths and entries that climb
// out of destination.
func extractZip(archive, destination string) error {
	reader, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer reader.Close()
	for _, file := range reader.File {
		target, err := safeJoin(destination, file.Name)
		if err != nil {
			return err
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if !file.Mode().IsRegular() {
			return fmt.Errorf("patchgen: %s is not a regular file", file.Name)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		source, err := file.Open()
		if err != nil {
			return err
		}
		output, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode().Perm())
		if err != nil {
			source.Close()
			return err
		}
		_, copyErr := io.Copy(output, source)
		source.Close()
		closeErr := output.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

// extractTarGz unpacks a tar.gz, refusing absolute paths and entries that
// climb out of destination.
func extractTarGz(archive, destination string) error {
	file, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gz.Close()
	reader := tar.NewReader(gz)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target, err := safeJoin(destination, header.Name)
		if err != nil {
			return err
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			output, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(header.Mode)&0o777)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(output, reader)
			closeErr := output.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		default:
			return fmt.Errorf("patchgen: %s is not a regular file or directory", header.Name)
		}
	}
}

func safeJoin(root, name string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(name))
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("patchgen: %q escapes the archive", name)
	}
	return filepath.Join(root, clean), nil
}
