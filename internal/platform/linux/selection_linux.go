package linux

import (
	"fmt"
	"runtime"
	"time"
	"unicode/utf8"
	"unsafe"
)

// The selections Viva owns: CLIPBOARD, which an explicit copy writes, and PRIMARY, which
// selecting text writes and a middle click reads. The owner is the window the application
// last opened or focused (activeWindow), so a selection lives as long as the window does.
// Before the window goes, CLIPBOARD is handed to the clipboard manager if there is one, as
// toolkits do, so that a copy outlives the application.

const (
	// xcbAtomPrimary is the predefined atom of the PRIMARY selection.
	xcbAtomPrimary = 1
	// xcbPropertyDelete is the state of a PropertyNotify for a deleted property.
	xcbPropertyDelete = 1

	// managerWait bounds how long a window that is closing waits for the clipboard manager to
	// take the clipboard.
	managerWait = 2 * time.Second
)

// selectionState is a window's selections beyond the CLIPBOARD text kept in clipboard.
type selectionState struct {
	primary     string
	ownsPrimary bool
	incrAtom    uint32
	incr        incrTransfers
}

func (win *window) ensureClipboardAtoms() error {
	if win.clipboardAtom != 0 {
		return nil
	}
	var err error
	if win.clipboardAtom, err = win.api.atom(win.connection, "CLIPBOARD"); err != nil {
		return err
	}
	if win.utf8Atom, err = win.api.atom(win.connection, "UTF8_STRING"); err != nil {
		return err
	}
	if win.textAtom, err = win.api.atom(win.connection, "TEXT"); err != nil {
		return err
	}
	if win.targetsAtom, err = win.api.atom(win.connection, "TARGETS"); err != nil {
		return err
	}
	return nil
}

// SetPrimaryText makes text the PRIMARY selection, which other applications paste with a
// middle click. Empty text gives the selection up.
func SetPrimaryText(value string) error {
	if len(value) > maxClipboardBytes {
		return fmt.Errorf("linux: selection text exceeds 64 MiB")
	}
	if !utf8.ValidString(value) {
		return fmt.Errorf("linux: selection text is not valid UTF-8")
	}
	if hooks := shellHooks.Load(); hooks != nil {
		return hooks.setPrimaryText(value)
	}
	win := activeWindow.Load()
	if win == nil {
		return fmt.Errorf("linux: selection ownership requires a running window")
	}
	win.mu.Lock()
	defer win.mu.Unlock()
	if activeWindow.Load() != win || win.connection == 0 {
		return fmt.Errorf("linux: selection ownership requires a running window")
	}
	if err := win.ensureClipboardAtoms(); err != nil {
		return err
	}
	if value == "" {
		if win.selections.ownsPrimary {
			win.api.setSelectionOwner(win.connection, xcbAtomNone, xcbAtomPrimary, 0)
			win.selections.primary, win.selections.ownsPrimary = "", false
			if win.api.flush(win.connection) <= 0 || win.api.connectionHasError(win.connection) != 0 {
				return fmt.Errorf("linux: release selection: XCB connection failed")
			}
		}
		return nil
	}
	win.api.setSelectionOwner(win.connection, win.id, xcbAtomPrimary, 0)
	if win.api.flush(win.connection) <= 0 || win.api.connectionHasError(win.connection) != 0 {
		return fmt.Errorf("linux: claim selection: XCB connection failed")
	}
	owner, err := selectionOwner(win.api, win.connection, xcbAtomPrimary)
	if err != nil {
		return err
	}
	if owner != win.id {
		return fmt.Errorf("linux: X server refused selection ownership")
	}
	win.selections.primary, win.selections.ownsPrimary = value, true
	return nil
}

// PrimaryText is the PRIMARY selection's text, empty when nobody owns it.
func PrimaryText() (string, error) {
	if hooks := shellHooks.Load(); hooks != nil {
		return hooks.primaryText()
	}
	if win := activeWindow.Load(); win != nil {
		win.mu.Lock()
		if activeWindow.Load() == win && win.connection != 0 && win.selections.ownsPrimary {
			value := win.selections.primary
			win.mu.Unlock()
			return value, nil
		}
		win.mu.Unlock()
	}
	return readSelectionTextX11("PRIMARY")
}

// selectionText is the text this window offers for a selection, and whether it owns it.
func (win *window) selectionText(selection uint32) (string, bool) {
	switch {
	case selection == xcbAtomPrimary:
		return win.selections.primary, win.selections.ownsPrimary
	case selection == win.clipboardAtom && win.clipboardAtom != 0:
		return win.clipboard, win.ownsClipboard
	}
	return "", false
}

func (win *window) handleSelectionRequest(request *xcbSelectionRequestEvent) error {
	win.mu.Lock()
	defer win.mu.Unlock()
	notify := xcbSelectionNotifyWire{
		xcbSelectionNotifyEvent: xcbSelectionNotifyEvent{
			ResponseType: xcbSelectionNotify,
			Time:         request.Time,
			Requestor:    request.Requestor,
			Selection:    request.Selection,
			Target:       request.Target,
			Property:     xcbAtomNone,
		},
	}
	win.dropStaleTransfers()
	if value, owned := win.selectionText(request.Selection); owned {
		property := request.Property
		if property == xcbAtomNone {
			property = request.Target
		}
		switch request.Target {
		case win.targetsAtom:
			targets := [...]uint32{win.targetsAtom, win.utf8Atom, win.textAtom, xcbAtomString}
			win.api.changeProperty(win.connection, xcbPropertyReplace, request.Requestor, property, xcbAtomAtom, 32, uint32(len(targets)), unsafe.Pointer(&targets[0]))
			notify.Property = property
		case win.utf8Atom, win.textAtom:
			if win.writeSelection(request.Requestor, property, request.Target, []byte(value)) {
				notify.Property = property
			}
		case xcbAtomString:
			if win.writeSelection(request.Requestor, property, xcbAtomString, latin1Clipboard(value)) {
				notify.Property = property
			}
		}
	}
	win.api.sendEvent(win.connection, 0, request.Requestor, 0, (*byte)(unsafe.Pointer(&notify)))
	if win.api.flush(win.connection) <= 0 || win.api.connectionHasError(win.connection) != 0 {
		return fmt.Errorf("linux: answer clipboard request: XCB connection failed")
	}
	return nil
}

// writeSelection puts data in the requestor's property, whole or, when it is more than a
// request carries, as an INCR stream the requestor then reads piece by piece. win.mu is held.
func (win *window) writeSelection(requestor, property, kind uint32, data []byte) bool {
	if len(data) <= incrChunk {
		var pointer unsafe.Pointer
		if len(data) > 0 {
			pointer = unsafe.Pointer(&data[0])
		}
		win.api.changeProperty(win.connection, xcbPropertyReplace, requestor, property, kind, 8, uint32(len(data)), pointer)
		return true
	}
	if win.selections.incrAtom == 0 {
		atom, err := win.api.atom(win.connection, "INCR")
		if err != nil {
			return false
		}
		win.selections.incrAtom = atom
	}
	// The requestor deleting the property is the cue for the next piece: hear of it.
	mask := uint32(xcbEventPropertyChange)
	win.api.changeWindow(win.connection, requestor, xcbCWEventMask, &mask)
	size := uint32(len(data))
	win.api.changeProperty(win.connection, xcbPropertyReplace, requestor, property, win.selections.incrAtom, 32, 1, unsafe.Pointer(&size))
	now := time.Now()
	for _, dropped := range win.selections.incr.begin(&incrTransfer{requestor: requestor, property: property, kind: kind, data: data}, now) {
		win.releaseRequestor(dropped.requestor)
	}
	return true
}

// releaseRequestor stops listening to a requestor window once no stream goes to it.
func (win *window) releaseRequestor(requestor uint32) {
	if win.selections.incr.active(requestor) {
		return
	}
	mask := uint32(0)
	win.api.changeWindow(win.connection, requestor, xcbCWEventMask, &mask)
}

func (win *window) dropStaleTransfers() {
	for _, dropped := range win.selections.incr.expire(time.Now()) {
		win.releaseRequestor(dropped.requestor)
	}
}

// incrEvent writes the next piece of a stream when its requestor has read the last. It reports
// whether the event was one the streams were waiting for.
func (win *window) incrEvent(event *xcbPropertyEvent) bool {
	if event.State != xcbPropertyDelete {
		return false
	}
	win.mu.Lock()
	defer win.mu.Unlock()
	transfer := win.selections.incr.find(event.Window, event.Atom)
	if transfer == nil {
		return false
	}
	piece, last := transfer.next(time.Now())
	var pointer unsafe.Pointer
	if len(piece) > 0 {
		pointer = unsafe.Pointer(&piece[0])
	}
	win.api.changeProperty(win.connection, xcbPropertyReplace, transfer.requestor, transfer.property, transfer.kind, 8, uint32(len(piece)), pointer)
	if last {
		win.selections.incr.remove(transfer)
		win.releaseRequestor(transfer.requestor)
	}
	win.api.flush(win.connection)
	return true
}

func (win *window) handleSelectionClear(event *xcbSelectionClearEvent) {
	win.mu.Lock()
	defer win.mu.Unlock()
	switch {
	case event.Selection == win.clipboardAtom && win.clipboardAtom != 0:
		win.clipboard = ""
		win.ownsClipboard = false
	case event.Selection == xcbAtomPrimary:
		win.selections.primary = ""
		win.selections.ownsPrimary = false
	}
}

// saveClipboardToManager hands the CLIPBOARD this window owns to the clipboard manager
// (the freedesktop clipboard manager protocol: ask CLIPBOARD_MANAGER to convert SAVE_TARGETS),
// so that it is still there when the window is gone. The manager asks for the data in
// requests this window answers while it waits, bounded by managerWait. A desktop without a
// manager loses the clipboard with the window, as before.
func (win *window) saveClipboardToManager() {
	win.mu.Lock()
	connection, api := win.connection, win.api
	owned := connection != 0 && win.ownsClipboard && win.clipboard != ""
	win.mu.Unlock()
	if !owned || api.convertSelection == nil {
		return
	}
	manager, err := api.atom(connection, "CLIPBOARD_MANAGER")
	if err != nil {
		return
	}
	owner, err := selectionOwner(api, connection, manager)
	if err != nil || owner == xcbAtomNone || owner == win.id {
		return
	}
	save, err := api.atom(connection, "SAVE_TARGETS")
	if err != nil {
		return
	}
	property, err := api.atom(connection, "VIVA_SAVE_TARGETS")
	if err != nil {
		return
	}
	win.mu.Lock()
	err = win.ensureClipboardAtoms()
	targets := [...]uint32{win.utf8Atom, win.textAtom, xcbAtomString}
	win.mu.Unlock()
	if err != nil {
		return
	}
	api.changeProperty(connection, xcbPropertyReplace, win.id, property, xcbAtomAtom, 32, uint32(len(targets)), unsafe.Pointer(&targets[0]))
	api.convertSelection(connection, win.id, manager, save, property, 0)
	if api.flush(connection) <= 0 || api.connectionHasError(connection) != 0 {
		return
	}
	file := pollFD{FD: api.fileDescriptor(connection), Events: xcbPollInput}
	deadline := time.Now().Add(managerWait)
	for {
		for event := api.pollForEvent(connection); event != nil; event = api.pollForEvent(connection) {
			kind := (*xcbGenericEvent)(event).ResponseType & 0x7f
			finished := false
			switch kind {
			case xcbSelectionRequest:
				_ = win.handleSelectionRequest((*xcbSelectionRequestEvent)(event))
			case xcbPropertyNotify:
				win.incrEvent((*xcbPropertyEvent)(event))
			case xcbSelectionNotify:
				notify := (*xcbSelectionNotifyEvent)(event)
				finished = notify.Selection == manager && notify.Target == save
			case xcbSelectionClear:
				finished = true
			}
			api.free(event)
			if finished {
				return
			}
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return
		}
		ready := api.poll(&file, 1, int32(max(remaining.Milliseconds(), 1)))
		runtime.KeepAlive(&file)
		if ready < 0 || api.connectionHasError(connection) != 0 {
			return
		}
	}
}
