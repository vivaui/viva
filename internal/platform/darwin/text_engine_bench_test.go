//go:build darwin

package darwin

import (
	"strconv"
	"testing"

	"github.com/vivaui/viva/gfx"
)

func BenchmarkTextEngineWidthHit(b *testing.B) {
	engine := SharedTextEngine()
	engine.Width("192.168.1.1 主机 office", 14, 0)
	b.ReportAllocs()
	for b.Loop() {
		engine.Width("192.168.1.1 主机 office", 14, 0)
	}
}

// A miss shapes a new line: what a first paint of every new string costs.
func BenchmarkTextEngineWidthMiss(b *testing.B) {
	engine := SharedTextEngine()
	engine.Reset()
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		engine.Width("192.168.1.1 主机 "+strconv.Itoa(i), 14, 0)
	}
}

func BenchmarkTextEngineCaretHit(b *testing.B) {
	engine := SharedTextEngine()
	engine.Caret("192.168.1.1 主机 office", 14, 0, 8)
	b.ReportAllocs()
	for b.Loop() {
		engine.Caret("192.168.1.1 主机 office", 14, 0, 8)
	}
}

func BenchmarkTextEngineOffsetAtHit(b *testing.B) {
	engine := SharedTextEngine()
	engine.OffsetAt("192.168.1.1 主机 office", 14, 0, 40)
	b.ReportAllocs()
	for b.Loop() {
		engine.OffsetAt("192.168.1.1 主机 office", 14, 0, 40)
	}
}

func BenchmarkTextEngineSelectionHit(b *testing.B) {
	engine := SharedTextEngine()
	dst := make([]gfx.TextSpan, 0, 8)
	dst = engine.Selection(dst[:0], "192.168.1.1 主机 office", 14, 0, 2, 14)
	b.ReportAllocs()
	for b.Loop() {
		dst = engine.Selection(dst[:0], "192.168.1.1 主机 office", 14, 0, 2, 14)
	}
}
