//go:build windows

package windows

import (
	"fmt"
	"io"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

// A promised drag offers files the application has not written yet: instead of
// CF_HDROP, its IDataObject hands the target a file group descriptor and, for
// each item, an IStream that the application fills as the target reads it. The
// shell's copy engine treats those as files: it creates them where the user let
// go, shows a progress dialog from the descriptor's sizes, and calls Read in
// order. This is what dragging a file off a remote server needs, because the
// file may not exist on this computer at all until the drop happens.
//
// The descriptor is the whole tree, so it is built before DoDragDrop starts:
// Windows has no way to add an item to a drag in progress. A very large folder
// is therefore enumerated before the pointer leaves the application (bounded by
// platform.MaxClipboardFiles).
//
// The shell may keep the streams after DoDragDrop returns, so the objects are
// kept in an explicit registry (promiseRoots) that the garbage collector cannot
// collect while the shell holds a raw pointer.

const (
	// TYMED_ISTREAM: the medium is an IStream the target reads and releases.
	tymedIStream = 4
	// FILEDESCRIPTOR flags.
	fdAttributes = 0x4
	fdFileSize   = 0x40
	fdProgressUI = 0x4000
	// File attributes.
	fileAttributeDirectory = 0x10
	// STG_E_READFAULT: the stream could not produce the bytes asked for.
	stgEReadFault = 0x8003001E
	// STATSTG types and modes.
	stgtYStream = 2
	stgmRead    = 0
	// IStream::Seek origins.
	streamSeekSet = 0
	streamSeekCur = 1
	// MAX_PATH is the name FILEDESCRIPTORW carries, NUL included.
	maxDescriptorName = 260
)

var (
	groupDescriptorOnce sync.Once
	groupDescriptorID   uint16
	fileContentsOnce    sync.Once
	fileContentsID      uint16
	iidStream           = guid{0x0000000c, 0x0000, 0x0000, [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}
)

// fileGroupDescriptorFormat is CFSTR_FILEDESCRIPTORW, the shell's name for the
// block of file descriptions a virtual-file drag offers.
func fileGroupDescriptorFormat() uint16 {
	groupDescriptorOnce.Do(func() {
		name, _ := syscall.UTF16PtrFromString("FileGroupDescriptorW")
		format, _, _ := registerClipboardFormat.Call(uintptr(unsafe.Pointer(name)))
		groupDescriptorID = uint16(format)
	})
	return groupDescriptorID
}

// fileContentsFormat is CFSTR_FILECONTENTS, the format whose lindex names the
// item an IStream produces.
func fileContentsFormat() uint16 {
	fileContentsOnce.Do(func() {
		name, _ := syscall.UTF16PtrFromString("FileContents")
		format, _, _ := registerClipboardFormat.Call(uintptr(unsafe.Pointer(name)))
		fileContentsID = uint16(format)
	})
	return fileContentsID
}

// fileDescriptorW mirrors FILEDESCRIPTORW: one item of the group descriptor.
type fileDescriptorW struct {
	Flags          uint32
	Clsid          guid
	Sizel          [2]int32
	Pointl         [2]int32
	Attributes     uint32
	CreationTime   fileTime
	LastAccessTime fileTime
	LastWriteTime  fileTime
	SizeHigh       uint32
	SizeLow        uint32
	Name           [maxDescriptorName]uint16
}

type fileTime struct{ Low, High uint32 }

// promiseItem is one flattened item of a promised drag: a file or a folder with
// its path inside the drag and the content callback of a file.
type promiseItem struct {
	name      string
	size      int64
	directory bool
	open      func() (io.ReadCloser, error)
}

// promiseDrag is the promised payload of a fileDataObject.
type promiseDrag struct {
	items      []promiseItem
	descriptor uint16
	contents   uint16
	// released runs once when the object's last reference is gone, which for a
	// promise put on the clipboard is when something replaced it. It is nil for
	// a drag, whose end is DoDragDrop returning.
	released func()
}

func (drag *promiseDrag) formatAt(index int) formatEtc {
	switch index {
	case 0:
		return formatEtc{Format: drag.descriptor, Aspect: dvAspectContent, Index: lindexNone, Medium: tymedHGlobal}
	case 1:
		return formatEtc{Format: drag.contents, Aspect: dvAspectContent, Index: lindexNone, Medium: tymedIStream}
	}
	return formatEtc{}
}

// takes reports whether the object can answer a FORMATETC. A contents request
// is answered for lindex -1 (the format as enumerated) and for each item.
func (drag *promiseDrag) takes(asked formatEtc) bool {
	if asked.Aspect&dvAspectContent == 0 {
		return false
	}
	switch asked.Format {
	case drag.descriptor:
		return asked.Index == lindexNone && asked.Medium&tymedHGlobal != 0
	case drag.contents:
		return asked.Index >= lindexNone && int(asked.Index) < len(drag.items) && asked.Medium&tymedIStream != 0
	}
	return false
}

// getData answers one question from the target: the group descriptor or a
// stream for one item. The stream starts its content on the first Read.
func (data *fileDataObject) promiseGetData(asked formatEtc, medium uintptr) uintptr {
	drag := data.promise
	if !drag.takes(asked) {
		return dvEFormatEtc
	}
	if asked.Format == drag.descriptor {
		handle, err := drag.descriptorHandle()
		if err != nil {
			return eFail
		}
		*(*storageMedium)(pointer(medium)) = storageMedium{Medium: tymedHGlobal, Handle: handle}
		return sOK
	}
	if asked.Index == lindexNone {
		return dvEFormatEtc
	}
	stream := drag.newStream(data, int(asked.Index))
	if stream == nil {
		return dvEFormatEtc
	}
	*(*storageMedium)(pointer(medium)) = storageMedium{Medium: tymedIStream, Handle: uintptr(unsafe.Pointer(stream))}
	return sOK
}

// descriptorHandle builds the FILEGROUPDESCRIPTORW in shared memory the target
// owns: a count, then one fixed-size descriptor per item.
func (drag *promiseDrag) descriptorHandle() (uintptr, error) {
	descriptorSize := int(unsafe.Sizeof(fileDescriptorW{}))
	total := 4 + len(drag.items)*descriptorSize
	handle, _, callErr := globalAlloc.Call(globalMoveable|globalZeroInit, uintptr(total))
	if handle == 0 {
		return 0, windowsCallError("allocate a dragged file group", callErr)
	}
	memory, _, callErr := globalLock.Call(handle)
	if memory == 0 {
		globalFree.Call(handle)
		return 0, windowsCallError("lock a dragged file group", callErr)
	}
	block := unsafe.Slice((*byte)(pointer(memory)), total)
	*(*uint32)(unsafe.Pointer(&block[0])) = uint32(len(drag.items))
	descriptors := unsafe.Slice((*fileDescriptorW)(unsafe.Pointer(&block[4])), len(drag.items))
	for index, item := range drag.items {
		descriptors[index] = item.descriptor()
	}
	globalUnlock.Call(handle)
	return handle, nil
}

// descriptor is this item's FILEDESCRIPTORW. The shell copies from the size the
// descriptor names, so a file whose size is unknown is still offered, without
// the size and its progress.
func (item promiseItem) descriptor() fileDescriptorW {
	var out fileDescriptorW
	out.Flags = fdAttributes
	if item.directory {
		out.Attributes = fileAttributeDirectory
	} else {
		out.Attributes = fileAttributeNormal
		out.Flags |= fdProgressUI
		if item.size >= 0 {
			out.Flags |= fdFileSize
			out.SizeHigh = uint32(uint64(item.size) >> 32)
			out.SizeLow = uint32(item.size)
		}
	}
	units, _ := syscall.UTF16FromString(item.name)
	if len(units) > len(out.Name) {
		units = units[:len(out.Name)]
	}
	copy(out.Name[:], units)
	return out
}

// promiseItems flattens the tree Windows must describe before the drag starts.
// The names become the backslash-separated relative paths the shell understands.
func promiseItems(files []platform.FilePromise) ([]promiseItem, error) {
	items := make([]promiseItem, 0, len(files))
	err := platform.WalkFilePromises(files, func(path string, promise platform.FilePromise) error {
		name := strings.ReplaceAll(path, "/", `\`)
		units, err := syscall.UTF16FromString(name)
		if err != nil || len(units) > maxDescriptorName {
			return fmt.Errorf("windows: the dragged file name %q is too long for the shell", name)
		}
		items = append(items, promiseItem{name: name, size: promise.Size, directory: promise.Directory, open: promise.Open})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return items, nil
}

// promiseStream is the IStream of one promised file. It produces its content on
// the first Read and serves the rest of the reads from the same reader, so the
// shell sees one sequential file.
type promiseStream struct {
	table *[14]uintptr
	refs  int32
	owner *fileDataObject
	item  promiseItem
	mu    sync.Mutex
	// reader is the content, opened on the first Read.
	reader  io.ReadCloser
	started bool
	failed  bool
	offset  uint64
}

var (
	streamTableOnce sync.Once
	streamTable     [14]uintptr
)

func newPromiseStreamTable() {
	streamTable = [14]uintptr{
		guarded3("drag and drop: stream QueryInterface", streamQueryInterface),
		guarded1("drag and drop: stream AddRef", streamAddRef),
		guarded1("drag and drop: stream Release", streamRelease),
		guarded4("drag and drop: stream Read", streamRead),
		guarded4("drag and drop: stream Write", streamWrite),
		guarded4("drag and drop: stream Seek", streamSeek),
		guarded2("drag and drop: stream SetSize", streamSetSize),
		guarded5("drag and drop: stream CopyTo", streamCopyTo),
		guarded2("drag and drop: stream Commit", streamCommit),
		guarded1("drag and drop: stream Revert", streamRevert),
		guarded4("drag and drop: stream LockRegion", streamLockRegion),
		guarded4("drag and drop: stream UnlockRegion", streamUnlockRegion),
		guarded3("drag and drop: stream Stat", streamStat),
		guarded2("drag and drop: stream Clone", streamClone),
	}
}

func streamOf(this uintptr) *promiseStream { return (*promiseStream)(pointer(this)) }

func (drag *promiseDrag) newStream(owner *fileDataObject, index int) *promiseStream {
	if index < 0 || index >= len(drag.items) || drag.items[index].directory {
		return nil
	}
	streamTableOnce.Do(newPromiseStreamTable)
	stream := &promiseStream{table: &streamTable, refs: 1, owner: owner, item: drag.items[index]}
	pinPromiseStream(stream)
	return stream
}

// promiseRoots are the objects the shell may still call through pointers the
// garbage collector cannot see: a promised drag's data object until its own
// reference count ends, and every stream until the target releases it.
var promiseRoots struct {
	mu      sync.Mutex
	objects map[*fileDataObject]struct{}
	streams map[*promiseStream]struct{}
}

func pinPromiseDrag(data *fileDataObject) {
	promiseRoots.mu.Lock()
	if promiseRoots.objects == nil {
		promiseRoots.objects = make(map[*fileDataObject]struct{})
	}
	promiseRoots.objects[data] = struct{}{}
	promiseRoots.mu.Unlock()
}

func unpinPromiseDrag(data *fileDataObject) {
	promiseRoots.mu.Lock()
	delete(promiseRoots.objects, data)
	promiseRoots.mu.Unlock()
}

func pinPromiseStream(stream *promiseStream) {
	promiseRoots.mu.Lock()
	if promiseRoots.streams == nil {
		promiseRoots.streams = make(map[*promiseStream]struct{})
	}
	promiseRoots.streams[stream] = struct{}{}
	promiseRoots.mu.Unlock()
}

func unpinPromiseStream(stream *promiseStream) {
	promiseRoots.mu.Lock()
	delete(promiseRoots.streams, stream)
	promiseRoots.mu.Unlock()
}

func streamQueryInterface(this, riid, object uintptr) uintptr {
	return answerInterface(this, riid, object, &streamOf(this).refs, iidStream)
}

func streamAddRef(this uintptr) uintptr {
	stream := streamOf(this)
	stream.refs++
	return uintptr(stream.refs)
}

func streamRelease(this uintptr) uintptr {
	stream := streamOf(this)
	if stream.refs > 0 {
		stream.refs--
	}
	if stream.refs == 0 {
		stream.mu.Lock()
		if stream.reader != nil {
			stream.reader.Close()
			stream.reader = nil
		}
		stream.mu.Unlock()
		unpinPromiseStream(stream)
	}
	return uintptr(stream.refs)
}

// streamRead produces the file's bytes. The first call opens the content; a
// short read is S_FALSE, which is how an IStream says the file ended. An error
// with nothing read is STG_E_READFAULT, which the shell shows as a failed copy.
func streamRead(this, pv, cb, pcbRead uintptr) uintptr {
	stream := streamOf(this)
	want := int(uint32(cb))
	stream.mu.Lock()
	defer stream.mu.Unlock()
	if stream.failed {
		setUint32(pcbRead, 0)
		return stgEReadFault
	}
	if !stream.started {
		reader, err := stream.item.open()
		if err != nil {
			stream.failed = true
			setUint32(pcbRead, 0)
			return stgEReadFault
		}
		stream.reader, stream.started = reader, true
	}
	read := 0
	if want > 0 && pv != 0 {
		buffer := unsafe.Slice((*byte)(pointer(pv)), want)
		for read < want {
			n, err := stream.reader.Read(buffer[read:])
			read += n
			if err != nil {
				if err != io.EOF {
					stream.failed = true
				}
				break
			}
			if n == 0 {
				break
			}
		}
	}
	stream.offset += uint64(read)
	setUint32(pcbRead, uint32(read))
	if stream.failed && read == 0 {
		return stgEReadFault
	}
	if read < want {
		return sFalse
	}
	return sOK
}

func setUint32(address uintptr, value uint32) {
	if address != 0 {
		*(*uint32)(pointer(address)) = value
	}
}

// streamSeek allows asking where the stream is and rewinding before it started.
// The content is produced forwards, so a target that seeks into it gets
// E_NOTIMPL, which the shell reports rather than corrupting the copy.
func streamSeek(this, move, origin, newPosition uintptr) uintptr {
	stream := streamOf(this)
	offset := int64(move)
	stream.mu.Lock()
	defer stream.mu.Unlock()
	switch uint32(origin) {
	case streamSeekCur:
		if offset != 0 {
			return eNotImpl
		}
	case streamSeekSet:
		if offset != 0 || stream.started || stream.offset != 0 {
			return eNotImpl
		}
	default:
		return eNotImpl
	}
	if newPosition != 0 {
		*(*uint64)(pointer(newPosition)) = stream.offset
	}
	return sOK
}

func streamWrite(this, pv, cb, written uintptr) uintptr {
	setUint32(written, 0)
	return eNotImpl
}

func streamSetSize(this, size uintptr) uintptr { return eNotImpl }

func streamCopyTo(this, other, cb, read, written uintptr) uintptr { return eNotImpl }

func streamCommit(this, flags uintptr) uintptr { return sOK }

func streamRevert(this uintptr) uintptr { return eNotImpl }

func streamLockRegion(this, offset, count, lockType uintptr) uintptr { return eNotImpl }

func streamUnlockRegion(this, offset, count, lockType uintptr) uintptr { return eNotImpl }

func streamClone(this, out uintptr) uintptr {
	if out != 0 {
		*(*uintptr)(pointer(out)) = 0
	}
	return eNotImpl
}

// statStg mirrors STATSTG, the description a target reads to learn the stream's
// size before copying it.
type statStg struct {
	Name     uintptr
	Type     uint32
	_        uint32
	Size     uint64
	Mtime    fileTime
	Ctime    fileTime
	Atime    fileTime
	Mode     uint32
	Locks    uint32
	Clsid    guid
	State    uint32
	Reserved uint32
}

func streamStat(this, stat, flags uintptr) uintptr {
	if stat == 0 {
		return eNotImpl
	}
	stream := streamOf(this)
	size := uint64(0)
	if stream.item.size > 0 {
		size = uint64(stream.item.size)
	}
	*(*statStg)(pointer(stat)) = statStg{Type: stgtYStream, Size: size, Mode: stgmRead}
	return sOK
}

// StartFilePromiseDrag takes files out of the application's active window whose
// content the application produces when the target reads it. The whole tree is
// described to the shell before the drag starts, so the call may walk a folder on
// the event-loop thread; Open runs when the shell reads each file.
func StartFilePromiseDrag(files []platform.FilePromise) error {
	state := active.Load()
	if state == nil || state.window == 0 {
		return fmt.Errorf("windows: no window to drag files out of")
	}
	thread, _, _ := getCurrentThread.Call()
	if uint32(thread) != state.application.thread {
		return fmt.Errorf("windows: files are dragged out on the event-loop thread that owns the window")
	}
	if len(files) == 0 {
		return fmt.Errorf("windows: no files to drag out of the window")
	}
	descriptor, contents := fileGroupDescriptorFormat(), fileContentsFormat()
	if descriptor == 0 || contents == 0 {
		return fmt.Errorf("windows: the shell's file formats could not be registered")
	}
	items, err := promiseItems(files)
	if err != nil {
		return err
	}
	if !state.application.oleReady {
		if status, _, _ := oleInitialize.Call(0); int32(uint32(status)) < 0 {
			return hresult(status)
		}
		state.application.oleReady = true
	}
	dataTableOnce.Do(newFileDataTable)
	sourceTableOnce.Do(newFileSourceTable)
	data := &fileDataObject{table: &dataTable, refs: 1}
	data.promise = &promiseDrag{items: items, descriptor: descriptor, contents: contents}
	pinPromiseDrag(data)
	source := &fileDropSource{table: &sourceTable, refs: 1}
	var effect uint32
	status, _, _ := doDragDrop.Call(
		uintptr(unsafe.Pointer(data)),
		uintptr(unsafe.Pointer(source)),
		uintptr(platform.DragEffectCopy),
		uintptr(unsafe.Pointer(&effect)),
	)
	runtime.KeepAlive(data)
	runtime.KeepAlive(source)
	if uint32(status) == dragdropSDrop || uint32(status) == dragdropSCancel {
		postMessage.Call(state.window, wmDragEnded, 0, 0)
	}
	if err := hresult(status); err != nil {
		// The drag never ran, so nothing holds the object it was built for.
		unpinPromiseDrag(data)
		return err
	}
	return nil
}
