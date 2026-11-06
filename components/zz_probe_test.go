package components

import (
	"strings"
	"testing"
	"time"

	"github.com/vivaui/viva/ui"
)

func TestProbeWhitespaceCost(t *testing.T) {
	value := strings.Repeat("a ", 200000) // 400 KB on one line
	fixture := newCodeFixture(value)
	fixture.snapshot.InvisibleColor = ui.RGB(33, 104, 171)
	fixture.snapshot.ShowWhitespace = true
	fixture.paint()
	start := time.Now()
	for range 20 {
		fixture.paint()
	}
	elapsed := time.Since(start)
	t.Logf("len=%d rows=%d 20 paints in %v (%.3f ms/paint) markers=%d",
		len(value), len(fixture.editor.rows), elapsed,
		float64(elapsed.Microseconds())/20/1000,
		len(fixture.fills(fixture.snapshot.InvisibleColor)))
}
