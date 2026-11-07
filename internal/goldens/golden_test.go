//go:build darwin || (js && wasm)

package goldens

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// The golden file is shared by both backends: the darwin test writes it when
// GOLDENS_UPDATE=1 and compares otherwise; the web build embeds the same
// bytes and the gallery-web entry verifies against them at startup in
// verify mode. Geometry, colors, and order must match exactly.
const goldenFile = "testdata/drawlist.json"

func TestDrawListMatchesGolden(t *testing.T) {
	list := Capture()
	path := filepath.Join("testdata", "drawlist.json")
	if os.Getenv("GOLDENS_UPDATE") == "1" {
		if runtime.GOOS != "darwin" {
			t.Fatal("GOLDENS_UPDATE is only supported on the darwin reference host")
		}
		if err := writeGolden(path, &list); err != nil {
			t.Fatal(err)
		}
		t.Logf("golden updated: %d commands", len(list.Commands))
		return
	}
	encoded, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("golden missing (run GOLDENS_UPDATE=1 go test ./internal/goldens/): %v", err)
	}
	var want DrawList
	if err := json.Unmarshal(encoded, &want); err != nil {
		t.Fatal(err)
	}
	if len(want.Commands) != len(list.Commands) {
		t.Fatalf("command count = %d, golden = %d", len(list.Commands), len(want.Commands))
	}
	for index := range want.Commands {
		if want.Commands[index] != list.Commands[index] {
			t.Fatalf("command %d diverged:\n got %+v\nwant %+v", index, list.Commands[index], want.Commands[index])
		}
	}
}

func writeGolden(path string, list *DrawList) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, encoded, 0o644)
}
