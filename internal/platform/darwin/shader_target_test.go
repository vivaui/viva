//go:build darwin && !ios

package darwin

import (
	"bytes"
	"strings"
	"testing"

	"github.com/vivaui/viva/tools/manifest"
)

// TestSceneShaderLibraryTargetsTheOldestSupportedMacOS guards the checked-in
// Metal library against being built for a newer macOS than Viva supports. A
// Metal library records the deployment target it was built for and refuses to
// load on anything older, so building it on a newer Mac would silently raise
// the application's real floor. The rest of the ordinary darwin tests cannot
// see that: they build the host's pipelines and skip when they cannot.
func TestSceneShaderLibraryTargetsTheOldestSupportedMacOS(t *testing.T) {
	target := metalLibraryTarget(sceneShaderLibrary)
	if target == "" {
		t.Fatal("scene.metallib records no apple-macosx deployment target")
	}
	if manifest.CompareVersions(target, manifest.MacOSFloor) > 0 {
		t.Fatalf("scene.metallib targets macOS %s, newer than the supported macOS %s; rebuild it with -mmacosx-version-min=%s", target, manifest.MacOSFloor, manifest.MacOSFloor)
	}
}

// metalLibraryTarget returns the newest macOS deployment target recorded in a
// Metal library, for example "12.0.0" from "air64-apple-macosx12.0.0". The
// target appears once per compiled function, so a library may carry several.
func metalLibraryTarget(library []byte) string {
	const marker = "apple-macosx"
	newest := ""
	for rest := library; ; {
		index := bytes.Index(rest, []byte(marker))
		if index < 0 {
			return newest
		}
		rest = rest[index+len(marker):]
		end := 0
		for end < len(rest) && (rest[end] >= '0' && rest[end] <= '9' || rest[end] == '.') {
			end++
		}
		if end == 0 {
			continue
		}
		candidate := strings.TrimSuffix(string(rest[:end]), ".")
		if newest == "" || manifest.CompareVersions(candidate, newest) > 0 {
			newest = candidate
		}
	}
}
