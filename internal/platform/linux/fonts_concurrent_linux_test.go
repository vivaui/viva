//go:build !android

package linux

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"testing"

	"github.com/ebitengine/purego"
)

func TestMonospaceFamiliesConcurrentWithTextRendering(t *testing.T) {
	// 原生库崩溃不可 recover，用子进程保护其余测试。
	const probeEnv = "VIVA_TEST_CONCURRENT_FONT_SCAN"
	if os.Getenv(probeEnv) != "1" {
		command := exec.Command(os.Args[0], "-test.run=^TestMonospaceFamiliesConcurrentWithTextRendering$", "-test.timeout=90s", "-test.v")
		command.Env = append(os.Environ(), probeEnv+"=1")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("concurrent font scan: %v\n%s", err, output)
		}
		return
	}

	// 模拟固定在窗口线程上的界面渲染和未绑定线程的后台字体扫描。
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	engine, err := newTextEngine()
	if err != nil {
		t.Skipf("Pango unavailable: %v", err)
	}
	defer engine.close()
	if len(FontFamilies()) == 0 {
		t.Skip("no installed fonts")
	}
	var workers sync.WaitGroup
	start := make(chan struct{})
	for worker := 0; worker < 4; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			for iteration := 0; iteration < 10; iteration++ {
				families := MonospaceFamilies()
				if len(families) == 0 {
					t.Error("concurrent scan returned no monospace fonts")
					return
				}
			}
		}()
	}
	close(start)
	for iteration := 0; iteration < 5000; iteration++ {
		width := engine.Width(fmt.Sprintf("Motty font scan %d", iteration), 14, 0)
		if width <= 0 {
			t.Errorf("text width = %v, want a positive width", width)
			break
		}
	}
	workers.Wait()
}

func TestFontScanServicesOwnIndependentFontMaps(t *testing.T) {
	first, err := newFontScanTextServices()
	if err != nil {
		t.Skipf("PangoFT2 unavailable: %v", err)
	}
	defer first.close()
	second, err := newFontScanTextServices()
	if err != nil {
		t.Fatal(err)
	}
	defer second.close()
	var contextFontMap func(uintptr) uintptr
	purego.RegisterLibFunc(&contextFontMap, first.pango, "pango_context_get_font_map")
	firstMap, secondMap := contextFontMap(first.context), contextFontMap(second.context)
	if firstMap == 0 || secondMap == 0 || firstMap == secondMap {
		t.Fatalf("font scans must own distinct font maps: %x, %x", firstMap, secondMap)
	}
	if !familyMeasuresMono(first, "monospace") || !familyMeasuresMono(second, "monospace") {
		t.Fatal("isolated font maps must measure the monospace family")
	}
}
