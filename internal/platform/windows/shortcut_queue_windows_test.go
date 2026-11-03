package windows

import (
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/vivaui/viva/internal/platform"
)

// Queue actual Win32 key messages so the real TranslateMessage/WM_CHAR path
// proves a consumed printable key cannot type into the newly focused editor.
func TestQueuedShortcutDoesNotGenerateACharacterAfterChangingTextInput(t *testing.T) {
	done := make(chan error, 1)
	ready := make(chan uintptr, 1)
	var typed strings.Builder
	shortcuts := 0
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		done <- RunApplication(func(application platform.Application) error {
			field := platform.TextInput{
				X: 10, Y: 10, Width: 200, Height: 30, FontSize: 13,
				OnEvent: func(event platform.TextEvent) (platform.TextState, error) {
					if event.Kind == platform.TextInsert {
						typed.WriteString(event.Text)
					}
					return platform.TextState{Value: typed.String()}, nil
				},
			}
			opened, err := application.OpenWindow(platform.Config{
				Title: "shortcut translation", Width: 300, Height: 160,
				DynamicTextInput: true,
				OnShortcut: func(event platform.KeyEvent) (platform.Frame, bool, error) {
					if event.Text != "x" {
						return platform.Frame{}, false, nil
					}
					shortcuts++
					return platform.Frame{TextInput: platform.TextInputUpdate{Changed: true, Input: &field}}, true, nil
				},
			})
			if err == nil {
				ready <- opened.(*nativeWindow).state.window
			}
			return err
		})
	}()
	var window uintptr
	select {
	case window = <-ready:
	case err := <-done:
		t.Fatalf("opening window: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("window did not open")
	}
	postMessage.Call(window, wmKeyDown, 'X', 0x2d<<16|1)
	postMessage.Call(window, wmKeyUp, 'X', 0x2d<<16|1<<31|1)
	postMessage.Call(window, wmKeyDown, 'A', 0x1e<<16|1)
	postMessage.Call(window, wmKeyUp, 'A', 0x1e<<16|1<<31|1)
	// TranslateMessage appends characters to the queue. A later timer closes
	// only once those characters have had a turn through windowProc.
	closeTimer := time.AfterFunc(300*time.Millisecond, func() { postMessage.Call(window, wmClose, 0, 0) })
	defer closeTimer.Stop()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		postMessage.Call(window, wmClose, 0, 0)
		t.Fatal("window did not close")
	}
	if shortcuts != 1 || !strings.EqualFold(typed.String(), "a") {
		t.Fatalf("shortcuts=%d text=%q; want one shortcut and only the unbound A", shortcuts, typed.String())
	}
}
