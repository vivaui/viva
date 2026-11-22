//go:build linux && !android

package vulkan

import (
	"os"
	"path/filepath"
)

// A desktop Linux has Vulkan when the loader finds a driver manifest; the loader's
// library is libvulkan.so.1.
const vulkanLibraryName = "libvulkan.so.1"

// Loader 1.1.97 segfaults in vkCreateInstance when no ICD manifest exists.
func vulkanDriverConfigured() bool {
	var paths []string
	if files, ok := os.LookupEnv("VK_DRIVER_FILES"); ok {
		paths = filepath.SplitList(files)
	} else if files, ok := os.LookupEnv("VK_ICD_FILENAMES"); ok {
		paths = filepath.SplitList(files)
	} else {
		paths = vulkanDefaultManifestDirs()
	}
	if files := os.Getenv("VK_ADD_DRIVER_FILES"); files != "" {
		paths = append(paths, filepath.SplitList(files)...)
	}
	return vulkanDriverManifestsExist(paths)
}

// vulkanDefaultManifestDirs lists the directories the loader searches for ICD manifests
// when no override is set, in its order: the user's config and data homes, then the
// system lists under each XDG base.
func vulkanDefaultManifestDirs() []string {
	home := os.Getenv("HOME")
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" && home != "" {
		configHome = filepath.Join(home, ".config")
	}
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" && home != "" {
		dataHome = filepath.Join(home, ".local", "share")
	}
	var paths []string
	if configHome != "" {
		paths = append(paths, filepath.Join(configHome, "vulkan", "icd.d"))
	}
	paths = appendVulkanDirs(paths, os.Getenv("XDG_CONFIG_DIRS"), "/etc/xdg")
	if dataHome != "" {
		paths = append(paths, filepath.Join(dataHome, "vulkan", "icd.d"))
	}
	paths = appendVulkanDirs(paths, os.Getenv("XDG_DATA_DIRS"), "/usr/local/share:/usr/share")
	return append(paths, "/etc/vulkan/icd.d")
}

// appendVulkanDirs adds the vulkan/icd.d directory under each entry of an XDG search list,
// or under the fallback list when the variable is unset.
func appendVulkanDirs(paths []string, list, fallback string) []string {
	if list == "" {
		list = fallback
	}
	for _, dir := range filepath.SplitList(list) {
		if dir != "" {
			paths = append(paths, filepath.Join(dir, "vulkan", "icd.d"))
		}
	}
	return paths
}

func vulkanDriverManifestsExist(paths []string) bool {
	for _, path := range paths {
		if path == "" {
			continue
		}
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if !info.IsDir() {
			return true
		}
		entries, err := os.ReadDir(path)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
				return true
			}
		}
	}
	return false
}
