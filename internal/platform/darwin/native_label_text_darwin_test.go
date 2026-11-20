//go:build darwin

package darwin

import (
	"reflect"
	"testing"
	"unicode/utf8"

	"github.com/vivaui/viva/gfx"
)

// A remote file name in another encoding is not UTF-8; the native layer must still get text
// it can decode, and the run byte counts must follow the replacement.
func TestNativeLabelTextReplacesInvalidUTF8(t *testing.T) {
	for _, test := range []struct {
		name  string
		value string
		runs  []gfx.TextRun
		want  string
		count []int32
	}{
		{
			name:  "no runs",
			value: "caf\xe9.txt",
			want:  "caf\uFFFD.txt",
		},
		{
			name:  "runs follow the replaced byte",
			value: "caf\xe9.txt",
			runs:  []gfx.TextRun{{Bytes: 4}, {Bytes: 4}},
			want:  "caf\uFFFD.txt",
			count: []int32{6, 4},
		},
		{
			name:  "runs past the end are clamped",
			value: "a\xffb",
			runs:  []gfx.TextRun{{Bytes: 2}, {Bytes: 100}},
			want:  "a\uFFFDb",
			count: []int32{4, 1},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, counts := nativeLabelText(test.value, test.runs)
			if got != test.want {
				t.Fatalf("text = %q, want %q", got, test.want)
			}
			if !utf8.ValidString(got) {
				t.Fatalf("text %q is not valid UTF-8", got)
			}
			if len(counts) != len(test.count) || len(counts) > 0 && !reflect.DeepEqual(counts, test.count) {
				t.Fatalf("run counts = %v, want %v", counts, test.count)
			}
		})
	}
}

// Valid text is the common case on every redraw of every label: it must pass through
// unchanged and without allocating.
func TestNativeLabelTextKeepsValidTextWithoutAllocating(t *testing.T) {
	value := "Viva 原生界面 👩‍💻"
	runs := []gfx.TextRun{{Bytes: 4}}
	allocs := testing.AllocsPerRun(100, func() {
		got, counts := nativeLabelText(value, runs)
		if got != value || counts != nil {
			t.Fatalf("valid text changed: %q, counts %v", got, counts)
		}
	})
	if allocs != 0 {
		t.Fatalf("valid text allocated %v times per call, want 0", allocs)
	}
}
