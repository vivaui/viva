package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gabstv/go-bsdiff/pkg/bsdiff"
)

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func writePatch(t *testing.T, manifest PatchManifest, blobs map[string][]byte) string {
	t.Helper()
	var encoded bytes.Buffer
	gz := gzip.NewWriter(&encoded)
	writer := tar.NewWriter(gz)
	manifestData, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	entries := []struct {
		name string
		data []byte
	}{{"patch.json", manifestData}}
	names := make([]string, 0, len(blobs))
	for name := range blobs {
		names = append(names, name)
	}
	for i := 0; i < len(names); i++ {
		for j := i + 1; j < len(names); j++ {
			if names[j] < names[i] {
				names[i], names[j] = names[j], names[i]
			}
		}
	}
	for _, name := range names {
		entries = append(entries, struct {
			name string
			data []byte
		}{name, blobs[name]})
	}
	for _, entry := range entries {
		header := &tar.Header{Name: entry.name, Mode: 0o644, Size: int64(len(entry.data)), Typeflag: tar.TypeReg}
		if err := writer.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write(entry.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "app.patch")
	if err := os.WriteFile(path, encoded.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func fileManifest(path string, data []byte, op, blob string, mode uint32) PatchFile {
	return PatchFile{Path: path, Mode: mode, Size: int64(len(data)), SHA256: digest(data), Op: op, Blob: blob}
}

func TestPatchApply(t *testing.T) {
	oldRoot := t.TempDir()
	oldData := []byte(strings.Repeat("old content ", 100))
	sameData := []byte("unchanged")
	if err := os.WriteFile(filepath.Join(oldRoot, "program"), oldData, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldRoot, "same.txt"), sameData, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldRoot, "dropped.txt"), []byte("gone"), 0o644); err != nil {
		t.Fatal(err)
	}
	newData := []byte(strings.Repeat("old content ", 100) + " and something new")
	addedData := []byte("a new resource")
	diff, err := bsdiff.Bytes(oldData, newData)
	if err != nil {
		t.Fatal(err)
	}
	manifest := PatchManifest{
		Schema: PatchSchema, App: "dev.example.notes", FromVersion: "1.0.0", ToVersion: "1.1.0",
		OS: "linux", Arch: "amd64", Payload: "file",
		Files: []PatchFile{
			fileManifest("program", newData, "diff", "files/0001", 0o755),
			fileManifest("same.txt", sameData, "same", "", 0o644),
			fileManifest("resource/added.txt", addedData, "new", "files/0002", 0o644),
		},
	}
	patchPath := writePatch(t, manifest, map[string][]byte{"files/0001": diff, "files/0002": addedData})

	patch, err := ReadPatch(patchPath)
	if err != nil {
		t.Fatal(err)
	}
	staging := filepath.Join(t.TempDir(), "staged")
	if err := patch.Apply(oldRoot, staging); err != nil {
		t.Fatal(err)
	}
	patch.Close()

	for name, want := range map[string][]byte{
		"program": newData, "same.txt": sameData, "resource/added.txt": addedData,
	} {
		got, err := os.ReadFile(filepath.Join(staging, filepath.FromSlash(name)))
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s differs", name)
		}
	}
	if _, err := os.Stat(filepath.Join(staging, "dropped.txt")); !os.IsNotExist(err) {
		t.Errorf("a deleted file was staged: %v", err)
	}
	if info, err := os.Stat(filepath.Join(staging, "program")); err != nil || info.Mode().Perm() != 0o755 {
		t.Errorf("program mode = %v, %v", info.Mode(), err)
	}
}

func TestPatchRefusesAChangedBase(t *testing.T) {
	oldRoot := t.TempDir()
	oldData := []byte("the installed file")
	newData := []byte("the installed file, updated")
	diff, _ := bsdiff.Bytes(oldData, newData)
	manifest := PatchManifest{
		Schema: PatchSchema, App: "dev.example.notes", FromVersion: "1.0.0", ToVersion: "1.1.0",
		OS: "linux", Arch: "amd64", Payload: "file",
		Files: []PatchFile{fileManifest("program", newData, "diff", "files/0001", 0o755)},
	}
	patchPath := writePatch(t, manifest, map[string][]byte{"files/0001": diff})
	patch, err := ReadPatch(patchPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldRoot, "program"), []byte("something else entirely"), 0o755); err != nil {
		t.Fatal(err)
	}
	staging := filepath.Join(t.TempDir(), "staged")
	if err := patch.Apply(oldRoot, staging); !errors.Is(err, ErrChecksum) {
		t.Fatalf("err = %v, want ErrChecksum", err)
	}
	if _, err := os.Stat(staging); !os.IsNotExist(err) {
		t.Errorf("staging survived a failed patch: %v", err)
	}
}

func TestPatchRefusesUnsafePaths(t *testing.T) {
	for _, path := range []string{"../evil", "/absolute", "a/../../evil", "a\\b", "a:b", "", "."} {
		manifest := PatchManifest{
			Schema: PatchSchema, App: "app", FromVersion: "1.0.0", ToVersion: "1.1.0",
			OS: "linux", Arch: "amd64", Payload: "file",
			Files: []PatchFile{{Path: path, Mode: 0o644, Size: 1, SHA256: digest([]byte("x")), Op: "new", Blob: "files/0001"}},
		}
		patchPath := writePatch(t, manifest, map[string][]byte{"files/0001": []byte("x")})
		if _, err := ReadPatch(patchPath); !errors.Is(err, ErrManifest) {
			t.Errorf("%q: err = %v, want ErrManifest", path, err)
		}
	}
}

func TestPatchRefusesBadManifests(t *testing.T) {
	base := func() PatchManifest {
		return PatchManifest{
			Schema: PatchSchema, App: "app", FromVersion: "1.0.0", ToVersion: "1.1.0",
			OS: "linux", Arch: "amd64", Payload: "file",
			Files: []PatchFile{{Path: "program", Mode: 0o755, Size: 1, SHA256: digest([]byte("x")), Op: "new", Blob: "files/0001"}},
		}
	}
	tests := map[string]func(*PatchManifest){
		"schema":       func(m *PatchManifest) { m.Schema = 2 },
		"os":           func(m *PatchManifest) { m.OS = "windows" },
		"arch":         func(m *PatchManifest) { m.Arch = "386" },
		"payload":      func(m *PatchManifest) { m.Payload = "dir" },
		"no files":     func(m *PatchManifest) { m.Files = nil },
		"bad version":  func(m *PatchManifest) { m.FromVersion = "1" },
		"mode setuid":  func(m *PatchManifest) { m.Files[0].Mode = 0o4755 },
		"mode zero":    func(m *PatchManifest) { m.Files[0].Mode = 0 },
		"bad digest":   func(m *PatchManifest) { m.Files[0].SHA256 = "abc" },
		"missing blob": func(m *PatchManifest) { m.Files[0].Blob = "" },
		"unknown op":   func(m *PatchManifest) { m.Files[0].Op = "copy" },
		"same with blob": func(m *PatchManifest) {
			m.Files[0].Op, m.Files[0].Blob = "same", "files/0001"
		},
	}
	for name, change := range tests {
		manifest := base()
		change(&manifest)
		patchPath := writePatch(t, manifest, map[string][]byte{"files/0001": []byte("x")})
		if _, err := ReadPatch(patchPath); !errors.Is(err, ErrManifest) {
			t.Errorf("%s: err = %v, want ErrManifest", name, err)
		}
	}
}

func TestPatchRefusesANonEmptyStaging(t *testing.T) {
	oldRoot := t.TempDir()
	data := []byte("x")
	manifest := PatchManifest{
		Schema: PatchSchema, App: "app", FromVersion: "1.0.0", ToVersion: "1.1.0",
		OS: "linux", Arch: "amd64", Payload: "file",
		Files: []PatchFile{fileManifest("program", data, "new", "files/0001", 0o755)},
	}
	patchPath := writePatch(t, manifest, map[string][]byte{"files/0001": data})
	patch, err := ReadPatch(patchPath)
	if err != nil {
		t.Fatal(err)
	}
	staging := filepath.Join(t.TempDir(), "staged")
	if err := os.MkdirAll(staging, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staging, "existing"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := patch.Apply(oldRoot, staging); err == nil {
		t.Fatal("a non-empty staging directory was accepted")
	}
	if data, err := os.ReadFile(filepath.Join(staging, "existing")); err != nil || string(data) != "keep" {
		t.Errorf("the existing staging content was removed: %v", err)
	}
}

func TestApplyPatchChecksNameAndVersions(t *testing.T) {
	oldRoot := t.TempDir()
	oldData := []byte("old program")
	newData := []byte("new program")
	if err := os.WriteFile(filepath.Join(oldRoot, "program"), oldData, 0o755); err != nil {
		t.Fatal(err)
	}
	diff, err := bsdiff.Bytes(oldData, newData)
	if err != nil {
		t.Fatal(err)
	}
	manifest := PatchManifest{
		Schema: PatchSchema, App: "dev.example.notes", FromVersion: "1.0.0", ToVersion: "1.1.0",
		OS: runtime.GOOS, Arch: runtime.GOARCH, Payload: "file",
		Files: []PatchFile{fileManifest("program", newData, "diff", "files/0001", 0o755)},
	}
	patchPath := writePatch(t, manifest, map[string][]byte{"files/0001": diff})

	original := locateInstalledPayload
	defer func() { locateInstalledPayload = original }()
	located := "file"
	locateInstalledPayload = func() (string, string, error) { return oldRoot, located, nil }

	staging, err := ApplyPatch(patchPath, "dev.example.notes", "1.0.0", "1.1.0")
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(staging, "program"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, newData) {
		t.Fatal("the staged program is not the new one")
	}
	if err := os.RemoveAll(staging); err != nil {
		t.Fatal(err)
	}

	for name, call := range map[string]func() (string, error){
		"wrong app":      func() (string, error) { return ApplyPatch(patchPath, "other", "1.0.0", "1.1.0") },
		"wrong from":     func() (string, error) { return ApplyPatch(patchPath, "dev.example.notes", "0.9.0", "1.1.0") },
		"wrong to":       func() (string, error) { return ApplyPatch(patchPath, "dev.example.notes", "1.0.0", "1.2.0") },
		"wrong platform": func() (string, error) { return ApplyPatch(patchPath, "dev.example.notes", "1.0.0", "1.1.0") },
	} {
		if name == "wrong platform" {
			locateInstalledPayload = func() (string, string, error) { return oldRoot, "app", nil }
		}
		if _, err := call(); !errors.Is(err, ErrManifest) {
			t.Errorf("%s: err = %v, want ErrManifest", name, err)
		}
		locateInstalledPayload = func() (string, string, error) { return oldRoot, "file", nil }
	}
}
