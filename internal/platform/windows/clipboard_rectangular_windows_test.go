package windows

import (
	"errors"
	"os"
	"runtime"
	"syscall"
	"testing"
	"unsafe"

	inputpkg "github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/internal/platform"
)

func columnClipboardWindow(t *testing.T) *windowState {
	t.Helper()
	if os.Getenv("VIVA_WINDOWS_CLIPBOARD") == "" {
		t.Skip("set VIVA_WINDOWS_CLIPBOARD=1 on a task-owned clipboard")
	}
	runtime.LockOSThread()
	t.Cleanup(runtime.UnlockOSThread)
	state := &windowState{window: staticWindow(t), ready: true, textEnabled: true}
	previous := active.Load()
	active.Store(state)
	t.Cleanup(func() { active.Store(previous) })
	t.Cleanup(func() { _ = SetClipboardText("") })
	return state
}

func columnClipboardFormat(t *testing.T, name string) uintptr {
	t.Helper()
	value, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		t.Fatal(err)
	}
	format, _, err := registerClipboardFormat.Call(uintptr(unsafe.Pointer(value)))
	if format == 0 {
		t.Fatalf("register %s: %v", name, err)
	}
	return format
}

func addColumnClipboardMarker(t *testing.T, window uintptr, name string, data []byte) {
	t.Helper()
	format := columnClipboardFormat(t, name)
	if opened, _, err := openClipboard.Call(window); opened == 0 {
		t.Fatalf("open marker fixture: %v", err)
	}
	defer closeClipboard.Call()
	if data == nil {
		// Scintilla advertises MSDEV by presence, using delayed-rendered NULL data.
		setClipboardData.Call(format, 0)
	} else if err := setClipboardBlock(format, data); err != nil {
		t.Fatal(err)
	}
	if available, _, _ := clipboardFormatAvailable.Call(format); available == 0 {
		t.Fatalf("fixture has no %s format", name)
	}
}

func TestColumnClipboardNativeWritesUnicodeAndBothMarkers(t *testing.T) {
	state := columnClipboardWindow(t)
	const value = "\u00e9\r\n\u6f22\r\n"
	if err := SetClipboardRectangularText(value); err != nil {
		t.Fatal(err)
	}
	if got, rectangular, err := ClipboardTextWithFormat(); err != nil || got != value || !rectangular {
		t.Fatalf("column snapshot = %q, rectangular=%v, err=%v", got, rectangular, err)
	}
	for _, name := range []string{"MSDEVColumnSelect", "Borland IDE Block Type"} {
		if available, _, _ := clipboardFormatAvailable.Call(columnClipboardFormat(t, name)); available == 0 {
			t.Fatalf("column copy omitted %s", name)
		}
	}
	if opened, _, err := openClipboard.Call(state.window); opened == 0 {
		t.Fatalf("open Borland payload: %v", err)
	}
	data, err := clipboardBytes(columnClipboardFormat(t, "Borland IDE Block Type"), 32)
	closeClipboard.Call()
	if err != nil || len(data) != 1 || data[0] != 0x02 {
		t.Fatalf("Borland payload = %x, err=%v; want one byte 02", data, err)
	}
	if err := SetClipboardRectangularText("invalid\x00column"); err == nil {
		t.Fatal("NUL-bearing native column text was accepted")
	}
	if got, rectangular, err := ClipboardTextWithFormat(); err != nil || got != value || !rectangular {
		t.Fatalf("refused write changed the clipboard: %q/%v/%v", got, rectangular, err)
	}
	if err := SetClipboardText("plain"); err != nil {
		t.Fatal(err)
	}
	if got, rectangular, err := ClipboardTextWithFormat(); err != nil || got != "plain" || rectangular {
		t.Fatalf("plain text retained column metadata: %q/%v/%v", got, rectangular, err)
	}
	for _, name := range []string{"MSDEVColumnSelect", "Borland IDE Block Type"} {
		if available, _, _ := clipboardFormatAvailable.Call(columnClipboardFormat(t, name)); available != 0 {
			t.Fatalf("plain copy retained %s", name)
		}
	}
}

func TestColumnClipboardNativeReadsIndependentMarkers(t *testing.T) {
	for _, test := range []struct {
		name, format string
		data         []byte
		rectangular  bool
	}{
		{name: "unmarked"},
		{name: "MSDEV data", format: "MSDEVColumnSelect", data: []byte{0}, rectangular: true},
		{name: "MSDEV delayed data", format: "MSDEVColumnSelect", rectangular: true},
		{name: "Borland column", format: "Borland IDE Block Type", data: []byte{2}, rectangular: true},
		{name: "Borland stream", format: "Borland IDE Block Type", data: []byte{1}},
		{name: "Borland oversized", format: "Borland IDE Block Type", data: []byte{2, 0}},
	} {
		t.Run(test.name, func(t *testing.T) {
			state := columnClipboardWindow(t)
			const value = "source\r\n\u6f22\r\n"
			if err := SetClipboardText(value); err != nil {
				t.Fatal(err)
			}
			if test.format != "" {
				addColumnClipboardMarker(t, state.window, test.format, test.data)
			}
			if got, rectangular, err := ClipboardTextWithFormat(); err != nil || got != value || rectangular != test.rectangular {
				t.Fatalf("independent clipboard = %q, rectangular=%v, err=%v; want rectangular=%v", got, rectangular, err, test.rectangular)
			}
		})
	}
}

func TestColumnClipboardNativeKeyboardRoutesTheSameSnapshot(t *testing.T) {
	for _, action := range []byte{'c', 'x', 'v'} {
		t.Run(string(action), func(t *testing.T) {
			state := columnClipboardWindow(t)
			const value = "row\r\n\u00e9\r\n"
			stop := errors.New("stop after actual clipboard delivery")
			var events []platform.TextEvent
			copies := 0
			state.textState = platform.TextState{Value: "primary"}
			state.textInput = platform.TextInput{State: state.textState, Multiline: true, CopyOverride: true, CopyRectangular: true,
				CopyText: func() string { copies++; return value },
				OnEvent: func(event platform.TextEvent) (platform.TextState, error) {
					events = append(events, event)
					return state.textState, stop
				}}
			if action == 'v' {
				if err := SetClipboardText(value); err != nil {
					t.Fatal(err)
				}
				addColumnClipboardMarker(t, state.window, "MSDEVColumnSelect", nil)
			}
			if !state.textKey(uintptr(action-'a'+'A'), 0, inputpkg.ModifierControl) {
				t.Fatal("native clipboard shortcut was not handled")
			}
			if action == 'c' || action == 'x' {
				if got, rectangular, err := ClipboardTextWithFormat(); err != nil || got != value || !rectangular || copies != 1 {
					t.Fatalf("native copy = %q/%v/%v, calls=%d", got, rectangular, err, copies)
				}
			}
			if action == 'c' {
				if len(events) != 0 || state.err != nil {
					t.Fatalf("copy delivered an edit: events=%+v err=%v", events, state.err)
				}
				return
			}
			if len(events) != 1 || state.err != stop || events[0].Kind != platform.TextPaste {
				t.Fatalf("clipboard delivery = %+v, err=%v", events, state.err)
			}
			want := ""
			if action == 'v' {
				want = value
			}
			if events[0].Text != want || events[0].Rectangular != (action == 'v') {
				t.Fatalf("native paste metadata/text diverged: %+v", events[0])
			}
		})
	}
}
