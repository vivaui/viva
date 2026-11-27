//go:build windows

package windows

import (
	"fmt"
	"io"
	"runtime"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

// outcome is what the event-loop part of the clipboard test found.
type clipboardOutcome struct {
	opened int
	err    error
}

// TestClipboardFilePromisesReachTheClipboardLazily puts a promise on the real
// OLE clipboard, reads it back as Explorer does, and checks that the file's
// content is produced only when the contents format is read, not when the
// clipboard is filled.
func TestClipboardFilePromisesReachTheClipboardLazily(t *testing.T) {
	events := make(chan platform.FileDropEvent, 4)
	state, closeWindow := openDropWindow(t, true, events)
	defer closeWindow()

	done := make(chan clipboardOutcome, 1)
	released := make(chan struct{}, 1)
	state.application.Post(func() {
		opened := 0
		file := platform.FilePromise{Name: "notes.txt", Size: 11, Open: func() (io.ReadCloser, error) {
			opened++
			return io.NopCloser(strings.NewReader("hello world")), nil
		}}
		if err := SetClipboardFilePromises([]platform.FilePromise{file}, func() { released <- struct{}{} }); err != nil {
			done <- clipboardOutcome{err: err}
			return
		}
		var data uintptr
		if status, _, _ := oleGetClipboard.Call(uintptr(unsafe.Pointer(&data))); hresult(status) != nil || data == 0 {
			done <- clipboardOutcome{opened: opened, err: fmt.Errorf("OleGetClipboard = %#x", status)}
			return
		}
		defer comCall0(data, 2) // Release the object OleGetClipboard handed over.
		descriptor := formatEtc{Format: fileGroupDescriptorFormat(), Aspect: dvAspectContent, Index: lindexNone, Medium: tymedHGlobal}
		var medium storageMedium
		if status := uint32(comCall2(data, 3, uintptr(unsafe.Pointer(&descriptor)), uintptr(unsafe.Pointer(&medium)))); status != sOK {
			done <- clipboardOutcome{opened: opened, err: fmt.Errorf("GetData(FileGroupDescriptorW) = %#x", status)}
			return
		}
		releaseStgMedium.Call(uintptr(unsafe.Pointer(&medium)))
		if opened != 0 {
			done <- clipboardOutcome{opened: opened, err: fmt.Errorf("the file was opened while only the descriptor was read")}
			return
		}
		contents := formatEtc{Format: fileContentsFormat(), Aspect: dvAspectContent, Index: 0, Medium: tymedIStream}
		if status := uint32(comCall2(data, 3, uintptr(unsafe.Pointer(&contents)), uintptr(unsafe.Pointer(&medium)))); status != sOK || medium.Medium != tymedIStream {
			done <- clipboardOutcome{opened: opened, err: fmt.Errorf("GetData(FileContents, 0) = %#x with medium %d", status, medium.Medium)}
			return
		}
		// A stream that returns fewer bytes than asked for says S_FALSE; the
		// shell takes either as the bytes it got.
		buffer := make([]byte, 11)
		var read uint32
		if status := uint32(comCall3(medium.Handle, 3, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)), uintptr(unsafe.Pointer(&read)))); status != sOK || read != 11 {
			done <- clipboardOutcome{opened: opened, err: fmt.Errorf("Read = %#x with %d bytes, want S_OK with 11", status, read)}
			return
		}
		if string(buffer) != "hello world" {
			done <- clipboardOutcome{opened: opened, err: fmt.Errorf("the bytes read are %q", buffer)}
			return
		}
		comCall0(medium.Handle, 2) // Release the stream.
		if opened != 1 {
			done <- clipboardOutcome{opened: opened, err: fmt.Errorf("the file was opened %d times while a paste read it, want 1", opened)}
			return
		}
		done <- clipboardOutcome{opened: opened}
	})
	outcome := <-done
	if outcome.err != nil {
		t.Fatal(outcome.err)
	}
	if outcome.opened != 1 {
		t.Fatalf("opened %d times", outcome.opened)
	}
	runtime.KeepAlive(state)

	// Emptying the clipboard releases the promise once, which is what tells the
	// application to let go of the remote data behind it.
	cleared := make(chan struct{})
	state.application.Post(func() {
		if status, _, _ := oleSetClipboard.Call(0); hresult(status) != nil {
			t.Errorf("OleSetClipboard(NULL) = %#x", status)
		}
		close(cleared)
	})
	<-cleared
	select {
	case <-released:
	case <-time.After(5 * time.Second):
		t.Fatal("the clipboard no longer holds the promise but its release callback never ran")
	}
	select {
	case <-released:
		t.Fatal("the release callback ran more than once")
	case <-time.After(100 * time.Millisecond):
	}
}

// TestClipboardFilePromisesWaitForTheEventLoop keeps the promise off a goroutine
// that does not own the window and its OLE apartment.
func TestClipboardFilePromisesWaitForTheEventLoop(t *testing.T) {
	events := make(chan platform.FileDropEvent, 4)
	state, closeWindow := openDropWindow(t, true, events)
	defer closeWindow()
	if state.application.thread == 0 {
		t.Fatal("the window's application has no event-loop thread")
	}
	file := platform.FilePromise{Name: "notes.txt", Open: func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("")), nil
	}}
	if err := SetClipboardFilePromises([]platform.FilePromise{file}, nil); err == nil {
		t.Fatal("a promise was put on the clipboard from another goroutine")
	} else if !strings.Contains(err.Error(), "event-loop thread") {
		t.Fatalf("the refusal says %q, want the event-loop thread", err)
	}
}

// TestReplacingAPromiseWithTextReleasesIt checks the path the application takes
// when the remote clipboard changes to text while its files are still promised:
// copying text over the promise must report the promise released, so the remote
// data behind it can be let go.
func TestReplacingAPromiseWithTextReleasesIt(t *testing.T) {
	events := make(chan platform.FileDropEvent, 4)
	state, closeWindow := openDropWindow(t, true, events)
	defer closeWindow()
	released := make(chan struct{}, 1)
	installed := make(chan error, 1)
	state.application.Post(func() {
		file := platform.FilePromise{Name: "notes.txt", Size: 11, Open: func() (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("hello world")), nil
		}}
		installed <- SetClipboardFilePromises([]platform.FilePromise{file}, func() { released <- struct{}{} })
	})
	if err := <-installed; err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	state.application.Post(func() {
		if err := SetClipboardText("copied text"); err != nil {
			t.Errorf("SetClipboardText = %v", err)
		}
		close(done)
	})
	<-done
	select {
	case <-released:
	case <-time.After(5 * time.Second):
		t.Fatal("text replaced the promised files but the promise was never released")
	}
	// Leave the clipboard as it was, with nothing promised.
	state.application.Post(func() { SetClipboardText("") })
}
