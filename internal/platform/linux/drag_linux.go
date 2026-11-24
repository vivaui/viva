//go:build linux && !android

package linux

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

// Dragging files out of a window is XDND from the source's side: the window
// under the pointer is told what the drag carries, follows the pointer with
// XdndPosition, and the target either answers XdndStatus (it takes the drop) or
// ignores it. The files themselves are the XdndSelection the target reads when
// the drop happens, as text/uri-list.
//
// X11 file managers only copy files that already exist, so a drag of promised
// content is staged first: the application's Open and List write a private
// temporary copy while the pointer moves, and XdndDrop waits for that copy when
// the user lets go before the drop would otherwise happen. A drag of paths has
// nothing to stage.
//
// The source runs entirely on the event-loop thread. The staging goroutine
// reports back through application.Post; it never touches the X connection.

const (
	// xdndSourceVersion is the XDND protocol version offered.
	xdndSourceVersion = 5
	// sourceDragAction is the only action a drag out offers: a copy leaves the
	// files where they came from. A move would have to delete the source.
	sourceDragAction = 1 // XdndActionCopy is interned; this is the fallback.
	// dragStagingPrefix names the private directories promised drags are staged in.
	dragStagingPrefix = "viva-drag-"
	// dragStagingAge is how old a staging directory must be before a later drag
	// removes it. No timer does this; the removal happens when a drag starts.
	dragStagingAge = 24 * time.Hour
)

// xdndSource is one drag out of this window. The event-loop thread owns every
// field; the staging goroutine only reports a generation through Post.
type xdndSource struct {
	active      bool
	target      uint32
	entered     bool
	accepted    bool
	time        uint32
	rootX       int16
	rootY       int16
	uris        []string
	staging     string
	ready       bool
	failed      bool
	pendingDrop bool
	dropped     bool
	generation  uint64
}

// StartFileDrag takes files that exist at paths out of the window handling the
// current press. X11 offers them as text/uri-list and only a copy.
func StartFileDrag(paths []string, allowed platform.DragEffect) error {
	if len(paths) == 0 {
		return fmt.Errorf("linux: no files to drag out of the window")
	}
	win := activeWindow.Load()
	if win == nil || win.connection == 0 {
		return fmt.Errorf("linux: dragging files out needs a running window")
	}
	return win.startDragOut(paths, nil)
}

// StartFilePromiseDrag takes promised files out of the window handling the
// current press: they are written to a private staging directory while the
// pointer moves and offered to the target from there.
func StartFilePromiseDrag(files []platform.FilePromise) error {
	if len(files) == 0 {
		return fmt.Errorf("linux: no files to drag out of the window")
	}
	win := activeWindow.Load()
	if win == nil || win.connection == 0 {
		return fmt.Errorf("linux: dragging files out needs a running window")
	}
	return win.startDragOut(nil, files)
}

func (win *window) startDragOut(paths []string, files []platform.FilePromise) error {
	if win.source.active {
		return fmt.Errorf("linux: another file drag is already running")
	}
	if err := win.ensureDragSourceAtoms(); err != nil {
		return err
	}
	if err := win.ensureClipboardAtoms(); err != nil {
		return err
	}
	target := win.dragTargetAt()
	if target == 0 {
		return fmt.Errorf("linux: no window under the pointer takes a drop")
	}
	removeStaleDragStaging()
	source := xdndSource{
		active: true, target: target, entered: true,
		time: win.lastEventTime, rootX: win.lastRootX, rootY: win.lastRootY,
		generation: win.dragGeneration + 1,
	}
	if paths != nil {
		source.uris = append([]string(nil), paths...)
		source.ready = true
	} else {
		dir, err := os.MkdirTemp("", dragStagingPrefix)
		if err != nil {
			return fmt.Errorf("linux: stage the dragged files: %w", err)
		}
		source.staging = dir
		source.uris = make([]string, len(files))
		for index, promise := range files {
			source.uris[index] = filepath.Join(dir, promise.Name)
		}
	}
	win.dragGeneration = source.generation
	win.source = source
	win.api.setSelectionOwner(win.connection, win.id, win.xdnd.selection, source.time)
	win.sendXdndEnter(target)
	win.sendXdndPosition(target, source.rootX, source.rootY, source.time)
	win.api.flush(win.connection)
	if files != nil {
		go win.stagePromisedFiles(files, source.staging, source.generation)
	}
	return nil
}

// stagePromisedFiles writes the promises into their staging directory and tells
// the loop when it is done. It runs off the event loop; all it shares with the
// loop is the generation it was started for.
func (win *window) stagePromisedFiles(files []platform.FilePromise, root string, generation uint64) {
	err := stageFilePromises(files, root)
	win.application.Post(func() {
		if !win.source.active || win.source.generation != generation {
			return
		}
		if err != nil {
			win.source.failed = true
			if win.source.pendingDrop {
				win.cancelSourceDrag()
			}
			return
		}
		win.source.ready = true
		if win.source.pendingDrop {
			win.dropSourceDrag()
		}
	})
}

func stageFilePromises(files []platform.FilePromise, root string) error {
	for _, promise := range files {
		target := filepath.Join(root, promise.Name)
		if promise.Directory {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			children, err := promise.List()
			if err != nil {
				return err
			}
			if err := platform.WalkFilePromises(children, func(relative string, child platform.FilePromise) error {
				target := filepath.Join(root, promise.Name, filepath.FromSlash(relative))
				if child.Directory {
					return os.MkdirAll(target, 0o755)
				}
				return stagePromiseFile(child, target)
			}); err != nil {
				return err
			}
			continue
		}
		if err := stagePromiseFile(promise, target); err != nil {
			return err
		}
	}
	return nil
}

func stagePromiseFile(promise platform.FilePromise, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	reader, err := promise.Open()
	if err != nil {
		return err
	}
	defer reader.Close()
	file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(file, reader); err != nil {
		file.Close()
		os.Remove(target)
		return err
	}
	if err := file.Close(); err != nil {
		os.Remove(target)
		return err
	}
	return nil
}

// removeStaleDragStaging removes staging directories a much earlier drag left
// behind. The target copies from them after the drop, so they cannot be removed
// when the drop is sent; this one scan, run when a drag starts, keeps /tmp from
// collecting them forever without a timer.
func removeStaleDragStaging() {
	entries, err := os.ReadDir(os.TempDir())
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), dragStagingPrefix) {
			continue
		}
		info, err := entry.Info()
		if err != nil || time.Since(info.ModTime()) < dragStagingAge {
			continue
		}
		os.RemoveAll(filepath.Join(os.TempDir(), entry.Name()))
	}
}

// noteInputEvent remembers the pointer event the loop just saw, so a drag the
// application starts from its handler knows where the press was and when.
func (win *window) noteInputEvent(input *xcbInputEvent) {
	win.lastRootX, win.lastRootY = input.RootX, input.RootY
	if input.Time != 0 {
		win.lastEventTime = input.Time
	}
}

// isSourceDragMessage reports whether a client message is the target answering
// this window's drag.
func (win *window) isSourceDragMessage(kind uint32) bool {
	return win.source.active && (kind == win.xdnd.status || kind == win.xdnd.finished)
}

// ensureDragSourceAtoms interns what both sides of the handshake use. The
// window may never have taken drops, so this does not depend on setupFileDrop.
func (win *window) ensureDragSourceAtoms() error {
	if win.xdnd.enter != 0 && win.xdnd.selection != 0 && win.xdnd.proxy != 0 {
		return nil
	}
	for _, entry := range []struct {
		atom *uint32
		name string
	}{
		{&win.xdnd.aware, "XdndAware"}, {&win.xdnd.enter, "XdndEnter"},
		{&win.xdnd.position, "XdndPosition"}, {&win.xdnd.status, "XdndStatus"},
		{&win.xdnd.leave, "XdndLeave"}, {&win.xdnd.drop, "XdndDrop"},
		{&win.xdnd.finished, "XdndFinished"}, {&win.xdnd.selection, "XdndSelection"},
		{&win.xdnd.actionCopy, "XdndActionCopy"}, {&win.xdnd.uriList, "text/uri-list"},
		{&win.xdnd.proxy, "XdndProxy"},
	} {
		atom, err := win.api.atom(win.connection, entry.name)
		if err != nil {
			return err
		}
		if *entry.atom == 0 {
			*entry.atom = atom
		}
	}
	return nil
}

// dragTargetAt finds the window under the pointer that speaks XDND. The pointer
// is over this window's children or another application's; the walk descends to
// the deepest window and then up to the first that set XdndAware, following a
// proxy when it has one.
func (win *window) dragTargetAt() uint32 {
	if win.xdnd.aware == 0 {
		return 0
	}
	window := win.deepestWindowAtPointer()
	for depth := 0; window != 0 && window != win.screen.Root && depth < 64; depth++ {
		if window == win.id {
			return 0
		}
		if win.hasAtomProperty(window, win.xdnd.aware) {
			if proxy, ok := win.uint32Property(window, win.xdnd.proxy); ok && proxy != 0 {
				return proxy
			}
			return window
		}
		parent, ok := win.windowParent(window)
		if !ok {
			return 0
		}
		window = parent
	}
	return 0
}

func (win *window) deepestWindowAtPointer() uint32 {
	window := win.screen.Root
	for depth := 0; depth < 64; depth++ {
		var nativeError unsafe.Pointer
		reply := win.api.queryPointerReply(win.connection, win.api.queryPointer(win.connection, window), &nativeError)
		if nativeError != nil {
			win.api.free(nativeError)
		}
		if reply == nil {
			return 0
		}
		child := reply.Child
		win.api.free(unsafe.Pointer(reply))
		if child == 0 {
			return window
		}
		window = child
	}
	return window
}

func (win *window) windowParent(window uint32) (uint32, bool) {
	var nativeError unsafe.Pointer
	reply := win.api.queryTreeReply(win.connection, win.api.queryTree(win.connection, window), &nativeError)
	if nativeError != nil {
		win.api.free(nativeError)
	}
	if reply == nil {
		return 0, false
	}
	parent := reply.Parent
	win.api.free(unsafe.Pointer(reply))
	return parent, true
}

func (win *window) hasAtomProperty(window, atom uint32) bool {
	if atom == 0 {
		return false
	}
	var nativeError unsafe.Pointer
	reply := win.api.getPropertyReply(win.connection, win.api.getProperty(win.connection, 0, window, atom, xcbAtomAtom, 0, 0), &nativeError)
	if nativeError != nil {
		win.api.free(nativeError)
	}
	if reply == nil {
		return false
	}
	present := reply.Type != 0
	win.api.free(unsafe.Pointer(reply))
	return present
}

func (win *window) uint32Property(window, atom uint32) (uint32, bool) {
	if atom == 0 {
		return 0, false
	}
	var nativeError unsafe.Pointer
	reply := win.api.getPropertyReply(win.connection, win.api.getProperty(win.connection, 0, window, atom, xcbAtomAtom, 0, 1), &nativeError)
	if nativeError != nil {
		win.api.free(nativeError)
	}
	if reply == nil {
		return 0, false
	}
	defer win.api.free(unsafe.Pointer(reply))
	if reply.Type == 0 || reply.ValueLength < 1 {
		return 0, false
	}
	return *(*uint32)(win.api.getPropertyValue(reply)), true
}

// sourceDragMotion follows the pointer: a target that changed gets XdndLeave,
// and the one under the pointer now gets XdndPosition.
func (win *window) sourceDragMotion(input *xcbInputEvent) {
	if !win.source.active || win.source.dropped {
		return
	}
	win.source.rootX, win.source.rootY, win.source.time = input.RootX, input.RootY, input.Time
	target := win.dragTargetAt()
	if target != win.source.target {
		if win.source.entered {
			win.sendXdndLeave(win.source.target)
		}
		win.source.target, win.source.entered, win.source.accepted = target, false, false
		if target != 0 {
			win.sendXdndEnter(target)
			win.source.entered = true
		}
	}
	if win.source.target != 0 {
		win.sendXdndPosition(win.source.target, input.RootX, input.RootY, input.Time)
	}
	win.api.flush(win.connection)
}

// sourceDragRelease ends the gesture where the pointer was released. A target
// that did not accept takes nothing; one that did gets XdndDrop, after the
// promised files are staged if they are not yet.
func (win *window) sourceDragRelease(input *xcbInputEvent) {
	if !win.source.active || win.source.dropped {
		return
	}
	win.source.time = input.Time
	if win.source.target == 0 || !win.source.accepted {
		win.cancelSourceDrag()
		return
	}
	if win.source.ready {
		win.dropSourceDrag()
		return
	}
	win.source.pendingDrop = true
}

// sourceDragEscape cancels the drag, as Escape cancels every drag on the desktop.
func (win *window) sourceDragEscape() {
	if win.source.active && !win.source.dropped {
		win.cancelSourceDrag()
	}
}

func (win *window) sendXdndEnter(target uint32) {
	var data [5]uint32
	data[0] = win.id
	data[1] = xdndSourceVersion << 24 // no type list: one type fits in the message
	data[2] = win.xdnd.uriList
	win.sendClientMessage(target, win.xdnd.enter, &data)
}

func (win *window) sendXdndPosition(target uint32, rootX, rootY int16, at uint32) {
	var data [5]uint32
	data[0] = win.id
	data[2] = uint32(uint16(rootX))<<16 | uint32(uint16(rootY))
	data[3] = at
	data[4] = win.xdnd.actionCopy
	win.sendClientMessage(target, win.xdnd.position, &data)
}

func (win *window) sendXdndLeave(target uint32) {
	if target == 0 {
		return
	}
	var data [5]uint32
	data[0] = win.id
	win.sendClientMessage(target, win.xdnd.leave, &data)
}

func (win *window) sendClientMessage(target, kind uint32, data *[5]uint32) {
	event := xcbClientEvent{
		ResponseType: xcbClientMessage,
		Format:       32,
		Window:       target,
		Type:         kind,
	}
	for index, value := range data {
		binary.LittleEndian.PutUint32(event.Data[index*4:], value)
	}
	win.api.sendEvent(win.connection, 0, target, 0, (*byte)(unsafe.Pointer(&event)))
}

// handleSourceDragMessage takes the target's answers: XdndStatus says whether
// the point takes the drop, and XdndFinished says the target is done with the
// selection.
func (win *window) handleSourceDragMessage(message *xcbClientEvent) {
	if !win.source.active {
		return
	}
	first, second := clientLong(message, 0), clientLong(message, 1)
	if first != win.source.target {
		return
	}
	switch message.Type {
	case win.xdnd.status:
		if win.source.dropped {
			return
		}
		win.source.accepted = second&1 != 0
	case win.xdnd.finished:
		win.finishSourceDrag()
	}
}

func (win *window) dropSourceDrag() {
	if !win.source.active || win.source.dropped {
		return
	}
	if win.source.target == 0 || !win.source.accepted {
		win.cancelSourceDrag()
		return
	}
	win.source.dropped = true
	var data [5]uint32
	data[0] = win.id
	data[2] = win.source.time
	win.sendClientMessage(win.source.target, win.xdnd.drop, &data)
	win.api.flush(win.connection)
}

func (win *window) cancelSourceDrag() {
	if !win.source.active {
		return
	}
	if win.source.entered && win.source.target != 0 {
		win.sendXdndLeave(win.source.target)
	}
	win.finishSourceDrag()
}

func (win *window) finishSourceDrag() {
	if !win.source.active {
		return
	}
	selection := win.xdnd.selection
	win.source = xdndSource{}
	if selection != 0 && win.connection != 0 {
		win.api.setSelectionOwner(win.connection, xcbAtomNone, selection, 0)
		win.api.flush(win.connection)
	}
}

// answerXdndSelection serves the target's request for the drag's file list. The
// caller holds win.mu, as it does for the clipboard's own selections.
func (win *window) answerXdndSelection(request *xcbSelectionRequestEvent, notify *xcbSelectionNotifyWire) {
	property := request.Property
	if property == xcbAtomNone {
		property = request.Target
	}
	switch request.Target {
	case win.targetsAtom:
		targets := []uint32{win.targetsAtom, win.xdnd.uriList}
		win.api.changeProperty(win.connection, xcbPropertyReplace, request.Requestor, property, xcbAtomAtom, 32, uint32(len(targets)), unsafe.Pointer(&targets[0]))
		notify.Property = property
	case win.xdnd.uriList:
		if win.writeSelection(request.Requestor, property, request.Target, platform.FileURIList(win.source.uris)) {
			notify.Property = property
		}
	}
}
