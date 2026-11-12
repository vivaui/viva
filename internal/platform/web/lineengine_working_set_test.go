package web

import (
	"fmt"
	"testing"
)

func TestDistinctLinesStayMeasuredUntilFontInvalidation(t *testing.T) {
	browser := &fakeBrowser{}
	engine := newLineEngine(browser)
	lines := make([]string, 2*lineCacheLimit)
	for index := range lines {
		lines[index] = fmt.Sprintf("host %04d", index)
	}
	measure := func() {
		for _, line := range lines {
			engine.Width(line, 14, 0)
		}
	}
	measure()
	measured := browser.widths
	for frame := 0; frame < 10; frame++ {
		measure()
	}
	if browser.widths != measured {
		t.Fatalf("warm lines were remeasured: %d -> %d", measured, browser.widths)
	}
	browser.invalidate()
	measure()
	if browser.widths != measured+len(lines) {
		t.Fatalf("font invalidation did not remeasure each line: %d -> %d", measured, browser.widths)
	}
}
