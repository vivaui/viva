package darwin

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime/cgo"
	"testing"
	"time"

	"github.com/vivaui/viva/internal/platform"
)

// Infer the private cgo integer type without importing C in a test file.
func callbackHandleForTest[T ~uint64](handle cgo.Handle, _ func(T) (*runState, bool)) T {
	return T(handle)
}

func TestLatePointerCallbackDoesNotResolveAReleasedHandle(t *testing.T) {
	for _, name := range []string{"zero", "deleted", "closed", "no handler", "live"} {
		t.Run(name, func(t *testing.T) {
			calls := 0
			state := &runState{pointerHandler: func(event PointerEvent) (Frame, error) {
				if event.Kind != platform.PointerExit || event.X != -1 || event.Y != -1 {
					t.Errorf("pointer event changed: %+v", event)
				}
				calls++
				return Frame{}, nil
			}}
			handle := cgo.NewHandle(state)
			native := callbackHandleForTest(handle, runStateFor)
			if name == "deleted" || name == "zero" {
				handle.Delete()
				if name == "zero" {
					native = 0
				}
			} else {
				defer handle.Delete()
			}
			state.closed = name == "closed"
			if name == "no handler" {
				state.pointerHandler = nil
			}
			defer func() {
				if value := recover(); value != nil {
					t.Fatalf("late pointer callback panicked instead of being ignored: %v", value)
				}
			}()
			vivaPointerEvent(native, nil, 4, 0, 0, 0, -1, -1, 0, 0)
			want := 0
			if name == "live" {
				want = 1
			}
			if calls != want {
				t.Fatalf("pointer handler called %d times, want %d", calls, want)
			}
			if state.err != nil {
				t.Fatalf("pointer callback recorded a fault: %v", state.err)
			}
		})
	}
}

func BenchmarkLivePointerCallback(b *testing.B) {
	state := &runState{pointerHandler: func(PointerEvent) (Frame, error) { return Frame{}, nil }}
	handle := cgo.NewHandle(state)
	defer handle.Delete()
	native := callbackHandleForTest(handle, runStateFor)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		vivaPointerEvent(native, nil, 4, 0, 0, 0, -1, -1, 0, 0)
	}
}

func TestNativeWindowCloseDetachesViewCallbacks(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	executable := filepath.Join(t.TempDir(), "window-close")
	command := exec.CommandContext(ctx, "xcrun", "clang", "-fno-objc-arc", "-Werror",
		"-Wno-deprecated-declarations", "testdata/window_close.m", "-o", executable,
		"-framework", "Cocoa", "-framework", "CoreText", "-framework", "Metal", "-framework", "QuartzCore")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("compile native window close test: %v\n%s", err, output)
	}
	if output, err := exec.CommandContext(ctx, executable).CombinedOutput(); err != nil {
		t.Fatalf("native window close: %v\n%s", err, output)
	} else {
		t.Logf("%s", output)
	}
}
