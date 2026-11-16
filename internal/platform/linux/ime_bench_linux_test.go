package linux

import (
	"reflect"
	"testing"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

// This measures frontend bookkeeping for an engine-consumed key, not engine IPC
// or text painting. The same fixture can benchmark the fixed prerequisite.
func BenchmarkIBusHandledKey(b *testing.B) {
	for _, tentative := range []bool{false, true} {
		name := "legacy"
		if tentative {
			name = "tentative"
		}
		b.Run(name, func(b *testing.B) {
			h := newIMEHarness(b)
			if tentative {
				owner, ok := any(h.ime).(interface{ bindOwner() })
				if !ok {
					b.Skip("prerequisite has no native tentative binding")
				}
				h.editor.EnableTentativeComposition()
				field := h.input.input
				field.State = platform.EditorState(h.editor)
				if _, _, err := h.input.prepare(platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: &field}}); err != nil {
					b.Fatal(err)
				}
				owner.bindOwner()
			}
			h.focus(b)
			event := &xcbInputEvent{Detail: 57}
			key := platform.KeyEvent{Text: "n"}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				h.context.keys = h.context.keys[:0]
				if !h.ime.processKey('n', event, key) {
					b.Fatal("key refused")
				}
				h.context.keys[0].done(true)
			}
			b.ReportMetric(float64(unsafe.Sizeof(linuxIME{})), "ime-B")
			if field, ok := reflect.TypeOf(*h.ime).FieldByName("phase"); ok {
				b.ReportMetric(float64(field.Type.Elem().Size()), "optional-B")
			}
		})
	}
}
