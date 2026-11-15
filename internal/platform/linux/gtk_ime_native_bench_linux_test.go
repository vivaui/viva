//go:build linux && vivagtk

package linux

import (
	"fmt"
	"os"
	"testing"
	"unsafe"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

func BenchmarkGTKNativeOrdinaryKey(b *testing.B) {
	if os.Getenv("VIVA_GTK_IME_NATIVE") != "1" {
		b.Skip("native GTK IME benchmark requires a private display")
	}
	p := &gtkIMEProbe{}
	var pressErr error
	calls := 0
	err := RunGTKApplication(func(application platform.Application) error {
		a := application.(*gtkApplication)
		for _, binding := range []struct {
			function any
			name     string
		}{
			{&p.keys.newEvent, "gdk_event_new"}, {&p.keys.freeEvent, "gdk_event_free"},
			{&p.keys.setDevice, "gdk_event_set_device"}, {&p.keys.keyboard, "gdk_seat_get_keyboard"},
			{&p.keys.ref, "g_object_ref"}, {&p.keys.entries, "gdk_keymap_get_entries_for_keyval"},
			{&p.keys.translate, "gdk_keymap_translate_keyboard_state"},
		} {
			if !a.g.lookup(binding.function, binding.name) {
				return fmt.Errorf("native benchmark cannot bind %s", binding.name)
			}
		}
		window, err := application.OpenWindow(platform.Config{
			Title: "Viva native IME benchmark", Width: 360, Height: 160,
			ClearColor: platform.Color{Alpha: 1}, DynamicTextInput: true,
		})
		if err != nil {
			return err
		}
		p.w = window.(*gtkWindow)
		a.enqueue(func() {
			state := platform.TextState{Value: "abc"}
			field := platform.TextInput{
				Target: gfx.Item(1), Width: 300, Height: 100, FontSize: 16,
				Multiline: true, State: state,
				OnEvent: func(event platform.TextEvent) (platform.TextState, error) {
					calls++
					return state, nil
				},
			}
			p.w.present(platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: &field}}, nil)
			p.w.ime.setFocused(true)
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				if pressErr = p.press(p.w.ime.ctx, 'r', 0); pressErr != nil {
					break
				}
			}
			b.StopTimer()
			b.ReportMetric(float64(unsafe.Sizeof(*p.w.ime)), "IME-B")
			b.ReportMetric(float64(unsafe.Sizeof(*callbacks())), "callbacks-B")
			p.w.destroy()
		})
		return nil
	})
	if err != nil || pressErr != nil || calls != b.N {
		b.Fatalf("application=%v key=%v callbacks=%d want=%d", err, pressErr, calls, b.N)
	}
}
