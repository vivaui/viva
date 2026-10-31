package linux

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

const (
	maxClipboardBytes = 64 << 20
	clipboardWait     = 3 * time.Second
	dialogSeparator   = "\x1f"
)

var ErrUnsupported = errors.New("linux: system integration is unsupported")

type FileFilter struct {
	Name    string
	Pattern string
}

type OpenFileOptions struct {
	Title         string
	Filters       []FileFilter
	AllowMultiple bool
}

type OpenDirectoryOptions struct {
	Title string
}

type SaveFileOptions struct {
	Title            string
	SuggestedName    string
	DefaultExtension string
	Filters          []FileFilter
}

func ClipboardText() (string, error) {
	if waylandApp := activeWaylandApp.Load(); waylandApp != nil && waylandApp.hasWindows() {
		var value string
		var wait func() (string, error)
		err := waylandApp.invoke(func() error {
			var err error
			value, wait, err = waylandApp.requestClipboardText()
			return err
		})
		if wait == nil {
			return value, err
		}
		return wait()
	}
	if win := activeWindow.Load(); win != nil {
		win.mu.Lock()
		if activeWindow.Load() == win && win.connection != 0 && win.ownsClipboard {
			value := win.clipboard
			win.mu.Unlock()
			return value, nil
		}
		win.mu.Unlock()
	}
	return readClipboardX11()
}

func readClipboardX11() (string, error) { return readSelectionTextX11("CLIPBOARD") }

// readSelectionTextX11 asks the owner of a selection (CLIPBOARD, PRIMARY) for its text; empty
// when nobody owns it.
func readSelectionTextX11(name string) (string, error) {
	api, connection, requestor, selection, owner, err := openSelectionRequest(name)
	if err != nil {
		return "", err
	}
	defer closeClipboardRequest(api, connection, requestor)
	if owner == xcbAtomNone {
		return "", nil
	}
	property, err := api.atom(connection, "VIVA_CLIPBOARD_TEXT")
	if err != nil {
		return "", err
	}
	utf8, err := api.atom(connection, "UTF8_STRING")
	if err != nil {
		return "", err
	}
	value, accepted, err := readSelection(api, connection, requestor, selection, utf8, property)
	if err != nil {
		return "", err
	}
	if accepted {
		return value, nil
	}
	return readLegacySelection(api, connection, requestor, selection, property)
}

// HasClipboardImage reports whether the X11 CLIPBOARD owner offers image/png.
// It asks the owner for its TARGETS, so it waits for another application for up
// to clipboardWait; a clipboard this process owns holds text only.
func HasClipboardImage() bool {
	has, _ := clipboardOffersImage()
	return has
}

func clipboardOffersImage() (bool, error) {
	if ownsClipboardText() {
		return false, nil
	}
	api, connection, requestor, clipboard, owner, err := openClipboardRequest()
	if err != nil {
		return false, err
	}
	defer closeClipboardRequest(api, connection, requestor)
	if owner == xcbAtomNone {
		return false, nil
	}
	targets, err := api.atom(connection, "TARGETS")
	if err != nil {
		return false, err
	}
	property, err := api.atom(connection, "VIVA_CLIPBOARD_TARGETS")
	if err != nil {
		return false, err
	}
	png, err := api.atom(connection, "image/png")
	if err != nil {
		return false, err
	}
	data, kind, accepted, err := readSelectionBytes(api, connection, requestor, clipboard, targets, property, 1<<16)
	if err != nil {
		return false, err
	}
	if !accepted || kind != xcbAtomAtom || len(data)%4 != 0 {
		return false, nil
	}
	for offset := 0; offset+4 <= len(data); offset += 4 {
		if binary.NativeEndian.Uint32(data[offset:]) == png {
			return true, nil
		}
	}
	return false, nil
}

// ClipboardImagePNG returns the X11 CLIPBOARD owner's image/png, whether the
// owner hands it over in one property or streams it with INCR (what a screenshot
// of any size usually needs). The total is capped at MaxClipboardImageBytes.
func ClipboardImagePNG() ([]byte, error) {
	if ownsClipboardText() {
		return nil, platform.ErrNoClipboardImage
	}
	// An owner that did not list image/png among its TARGETS has no image, whatever
	// it would answer if asked for one anyway: some hand over their text bytes for
	// any target they are asked for.
	if has, err := clipboardOffersImage(); err != nil {
		return nil, err
	} else if !has {
		return nil, platform.ErrNoClipboardImage
	}
	api, connection, requestor, clipboard, owner, err := openClipboardRequest()
	if err != nil {
		return nil, err
	}
	defer closeClipboardRequest(api, connection, requestor)
	if owner == xcbAtomNone {
		return nil, platform.ErrNoClipboardImage
	}
	png, err := api.atom(connection, "image/png")
	if err != nil {
		return nil, err
	}
	property, err := api.atom(connection, "VIVA_CLIPBOARD_IMAGE")
	if err != nil {
		return nil, err
	}
	data, _, accepted, err := readSelectionBytes(api, connection, requestor, clipboard, png, property, platform.MaxClipboardImageBytes)
	if err != nil {
		return nil, err
	}
	if !accepted {
		return nil, platform.ErrNoClipboardImage
	}
	if err := platform.CheckClipboardPNG(data); err != nil {
		return nil, err
	}
	return data, nil
}

// ownsClipboardText reports whether this process's window is the CLIPBOARD
// owner, in which case it holds text and nothing else.
func ownsClipboardText() bool {
	win := activeWindow.Load()
	if win == nil {
		return false
	}
	win.mu.Lock()
	defer win.mu.Unlock()
	return activeWindow.Load() == win && win.connection != 0 && win.ownsClipboard
}

// openClipboardRequest connects to the X server with a 1x1 input window to
// receive a selection reply, and finds the CLIPBOARD owner.
func openClipboardRequest() (api *xcbAPI, connection uintptr, requestor, clipboard, owner uint32, err error) {
	return openSelectionRequest("CLIPBOARD")
}

// openSelectionRequest is openClipboardRequest for any selection, by name.
func openSelectionRequest(name string) (api *xcbAPI, connection uintptr, requestor, selection, owner uint32, err error) {
	var screen *xcbScreen
	api, connection, screen, err = openSystemConnection()
	if err != nil {
		return nil, 0, 0, 0, 0, err
	}
	requestor = api.generateID(connection)
	// PropertyNotify events pace an INCR transfer: the owner writes each chunk when
	// the previous one is deleted.
	eventMask := uint32(xcbEventPropertyChange)
	api.createWindow(connection, xcbCopyFromParent, requestor, screen.Root, 0, 0, 1, 1, 0, xcbWindowClassInput, screen.RootVisual, xcbCWEventMask, &eventMask)
	if selection, err = api.atom(connection, name); err == nil {
		owner, err = selectionOwner(api, connection, selection)
	}
	if err != nil {
		closeClipboardRequest(api, connection, requestor)
		return nil, 0, 0, 0, 0, err
	}
	return api, connection, requestor, selection, owner, nil
}

func closeClipboardRequest(api *xcbAPI, connection uintptr, requestor uint32) {
	api.destroyWindow(connection, requestor)
	api.disconnect(connection)
	api.close()
}

// readSelectionBytes asks the selection owner to convert to target and returns
// the property it wrote, with its type. accepted is false when the owner
// refused the target. A property above limit bytes is ErrClipboardImageTooLarge.
func readSelectionBytes(api *xcbAPI, connection uintptr, requestor, selection, target, property uint32, limit int) (data []byte, kind uint32, accepted bool, err error) {
	api.convertSelection(connection, requestor, selection, target, property, 0)
	if api.flush(connection) <= 0 || api.connectionHasError(connection) != 0 {
		return nil, 0, false, fmt.Errorf("linux: request clipboard data: XCB connection failed")
	}
	event, err := waitForSelection(api, connection, requestor, selection, target, clipboardWait)
	if err != nil {
		return nil, 0, false, err
	}
	defer api.free(event)
	if (*xcbSelectionNotifyEvent)(event).Property == xcbAtomNone {
		return nil, 0, false, nil
	}
	cookie := api.getProperty(connection, 1, requestor, property, xcbAtomAny, 0, uint32(limit/4)+1)
	var nativeError unsafe.Pointer
	reply := api.getPropertyReply(connection, cookie, &nativeError)
	if nativeError != nil {
		api.free(nativeError)
		if reply != nil {
			api.free(unsafe.Pointer(reply))
		}
		return nil, 0, false, fmt.Errorf("linux: read clipboard property")
	}
	if reply == nil {
		return nil, 0, false, fmt.Errorf("linux: read clipboard property returned no reply")
	}
	defer api.free(unsafe.Pointer(reply))
	if incr, atomErr := api.atom(connection, "INCR"); atomErr == nil && reply.Type == incr {
		// The first reply only announces a stream (and, in a CARD32, its size,
		// which an owner may get wrong, so it is not trusted); reading it with
		// delete set already told the owner to begin.
		data, err := readIncrementally(api, connection, requestor, property, limit)
		return data, reply.Type, true, err
	}
	length := int(api.getPropertyLength(reply))
	if length < 0 || length > limit || reply.BytesAfter != 0 {
		return nil, reply.Type, true, platform.ErrClipboardImageTooLarge
	}
	if reply.Format != 8 && reply.Format != 32 {
		return nil, reply.Type, true, fmt.Errorf("linux: clipboard property uses unsupported format %d", reply.Format)
	}
	if length == 0 {
		return nil, reply.Type, true, nil
	}
	return append([]byte(nil), unsafe.Slice((*byte)(api.getPropertyValue(reply)), length)...), reply.Type, true, nil
}

// readIncrementally collects an INCR transfer (ICCCM 2.7.2): each time the owner
// writes a chunk to the property the requestor reads and deletes it, and a chunk of
// zero length ends the stream. Every wait is bounded and the total never passes
// limit, so an owner that stalls or never stops costs a timeout or ErrTooLarge and
// not memory.
func readIncrementally(api *xcbAPI, connection uintptr, requestor, property uint32, limit int) ([]byte, error) {
	var data []byte
	file := pollFD{FD: api.fileDescriptor(connection), Events: xcbPollInput}
	for {
		if err := waitForNewValue(api, connection, requestor, property, &file); err != nil {
			return nil, err
		}
		cookie := api.getProperty(connection, 1, requestor, property, xcbAtomAny, 0, uint32(limit/4)+1)
		var nativeError unsafe.Pointer
		reply := api.getPropertyReply(connection, cookie, &nativeError)
		if nativeError != nil {
			api.free(nativeError)
			if reply != nil {
				api.free(unsafe.Pointer(reply))
			}
			return nil, fmt.Errorf("linux: read clipboard chunk")
		}
		if reply == nil {
			return nil, fmt.Errorf("linux: read clipboard chunk returned no reply")
		}
		length := int(api.getPropertyLength(reply))
		format := reply.Format
		var chunk []byte
		if length > 0 && (format == 8 || format == 32) {
			chunk = unsafe.Slice((*byte)(api.getPropertyValue(reply)), length)
		}
		tooBig := length < 0 || reply.BytesAfter != 0 || len(data)+length > limit
		unsupported := length > 0 && format != 8 && format != 32
		if !tooBig && !unsupported {
			data = append(data, chunk...)
		}
		api.free(unsafe.Pointer(reply))
		switch {
		case tooBig:
			return nil, platform.ErrClipboardImageTooLarge
		case unsupported:
			return nil, fmt.Errorf("linux: clipboard chunk uses unsupported format %d", format)
		case length == 0:
			return data, nil
		}
	}
}

// waitForNewValue waits, up to clipboardWait, for the owner to write the next
// chunk: a PropertyNotify on property of the requestor with state NewValue.
func waitForNewValue(api *xcbAPI, connection uintptr, requestor, property uint32, file *pollFD) error {
	deadline := time.Now().Add(clipboardWait)
	for {
		for event := api.pollForEvent(connection); event != nil; event = api.pollForEvent(connection) {
			notify := (*xcbPropertyEvent)(event)
			match := notify.ResponseType&0x7f == xcbPropertyNotify && notify.Window == requestor && notify.Atom == property && notify.State == 0
			api.free(event)
			if match {
				return nil
			}
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return fmt.Errorf("linux: clipboard owner stopped sending the image within %s", clipboardWait)
		}
		milliseconds := max(remaining.Milliseconds(), 1)
		ready := api.poll(file, 1, int32(min(milliseconds, int64(^uint32(0)>>1))))
		runtime.KeepAlive(file)
		if ready < 0 || api.connectionHasError(connection) != 0 {
			return fmt.Errorf("linux: wait for clipboard chunk failed")
		}
	}
}

func SetClipboardText(value string) error {
	if len(value) > maxClipboardBytes {
		return fmt.Errorf("linux: clipboard text exceeds 64 MiB")
	}
	if !utf8.ValidString(value) {
		return fmt.Errorf("linux: clipboard text is not valid UTF-8")
	}
	if waylandApp := activeWaylandApp.Load(); waylandApp != nil && waylandApp.hasWindows() {
		return waylandApp.invoke(func() error {
			return waylandApp.setClipboardText(value)
		})
	}
	win := activeWindow.Load()
	if win == nil {
		return fmt.Errorf("linux: clipboard ownership requires a running window")
	}
	win.mu.Lock()
	defer win.mu.Unlock()
	if activeWindow.Load() != win || win.connection == 0 {
		return fmt.Errorf("linux: clipboard ownership requires a running window")
	}
	if err := win.ensureClipboardAtoms(); err != nil {
		return err
	}
	win.api.setSelectionOwner(win.connection, win.id, win.clipboardAtom, 0)
	if win.api.flush(win.connection) <= 0 || win.api.connectionHasError(win.connection) != 0 {
		return fmt.Errorf("linux: claim clipboard: XCB connection failed")
	}
	owner, err := selectionOwner(win.api, win.connection, win.clipboardAtom)
	if err != nil {
		return err
	}
	if owner != win.id {
		return fmt.Errorf("linux: X server refused clipboard ownership")
	}
	win.clipboard = value
	win.ownsClipboard = true
	return nil
}

func SetCursorShape(shape uint8) error {
	glyphs := [...]uint16{68, 152, 60, 34, 150, 108, 116, 134, 136, 88}
	if int(shape) >= len(glyphs) {
		return fmt.Errorf("linux: invalid cursor %d", shape)
	}
	if waylandApp := activeWaylandApp.Load(); waylandApp != nil && waylandApp.hasWindows() {
		return waylandApp.invoke(func() error {
			waylandApp.setCursor(shape)
			return nil
		})
	}
	win := activeWindow.Load()
	if win == nil {
		return fmt.Errorf("linux: cursor requires a running window")
	}
	win.mu.Lock()
	defer win.mu.Unlock()
	if activeWindow.Load() != win || win.connection == 0 {
		return fmt.Errorf("linux: cursor requires a running window")
	}
	// A test's stand-in for the library has none to ask for themes.
	if !win.cursorsTried && win.api.library != 0 {
		win.cursorsTried = true
		win.themed = openThemedCursors(win.api, win.connection, win.screen)
	}
	if themed := win.themed.cursor(shape); themed != 0 {
		if err := win.checkRequest("apply cursor", win.api.changeWindow(win.connection, win.id, xcbCWCursor, &themed)); err == nil {
			if previous := win.cursor; previous != 0 {
				win.api.freeCursor(win.connection, previous)
			}
			win.cursor = themed
			if win.api.flush(win.connection) <= 0 || win.api.connectionHasError(win.connection) != 0 {
				return fmt.Errorf("linux: apply cursor: XCB connection failed")
			}
			return nil
		}
		win.api.freeCursor(win.connection, themed)
	}
	font := win.api.generateID(win.connection)
	fontName := []byte("cursor")
	if err := win.checkRequest("open cursor font", win.api.openFont(win.connection, font, uint16(len(fontName)), &fontName[0])); err != nil {
		return err
	}
	cursor := win.api.generateID(win.connection)
	glyph := glyphs[shape]
	if err := win.checkRequest("create cursor", win.api.createGlyphCursor(
		win.connection, cursor, font, font, glyph, glyph+1,
		0, 0, 0, 0xffff, 0xffff, 0xffff,
	)); err != nil {
		win.api.closeFont(win.connection, font)
		win.api.flush(win.connection)
		return err
	}
	win.api.closeFont(win.connection, font)
	if err := win.checkRequest("apply cursor", win.api.changeWindow(win.connection, win.id, xcbCWCursor, &cursor)); err != nil {
		win.api.freeCursor(win.connection, cursor)
		win.api.flush(win.connection)
		return err
	}
	previous := win.cursor
	if previous != 0 {
		win.api.freeCursor(win.connection, previous)
	}
	if win.api.flush(win.connection) <= 0 || win.api.connectionHasError(win.connection) != 0 {
		win.api.freeCursor(win.connection, cursor)
		return fmt.Errorf("linux: apply cursor: XCB connection failed")
	}
	win.cursor = cursor
	return nil
}

func OpenFiles(options OpenFileOptions) ([]string, bool, error) {
	filters, err := portalFilters(options.Filters)
	if err != nil {
		return nil, false, err
	}
	paths, canceled, err := portalChoose(portalFileRequest{
		method: "OpenFile", parent: portalParent(), title: options.Title, multiple: options.AllowMultiple, filters: filters,
	})
	if !errors.Is(err, errServiceMissing) {
		return paths, canceled, err
	}
	backend, path, err := fileDialogBackend()
	if err != nil {
		return nil, false, err
	}
	arguments, separator, err := openDialogArguments(backend, options)
	if err != nil {
		return nil, false, err
	}
	output, canceled, err := runDialog("open files", path, arguments)
	if err != nil || canceled {
		return nil, canceled, err
	}
	paths = splitDialogOutput(string(output), separator)
	if len(paths) == 0 {
		return nil, false, fmt.Errorf("linux: open file dialog returned no files")
	}
	return paths, false, nil
}

func OpenDirectory(options OpenDirectoryOptions) (string, bool, error) {
	paths, canceled, err := portalChoose(portalFileRequest{
		method: "OpenFile", parent: portalParent(), title: options.Title, directory: true,
	})
	if !errors.Is(err, errServiceMissing) {
		return firstPath(paths), canceled, singlePathError(paths, canceled, err, "open directory")
	}
	backend, executable, err := fileDialogBackend()
	if err != nil {
		return "", false, err
	}
	arguments, err := openDirectoryDialogArguments(backend, options)
	if err != nil {
		return "", false, err
	}
	output, canceled, err := runDialog("open directory", executable, arguments)
	if err != nil || canceled {
		return "", canceled, err
	}
	paths = splitDialogOutput(string(output), "\n")
	if len(paths) != 1 {
		return "", false, fmt.Errorf("linux: open directory dialog returned %d paths", len(paths))
	}
	return paths[0], false, nil
}

func SaveFile(options SaveFileOptions) (string, bool, error) {
	filters, err := portalFilters(options.Filters)
	if err != nil {
		return "", false, err
	}
	name, err := suggestedSaveName(options.SuggestedName, options.DefaultExtension)
	if err != nil {
		return "", false, err
	}
	paths, canceled, err := portalChoose(portalFileRequest{
		method: "SaveFile", parent: portalParent(), title: options.Title, filters: filters, currentName: name,
	})
	if !errors.Is(err, errServiceMissing) {
		return firstPath(paths), canceled, singlePathError(paths, canceled, err, "save file")
	}
	backend, path, err := fileDialogBackend()
	if err != nil {
		return "", false, err
	}
	arguments, err := saveDialogArguments(backend, options)
	if err != nil {
		return "", false, err
	}
	output, canceled, err := runDialog("save file", path, arguments)
	if err != nil || canceled {
		return "", canceled, err
	}
	paths = splitDialogOutput(string(output), "\n")
	if len(paths) != 1 {
		return "", false, fmt.Errorf("linux: save file dialog returned %d paths", len(paths))
	}
	return paths[0], false, nil
}

func OpenURL(target string) error {
	if strings.IndexByte(target, 0) >= 0 {
		return fmt.Errorf("linux: external URL contains NUL")
	}
	return runFirstDesktopCommand("open external URL", []desktopCommand{
		{name: "xdg-open", arguments: []string{target}},
		{name: "gio", arguments: []string{"open", target}},
	})
}

func PlaySystemSound(sound uint8) error {
	events := [...]string{"bell", "dialog-information", "dialog-warning", "dialog-error", "dialog-question"}
	if int(sound) >= len(events) {
		return fmt.Errorf("linux: invalid system sound %d", sound)
	}
	if path, err := exec.LookPath("canberra-gtk-play"); err == nil {
		return runCommand("play system sound", path, "--id", events[sound], "--description", "Viva")
	}
	win := activeWindow.Load()
	if win == nil {
		return unsupported("system sounds require canberra-gtk-play or a running X11 window")
	}
	win.mu.Lock()
	defer win.mu.Unlock()
	if activeWindow.Load() != win || win.connection == 0 {
		return unsupported("system sounds require canberra-gtk-play or a running X11 window")
	}
	win.api.bell(win.connection, 0)
	if win.api.flush(win.connection) <= 0 || win.api.connectionHasError(win.connection) != 0 {
		return fmt.Errorf("linux: play X11 bell: XCB connection failed")
	}
	return nil
}

func SendNotification(title, body string) error {
	if strings.IndexByte(title, 0) >= 0 || strings.IndexByte(body, 0) >= 0 {
		return fmt.Errorf("linux: notification text contains NUL")
	}
	name := notificationAppName()
	err := notifyOverBus(name, title, body)
	if !errors.Is(err, errServiceMissing) {
		return err
	}
	return runFirstDesktopCommand("send notification", []desktopCommand{
		{name: "notify-send", arguments: []string{"--app-name=" + name, "--", title, body}},
	})
}

// ShowFatalError shows the message with zenity or kdialog and waits for it to
// be dismissed. A session with neither prints nothing: the caller has already
// logged the reason.
func ShowFatalError(title, message string) {
	if strings.IndexByte(title, 0) >= 0 || strings.IndexByte(message, 0) >= 0 {
		return
	}
	_ = runFirstDesktopCommand("show fatal error", []desktopCommand{
		{name: "zenity", arguments: []string{"--error", "--no-markup", "--title=" + title, "--text=" + message}},
		{name: "kdialog", arguments: []string{"--title", title, "--error", message}},
	})
}

type desktopCommand struct {
	name      string
	arguments []string
}

func runFirstDesktopCommand(action string, candidates []desktopCommand) error {
	for _, candidate := range candidates {
		path, err := exec.LookPath(candidate.name)
		if err != nil {
			continue
		}
		return runCommand(action, path, candidate.arguments...)
	}
	names := make([]string, len(candidates))
	for index, candidate := range candidates {
		names[index] = candidate.name
	}
	return unsupported(fmt.Sprintf("%s requires %s", action, strings.Join(names, " or ")))
}

func runCommand(action, path string, arguments ...string) error {
	command := exec.Command(path, arguments...)
	output, err := command.CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(output))
		if detail != "" {
			return fmt.Errorf("linux: %s with %s: %w: %s", action, filepath.Base(path), err, detail)
		}
		return fmt.Errorf("linux: %s with %s: %w", action, filepath.Base(path), err)
	}
	return nil
}

func fileDialogBackend() (backend, path string, err error) {
	for _, candidate := range []string{"zenity", "kdialog"} {
		path, lookupErr := exec.LookPath(candidate)
		if lookupErr == nil {
			return candidate, path, nil
		}
	}
	return "", "", unsupported("file dialogs require the xdg-desktop-portal service, zenity or kdialog")
}

func openDialogArguments(backend string, options OpenFileOptions) ([]string, string, error) {
	filter, err := dialogFilter(options.Filters, backend)
	if err != nil {
		return nil, "", err
	}
	switch backend {
	case "zenity":
		arguments := []string{"--file-selection"}
		if options.Title != "" {
			arguments = append(arguments, "--title="+options.Title)
		}
		if options.AllowMultiple {
			arguments = append(arguments, "--multiple", "--separator="+dialogSeparator)
		}
		arguments = append(arguments, filter...)
		separator := "\n"
		if options.AllowMultiple {
			separator = dialogSeparator
		}
		return arguments, separator, nil
	case "kdialog":
		arguments := []string{"--getopenfilename", "", strings.Join(filter, "\n")}
		if options.AllowMultiple {
			arguments = append(arguments, "--multiple", "--separate-output")
		}
		if options.Title != "" {
			arguments = append(arguments, "--title", options.Title)
		}
		return arguments, "\n", nil
	default:
		return nil, "", fmt.Errorf("linux: unknown file dialog backend %q", backend)
	}
}

func openDirectoryDialogArguments(backend string, options OpenDirectoryOptions) ([]string, error) {
	switch backend {
	case "zenity":
		arguments := []string{"--file-selection", "--directory"}
		if options.Title != "" {
			arguments = append(arguments, "--title="+options.Title)
		}
		return arguments, nil
	case "kdialog":
		arguments := []string{"--getexistingdirectory", ""}
		if options.Title != "" {
			arguments = append(arguments, "--title", options.Title)
		}
		return arguments, nil
	default:
		return nil, fmt.Errorf("linux: unknown file dialog backend %q", backend)
	}
}

func saveDialogArguments(backend string, options SaveFileOptions) ([]string, error) {
	filter, err := dialogFilter(options.Filters, backend)
	if err != nil {
		return nil, err
	}
	name, err := suggestedSaveName(options.SuggestedName, options.DefaultExtension)
	if err != nil {
		return nil, err
	}
	switch backend {
	case "zenity":
		arguments := []string{"--file-selection", "--save", "--confirm-overwrite"}
		if options.Title != "" {
			arguments = append(arguments, "--title="+options.Title)
		}
		if name != "" {
			arguments = append(arguments, "--filename="+name)
		}
		return append(arguments, filter...), nil
	case "kdialog":
		arguments := []string{"--getsavefilename", name, strings.Join(filter, "\n")}
		if options.Title != "" {
			arguments = append(arguments, "--title", options.Title)
		}
		return arguments, nil
	default:
		return nil, fmt.Errorf("linux: unknown file dialog backend %q", backend)
	}
}

func dialogFilter(filters []FileFilter, backend string) ([]string, error) {
	result := make([]string, 0, len(filters))
	for index, filter := range filters {
		patterns, err := splitFilterPatterns(index, filter.Name, filter.Pattern)
		if err != nil {
			return nil, err
		}
		switch backend {
		case "zenity":
			result = append(result, "--file-filter="+filter.Name+" | "+strings.Join(patterns, " "))
		case "kdialog":
			result = append(result, strings.Join(patterns, " ")+"|"+filter.Name)
		default:
			return nil, fmt.Errorf("linux: unknown file dialog backend %q", backend)
		}
	}
	return result, nil
}

// portalFilters is the file chooser portal's form of the filters.
func portalFilters(filters []FileFilter) ([]portalFilter, error) {
	result := make([]portalFilter, 0, len(filters))
	for index, filter := range filters {
		patterns, err := splitFilterPatterns(index, filter.Name, filter.Pattern)
		if err != nil {
			return nil, err
		}
		result = append(result, portalFilter{name: filter.Name, patterns: patterns})
	}
	return result, nil
}

// portalParent names the window a dialog belongs to in the portal's terms, so that the
// desktop shows it over that window; "" for none.
func portalParent() string {
	if win := activeWindow.Load(); win != nil && win.id != 0 {
		return fmt.Sprintf("x11:%x", win.id)
	}
	return ""
}

// firstPath is the one path a single-choice dialog returned.
func firstPath(paths []string) string {
	if len(paths) == 1 {
		return paths[0]
	}
	return ""
}

// singlePathError is the error of a single-choice dialog that answered with no path or more
// than one.
func singlePathError(paths []string, canceled bool, err error, action string) error {
	if err != nil || canceled || len(paths) == 1 {
		return err
	}
	return fmt.Errorf("linux: %s dialog returned %d paths", action, len(paths))
}

func suggestedSaveName(name, extension string) (string, error) {
	if strings.IndexByte(name, 0) >= 0 || strings.IndexByte(extension, 0) >= 0 {
		return "", fmt.Errorf("linux: save file option contains NUL")
	}
	extension = strings.TrimSpace(strings.TrimPrefix(extension, "."))
	if strings.ContainsAny(extension, "*?[]/\\;") {
		return "", fmt.Errorf("linux: default extension %q contains wildcard or path syntax", extension)
	}
	if name != "" && extension != "" && filepath.Ext(name) == "" {
		name += "." + extension
	}
	return name, nil
}

func runDialog(action, path string, arguments []string) ([]byte, bool, error) {
	command := exec.Command(path, arguments...)
	output, err := command.Output()
	if err == nil {
		return output, false, nil
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) && exitError.ExitCode() == 1 {
		return nil, true, nil
	}
	if errors.As(err, &exitError) {
		detail := strings.TrimSpace(string(exitError.Stderr))
		if detail != "" {
			return nil, false, fmt.Errorf("linux: %s with %s: %w: %s", action, filepath.Base(path), err, detail)
		}
	}
	return nil, false, fmt.Errorf("linux: %s with %s: %w", action, filepath.Base(path), err)
}

func splitDialogOutput(output, separator string) []string {
	output = strings.TrimSuffix(output, "\n")
	output = strings.TrimSuffix(output, "\r")
	if output == "" {
		return nil
	}
	return strings.Split(output, separator)
}

func unsupported(detail string) error {
	return fmt.Errorf("%w: %s", ErrUnsupported, detail)
}

func openSystemConnection() (*xcbAPI, uintptr, *xcbScreen, error) {
	api, err := loadXCB()
	if err != nil {
		return nil, 0, nil, err
	}
	var screenIndex int32
	connection := api.connect(nil, &screenIndex)
	if connection == 0 || api.connectionHasError(connection) != 0 {
		if connection != 0 {
			api.disconnect(connection)
		}
		api.close()
		return nil, 0, nil, fmt.Errorf("linux: connect to X server")
	}
	iterator := api.rootsIterator(api.getSetup(connection))
	for index := int32(0); index < screenIndex && iterator.Remaining > 0; index++ {
		api.screenNext(&iterator)
	}
	if iterator.Data == nil || iterator.Remaining == 0 {
		api.disconnect(connection)
		api.close()
		return nil, 0, nil, fmt.Errorf("linux: X server has no screen")
	}
	return api, connection, iterator.Data, nil
}

func selectionOwner(api *xcbAPI, connection uintptr, selection uint32) (uint32, error) {
	cookie := api.getSelectionOwner(connection, selection)
	var nativeError unsafe.Pointer
	reply := api.selectionOwnerReply(connection, cookie, &nativeError)
	if nativeError != nil {
		api.free(nativeError)
		if reply != nil {
			api.free(unsafe.Pointer(reply))
		}
		return 0, fmt.Errorf("linux: query clipboard owner")
	}
	if reply == nil {
		return 0, fmt.Errorf("linux: query clipboard owner returned no reply")
	}
	defer api.free(unsafe.Pointer(reply))
	return reply.Owner, nil
}

func readLegacySelection(api *xcbAPI, connection uintptr, requestor, selection, property uint32) (string, error) {
	value, accepted, err := readSelection(api, connection, requestor, selection, xcbAtomString, property)
	if err != nil {
		return "", err
	}
	if !accepted {
		return "", unsupported("clipboard owner provides neither UTF8_STRING nor STRING")
	}
	return decodeLatin1(value), nil
}

// readSelection asks the owner for target as text. A text bigger than one property comes in
// pieces (INCR), as readSelectionBytes collects them.
func readSelection(api *xcbAPI, connection uintptr, requestor, selection, target, property uint32) (string, bool, error) {
	data, _, accepted, err := readSelectionBytes(api, connection, requestor, selection, target, property, maxClipboardBytes)
	if errors.Is(err, platform.ErrClipboardImageTooLarge) {
		return "", false, fmt.Errorf("linux: clipboard text exceeds 64 MiB")
	}
	if err != nil || !accepted {
		return "", false, err
	}
	return string(data), true, nil
}

func waitForSelection(api *xcbAPI, connection uintptr, requestor, selection, target uint32, timeout time.Duration) (unsafe.Pointer, error) {
	deadline := time.Now().Add(timeout)
	file := pollFD{FD: api.fileDescriptor(connection), Events: xcbPollInput}
	for {
		for event := api.pollForEvent(connection); event != nil; event = api.pollForEvent(connection) {
			notify := (*xcbSelectionNotifyEvent)(event)
			if notify.ResponseType&0x7f == xcbSelectionNotify && notify.Requestor == requestor && notify.Selection == selection && notify.Target == target {
				return event, nil
			}
			api.free(event)
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, fmt.Errorf("linux: clipboard owner did not respond within %s", timeout)
		}
		milliseconds := remaining.Milliseconds()
		if milliseconds < 1 {
			milliseconds = 1
		}
		if milliseconds > int64(^uint32(0)>>1) {
			milliseconds = int64(^uint32(0) >> 1)
		}
		ready := api.poll(&file, 1, int32(milliseconds))
		runtime.KeepAlive(&file)
		if ready == 0 {
			return nil, fmt.Errorf("linux: clipboard owner did not respond within %s", timeout)
		}
		if ready < 0 {
			return nil, fmt.Errorf("linux: wait for clipboard owner failed")
		}
		if api.connectionHasError(connection) != 0 {
			return nil, fmt.Errorf("linux: clipboard XCB connection failed")
		}
	}
}

func latin1Clipboard(value string) []byte {
	result := make([]byte, 0, len(value))
	for _, character := range value {
		if character > 255 {
			result = append(result, '?')
		} else {
			result = append(result, byte(character))
		}
	}
	return result
}

func decodeLatin1(value string) string {
	var result strings.Builder
	result.Grow(len(value))
	for index := 0; index < len(value); index++ {
		result.WriteRune(rune(value[index]))
	}
	return result.String()
}

func (win *window) checkRequest(action string, cookie xcbVoidCookie) error {
	nativeError := win.api.requestCheck(win.connection, cookie)
	if nativeError == nil {
		return nil
	}
	errorCode := nativeError.ErrorCode
	majorCode := nativeError.MajorCode
	minorCode := nativeError.MinorCode
	resourceID := nativeError.ResourceID
	win.api.free(unsafe.Pointer(nativeError))
	return fmt.Errorf("linux: %s: X11 error %d for request %d.%d on resource %d", action, errorCode, majorCode, minorCode, resourceID)
}
