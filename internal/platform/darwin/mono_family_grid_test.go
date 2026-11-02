//go:build darwin && vivahead

package darwin

import (
	"path/filepath"
	"slices"
	"testing"
)

// The two fixtures share a trait with the coding families this test stands in for: neither
// sets the font's fixed-pitch flag, so only the measurement can tell them apart. Viva Test
// Grid gives every printable ASCII character the same advance; Viva Test Proportional gives
// each its own. Both are generated, carry no third-party outlines, and are registered for
// this process only, so the result does not depend on what the machine has installed.

func TestMonospaceFamiliesMeasureTheFontsThatDoNotSaySo(t *testing.T) {
	for _, file := range []string{"VivaTestGrid-Regular.ttf", "VivaTestProportional-Regular.ttf"} {
		unregister, err := RegisterFontFile(filepath.Join("testdata", file))
		if err != nil {
			t.Fatalf("register %s: %v", file, err)
		}
		t.Cleanup(unregister)
	}
	families := MonospaceFamilies()
	if !slices.Contains(families, "Viva Test Grid") {
		t.Errorf("a family that advances every character alike is missing from %v", families)
	}
	if slices.Contains(families, "Viva Test Proportional") {
		t.Errorf("a family whose characters differ in width is listed in %v", families)
	}
}
