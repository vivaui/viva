package main

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPackageMakesADiskImageHoldingTheBundleAndAnApplicationsLink(t *testing.T) {
	requireDiskImages(t)
	dir := fixture(t, packageTOML)
	writeIcon(t, filepath.Join(dir, "icon.png"), 512)
	var output strings.Builder
	if err := runCLI([]string{"package", "-C", dir, "--format", "dmg"}, &output, io.Discard); err != nil {
		t.Fatal(err)
	}
	image := filepath.Join(dir, "build", "fixture-notes-2.0.1-darwin-"+runtime.GOARCH+".dmg")
	if info, err := os.Stat(image); err != nil || info.Size() == 0 {
		t.Fatalf("image: %v", err)
	}
	if !strings.Contains(output.String(), "  dmg      ") {
		t.Fatalf("the summary does not mention the image:\n%s", output.String())
	}
	if out, err := exec.Command("hdiutil", "verify", image).CombinedOutput(); err != nil {
		t.Fatalf("hdiutil verify: %v\n%s", err, out)
	}

	mount := t.TempDir()
	// -noautoopen keeps the Finder from opening this read-only mount: -nobrowse
	// hides it from the sidebar but does not stop the automatic open that offers
	// to install the application.
	if out, err := exec.Command("hdiutil", "attach", "-nobrowse", "-noautoopen", "-readonly", "-mountpoint", mount, image).CombinedOutput(); err != nil {
		t.Fatalf("hdiutil attach: %v\n%s", err, out)
	}
	t.Cleanup(func() { _ = exec.Command("hdiutil", "detach", "-force", mount).Run() })
	if target, err := os.Readlink(filepath.Join(mount, "Applications")); err != nil || target != "/Applications" {
		t.Fatalf("Applications link = %q, %v", target, err)
	}
	if _, err := os.Stat(filepath.Join(mount, ".DS_Store")); err != nil {
		t.Fatalf("the window layout is missing: %v", err)
	}
	if out, err := exec.Command("plutil", "-lint", filepath.Join(mount, "Fixture Notes.app", "Contents", "Info.plist")).CombinedOutput(); err != nil {
		t.Fatalf("the bundle in the image is damaged: %v\n%s", err, out)
	}
	if info, err := os.Stat(filepath.Join(mount, "Fixture Notes.app", "Contents", "MacOS", "fixture-notes")); err != nil || info.Mode()&0o111 == 0 {
		t.Fatalf("the program in the image is not executable: %v", err)
	}
}

func TestPackageRefusesAFormatTheTargetDoesNotHave(t *testing.T) {
	dir := fixture(t, packageTOML)
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"--format", "nsis"}, `darwin packages cannot be made as "nsis"; the formats are: app, zip, dmg`},
		{[]string{"--target", "windows", "--format", "dmg"}, `windows packages cannot be made as "dmg"`},
		{[]string{"--target", "linux", "--zip"}, "--zip does not apply to linux"},
	} {
		err := runCLI(append([]string{"package", "-C", dir}, test.args...), io.Discard, io.Discard)
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Errorf("%v: error = %v, want it to contain %q", test.args, err, test.want)
		}
	}
}
