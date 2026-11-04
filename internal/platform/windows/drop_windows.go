//go:build windows

package windows

import (
	"sync"
	"syscall"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

// Files dragged from Explorer reach a window through OLE: the window registers an
// IDropTarget, and the shell reports the pointer entering, moving over and
// leaving it, and the drop. That is what lets the application say "not here" (the
// no-drop cursor) and light up a target while files hover over it. An elevated
// process receives no OLE drag from the ordinary-privilege Explorer, which blocks
// it between integrity levels, so it falls back to WM_DROPFILES, which carries the
// drop alone and which the message filter lets through.

const (
	wmDropFiles    = 0x0233
	wmCopyGlobal   = 0x0049 // the companion message of a drop from a process at another integrity level
	wmCopyData     = 0x004a
	msgfltAllow    = 1
	maxDroppedPath = 32768
	maxDroppedFile = 4096

	cfHDrop         = 15
	dvAspectContent = 1
	tymedHGlobal    = 1
	sOK             = 0
	eNoInterface    = 0x80004002
)

var (
	dragAcceptFiles       = shell.NewProc("DragAcceptFiles")
	dragQueryFile         = shell.NewProc("DragQueryFileW")
	dragQueryPoint        = shell.NewProc("DragQueryPoint")
	dragFinish            = shell.NewProc("DragFinish")
	changeWindowMsgFilter = user.NewProc("ChangeWindowMessageFilterEx")
	oleInitialize         = ole.NewProc("OleInitialize")
	oleUninitialize       = ole.NewProc("OleUninitialize")
	registerDragDrop      = ole.NewProc("RegisterDragDrop")
	revokeDragDrop        = ole.NewProc("RevokeDragDrop")
	releaseStgMedium      = ole.NewProc("ReleaseStgMedium")
	getCurrentProcess     = kernel.NewProc("GetCurrentProcess")
	openProcessToken      = advapi.NewProc("OpenProcessToken")
	getTokenInformation   = advapi.NewProc("GetTokenInformation")
	closeHandle           = kernel.NewProc("CloseHandle")
)

var (
	iidUnknown    = guid{0x00000000, 0x0000, 0x0000, [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}
	iidDropTarget = guid{0x00000122, 0x0000, 0x0000, [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}
)

// formatEtc is FORMATETC and storageMedium STGMEDIUM.
type formatEtc struct {
	Format uint16
	Device uintptr
	Aspect uint32
	Index  int32
	Medium uint32
}

type storageMedium struct {
	Medium  uint32
	Handle  uintptr
	Release uintptr
}

// dropTarget is the COM object a window registers: a pointer to the method table
// first, as every COM object begins. The window state owns it, so the garbage
// collector keeps it for as long as the shell may call it.
type dropTarget struct {
	table    *[7]uintptr
	refs     int32
	state    *windowState
	hasFiles bool // the drag in progress carries files
	accepts  bool // and the application takes them where the pointer is
}

var (
	dropTableOnce sync.Once
	dropTable     [7]uintptr
)

func newDropTable() {
	dropTable = [7]uintptr{
		guarded3("drag and drop: QueryInterface", dropQueryInterface),
		guarded1("drag and drop: AddRef", dropAddRef),
		guarded1("drag and drop: Release", dropRelease),
		guarded5("drag and drop: DragEnter", dropDragEnter),
		guarded4("drag and drop: DragOver", dropDragOver),
		guarded1("drag and drop: DragLeave", dropDragLeave),
		guarded5("drag and drop: Drop", dropDrop),
	}
}

func dropOf(this uintptr) *dropTarget { return (*dropTarget)(pointer(this)) }

func dropQueryInterface(this, riid, object uintptr) uintptr {
	requested := *(*guid)(pointer(riid))
	if requested != iidUnknown && requested != iidDropTarget {
		*(*uintptr)(pointer(object)) = 0
		return eNoInterface
	}
	*(*uintptr)(pointer(object)) = this
	dropOf(this).refs++
	return sOK
}

func dropAddRef(this uintptr) uintptr {
	target := dropOf(this)
	target.refs++
	return uintptr(target.refs)
}

func dropRelease(this uintptr) uintptr {
	target := dropOf(this)
	if target.refs > 0 {
		target.refs--
	}
	return uintptr(target.refs)
}

// containsFiles reports whether a data object offers a list of files.
func containsFiles(data uintptr) bool {
	format := formatEtc{Format: cfHDrop, Aspect: dvAspectContent, Index: -1, Medium: tymedHGlobal}
	return data != 0 && uint32(comCall(data, 5, uintptr(unsafe.Pointer(&format)))) == sOK
}

// filesOf reads the paths out of a data object that offers files.
func filesOf(data uintptr) []string {
	format := formatEtc{Format: cfHDrop, Aspect: dvAspectContent, Index: -1, Medium: tymedHGlobal}
	var medium storageMedium
	if hresult(comCall(data, 3, uintptr(unsafe.Pointer(&format)), uintptr(unsafe.Pointer(&medium)))) != nil {
		return nil
	}
	defer releaseStgMedium.Call(uintptr(unsafe.Pointer(&medium)))
	if medium.Medium != tymedHGlobal {
		return nil
	}
	return droppedPaths(medium.Handle)
}

// point turns a screen point the shell reports (a POINTL passed by value, which
// the calling convention packs into one register) into logical window pixels.
func (target *dropTarget) point(packed uintptr) (x, y float32) {
	screen := point{X: int32(uint32(packed)), Y: int32(uint32(packed >> 32))}
	screenToClient.Call(target.state.window, uintptr(unsafe.Pointer(&screen)))
	scale := target.state.scale
	return float32(screen.X) / scale, float32(screen.Y) / scale
}

// step tells the application one step of the drag and records whether the point
// takes files.
func (target *dropTarget) step(phase platform.FileDropPhase, x, y float32, paths []string) {
	state := target.state
	if state.config.OnFileDrop == nil {
		return
	}
	frame, accepted, err := state.config.OnFileDrop(platform.FileDropEvent{Phase: phase, X: x, Y: y, Paths: paths})
	target.accepts = accepted
	state.frame(frame, err)
}

func (target *dropTarget) answer(effect uintptr) {
	allowed := *(*uint32)(pointer(effect))
	*(*uint32)(pointer(effect)) = platform.DropEffect(target.hasFiles && target.accepts, allowed)
}

func dropDragEnter(this, data, keys, packed, effect uintptr) uintptr {
	target := dropOf(this)
	target.hasFiles, target.accepts = containsFiles(data), false
	if target.hasFiles {
		x, y := target.point(packed)
		target.step(platform.FileDropOver, x, y, nil)
	}
	target.answer(effect)
	return sOK
}

func dropDragOver(this, keys, packed, effect uintptr) uintptr {
	target := dropOf(this)
	if target.hasFiles {
		x, y := target.point(packed)
		target.step(platform.FileDropOver, x, y, nil)
	}
	target.answer(effect)
	return sOK
}

func dropDragLeave(this uintptr) uintptr {
	target := dropOf(this)
	if target.hasFiles {
		target.step(platform.FileDropLeave, 0, 0, nil)
	}
	target.hasFiles, target.accepts = false, false
	return sOK
}

func dropDrop(this, data, keys, packed, effect uintptr) uintptr {
	target := dropOf(this)
	allowed := *(*uint32)(pointer(effect))
	*(*uint32)(pointer(effect)) = 0
	if !target.hasFiles && !containsFiles(data) {
		return sOK
	}
	target.hasFiles = false
	paths := filesOf(data)
	if len(paths) == 0 {
		target.step(platform.FileDropLeave, 0, 0, nil)
		return sOK
	}
	x, y := target.point(packed)
	target.step(platform.FileDropDrop, x, y, paths)
	*(*uint32)(pointer(effect)) = platform.DropEffect(target.accepts, allowed)
	target.accepts = false
	return sOK
}

// elevated reports whether this process runs with an administrator's token.
func elevated() bool {
	process, _, _ := getCurrentProcess.Call()
	var token uintptr
	if ok, _, _ := openProcessToken.Call(process, 8 /* TOKEN_QUERY */, uintptr(unsafe.Pointer(&token))); ok == 0 {
		return false
	}
	defer closeHandle.Call(token)
	var elevation, size uint32
	ok, _, _ := getTokenInformation.Call(token, 20 /* TokenElevation */, uintptr(unsafe.Pointer(&elevation)), 4, uintptr(unsafe.Pointer(&size)))
	return ok != 0 && elevation != 0
}

// acceptFiles makes the window a target for files dragged from Explorer.
func (state *windowState) acceptFiles() {
	if state.config.OnFileDrop == nil {
		return
	}
	if !elevated() && state.registerDropTarget() {
		return
	}
	state.acceptDropMessages()
}

// acceptDropMessages makes the window take files as WM_DROPFILES, which carries
// the drop alone. The message filter lets a drop through from an Explorer running
// at a lower integrity level than an elevated application, which Windows blocks
// otherwise.
func (state *windowState) acceptDropMessages() {
	if changeWindowMsgFilter.Find() == nil {
		changeWindowMsgFilter.Call(state.window, wmDropFiles, msgfltAllow, 0)
		changeWindowMsgFilter.Call(state.window, wmCopyGlobal, msgfltAllow, 0)
		changeWindowMsgFilter.Call(state.window, wmCopyData, msgfltAllow, 0)
	}
	dragAcceptFiles.Call(state.window, 1)
}

// registerDropTarget registers the window's IDropTarget, and reports whether OLE
// took it.
func (state *windowState) registerDropTarget() bool {
	if !state.application.oleReady {
		if status, _, _ := oleInitialize.Call(0); int32(uint32(status)) < 0 {
			return false
		}
		state.application.oleReady = true
	}
	dropTableOnce.Do(newDropTable)
	target := &dropTarget{table: &dropTable, refs: 1, state: state}
	if status, _, _ := registerDragDrop.Call(state.window, uintptr(unsafe.Pointer(target))); hresult(status) != nil {
		return false
	}
	state.drop = target
	return true
}

// closeDrop takes the window out of the shell's list of drop targets.
func (state *windowState) closeDrop() {
	if state.drop != nil && state.window != 0 {
		revokeDragDrop.Call(state.window)
	}
	state.drop = nil
}

// droppedPaths reads the file names out of a drop handle.
func droppedPaths(drop uintptr) []string {
	count, _, _ := dragQueryFile.Call(drop, 0xFFFFFFFF, 0, 0)
	if count > maxDroppedFile {
		count = maxDroppedFile
	}
	paths := make([]string, 0, count)
	for index := uintptr(0); index < count; index++ {
		length, _, _ := dragQueryFile.Call(drop, index, 0, 0)
		if length == 0 || length > maxDroppedPath {
			continue
		}
		buffer := make([]uint16, length+1)
		written, _, _ := dragQueryFile.Call(drop, index, uintptr(unsafe.Pointer(&buffer[0])), length+1)
		if written > 0 {
			paths = append(paths, syscall.UTF16ToString(buffer[:written]))
		}
	}
	return paths
}

// handleDropFiles reports a drop that arrived as WM_DROPFILES, the elevated
// process's way, and releases the handle.
func (state *windowState) handleDropFiles(drop uintptr) {
	defer dragFinish.Call(drop)
	if state.config.OnFileDrop == nil {
		return
	}
	var position point
	dragQueryPoint.Call(drop, uintptr(unsafe.Pointer(&position)))
	paths := droppedPaths(drop)
	if len(paths) == 0 {
		return
	}
	frame, _, err := state.config.OnFileDrop(platform.FileDropEvent{
		Phase: platform.FileDropDrop,
		X:     float32(position.X) / state.scale, Y: float32(position.Y) / state.scale,
		Paths: paths,
	})
	state.frame(frame, err)
}

// closeOLE ends the application's use of OLE, once every window has revoked its
// drop target. The taskbar list goes first: it is an OLE object, and it cannot
// outlive the thread's use of OLE.
func (application *windowsApplication) closeOLE() {
	release(&application.taskbar)
	if application.oleReady {
		oleUninitialize.Call()
		application.oleReady = false
	}
}
