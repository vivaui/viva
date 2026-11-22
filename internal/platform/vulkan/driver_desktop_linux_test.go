//go:build linux && !android

package vulkan

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/internal/platform"
)

func TestVulkanDriverManifestDetection(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("not a driver"), 0o600); err != nil {
		t.Fatal(err)
	}
	if vulkanDriverManifestsExist([]string{dir}) {
		t.Fatal("a directory without an ICD manifest counts as a Vulkan driver")
	}
	manifest := filepath.Join(dir, "driver.json")
	if err := os.WriteFile(manifest, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !vulkanDriverManifestsExist([]string{dir}) || !vulkanDriverManifestsExist([]string{manifest}) {
		t.Fatal("an ICD manifest was not detected")
	}
}

func TestVulkanWithoutDriverManifestDoesNotLoadDriver(t *testing.T) {
	t.Setenv("VK_DRIVER_FILES", filepath.Join(t.TempDir(), "missing.json"))
	t.Setenv("VK_ICD_FILENAMES", "")
	t.Setenv("VK_ADD_DRIVER_FILES", "")
	if vulkanDriverConfigured() {
		t.Fatal("a missing ICD manifest counts as a Vulkan driver")
	}
	_, err := openVulkan()
	if !errors.Is(err, platform.ErrNoGraphics) {
		t.Fatalf("openVulkan error = %v, want platform.ErrNoGraphics", err)
	}
}

func TestVulkanEmptyICDOverrideDoesNotUseDefaultManifests(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_DATA_HOME", root)
	writeVulkanManifest(t, filepath.Join(root, "vulkan", "icd.d"))
	t.Setenv("VK_DRIVER_FILES", "")
	t.Setenv("VK_ICD_FILENAMES", "")
	unsetTestEnv(t, "VK_ADD_DRIVER_FILES")
	if vulkanDriverConfigured() {
		t.Fatal("an empty ICD override falls back to the default driver manifests")
	}
}

// The default search reads each XDG base the loader reads. Only positive cases are
// checked: the system's /etc/vulkan/icd.d is always searched, so an empty result
// cannot be asserted on a machine that has a driver installed.
func TestVulkanDefaultManifestSearchFindsEachXDGDirectory(t *testing.T) {
	cases := []struct {
		name string
		// env sets the search variables under root, and returns the base whose
		// vulkan/icd.d holds the manifest.
		env func(t *testing.T, root string) string
	}{
		{"config home", func(t *testing.T, root string) string {
			t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
			return filepath.Join(root, "config")
		}},
		{"home default config", func(t *testing.T, root string) string {
			t.Setenv("HOME", root)
			return filepath.Join(root, ".config")
		}},
		{"config dirs", func(t *testing.T, root string) string {
			t.Setenv("XDG_CONFIG_DIRS", filepath.Join(root, "missing")+":"+filepath.Join(root, "xdg"))
			return filepath.Join(root, "xdg")
		}},
		{"data home", func(t *testing.T, root string) string {
			t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
			return filepath.Join(root, "data")
		}},
		{"data dirs", func(t *testing.T, root string) string {
			t.Setenv("XDG_DATA_DIRS", filepath.Join(root, "share"))
			return filepath.Join(root, "share")
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("HOME", filepath.Join(root, "home"))
			t.Setenv("XDG_CONFIG_DIRS", "")
			t.Setenv("XDG_DATA_DIRS", "")
			unsetTestEnv(t, "XDG_CONFIG_HOME")
			unsetTestEnv(t, "XDG_DATA_HOME")
			unsetTestEnv(t, "VK_DRIVER_FILES")
			unsetTestEnv(t, "VK_ICD_FILENAMES")
			unsetTestEnv(t, "VK_ADD_DRIVER_FILES")
			base := tc.env(t, root)
			writeVulkanManifest(t, filepath.Join(base, "vulkan", "icd.d"))
			if !vulkanDriverConfigured() {
				t.Fatalf("a manifest under the %s was not found", tc.name)
			}
		})
	}
}

// unsetTestEnv removes name for the rest of the test. t.Setenv restores the original
// value at cleanup, which the bare os.Unsetenv alone would not.
func unsetTestEnv(t *testing.T, name string) {
	t.Helper()
	t.Setenv(name, "")
	if err := os.Unsetenv(name); err != nil {
		t.Fatal(err)
	}
}

func writeVulkanManifest(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "driver.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
}
