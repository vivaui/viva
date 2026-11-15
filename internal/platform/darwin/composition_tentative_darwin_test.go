package darwin

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// VIVA_TENTATIVE_BENCH=1 runs the same native bridge overlay's paired input and
// owner benchmarks instead of the native behavior tests.
func TestNativeTentativeCompositionBridge(t *testing.T) {
	directory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	replacements := make(map[string]string)
	for _, name := range []string{"tentative_bridge_darwin.go", "tentative_bridge_darwin_test.go"} {
		replacements[filepath.Join(directory, name)] = filepath.Join(directory, "testdata", name)
	}
	data, err := json.Marshal(struct{ Replace map[string]string }{Replace: replacements})
	if err != nil {
		t.Fatal(err)
	}
	overlay := filepath.Join(t.TempDir(), "overlay.json")
	if err := os.WriteFile(overlay, data, 0o600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "test", "-race", "-overlay", overlay,
		"-run", "^TestDarwinTentativeProbe", "-count=1", "-v", ".")
	if os.Getenv("VIVA_TENTATIVE_BENCH") == "1" {
		command = exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "test", "-overlay", overlay,
			"-run", "^$", "-bench", "^BenchmarkDarwinTentativeProbe", "-benchmem", "-benchtime=200ms", "-count=5", ".")
	}
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("native tentative composition bridge: %v\n%s", err, output)
	}
	t.Logf("%s", output)
}
