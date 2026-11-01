//go:build linux && vivagtk

package linux

import (
	"github.com/vivaui/viva/internal/platform"
)

// Files dragged from a file manager arrive through GTK's drag and drop (XDND on X11, the
// compositor's data device on Wayland, where GDK handles only the parts of the window that are
// GTK's own: see gtk_wayland_linux.go for the rest). The files are the text/uri-list of the drag.

const gdkActionCopy = 1 << 1

// dragMotion is a drag moving over the window. The application says whether the point takes files.
func (w *gtkWindow) dragMotion(context uintptr, x, y int32, t uint32) uintptr {
	lx, ly := w.logical(float64(x), float64(y))
	accepted := w.fileDrop(platform.FileDropEvent{Phase: platform.FileDropOver, X: lx, Y: ly})
	action := int32(0)
	if accepted {
		action = gdkActionCopy
	}
	w.g.dragStatus(context, action, t)
	w.dragging = true
	return 1
}

// dragLeave is the drag leaving, or ending in a drop (which comes next, as data).
func (w *gtkWindow) dragLeave() {
	if w.dragging && !w.dropping {
		w.fileDrop(platform.FileDropEvent{Phase: platform.FileDropLeave})
	}
	w.dragging = false
}

// dragDrop is the button released over the window: ask for the list of files.
func (w *gtkWindow) dragDrop(widget, context uintptr, x, y int32, t uint32) uintptr {
	name := cString("text/uri-list")
	w.dropping = true
	w.g.dragGetData(widget, context, w.g.atomIntern(&name[0], 0), t)
	return 1
}

// dragData receives the files of a drop.
func (w *gtkWindow) dragData(context uintptr, x, y int32, data uintptr, t uint32) {
	w.dropping = false
	success := int32(0)
	if data != 0 {
		if length := w.g.selectionLength(data); length > 0 {
			raw := w.g.selectionData(data)
			text := string(unsafeSlice(raw, int(length)))
			if paths := parseURIList(text); len(paths) > 0 {
				lx, ly := w.logical(float64(x), float64(y))
				w.fileDrop(platform.FileDropEvent{Phase: platform.FileDropDrop, X: lx, Y: ly, Paths: paths})
				success = 1
			}
		}
	}
	w.g.dragFinish(context, success, 0, t)
}

// fileDrop gives the application one step of a drag of files.
func (w *gtkWindow) fileDrop(event platform.FileDropEvent) bool {
	if w.config.OnFileDrop == nil {
		return false
	}
	frame, accepted, err := w.config.OnFileDrop(event)
	w.present(frame, err)
	return accepted
}
