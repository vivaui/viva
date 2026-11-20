//go:build windows

package windows

import (
	"fmt"
	"runtime"
	"sync"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

// Dragging files out of a window is the other half of drop_windows.go: there the
// shell owns a drag and asks the window, here the window owns one and asks the
// shell. Windows has one call for it, DoDragDrop, and it needs two COM objects
// from the application: an IDataObject that hands out the files, and an IDropSource
// that says what ends the drag and which cursors it shows. Both are built here of
// the callbacks a COM object always is, as dropTarget is.
//
// DoDragDrop runs the drag itself: it takes the mouse capture, pumps the messages
// the drag generates, and returns when the pointer is released or Escape is
// pressed, with the effect the target took. It is therefore called on the
// event-loop thread, from the handler the press reached, and this package's event
// loop is inside it until it returns rather than beside it. A message arriving
// meanwhile — a repaint, a timer, another window's input — is dispatched by that
// drag loop into the same window procedure, so the handler the drag started from
// can be reentered; a window move's modal loop already does the same, and the
// window's state machine takes it the same way.
//
// The files are offered as CF_HDROP, the list a copied selection puts on the
// clipboard (platform.DropFilesBytes), because that is what Explorer, a folder
// window and every other file target read. Nothing is copied when the drag
// starts: a target that takes a copy or a move reads the paths while the drop
// happens, so the files must still be where they were until the user lets go.
// A target in another process reads them through OLE's standard IDataObject
// marshaling, which asks EnumFormatEtc and GetData for a list in shared memory
// rather than for the object, so those two are what makes a drop out of the
// process work; a link needs a shell link the object does not offer, so a target
// told it may link leaves it to the modifiers or copies.

const (
	// The answers an IDropSource gives: end with a drop, end with a cancel, and
	// keep the drag going. S_OK says the drag has not ended.
	dragdropSDrop              = 0x00040100
	dragdropSCancel            = 0x00040101
	dragdropSUseDefaultCursors = 0x00040102
	// DV_E_FORMATETC: the object does not offer that format.
	dvEFormatEtc = 0x80040064
	// OLE_E_ADVISENOTSUPPORTED: the object takes no advice.
	oleEAdviseNotSupported = 0x80040003
	// E_NOTIMPL: the object does not do that at all.
	eNotImpl = 0x80004001
	// S_FALSE: an enumerator had less to give than it was asked for.
	sFalse = 1
	// DATADIR_GET enumerates the formats GetData reads.
	dataDirGet = 1
	lindexNone = -1
)

var doDragDrop = ole.NewProc("DoDragDrop")

var (
	iidDataObject    = guid{0x0000010e, 0x0000, 0x0000, [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}
	iidDropSource    = guid{0x00000121, 0x0000, 0x0000, [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}
	iidEnumFORMATETC = guid{0x00000103, 0x0000, 0x0000, [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}
)

// fileDataObject is the IDataObject of a drag of files: a pointer to the method
// table first, as every COM object begins, then the paths it hands out. The
// drag owns it, so the garbage collector keeps it for as long as the shell may
// call it.
type fileDataObject struct {
	table *[12]uintptr
	refs  int32
	paths []string
	// enums are the enumerators given away. The shell holds them by a pointer
	// the garbage collector cannot see, so the object keeps them for as long as
	// the drag it belongs to lives. Only the drag's own thread appends.
	enums []*fileFormatEnum
}

// fileDropSource is the IDropSource of the same drag: what ends it and which
// cursors it shows.
type fileDropSource struct {
	table *[5]uintptr
	refs  int32
}

// fileFormatEnum is the IEnumFORMATETC that lists what the data object offers:
// one format, the list of files.
type fileFormatEnum struct {
	table *[7]uintptr
	refs  int32
	at    uint32 // how many formats it has given
	// owner is the data object whose drag keeps this enumerator alive, and every
	// clone of it: the shell holds them by pointers the collector cannot follow.
	owner *fileDataObject
}

var (
	dataTableOnce   sync.Once
	dataTable       [12]uintptr
	sourceTableOnce sync.Once
	sourceTable     [5]uintptr
	enumTableOnce   sync.Once
	enumTable       [7]uintptr
)

func newFileDataTable() {
	dataTable = [12]uintptr{
		guarded3("drag and drop: QueryInterface", dataQueryInterface),
		guarded1("drag and drop: AddRef", dataAddRef),
		guarded1("drag and drop: Release", dataRelease),
		guarded3("drag and drop: GetData", dataGetData),
		guarded3("drag and drop: GetDataHere", dataGetDataHere),
		guarded2("drag and drop: QueryGetData", dataQueryGetData),
		guarded3("drag and drop: GetCanonicalFormatEtc", dataGetCanonicalFormatEtc),
		guarded4("drag and drop: SetData", dataSetData),
		guarded3("drag and drop: EnumFormatEtc", dataEnumFormatEtc),
		guarded5("drag and drop: DAdvise", dataDAdvise),
		guarded2("drag and drop: DUnadvise", dataDUnadvise),
		guarded2("drag and drop: EnumDAdvise", dataEnumDAdvise),
	}
}

func newFileSourceTable() {
	sourceTable = [5]uintptr{
		guarded3("drag and drop: QueryInterface", sourceQueryInterface),
		guarded1("drag and drop: AddRef", sourceAddRef),
		guarded1("drag and drop: Release", sourceRelease),
		guarded3("drag and drop: QueryContinueDrag", sourceQueryContinueDrag),
		guarded2("drag and drop: GiveFeedback", sourceGiveFeedback),
	}
}

func newFileEnumTable() {
	enumTable = [7]uintptr{
		guarded3("drag and drop: QueryInterface", enumQueryInterface),
		guarded1("drag and drop: AddRef", enumAddRef),
		guarded1("drag and drop: Release", enumRelease),
		guarded4("drag and drop: Next", enumNext),
		guarded2("drag and drop: Skip", enumSkip),
		guarded1("drag and drop: Reset", enumReset),
		guarded2("drag and drop: Clone", enumClone),
	}
}

func dataOf(this uintptr) *fileDataObject   { return (*fileDataObject)(pointer(this)) }
func sourceOf(this uintptr) *fileDropSource { return (*fileDropSource)(pointer(this)) }
func enumOf(this uintptr) *fileFormatEnum   { return (*fileFormatEnum)(pointer(this)) }

// fileFormat is the one FORMATETC the data object offers: a list of files in
// memory the system owns, its index meaning "the whole list".
func fileFormat() formatEtc {
	return formatEtc{Format: cfHDrop, Aspect: dvAspectContent, Index: lindexNone, Medium: tymedHGlobal}
}

// takesFileFormat reports whether a FORMATETC asks for that list.
func takesFileFormat(asked formatEtc) bool {
	offered := fileFormat()
	return asked.Format == offered.Format &&
		asked.Aspect&dvAspectContent != 0 &&
		(asked.Index == lindexNone || asked.Index == 0) &&
		asked.Medium&tymedHGlobal != 0
}

// answerInterface is every COM object's QueryInterface: the object answers a
// request for one of its own interfaces or for IUnknown, and refuses the rest.
// refs is the object's own count, which a granted request adds to.
func answerInterface(this, riid, object uintptr, refs *int32, known ...guid) uintptr {
	requested := *(*guid)(pointer(riid))
	matched := requested == iidUnknown
	for _, iid := range known {
		if requested == iid {
			matched = true
			break
		}
	}
	if !matched {
		*(*uintptr)(pointer(object)) = 0
		return eNoInterface
	}
	*(*uintptr)(pointer(object)) = this
	*refs++
	return sOK
}

// dropListHandle copies the files into the shared memory a drop target reads
// them from. The caller owns the memory and hands it to the target through the
// storage medium, or frees it.
func dropListHandle(paths []string) (uintptr, error) {
	data := platform.DropFilesBytes(paths)
	handle, _, callErr := globalAlloc.Call(globalMoveable|globalZeroInit, uintptr(len(data)))
	if handle == 0 {
		return 0, windowsCallError("allocate dragged files", callErr)
	}
	memory, _, callErr := globalLock.Call(handle)
	if memory == 0 {
		globalFree.Call(handle)
		return 0, windowsCallError("lock dragged files", callErr)
	}
	copy(unsafe.Slice((*byte)(pointer(memory)), len(data)), data)
	globalUnlock.Call(handle)
	return handle, nil
}

func dataQueryInterface(this, riid, object uintptr) uintptr {
	return answerInterface(this, riid, object, &dataOf(this).refs, iidDataObject)
}

func dataAddRef(this uintptr) uintptr {
	data := dataOf(this)
	data.refs++
	return uintptr(data.refs)
}

func dataRelease(this uintptr) uintptr {
	data := dataOf(this)
	if data.refs > 0 {
		data.refs--
	}
	return uintptr(data.refs)
}

// dataGetData hands the target a fresh copy of the list: OLE gives the memory to
// the target, and the next question asks for another.
func dataGetData(this, format, medium uintptr) uintptr {
	if !takesFileFormat(*(*formatEtc)(pointer(format))) {
		return dvEFormatEtc
	}
	handle, err := dropListHandle(dataOf(this).paths)
	if err != nil {
		return eFail
	}
	*(*storageMedium)(pointer(medium)) = storageMedium{Medium: tymedHGlobal, Handle: handle}
	return sOK
}

func dataGetDataHere(this, format, medium uintptr) uintptr { return eNotImpl }

func dataQueryGetData(this, format uintptr) uintptr {
	if !takesFileFormat(*(*formatEtc)(pointer(format))) {
		return dvEFormatEtc
	}
	return sOK
}

func dataGetCanonicalFormatEtc(this, format, canonical uintptr) uintptr { return eNotImpl }

func dataSetData(this, format, medium, release uintptr) uintptr { return eNotImpl }

// dataEnumFormatEtc lends the target an enumerator over the formats it may ask
// for, which is what OLE's standard marshaling reads to learn what the object
// has: one format, the list of files.
func dataEnumFormatEtc(this, direction, out uintptr) uintptr {
	if uint32(direction) != dataDirGet {
		if out != 0 {
			*(*uintptr)(pointer(out)) = 0
		}
		return eNotImpl
	}
	enumTableOnce.Do(newFileEnumTable)
	data := dataOf(this)
	enum := &fileFormatEnum{table: &enumTable, refs: 1, owner: data}
	data.enums = append(data.enums, enum)
	*(*uintptr)(pointer(out)) = uintptr(unsafe.Pointer(enum))
	return sOK
}

func dataDAdvise(this, format, flags, advise, out uintptr) uintptr {
	return oleEAdviseNotSupported
}

func dataDUnadvise(this, connection uintptr) uintptr { return oleEAdviseNotSupported }

func dataEnumDAdvise(this, out uintptr) uintptr {
	if out != 0 {
		*(*uintptr)(pointer(out)) = 0
	}
	return oleEAdviseNotSupported
}

func sourceQueryInterface(this, riid, object uintptr) uintptr {
	return answerInterface(this, riid, object, &sourceOf(this).refs, iidDropSource)
}

func sourceAddRef(this uintptr) uintptr {
	source := sourceOf(this)
	source.refs++
	return uintptr(source.refs)
}

func sourceRelease(this uintptr) uintptr {
	source := sourceOf(this)
	if source.refs > 0 {
		source.refs--
	}
	return uintptr(source.refs)
}

// sourceQueryContinueDrag answers whether the drag goes on. Escape cancels it as
// every Windows application does, and letting go of the last mouse button ends
// it with a drop; the target still decides whether it takes anything. The
// modifier keys are left to the target, which is what the default cursors show.
func sourceQueryContinueDrag(this, escape, keys uintptr) uintptr {
	if escape != 0 {
		return dragdropSCancel
	}
	if keys&mkAnyButton == 0 {
		return dragdropSDrop
	}
	return sOK
}

// sourceGiveFeedback asks for the shell's own cursors, the copy, move and no-drop
// arrows the user knows, rather than one drawn here.
func sourceGiveFeedback(this, effect uintptr) uintptr { return dragdropSUseDefaultCursors }

func enumQueryInterface(this, riid, object uintptr) uintptr {
	return answerInterface(this, riid, object, &enumOf(this).refs, iidEnumFORMATETC)
}

func enumAddRef(this uintptr) uintptr {
	enum := enumOf(this)
	enum.refs++
	return uintptr(enum.refs)
}

func enumRelease(this uintptr) uintptr {
	enum := enumOf(this)
	if enum.refs > 0 {
		enum.refs--
	}
	return uintptr(enum.refs)
}

func enumNext(this, count, formats, fetched uintptr) uintptr {
	enum := enumOf(this)
	given := uint32(0)
	if asked := uint32(count); asked > 0 && enum.at == 0 {
		*(*formatEtc)(pointer(formats)) = fileFormat()
		enum.at = 1
		given = 1
	}
	if fetched != 0 {
		*(*uint32)(pointer(fetched)) = given
	}
	if given != uint32(count) {
		return sFalse
	}
	return sOK
}

func enumSkip(this, count uintptr) uintptr {
	enum := enumOf(this)
	if uint32(count) == 0 {
		return sOK
	}
	if enum.at != 0 {
		return sFalse
	}
	enum.at = 1
	return sOK
}

func enumReset(this uintptr) uintptr {
	enumOf(this).at = 0
	return sOK
}

func enumClone(this, out uintptr) uintptr {
	enum := enumOf(this)
	clone := &fileFormatEnum{table: &enumTable, refs: 1, at: enum.at, owner: enum.owner}
	if owner := clone.owner; owner != nil {
		owner.enums = append(owner.enums, clone)
	}
	*(*uintptr)(pointer(out)) = uintptr(unsafe.Pointer(clone))
	return sOK
}

// StartFileDrag takes files out of the application's active window, as a file
// manager does when the user drags a selection to another window: the drag lasts
// until the pointer is released or Escape is pressed, and the effect the target
// took is returned, DragEffectNone when it took nothing.
//
// allowed is the mask of effects the application offers; a target that takes one
// of them reports it. It is called on the event-loop thread, from the handler of
// the pointer event that starts the drag, and does not return until the drag has
// ended. The files are read while the drop happens, so they must still exist
// where they were when the user lets go.
func StartFileDrag(paths []string, allowed platform.DragEffect) (platform.DragEffect, error) {
	state := active.Load()
	if state == nil || state.window == 0 {
		return platform.DragEffectNone, fmt.Errorf("windows: no window to drag files out of")
	}
	thread, _, _ := getCurrentThread.Call()
	if uint32(thread) != state.application.thread {
		return platform.DragEffectNone, fmt.Errorf("windows: files are dragged out on the event-loop thread that owns the window")
	}
	if len(paths) == 0 {
		return platform.DragEffectNone, fmt.Errorf("windows: no files to drag out of the window")
	}
	// DoDragDrop needs OLE, which CoInitializeEx does not set up: the drag-and-drop
	// services answer CO_E_NOTINITIALIZED until OleInitialize has run on the
	// thread. A window that takes drops asks the same way (registerDropTarget),
	// and closeOLE gives it back once.
	if !state.application.oleReady {
		if status, _, _ := oleInitialize.Call(0); int32(uint32(status)) < 0 {
			return platform.DragEffectNone, hresult(status)
		}
		state.application.oleReady = true
	}
	dataTableOnce.Do(newFileDataTable)
	sourceTableOnce.Do(newFileSourceTable)
	data := &fileDataObject{table: &dataTable, refs: 1, paths: paths}
	source := &fileDropSource{table: &sourceTable, refs: 1}
	var effect uint32
	// DoDragDrop takes no window: it finds the drag's source from the capture the
	// press took, which is why it is the window the user pressed in that owns the
	// drag. The parameters are the data object, the drop source, the effects the
	// application offers, and where the effect the target took is written.
	status, _, _ := doDragDrop.Call(
		uintptr(unsafe.Pointer(data)),
		uintptr(unsafe.Pointer(source)),
		uintptr(allowed),
		uintptr(unsafe.Pointer(&effect)),
	)
	// The objects outlive the drag: the shell held them by pointers the garbage
	// collector cannot follow until DoDragDrop let them go.
	runtime.KeepAlive(data)
	runtime.KeepAlive(source)
	if err := hresult(status); err != nil {
		return platform.DragEffectNone, err
	}
	return platform.DragEffectOf(allowed, platform.DragEffect(effect)), nil
}
