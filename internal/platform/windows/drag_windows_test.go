//go:build windows

package windows

import (
	"bytes"
	"runtime"
	"slices"
	"strings"
	"testing"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

// newDragObject is a data object for paths, as a drag of them builds it. The
// caller keeps it alive for as long as it drives it, as StartFileDrag does: the
// shell between them holds it by a pointer the collector cannot follow.
func newDragObject(paths []string) (*fileDataObject, uintptr) {
	dataTableOnce.Do(newFileDataTable)
	data := &fileDataObject{table: &dataTable, refs: 1, paths: paths}
	return data, uintptr(unsafe.Pointer(data))
}

func newDragSource() (*fileDropSource, uintptr) {
	sourceTableOnce.Do(newFileSourceTable)
	source := &fileDropSource{table: &sourceTable, refs: 1}
	return source, uintptr(unsafe.Pointer(source))
}

// TestTheDataObjectHandsOutTheFilesAsADropList asks the object for the files the
// way a target does and reads them back as a list of paths, which is all a drag
// of files ever carries.
func TestTheDataObjectHandsOutTheFilesAsADropList(t *testing.T) {
	paths := []string{`C:\Temp\one.txt`, `C:\Temp\文件夹`, `D:\x`}
	data, object := newDragObject(paths)
	defer runtime.KeepAlive(data)
	format := fileFormat()
	if status := uint32(comCall(object, 5, uintptr(unsafe.Pointer(&format)))); status != sOK {
		t.Fatalf("QueryGetData = %#x, want S_OK", status)
	}
	var medium storageMedium
	if status := uint32(comCall(object, 3, uintptr(unsafe.Pointer(&format)), uintptr(unsafe.Pointer(&medium)))); status != sOK {
		t.Fatalf("GetData = %#x, want S_OK", status)
	}
	defer releaseStgMedium.Call(uintptr(unsafe.Pointer(&medium)))
	if medium.Medium != tymedHGlobal || medium.Handle == 0 {
		t.Fatalf("the medium is %+v, want memory the target owns", medium)
	}
	want := platform.DropFilesBytes(paths)
	memory, _, _ := globalLock.Call(medium.Handle)
	if memory == 0 {
		t.Fatal("the memory the object handed over cannot be locked")
	}
	block := unsafe.Slice((*byte)(pointer(memory)), len(want))
	if !bytes.Equal(block, want) {
		t.Errorf("the block is %q, want %q", block, want)
	}
	globalUnlock.Call(medium.Handle)
	if back := platform.FilesFromDropFiles(append([]byte(nil), block...)); !slices.Equal(back, paths) {
		t.Errorf("the list reads back as %q, want %q", back, paths)
	}

	// Every answer is the target's own memory: a second question cannot be given
	// the block the first one took.
	var again storageMedium
	if status := uint32(comCall(object, 3, uintptr(unsafe.Pointer(&format)), uintptr(unsafe.Pointer(&again)))); status != sOK {
		t.Fatalf("the second GetData = %#x, want S_OK", status)
	}
	defer releaseStgMedium.Call(uintptr(unsafe.Pointer(&again)))
	if again.Handle == medium.Handle {
		t.Error("both answers are the same memory, so the target would free it twice")
	}
}

// TestTheDataObjectRefusesWhatItDoesNotOffer keeps the formats a target asks for
// that a drag of files cannot answer from being taken as an empty answer.
func TestTheDataObjectRefusesWhatItDoesNotOffer(t *testing.T) {
	data, object := newDragObject([]string{`C:\Temp\one.txt`})
	defer runtime.KeepAlive(data)
	text := formatEtc{Format: 13 /* CF_UNICODETEXT */, Aspect: dvAspectContent, Index: lindexNone, Medium: tymedHGlobal}
	if status := uint32(comCall(object, 5, uintptr(unsafe.Pointer(&text)))); status != dvEFormatEtc {
		t.Errorf("QueryGetData of text = %#x, want DV_E_FORMATETC", status)
	}
	var medium storageMedium
	if status := uint32(comCall(object, 3, uintptr(unsafe.Pointer(&text)), uintptr(unsafe.Pointer(&medium)))); status != dvEFormatEtc {
		t.Errorf("GetData of text = %#x, want DV_E_FORMATETC", status)
	}
	thumbnail := fileFormat()
	thumbnail.Aspect = 2 // DVASPECT_THUMBNAIL
	if status := uint32(comCall(object, 5, uintptr(unsafe.Pointer(&thumbnail)))); status != dvEFormatEtc {
		t.Errorf("QueryGetData of a thumbnail = %#x, want DV_E_FORMATETC", status)
	}
	stream := fileFormat()
	stream.Medium = 4 // TYMED_ISTREAM
	if status := uint32(comCall(object, 5, uintptr(unsafe.Pointer(&stream)))); status != dvEFormatEtc {
		t.Errorf("QueryGetData of a stream = %#x, want DV_E_FORMATETC", status)
	}
	if status := uint32(comCall(object, 4, uintptr(unsafe.Pointer(&stream)), uintptr(unsafe.Pointer(&medium)))); status != eNotImpl {
		t.Errorf("GetDataHere = %#x, want E_NOTIMPL", status)
	}
	if status := uint32(comCall(object, 9, uintptr(unsafe.Pointer(&stream)), 0, 0, 0)); status != oleEAdviseNotSupported {
		t.Errorf("DAdvise = %#x, want OLE_E_ADVISENOTSUPPORTED", status)
	}
	if status := uint32(comCall(object, 10, 1)); status != oleEAdviseNotSupported {
		t.Errorf("DUnadvise = %#x, want OLE_E_ADVISENOTSUPPORTED", status)
	}
}

// TestTheFormatEnumeratorListsTheDropList walks the enumerator OLE's standard
// marshaling reads: one format, the list of files, then nothing more.
func TestTheFormatEnumeratorListsTheDropList(t *testing.T) {
	data, object := newDragObject([]string{`C:\Temp\one.txt`})
	defer runtime.KeepAlive(data)
	var enum uintptr
	if status := uint32(comCall(object, 8, dataDirGet, uintptr(unsafe.Pointer(&enum)))); status != sOK {
		t.Fatalf("EnumFormatEtc = %#x, want S_OK", status)
	}
	if enum == 0 {
		t.Fatal("EnumFormatEtc handed out no enumerator")
	}
	var offered formatEtc
	var fetched uint32
	if status := uint32(comCall(enum, 3, 1, uintptr(unsafe.Pointer(&offered)), uintptr(unsafe.Pointer(&fetched)))); status != sOK || fetched != 1 {
		t.Fatalf("Next(1) = %#x with %d formats, want S_OK with 1", status, fetched)
	}
	if offered.Format != cfHDrop || offered.Aspect != dvAspectContent || offered.Index != lindexNone || offered.Medium != tymedHGlobal {
		t.Errorf("the format is %+v, want the list of files", offered)
	}
	if status := uint32(comCall(enum, 3, 1, uintptr(unsafe.Pointer(&offered)), uintptr(unsafe.Pointer(&fetched)))); status != sFalse || fetched != 0 {
		t.Errorf("Next after the end = %#x with %d formats, want S_FALSE with 0", status, fetched)
	}
	comCall(enum, 5) // Reset
	if status := uint32(comCall(enum, 3, 2, uintptr(unsafe.Pointer(&offered)), uintptr(unsafe.Pointer(&fetched)))); status != sFalse || fetched != 1 {
		t.Errorf("Next(2) = %#x with %d formats, want S_FALSE with 1", status, fetched)
	}
	comCall(enum, 5)
	comCall(enum, 4, 1) // Skip
	if status := uint32(comCall(enum, 3, 1, uintptr(unsafe.Pointer(&offered)), uintptr(unsafe.Pointer(&fetched)))); status != sFalse || fetched != 0 {
		t.Errorf("Next after Skip = %#x with %d formats, want S_FALSE with 0", status, fetched)
	}
	comCall(enum, 5)
	var clone uintptr
	if status := uint32(comCall(enum, 6, uintptr(unsafe.Pointer(&clone)))); status != sOK || clone == 0 {
		t.Fatalf("Clone = %#x, want S_OK with an enumerator", status)
	}
	if status := uint32(comCall(clone, 3, 1, uintptr(unsafe.Pointer(&offered)), uintptr(unsafe.Pointer(&fetched)))); status != sOK || fetched != 1 {
		t.Errorf("the clone's Next = %#x with %d formats, want S_OK with 1", status, fetched)
	}
}

// TestTheDropSourceEndsTheDragWhereWindowsDoes checks the three answers a drag
// gives: Escape cancels, letting go of the mouse drops, and a drag that still
// holds a button goes on.
func TestTheDropSourceEndsTheDragWhereWindowsDoes(t *testing.T) {
	source, object := newDragSource()
	defer runtime.KeepAlive(source)
	for _, test := range []struct {
		escape uintptr
		keys   uintptr
		want   uintptr
	}{
		{1, mkLButton, dragdropSCancel},
		{1, 0, dragdropSCancel},
		{0, 0, dragdropSDrop},
		{0, mkLButton, sOK},
		{0, 2, sOK},           // MK_RBUTTON
		{0, 0x10 | 8, sOK},    // MK_MBUTTON with MK_CONTROL beside it
		{0, 8, dragdropSDrop}, // no button, only a modifier
	} {
		if got := uint32(comCall(object, 3, test.escape, test.keys)); got != uint32(test.want) {
			t.Errorf("QueryContinueDrag(escape=%d, keys=%#x) = %#x, want %#x", test.escape, test.keys, got, uint32(test.want))
		}
	}
	if got := uint32(comCall(object, 4, uintptr(platform.DragEffectCopy))); got != dragdropSUseDefaultCursors {
		t.Errorf("GiveFeedback = %#x, want DRAGDROP_S_USEDEFAULTCURSORS", got)
	}
}

// TestTheObjectsAnswerForTheirOwnInterfaces is what OLE asks before it hands an
// object to another process, and a wrong answer there fails the drop with
// nothing said.
func TestTheObjectsAnswerForTheirOwnInterfaces(t *testing.T) {
	data, object := newDragObject([]string{`C:\Temp\one.txt`})
	defer runtime.KeepAlive(data)
	var asked uintptr
	if status := uint32(comCall(object, 0, uintptr(unsafe.Pointer(&iidDataObject)), uintptr(unsafe.Pointer(&asked)))); status != sOK || asked != object {
		t.Errorf("the data object answered IDataObject with %#x and %#x, want S_OK and itself", status, asked)
	}
	if data.refs != 2 {
		t.Errorf("the data object counts %d references after one grant, want 2", data.refs)
	}
	asked = 1
	if status := uint32(comCall(object, 0, uintptr(unsafe.Pointer(&iidDropSource)), uintptr(unsafe.Pointer(&asked)))); status != eNoInterface || asked != 0 {
		t.Errorf("the data object answered IDropSource with %#x and %#x, want E_NOINTERFACE and no object", status, asked)
	}
	source, sourceObject := newDragSource()
	defer runtime.KeepAlive(source)
	if status := uint32(comCall(sourceObject, 0, uintptr(unsafe.Pointer(&iidDropSource)), uintptr(unsafe.Pointer(&asked)))); status != sOK {
		t.Errorf("the drop source refused IDropSource with %#x", status)
	}
	if status := uint32(comCall(sourceObject, 0, uintptr(unsafe.Pointer(&iidDataObject)), uintptr(unsafe.Pointer(&asked)))); status != eNoInterface {
		t.Errorf("the drop source answered IDataObject with %#x, want E_NOINTERFACE", status)
	}
}

// TestAFileDragIsRefusedOffTheEventLoopThread keeps a drag from being started on
// a goroutine without the window's message loop, where DoDragDrop would wait for
// a pointer that never reaches it.
func TestAFileDragIsRefusedOffTheEventLoopThread(t *testing.T) {
	events := make(chan platform.FileDropEvent, 4)
	state, closeWindow := openDropWindow(t, true, events)
	defer closeWindow()
	if state.application.thread == 0 {
		t.Fatal("the window's application has no event-loop thread")
	}
	if _, err := StartFileDrag([]string{`C:\Temp\one.txt`}, platform.DragEffectCopy); err == nil {
		t.Fatal("a drag started from another goroutine")
	} else if !strings.Contains(err.Error(), "event-loop thread") {
		t.Fatalf("the refusal says %q, want the event-loop thread", err)
	}
}
