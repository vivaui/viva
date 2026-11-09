package gfx

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func FuzzWrapLines(f *testing.F) {
	f.Add("connect to host now", float32(10), 5)
	f.Add("你好，世界。这是一段很长的中文文本", float32(6), 3)
	f.Add("SHA256:abcdefghijklmnopqrstuvwxyz0123456789", float32(16), 2)
	f.Add("a\n\nb\n", float32(1), 9)
	f.Add("  \xff\xfe  ", float32(0.5), 4)
	f.Fuzz(func(t *testing.T, value string, width float32, maxLines int) {
		canvas := wrapper()
		lines := canvas.WrapLines(nil, value, 1, 0, width, maxLines)
		if maxLines <= 0 || !(width > 0) {
			if len(lines) != 0 {
				t.Fatalf("width %v and %d lines produced %q", width, maxLines, lines)
			}
			return
		}
		if len(lines) > maxLines {
			t.Fatalf("%d lines for a limit of %d: %q", len(lines), maxLines, lines)
		}
		if !utf8.ValidString(value) {
			return // measuring bytes that are not text has no promise
		}
		for _, line := range lines {
			if !utf8.ValidString(line) {
				t.Fatalf("line %q is not valid UTF-8", line)
			}
			// A line fits the width, except that a character (a cluster of runes) always gets a line
			// of its own however narrow the width.
			if cells := utf8.RuneCountInString(line); float32(cells) > width && nextCluster(line, 0) < len(line) {
				t.Fatalf("line %q is %d wide in %v", line, cells, width)
			}
			if strings.Contains(line, "\n") {
				t.Fatalf("line %q holds a newline", line)
			}
		}
	})
}
