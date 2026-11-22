//go:build windows

package windows

import (
	"runtime"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

// buildDrop makes the memory Explorer hands over in WM_DROPFILES: a DROPFILES
// header followed by wide file names, each ended by a NUL, the list by another.
func buildDrop(t *testing.T, owned bool, x, y int32, names ...string) uintptr {
	t.Helper()
	type dropFiles struct {
		Files uint32
		X, Y  int32
		NC    int32
		Wide  int32
	}
	var list []uint16
	for _, name := range names {
		encoded, err := syscall.UTF16FromString(name)
		if err != nil {
			t.Fatal(err)
		}
		list = append(list, encoded...)
	}
	list = append(list, 0)
	header := dropFiles{Files: uint32(unsafe.Sizeof(dropFiles{})), X: x, Y: y, Wide: 1}
	size := uintptr(unsafe.Sizeof(header)) + uintptr(len(list))*2
	handle, _, err := globalAlloc.Call(globalMoveable|globalZeroInit, size)
	if handle == 0 {
		t.Fatal(err)
	}
	memory, _, _ := globalLock.Call(handle)
	*(*dropFiles)(pointer(memory)) = header
	copy(unsafe.Slice((*uint16)(pointer(memory+uintptr(unsafe.Sizeof(header)))), len(list)), list)
	globalUnlock.Call(handle)
	if owned {
		t.Cleanup(func() { globalFree.Call(handle) })
	}
	return handle
}

func TestDroppedPathsAreReadFromTheShellsHandle(t *testing.T) {
	drop := buildDrop(t, true, 30, 40, `C:\Users\me\Documents\a file.txt`, `C:\Temp\文件夹`, `D:\x`)
	paths := droppedPaths(drop)
	want := []string{`C:\Users\me\Documents\a file.txt`, `C:\Temp\文件夹`, `D:\x`}
	if len(paths) != len(want) {
		t.Fatalf("paths = %q", paths)
	}
	for index := range want {
		if paths[index] != want[index] {
			t.Errorf("path %d = %q, want %q", index, paths[index], want[index])
		}
	}
	var position point
	dragQueryPoint.Call(drop, uintptr(unsafe.Pointer(&position)))
	if position.X != 30 || position.Y != 40 {
		t.Errorf("point = %+v", position)
	}
}

func TestAnEmptyDropHasNoPaths(t *testing.T) {
	if paths := droppedPaths(buildDrop(t, true, 0, 0)); len(paths) != 0 {
		t.Fatalf("paths = %q", paths)
	}
}

var getWindowLongPtr = user.NewProc("GetWindowLongPtrW")

// openDropWindow opens a window that records what the application is told about
// files dragged over it, and returns its state and a function that closes it.
func openDropWindow(t *testing.T, accepts bool, events chan platform.FileDropEvent) (*windowState, func()) {
	t.Helper()
	done := make(chan error, 1)
	ready := make(chan *windowState, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		done <- RunApplication(func(application platform.Application) error {
			window, err := application.OpenWindow(platform.Config{
				Title: "drop", Width: 300, Height: 200,
				OnFileDrop: func(event platform.FileDropEvent) (platform.Frame, bool, error) {
					events <- event
					return platform.Frame{}, accepts, nil
				},
			})
			if err == nil {
				ready <- window.(*nativeWindow).state
			}
			return err
		})
	}()
	var state *windowState
	select {
	case state = <-ready:
	case err := <-done:
		t.Fatalf("window failed to open: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("the window never opened")
	}
	return state, func() {
		postMessage.Call(state.window, wmClose, 0, 0)
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Error("the window did not close")
		}
	}
}

// TestAWindowReportsFilesDroppedOnItAsAMessage sends a real window the message
// Explorer sends to a window that cannot take an OLE drag (an elevated process's),
// and checks what the application is told and that the window asked the system for
// drops at all.
func TestAWindowReportsFilesDroppedOnItAsAMessage(t *testing.T) {
	dropped := make(chan platform.FileDropEvent, 4)
	state, closeWindow := openDropWindow(t, true, dropped)
	defer closeWindow()
	state.acceptDropMessages()
	style, _, _ := getWindowLongPtr.Call(state.window, ^uintptr(19)) // GWL_EXSTYLE is -20
	if style&0x10 == 0 {                                             // WS_EX_ACCEPTFILES
		t.Fatalf("the window does not accept files: extended style %#x", style)
	}
	drop := buildDrop(t, false, int32(60*state.scale), int32(30*state.scale), `C:\Temp\one.txt`, `C:\Temp\two`)
	postMessage.Call(state.window, wmDropFiles, drop, 0)
	select {
	case event := <-dropped:
		if event.Phase != platform.FileDropDrop || len(event.Paths) != 2 || event.Paths[1] != `C:\Temp\two` {
			t.Fatalf("event = %+v", event)
		}
		if event.X < 59 || event.X > 61 || event.Y < 29 || event.Y > 31 {
			t.Fatalf("the point is %v,%v in logical pixels, want 60,30", event.X, event.Y)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the drop never reached the application")
	}
}

// fakeDataObject is an IDataObject that offers a list of files, as Explorer does,
// built of the same callbacks a COM object is.
type fakeDataObject struct {
	table *[12]uintptr
	drop  uintptr
}

var fakeDataTable [12]uintptr

func init() {
	unimplemented := syscall.NewCallback(func(this uintptr) uintptr { return 0x80004001 }) // E_NOTIMPL
	for index := range fakeDataTable {
		fakeDataTable[index] = unimplemented
	}
	fakeDataTable[1] = syscall.NewCallback(func(this uintptr) uintptr { return 2 })
	fakeDataTable[2] = syscall.NewCallback(func(this uintptr) uintptr { return 1 })
	// GetData
	fakeDataTable[3] = syscall.NewCallback(func(this, format, medium uintptr) uintptr {
		*(*storageMedium)(pointer(medium)) = storageMedium{Medium: tymedHGlobal, Handle: (*fakeDataObject)(pointer(this)).drop}
		return 0
	})
	// QueryGetData
	fakeDataTable[5] = syscall.NewCallback(func(this, format uintptr) uintptr {
		if (*formatEtc)(pointer(format)).Format == cfHDrop {
			return 0
		}
		return 0x80040064 // DV_E_FORMATETC
	})
}

// TestAWindowFollowsAnOLEDragOverIt drives the window's drop target the way the
// shell does: the pointer enters, moves, leaves, and enters again to drop.
func TestAWindowFollowsAnOLEDragOverIt(t *testing.T) {
	if elevated() {
		t.Skip("an elevated process takes files as messages")
	}
	events := make(chan platform.FileDropEvent, 8)
	state, closeWindow := openDropWindow(t, true, events)
	defer closeWindow()
	if state.drop == nil {
		t.Fatal("the window did not register a drop target")
	}
	target := uintptr(unsafe.Pointer(state.drop))
	data := &fakeDataObject{table: &fakeDataTable, drop: buildDrop(t, false, 0, 0, `C:\Temp\one.txt`, `C:\Temp\two`)}
	object := uintptr(unsafe.Pointer(data))
	var origin point
	clientToScreen.Call(state.window, uintptr(unsafe.Pointer(&origin)))
	at := func(x, y float32) uintptr {
		return uintptr(uint32(origin.X+int32(x*state.scale))) | uintptr(uint32(origin.Y+int32(y*state.scale)))<<32
	}
	next := func() platform.FileDropEvent {
		t.Helper()
		select {
		case event := <-events:
			return event
		case <-time.After(5 * time.Second):
			t.Fatal("the application was told nothing")
			return platform.FileDropEvent{}
		}
	}
	const copyEffect, allEffects = 1, 1 | 2 | 4
	effect := uint32(allEffects)
	comCall4(target, 3, object, 0, at(40, 20), uintptr(unsafe.Pointer(&effect)))
	if event := next(); event.Phase != platform.FileDropOver || event.X < 39 || event.X > 41 || event.Y < 19 || event.Y > 21 {
		t.Fatalf("enter reported %+v", event)
	}
	if effect != copyEffect {
		t.Fatalf("a point that takes files answered effect %d, want copy", effect)
	}
	effect = allEffects
	comCall3(target, 4, 0, at(60, 30), uintptr(unsafe.Pointer(&effect)))
	if event := next(); event.Phase != platform.FileDropOver || event.X < 59 || event.X > 61 {
		t.Fatalf("move reported %+v", event)
	}
	comCall0(target, 5)
	if event := next(); event.Phase != platform.FileDropLeave {
		t.Fatalf("leave reported %+v", event)
	}
	effect = allEffects
	comCall4(target, 3, object, 0, at(10, 10), uintptr(unsafe.Pointer(&effect)))
	next()
	effect = allEffects
	comCall4(target, 6, object, 0, at(70, 40), uintptr(unsafe.Pointer(&effect)))
	event := next()
	if event.Phase != platform.FileDropDrop || len(event.Paths) != 2 || event.Paths[0] != `C:\Temp\one.txt` || event.X < 69 || event.X > 71 {
		t.Fatalf("drop reported %+v", event)
	}
	if effect != copyEffect {
		t.Fatalf("an accepted drop answered effect %d, want copy", effect)
	}
}

// TestAWindowThatRefusesFilesShowsTheNoDropCursor checks that a point the
// application does not take answers "no effect", whatever the source allows.
func TestAWindowThatRefusesFilesShowsTheNoDropCursor(t *testing.T) {
	if elevated() {
		t.Skip("an elevated process takes files as messages")
	}
	events := make(chan platform.FileDropEvent, 8)
	state, closeWindow := openDropWindow(t, false, events)
	defer closeWindow()
	data := &fakeDataObject{table: &fakeDataTable, drop: buildDrop(t, true, 0, 0, `C:\Temp\one.txt`)}
	effect := uint32(1 | 2 | 4)
	comCall4(uintptr(unsafe.Pointer(state.drop)), 3, uintptr(unsafe.Pointer(data)), 0, 0, uintptr(unsafe.Pointer(&effect)))
	if effect != 0 {
		t.Fatalf("a refusing point answered effect %d", effect)
	}
}
