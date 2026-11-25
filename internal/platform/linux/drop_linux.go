//go:build linux && !android

package linux

import (
	"encoding/binary"
	"net/url"
	"strings"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

// Files dragged from a file manager arrive by the XDND protocol: the source
// window sends XdndEnter, XdndPosition as the pointer moves and XdndDrop at the
// end, and the window answers XdndStatus to say whether the point takes the files
// and XdndFinished once it has read them. The files themselves are the
// XdndSelection, asked for as text/uri-list. A window that wants drops marks
// itself XdndAware, with the protocol version it speaks.

const xdndVersion = 5

// maxDroppedURIs bounds how many files one drop can name.
const maxDroppedURIs = 4096

// xdndAtoms are the atoms the protocol uses.
type xdndAtoms struct {
	aware, enter, position, status, leave, drop, finished  uint32
	selection, actionCopy, typeList, uriList, dropProperty uint32
	proxy                                                  uint32
}

// xdndDrag is the drag in progress, as the source has told so far.
type xdndDrag struct {
	source    uint32
	wantsURIs bool
	x, y      float32
}

// setupFileDrop marks the window as a target for drags, when the application
// takes files.
func (win *window) setupFileDrop() error {
	var err error
	atoms := &win.xdnd
	for _, entry := range []struct {
		atom *uint32
		name string
	}{
		{&atoms.aware, "XdndAware"}, {&atoms.enter, "XdndEnter"}, {&atoms.position, "XdndPosition"},
		{&atoms.status, "XdndStatus"}, {&atoms.leave, "XdndLeave"}, {&atoms.drop, "XdndDrop"},
		{&atoms.finished, "XdndFinished"}, {&atoms.selection, "XdndSelection"}, {&atoms.actionCopy, "XdndActionCopy"},
		{&atoms.typeList, "XdndTypeList"}, {&atoms.uriList, "text/uri-list"}, {&atoms.dropProperty, "VIVA_DROP_DATA"},
	} {
		if *entry.atom, err = win.api.atom(win.connection, entry.name); err != nil {
			return err
		}
	}
	version := uint32(xdndVersion)
	win.api.changeProperty(win.connection, xcbPropertyReplace, win.id, atoms.aware, xcbAtomAtom, 32, 1, unsafe.Pointer(&version))
	return nil
}

// isDragMessage reports whether a client message belongs to the protocol.
func (win *window) isDragMessage(kind uint32) bool {
	if win.xdnd.aware == 0 {
		return false
	}
	atoms := &win.xdnd
	return kind == atoms.enter || kind == atoms.position || kind == atoms.leave || kind == atoms.drop
}

func clientLong(message *xcbClientEvent, index int) uint32 {
	return binary.LittleEndian.Uint32(message.Data[index*4:])
}

// handleDragMessage takes one step of a drag.
func (state *linuxWindowState) handleDragMessage(message *xcbClientEvent) error {
	win, config, atoms := state.win, state.config, &state.win.xdnd
	if config.OnFileDrop == nil || message.Format != 32 {
		return nil
	}
	switch message.Type {
	case atoms.enter:
		source, flags := clientLong(message, 0), clientLong(message, 1)
		drag := xdndDrag{source: source}
		types := []uint32{clientLong(message, 2), clientLong(message, 3), clientLong(message, 4)}
		if flags&1 != 0 {
			types = win.dragTypeList(source)
		}
		for _, kind := range types {
			if kind == atoms.uriList {
				drag.wantsURIs = true
			}
		}
		state.drag = drag
	case atoms.position:
		source := clientLong(message, 0)
		if source != state.drag.source {
			return nil
		}
		x, y := win.rootToWindow(int16(clientLong(message, 2)>>16), int16(clientLong(message, 2)))
		state.drag.x, state.drag.y = x, y
		accepted := false
		if state.drag.wantsURIs {
			frame, ok, err := config.OnFileDrop(platform.FileDropEvent{Phase: platform.FileDropOver, X: x, Y: y})
			accepted = ok
			if err == nil {
				err = state.router.present(frame)
			}
			if err != nil {
				return err
			}
		}
		win.sendDragReply(source, atoms.status, accepted, 0)
	case atoms.leave:
		if clientLong(message, 0) != state.drag.source {
			return nil
		}
		state.drag = xdndDrag{}
		frame, _, err := config.OnFileDrop(platform.FileDropEvent{Phase: platform.FileDropLeave})
		if err == nil {
			err = state.router.present(frame)
		}
		return err
	case atoms.drop:
		source := clientLong(message, 0)
		drag := state.drag
		state.drag = xdndDrag{}
		if source != drag.source || !drag.wantsURIs {
			win.sendDragReply(source, atoms.finished, false, 0)
			return nil
		}
		text, ok, err := readSelection(win.api, win.connection, win.id, atoms.selection, atoms.uriList, atoms.dropProperty)
		paths := []string(nil)
		if err == nil && ok {
			paths = parseURIList(text)
		}
		accepted := false
		if len(paths) > 0 {
			frame, taken, dropErr := config.OnFileDrop(platform.FileDropEvent{Phase: platform.FileDropDrop, X: drag.x, Y: drag.y, Paths: paths})
			accepted = taken
			if dropErr == nil {
				dropErr = state.router.present(frame)
			}
			err = dropErr
		}
		win.sendDragReply(source, atoms.finished, accepted, 0)
		return err
	}
	return nil
}

// dragTypeList reads the types the source offers when there are more than three.
func (win *window) dragTypeList(source uint32) []uint32 {
	cookie := win.api.getProperty(win.connection, 0, source, win.xdnd.typeList, xcbAtomAtom, 0, 256)
	var nativeError unsafe.Pointer
	reply := win.api.getPropertyReply(win.connection, cookie, &nativeError)
	if nativeError != nil {
		win.api.free(nativeError)
	}
	if reply == nil {
		return nil
	}
	defer win.api.free(unsafe.Pointer(reply))
	if reply.Format != 32 {
		return nil
	}
	length := int(win.api.getPropertyLength(reply)) / 4
	if length <= 0 {
		return nil
	}
	return append([]uint32(nil), unsafe.Slice((*uint32)(win.api.getPropertyValue(reply)), length)...)
}

// rootToWindow turns a point in root coordinates into the window's.
func (win *window) rootToWindow(x, y int16) (float32, float32) {
	cookie := win.api.translateCoordinates(win.connection, win.screen.Root, win.id, x, y)
	var nativeError unsafe.Pointer
	reply := win.api.translateCoordinatesReply(win.connection, cookie, &nativeError)
	if nativeError != nil {
		win.api.free(nativeError)
	}
	scale := win.displayScale()
	// The content starts after the window's margin, if it has one.
	margin := int16(win.margin)
	if reply == nil {
		return float32(x-margin) / scale, float32(y-margin) / scale
	}
	defer win.api.free(unsafe.Pointer(reply))
	return float32(reply.DstX-margin) / scale, float32(reply.DstY-margin) / scale
}

// sendDragReply answers the source with XdndStatus or XdndFinished.
func (win *window) sendDragReply(source, kind uint32, accepted bool, _ uint32) {
	win.mu.Lock()
	defer win.mu.Unlock()
	if win.connection == 0 || source == 0 {
		return
	}
	event := xcbClientEvent{ResponseType: xcbClientMessage, Format: 32, Window: source, Type: kind}
	binary.LittleEndian.PutUint32(event.Data[0:], win.id)
	if accepted {
		binary.LittleEndian.PutUint32(event.Data[4:], 1)
		binary.LittleEndian.PutUint32(event.Data[16:], win.xdnd.actionCopy)
	}
	win.api.sendEvent(win.connection, 0, source, 0, (*byte)(unsafe.Pointer(&event)))
	win.api.flush(win.connection)
}

// parseURIList reads a text/uri-list into file system paths. Only file: URIs on
// this machine are taken; comments and anything else are ignored.
func parseURIList(text string) []string {
	var paths []string
	for _, line := range strings.FieldsFunc(text, func(r rune) bool { return r == '\n' || r == '\r' }) {
		if len(paths) == maxDroppedURIs {
			break
		}
		if line == "" || line[0] == '#' {
			continue
		}
		parsed, err := url.Parse(line)
		if err != nil || parsed.Scheme != "file" || parsed.Path == "" || parsed.Path[0] != '/' {
			continue
		}
		if host := parsed.Hostname(); host != "" && host != "localhost" {
			continue
		}
		if strings.ContainsRune(parsed.Path, 0) {
			continue
		}
		paths = append(paths, parsed.Path)
	}
	return paths
}
