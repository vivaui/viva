package windows

import (
	"errors"
	"os"
	"runtime"
	"syscall"
	"testing"
	"unsafe"

	"github.com/vivaui/viva/gfx"
	inputpkg "github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/text"
)

func columnClipboardWindow(t *testing.T) *windowState {
	t.Helper()
	if os.Getenv("VIVA_WINDOWS_CLIPBOARD") == "" {
		t.Skip("set VIVA_WINDOWS_CLIPBOARD=1 on a task-owned clipboard")
	}
	runtime.LockOSThread()
	t.Cleanup(runtime.UnlockOSThread)
	state := &windowState{window: staticWindow(t), ready: true, textEnabled: true, application: &windowsApplication{windows: 1}}
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

func TestColumnClipboardNativeCallbackCannotUseAnObsoleteTarget(t *testing.T) {
	for _, mode := range []string{"geometry", "replacement", "metadata", "destroyed"} {
		for _, action := range []byte{'c', 'x'} {
			t.Run(mode+"/"+string(action), func(t *testing.T) {
				state := columnClipboardWindow(t)
				if mode == "destroyed" {
					window := state.window
					windowStates.Store(window, state)
					t.Cleanup(func() { windowStates.Delete(window) })
				}
				state.config.DynamicTextInput = true
				incoming := text.NewEditor("abc")
				incoming.SetSelection(text.Range{Start: 1, End: 2})
				oldEvents, newEvents := 0, 0
				stop := errors.New("stop after live Cut delivery")
				state.textState = platform.TextState{Value: "outgoing"}
				state.textInput = platform.TextInput{Target: gfx.Item(1), Width: 120, Height: 24, FontSize: 13,
					Placed: true, CaretHeight: 1, State: state.textState, CopyOverride: true, CopyRectangular: true,
					OnEvent: func(platform.TextEvent) (platform.TextState, error) {
						oldEvents++
						return state.textState, stop
					}}
				state.textInput.CopyText = func() string {
					if mode == "destroyed" {
						windowProc(state.window, wmDestroy, 0, 0)
						return "rows\r\n"
					}
					next := state.textInput
					switch mode {
					case "geometry":
						next.X++
					case "metadata":
						next.CopyRectangular = false
					case "replacement":
						next.Target, next.State = gfx.Item(2), platform.EditorState(incoming)
						next.CopyOverride, next.CopyRectangular, next.CopyText = false, false, nil
						next.OnEvent = func(event platform.TextEvent) (platform.TextState, error) {
							newEvents++
							event.Apply(incoming)
							return platform.EditorState(incoming), stop
						}
					}
					if _, _, err := state.applyTextInputUpdateWithCancel(platform.TextInputUpdate{Changed: true, Input: &next}, func() {
						t.Fatal("non-composing clipboard fixture unexpectedly canceled IME")
					}); err != nil {
						t.Fatal(err)
					}
					return "rows\r\n"
				}
				if err := SetClipboardText("sentinel"); err != nil {
					t.Fatal(err)
				}
				if !state.textKey(uintptr(action-'a'+'A'), 0, inputpkg.ModifierControl) {
					t.Fatal("clipboard key was not consumed")
				}
				if incoming.String() != "abc" || incoming.Selection() != (text.Range{Start: 1, End: 2}) || newEvents != 0 {
					t.Fatalf("clipboard callback entered new owner: text=%q selection=%+v events=%d", incoming.String(), incoming.Selection(), newEvents)
				}
				want, wantEvents := "sentinel", 0
				var wantError error
				if mode == "geometry" {
					want = "rows\r\n"
					if action == 'x' {
						wantEvents, wantError = 1, stop
					}
				}
				if got, err := ClipboardText(); err != nil || got != want || oldEvents != wantEvents || state.err != wantError {
					t.Fatalf("stale clipboard action: text=%q err=%v oldEvents=%d stateErr=%v; want %q/%d/%v", got, err, oldEvents, state.err, want, wantEvents, wantError)
				}
				if mode == "destroyed" && (state.window != 0 || state.textEnabled || state.application.windows != 0) {
					t.Fatal("closed clipboard callback retained live native input")
				}
			})
		}
	}
}

func TestColumnClipboardNativeBusyClipboardNeverCuts(t *testing.T) {
	state := columnClipboardWindow(t)
	incoming := text.NewEditor("abc")
	incoming.SetSelection(text.Range{Start: 1, End: 2})
	state.textState = platform.EditorState(incoming)
	events := 0
	state.textInput = platform.TextInput{State: state.textState, CopyOverride: true, CopyRectangular: true,
		CopyText: func() string { return "rows\r\n" },
		OnEvent: func(event platform.TextEvent) (platform.TextState, error) {
			events++
			event.Apply(incoming)
			return platform.EditorState(incoming), errors.New("unexpected Cut")
		}}
	if err := SetClipboardText("sentinel"); err != nil {
		t.Fatal(err)
	}
	ready := make(chan error, 1)
	release, done := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		class, err := syscall.UTF16PtrFromString("STATIC")
		if err != nil {
			ready <- err
			return
		}
		window, _, err := createWindow.Call(0, uintptr(unsafe.Pointer(class)), 0, wsPopup, 0, 0, 1, 1, 0, 0, 0, 0)
		if window == 0 {
			ready <- windowsCallError("create clipboard holder", err)
			return
		}
		defer destroyWindow.Call(window)
		if opened, _, err := openClipboard.Call(window); opened == 0 {
			ready <- windowsCallError("hold clipboard fixture", err)
			return
		}
		ready <- nil
		<-release
		closeClipboard.Call()
	}()
	released := false
	defer func() {
		if !released {
			close(release)
			<-done
		}
	}()
	if err := <-ready; err != nil {
		t.Fatal(err)
	}
	if err := SetClipboardRectangularText("blocked"); err == nil {
		t.Fatal("rectangular write ignored an externally held clipboard")
	}
	if _, _, err := ClipboardTextWithFormat(); err == nil {
		t.Fatal("formatted read guessed data from a busy clipboard")
	}
	if !state.textKey('X', 0, inputpkg.ModifierControl) {
		t.Fatal("failed Cut key was not consumed")
	}
	close(release)
	<-done
	released = true
	if got, err := ClipboardText(); err != nil || got != "sentinel" || events != 0 || state.err != nil ||
		incoming.String() != "abc" || incoming.Selection() != (text.Range{Start: 1, End: 2}) {
		t.Fatalf("busy clipboard Cut mutated state: clipboard=%q/%v events=%d stateErr=%v text=%q selection=%+v", got, err, events, state.err, incoming.String(), incoming.Selection())
	}
}

func TestColumnClipboardNativeUnreadableMarkerPastesAsStream(t *testing.T) {
	for _, msdev := range []bool{false, true} {
		name := "unreadable Borland"
		if msdev {
			name = "MSDEV has priority"
		}
		t.Run(name, func(t *testing.T) {
			state := columnClipboardWindow(t)
			if err := SetClipboardText("unchanged"); err != nil {
				t.Fatal(err)
			}
			addColumnClipboardMarker(t, state.window, "Borland IDE Block Type", nil)
			if msdev {
				addColumnClipboardMarker(t, state.window, "MSDEVColumnSelect", nil)
			}
			value, rectangular, err := ClipboardTextWithFormat()
			if msdev {
				if err != nil || value != "unchanged" || !rectangular {
					t.Fatalf("valid MSDEV marker consulted broken Borland data: %q/%v/%v", value, rectangular, err)
				}
			} else if err != nil || value != "unchanged" || rectangular {
				t.Fatalf("unreadable marker was not pasted as stream text: %q/%v/%v", value, rectangular, err)
			}
			if !msdev {
				events := 0
				state.textInput = platform.TextInput{OnEvent: func(event platform.TextEvent) (platform.TextState, error) {
					events++
					if event.Kind != platform.TextPaste || event.Text != "unchanged" || event.Rectangular {
						t.Errorf("paste event = %+v; want the clipboard text pasted as a stream", event)
					}
					// The fixture has no text engine to paint the paste into; the
					// event is the assertion, so stop before the frame.
					return state.textState, errors.New("stop the fixture paste")
				}}
				if !state.textKey('V', 0, inputpkg.ModifierControl) || events != 1 {
					t.Fatalf("unreadable marker did not deliver a stream Paste: events=%d", events)
				}
			}
			if plain, err := ClipboardText(); err != nil || plain != "unchanged" {
				t.Fatalf("marker read failure changed ordinary clipboard behavior: %q/%v", plain, err)
			}
		})
	}
}

func TestColumnClipboardNativeDiscardedMarkerIsNotGuessed(t *testing.T) {
	for _, test := range []struct {
		name        string
		size        uintptr
		value       byte
		rectangular bool
	}{
		{name: "discarded"},
		{name: "column", size: 1, value: 2, rectangular: true},
		{name: "stream", size: 1, value: 1},
		{name: "oversized", size: 2, value: 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			// A zero-sized movable allocation has a handle but no readable data.
			handle, _, err := globalAlloc.Call(globalMoveable|globalZeroInit, test.size)
			if handle == 0 {
				t.Fatalf("allocate marker: %v", err)
			}
			defer globalFree.Call(handle)
			if test.size != 0 {
				memory, _, err := globalLock.Call(handle)
				if memory == 0 {
					t.Fatalf("lock marker: %v", err)
				}
				*(*byte)(pointer(memory)) = test.value
				globalUnlock.Call(handle)
			}
			if size, _, _ := globalSize.Call(handle); size != test.size {
				t.Fatalf("marker size = %d; want %d", size, test.size)
			}
			rectangular := borlandClipboardMarker(handle)
			if rectangular != test.rectangular {
				t.Fatalf("marker = rectangular %v; want rectangular %v", rectangular, test.rectangular)
			}
		})
	}
}
