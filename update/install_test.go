package update

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInstallRefusesIncompleteOptions(t *testing.T) {
	artifact := filepath.Join(t.TempDir(), "archive.zip")
	if err := os.WriteFile(artifact, []byte("zip"), 0o644); err != nil {
		t.Fatal(err)
	}
	base := InstallOptions{
		Artifact: artifact,
		Asset:    Asset{OS: runtime.GOOS, Arch: runtime.GOARCH, Format: FormatZIP},
		Version:  "1.4.2",
	}
	tests := []struct {
		name   string
		mutate func(*InstallOptions)
		want   string
	}{
		{"no artifact", func(options *InstallOptions) { options.Artifact = "" }, "artifact"},
		{"missing artifact", func(options *InstallOptions) { options.Artifact = filepath.Join(t.TempDir(), "gone.zip") }, "no such file"},
		{"directory", func(options *InstallOptions) { options.Artifact = t.TempDir() }, "regular file"},
		{"no version", func(options *InstallOptions) { options.Version = "" }, "version"},
		{"bad version", func(options *InstallOptions) { options.Version = "latest" }, "version"},
		{"no format", func(options *InstallOptions) { options.Asset.Format = "" }, "format"},
		{"unknown format", func(options *InstallOptions) { options.Asset.Format = "dmg" }, "format"},
		{"another os", func(options *InstallOptions) { options.Asset.OS = "plan9" }, "archive is for"},
		{"another arch", func(options *InstallOptions) { options.Asset.Arch = "riscv64" }, "archive is for"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			options := base
			test.mutate(&options)
			if err := options.validate(); err == nil {
				t.Fatalf("validate accepted %s", test.name)
			} else if !strings.Contains(err.Error(), test.want) {
				t.Errorf("error %q does not mention %q", err, test.want)
			}
		})
	}
}

func TestInstallNeedsAQuitFunction(t *testing.T) {
	err := Install(InstallOptions{}, nil)
	if err == nil || !strings.Contains(err.Error(), "quit") {
		t.Errorf("error = %v, want a missing quit function", err)
	}
}

func TestInstallProgressIsOptional(t *testing.T) {
	options := InstallOptions{}
	options.report("unpacking")
	if options.Progress != nil {
		t.Error("a zero option grew a progress callback")
	}
}

func TestInstallRefusesAnArtifactThatChangedAfterDownload(t *testing.T) {
	artifact := filepath.Join(t.TempDir(), "archive.zip")
	if err := os.WriteFile(artifact, []byte("zip"), 0o644); err != nil {
		t.Fatal(err)
	}
	base := InstallOptions{
		Artifact: artifact,
		Asset: Asset{
			OS: runtime.GOOS, Arch: runtime.GOARCH, Format: FormatZIP,
			Size: 3, SHA256: strings.Repeat("0", 64),
		},
		Version: "1.4.2",
	}
	err := Install(base, func() { t.Error("quit was called for a changed artifact") })
	if !errors.Is(err, ErrChecksum) {
		t.Errorf("error %v does not wrap ErrChecksum", err)
	}
	base.Asset.SHA256 = ""
	base.Asset.Size = 4
	err = Install(base, func() { t.Error("quit was called for a changed artifact") })
	if !errors.Is(err, ErrSize) {
		t.Errorf("error %v does not wrap ErrSize", err)
	}
}

func TestInstallRefusesASymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "archive.zip")
	if err := os.WriteFile(target, []byte("zip"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.zip")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}
	err := Install(InstallOptions{
		Artifact: link,
		Asset:    Asset{OS: runtime.GOOS, Arch: runtime.GOARCH, Format: FormatZIP},
		Version:  "1.4.2",
	}, func() { t.Error("quit was called for a link") })
	if !errors.Is(err, ErrManifest) {
		t.Errorf("error %v does not wrap ErrManifest", err)
	}
}
