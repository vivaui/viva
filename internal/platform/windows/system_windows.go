package windows

import (
	"encoding/binary"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

const (
	clipboardUnicodeText = 13
	clipboardDIB         = 8
	clipboardDIBV5       = 17
	clipboardHDrop       = 15
	globalMoveable       = 0x2
	globalZeroInit       = 0x40
	classInProcess       = 1
	fileSystemPath       = 0x80058000
	fileMustExist        = 0x1000
	pathMustExist        = 0x800
	forceFileSystem      = 0x40
	allowMultiSelect     = 0x200
	pickFolders          = 0x20
	overwritePrompt      = 0x2
	noChangeDirectory    = 0x8
	dialogCanceled       = 0x800704c7
	notifyAdd            = 0
	notifyModify         = 1
	notifyDelete         = 2
	notifyMessage        = 1
	notifyIcon           = 2
	notifyTip            = 4
	notifyInfo           = 0x10
	notifyInfoIcon       = 1

	maxClipboardBytes = 64 << 20
)

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

type dialogFilter struct {
	Name    *uint16
	Pattern *uint16
}

type notifyIconData struct {
	Size            uint32
	Padding0        uint32
	Window          uintptr
	ID              uint32
	Flags           uint32
	CallbackMessage uint32
	Padding1        uint32
	Icon            uintptr
	Tip             [128]uint16
	State           uint32
	StateMask       uint32
	Info            [256]uint16
	Version         uint32
	InfoTitle       [64]uint16
	InfoFlags       uint32
	Item            guid
	BalloonIcon     uintptr
}

var shell = systemDLL("shell32.dll")
var shellExecute = shell.NewProc("ShellExecuteW")
var shellNotifyIcon = shell.NewProc("Shell_NotifyIconW")
var loadIcon = user.NewProc("LoadIconW")
var setCursorNative = user.NewProc("SetCursor")
var messageBeep = user.NewProc("MessageBeep")
var messageBox = user.NewProc("MessageBoxW")
var openClipboard = user.NewProc("OpenClipboard")
var closeClipboard = user.NewProc("CloseClipboard")
var emptyClipboard = user.NewProc("EmptyClipboard")
var clipboardData = user.NewProc("GetClipboardData")
var setClipboardData = user.NewProc("SetClipboardData")
var clipboardFormatAvailable = user.NewProc("IsClipboardFormatAvailable")
var registerClipboardFormat = user.NewProc("RegisterClipboardFormatW")
var wideStringLength = kernel.NewProc("lstrlenW")
var globalAlloc = kernel.NewProc("GlobalAlloc")
var globalFree = kernel.NewProc("GlobalFree")
var globalLock = kernel.NewProc("GlobalLock")
var globalUnlock = kernel.NewProc("GlobalUnlock")
var globalSize = kernel.NewProc("GlobalSize")
var taskMemoryFree = ole.NewProc("CoTaskMemFree")

var clipboardMu sync.Mutex
var notificationMu sync.Mutex
var notificationWindow uintptr
var notificationAdded bool

var openDialogClass = guid{Data1: 0xdc1c5a9c, Data2: 0xe88a, Data3: 0x4dde, Data4: [8]byte{0xa5, 0xa1, 0x60, 0xf8, 0x2a, 0x20, 0xae, 0xf7}}
var openDialogInterface = guid{Data1: 0xd57c7288, Data2: 0xd4ad, Data3: 0x4768, Data4: [8]byte{0xbe, 0x02, 0x9d, 0x96, 0x95, 0x32, 0xd9, 0x60}}
var saveDialogClass = guid{Data1: 0xc0b4e2f3, Data2: 0xba21, Data3: 0x4773, Data4: [8]byte{0x8d, 0xba, 0x33, 0x5e, 0xc9, 0x46, 0xeb, 0x8b}}
var saveDialogInterface = guid{Data1: 0x84bccd23, Data2: 0x5fde, Data3: 0x4cdb, Data4: [8]byte{0xae, 0xa4, 0xaf, 0x64, 0xb8, 0x3d, 0x78, 0xab}}

func ClipboardText() (string, error) {
	clipboardMu.Lock()
	defer clipboardMu.Unlock()
	available, _, _ := clipboardFormatAvailable.Call(clipboardUnicodeText)
	if available == 0 {
		return "", nil
	}
	if opened, _, callErr := openClipboard.Call(0); opened == 0 {
		return "", windowsCallError("open clipboard", callErr)
	}
	defer closeClipboard.Call()
	handle, _, callErr := clipboardData.Call(clipboardUnicodeText)
	if handle == 0 {
		return "", windowsCallError("read clipboard", callErr)
	}
	size, _, _ := globalSize.Call(handle)
	if size == 0 || size > maxClipboardBytes || size%2 != 0 {
		return "", fmt.Errorf("windows: invalid clipboard text size %d", size)
	}
	memory, _, callErr := globalLock.Call(handle)
	if memory == 0 {
		return "", windowsCallError("lock clipboard text", callErr)
	}
	defer globalUnlock.Call(handle)
	values := unsafe.Slice((*uint16)(pointer(memory)), int(size/2))
	end := 0
	for end < len(values) && values[end] != 0 {
		end++
	}
	return syscall.UTF16ToString(values[:end]), nil
}

var pngFormatOnce sync.Once
var pngFormat uintptr
var dropEffectFormat uintptr
var dropEffectFormatOnce sync.Once

// registeredPNGFormat is the clipboard format named "PNG", which browsers, Paint
// and the Snipping Tool offer alongside the bitmap formats; 0 when it cannot be
// registered.
func registeredPNGFormat() uintptr {
	pngFormatOnce.Do(func() {
		name, _ := syscall.UTF16PtrFromString("PNG")
		pngFormat, _, _ = registerClipboardFormat.Call(uintptr(unsafe.Pointer(name)))
	})
	return pngFormat
}

// registeredDropEffectFormat is the clipboard format Explorer reads to know a
// drag or paste is a copy.
func registeredDropEffectFormat() uintptr {
	dropEffectFormatOnce.Do(func() {
		name, _ := syscall.UTF16PtrFromString("Preferred DropEffect")
		dropEffectFormat, _, _ = registerClipboardFormat.Call(uintptr(unsafe.Pointer(name)))
	})
	return dropEffectFormat
}

// HasClipboardImage reports whether the clipboard offers a PNG or a bitmap
// (Windows converts CF_BITMAP, CF_DIB and CF_DIBV5 into one another). It does
// not open the clipboard.
func HasClipboardImage() bool {
	for _, format := range [...]uintptr{registeredPNGFormat(), clipboardDIBV5, clipboardDIB} {
		if format == 0 {
			continue
		}
		if available, _, _ := clipboardFormatAvailable.Call(format); available != 0 {
			return true
		}
	}
	return false
}

// ClipboardImagePNG returns the clipboard's image as PNG bytes: the registered
// "PNG" format as it is, otherwise CF_DIBV5 or CF_DIB converted.
func ClipboardImagePNG() ([]byte, error) {
	clipboardMu.Lock()
	defer clipboardMu.Unlock()
	if !HasClipboardImage() {
		return nil, platform.ErrNoClipboardImage
	}
	// Another program may hold the clipboard for a moment while it writes.
	var opened uintptr
	var callErr error
	for attempt := 0; attempt < 5; attempt++ {
		if opened, _, callErr = openClipboard.Call(0); opened != 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if opened == 0 {
		return nil, windowsCallError("open clipboard", callErr)
	}
	defer closeClipboard.Call()
	if format := registeredPNGFormat(); format != 0 {
		if available, _, _ := clipboardFormatAvailable.Call(format); available != 0 {
			data, err := clipboardBytes(format, platform.MaxClipboardImageBytes+1<<10)
			if err != nil {
				return nil, err
			}
			if data, err = platform.TrimClipboardPNG(data); err != nil {
				return nil, err
			}
			return data, platform.CheckClipboardPNG(data)
		}
	}
	for _, format := range [...]uintptr{clipboardDIBV5, clipboardDIB} {
		if available, _, _ := clipboardFormatAvailable.Call(format); available == 0 {
			continue
		}
		dib, err := clipboardBytes(format, platform.MaxClipboardRawBytes)
		if err != nil {
			return nil, err
		}
		return platform.DIBToPNG(dib)
	}
	return nil, platform.ErrNoClipboardImage
}

// maxClipboardFileListBytes bounds the CF_HDROP block: MaxClipboardFiles long
// paths fit with room to spare.
const maxClipboardFileListBytes = 16 << 20

// ClipboardFiles returns the paths of the files the clipboard holds: the
// CF_HDROP list Explorer leaves there after a copy.
func ClipboardFiles() ([]string, error) {
	clipboardMu.Lock()
	defer clipboardMu.Unlock()
	if available, _, _ := clipboardFormatAvailable.Call(clipboardHDrop); available == 0 {
		return nil, platform.ErrNoClipboardFiles
	}
	// Another program may hold the clipboard for a moment while it writes.
	var opened uintptr
	var callErr error
	for attempt := 0; attempt < 5; attempt++ {
		if opened, _, callErr = openClipboard.Call(0); opened != 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if opened == 0 {
		return nil, windowsCallError("open clipboard", callErr)
	}
	defer closeClipboard.Call()
	handle, _, callErr := clipboardData.Call(clipboardHDrop)
	if handle == 0 {
		return nil, windowsCallError("read clipboard files", callErr)
	}
	size, _, _ := globalSize.Call(handle)
	if size == 0 || size > maxClipboardFileListBytes {
		return nil, fmt.Errorf("windows: invalid clipboard file list size %d", size)
	}
	memory, _, callErr := globalLock.Call(handle)
	if memory == 0 {
		return nil, windowsCallError("lock clipboard files", callErr)
	}
	defer globalUnlock.Call(handle)
	paths := platform.FilesFromDropFiles(unsafe.Slice((*byte)(pointer(memory)), int(size)))
	if len(paths) == 0 {
		return nil, platform.ErrNoClipboardFiles
	}
	return paths, nil
}

// clipboardBytes copies the global memory block of one format of the open
// clipboard; a block above limit is ErrClipboardImageTooLarge.
func clipboardBytes(format uintptr, limit int) ([]byte, error) {
	handle, _, callErr := clipboardData.Call(format)
	if handle == 0 {
		return nil, windowsCallError("read clipboard", callErr)
	}
	size, _, _ := globalSize.Call(handle)
	if size == 0 {
		return nil, platform.ErrNoClipboardImage
	}
	if size > uintptr(limit) {
		return nil, platform.ErrClipboardImageTooLarge
	}
	memory, _, callErr := globalLock.Call(handle)
	if memory == 0 {
		return nil, windowsCallError("lock clipboard image", callErr)
	}
	defer globalUnlock.Call(handle)
	return append([]byte(nil), unsafe.Slice((*byte)(pointer(memory)), int(size))...), nil
}

func SetClipboardText(value string) error {
	encoded, err := syscall.UTF16FromString(value)
	if err != nil {
		return fmt.Errorf("windows: encode clipboard text: %w", err)
	}
	clipboardMu.Lock()
	defer clipboardMu.Unlock()
	bytes := uintptr(len(encoded) * 2)
	handle, _, callErr := globalAlloc.Call(globalMoveable|globalZeroInit, bytes)
	if handle == 0 {
		return windowsCallError("allocate clipboard text", callErr)
	}
	transferred := false
	defer func() {
		if !transferred {
			globalFree.Call(handle)
		}
	}()
	memory, _, callErr := globalLock.Call(handle)
	if memory == 0 {
		return windowsCallError("lock clipboard text", callErr)
	}
	copy(unsafe.Slice((*uint16)(pointer(memory)), len(encoded)), encoded)
	globalUnlock.Call(handle)
	owner := uintptr(0)
	if state := active.Load(); state != nil {
		owner = state.window
	}
	if opened, _, callErr := openClipboard.Call(owner); opened == 0 {
		return windowsCallError("open clipboard", callErr)
	}
	defer closeClipboard.Call()
	if emptied, _, callErr := emptyClipboard.Call(); emptied == 0 {
		return windowsCallError("empty clipboard", callErr)
	}
	if result, _, callErr := setClipboardData.Call(clipboardUnicodeText, handle); result == 0 {
		return windowsCallError("write clipboard", callErr)
	}
	transferred = true
	return nil
}

// SetClipboardFiles makes paths the files the clipboard holds, as Explorer does
// after the user copies a selection: a CF_HDROP list, the file names as text,
// and the copy effect.
func SetClipboardFiles(paths []string) error {
	drop := platform.DropFilesBytes(paths)
	names := make([]string, len(paths))
	for index, path := range paths {
		names[index] = filepath.Base(path)
	}
	text, err := syscall.UTF16FromString(strings.Join(names, "\r\n"))
	if err != nil {
		return fmt.Errorf("windows: encode clipboard file names: %w", err)
	}
	clipboardMu.Lock()
	defer clipboardMu.Unlock()
	owner := uintptr(0)
	if state := active.Load(); state != nil {
		owner = state.window
	}
	if opened, _, callErr := openClipboard.Call(owner); opened == 0 {
		return windowsCallError("open clipboard", callErr)
	}
	defer closeClipboard.Call()
	if emptied, _, callErr := emptyClipboard.Call(); emptied == 0 {
		return windowsCallError("empty clipboard", callErr)
	}
	if err := setClipboardBlock(clipboardHDrop, drop); err != nil {
		return err
	}
	// The names and the effect are what a file manager adds beside the list;
	// a clipboard that already holds the files is still usable without them.
	textBytes := make([]byte, len(text)*2)
	for index, unit := range text {
		binary.LittleEndian.PutUint16(textBytes[index*2:], unit)
	}
	_ = setClipboardBlock(clipboardUnicodeText, textBytes)
	if format := registeredDropEffectFormat(); format != 0 {
		_ = setClipboardBlock(format, []byte{1, 0, 0, 0}) // DROPEFFECT_COPY
	}
	return nil
}

// setClipboardBlock puts one clipboard format on the open clipboard. The system
// owns the memory after a successful SetClipboardData; a failure frees it.
func setClipboardBlock(format uintptr, data []byte) error {
	handle, _, callErr := globalAlloc.Call(globalMoveable|globalZeroInit, uintptr(len(data)))
	if handle == 0 {
		return windowsCallError("allocate clipboard data", callErr)
	}
	transferred := false
	defer func() {
		if !transferred {
			globalFree.Call(handle)
		}
	}()
	memory, _, callErr := globalLock.Call(handle)
	if memory == 0 {
		return windowsCallError("lock clipboard data", callErr)
	}
	copy(unsafe.Slice((*byte)(pointer(memory)), len(data)), data)
	globalUnlock.Call(handle)
	if result, _, callErr := setClipboardData.Call(format, handle); result == 0 {
		return windowsCallError("write clipboard data", callErr)
	}
	transferred = true
	return nil
}

func SetCursorShape(shape uint8) error {
	handle, err := cursorHandle(shape)
	if err != nil {
		return err
	}
	state := active.Load()
	if state == nil || state.window == 0 {
		return fmt.Errorf("windows: cursor requires a running window")
	}
	state.cursor.Store(handle)
	if posted, _, callErr := postMessage.Call(state.window, wmApplyCursor, 0, 0); posted == 0 {
		return windowsCallError("apply cursor", callErr)
	}
	return nil
}

func OpenFiles(options OpenFileOptions) ([]string, bool, error) {
	var paths []string
	canceled := false
	err := withSTA(func() error {
		dialog, err := createFileDialog(&openDialogClass, &openDialogInterface)
		if err != nil {
			return err
		}
		defer release(&dialog)
		flags := uint32(forceFileSystem | fileMustExist | pathMustExist | noChangeDirectory)
		if options.AllowMultiple {
			flags |= allowMultiSelect
		}
		if err := configureFileDialog(dialog, options.Title, options.Filters, flags); err != nil {
			return err
		}
		status := uint32(comCall(dialog, 3, dialogOwner()))
		if status == dialogCanceled {
			canceled = true
			return nil
		}
		if err := hresult(uintptr(status)); err != nil {
			return fmt.Errorf("windows: show open file dialog: %w", err)
		}
		if options.AllowMultiple {
			paths, err = dialogResults(dialog)
		} else {
			var item uintptr
			if err := hresult(comCall(dialog, 20, uintptr(unsafe.Pointer(&item)))); err != nil {
				return fmt.Errorf("windows: get open file result: %w", err)
			}
			defer release(&item)
			path, err := shellItemPath(item)
			if err != nil {
				return err
			}
			paths = []string{path}
		}
		return err
	})
	return paths, canceled, err
}

func OpenDirectory(options OpenDirectoryOptions) (string, bool, error) {
	var path string
	canceled := false
	err := withSTA(func() error {
		dialog, err := createFileDialog(&openDialogClass, &openDialogInterface)
		if err != nil {
			return err
		}
		defer release(&dialog)
		flags := uint32(forceFileSystem | pathMustExist | noChangeDirectory | pickFolders)
		if err := configureFileDialog(dialog, options.Title, nil, flags); err != nil {
			return err
		}
		status := uint32(comCall(dialog, 3, dialogOwner()))
		if status == dialogCanceled {
			canceled = true
			return nil
		}
		if err := hresult(uintptr(status)); err != nil {
			return fmt.Errorf("windows: show open directory dialog: %w", err)
		}
		var item uintptr
		if err := hresult(comCall(dialog, 20, uintptr(unsafe.Pointer(&item)))); err != nil {
			return fmt.Errorf("windows: get open directory result: %w", err)
		}
		defer release(&item)
		path, err = shellItemPath(item)
		return err
	})
	return path, canceled, err
}

func SaveFile(options SaveFileOptions) (string, bool, error) {
	path, _, canceled, err := saveFile(options, "", false)
	return path, canceled, err
}

func SaveFileWithCheckbox(options SaveFileOptions, label string, initial bool) (string, bool, bool, error) {
	return saveFile(options, label, initial)
}

func saveFile(options SaveFileOptions, checkboxLabel string, initial bool) (string, bool, bool, error) {
	var path string
	var checked bool
	canceled := false
	err := withSTA(func() error {
		dialog, err := createFileDialog(&saveDialogClass, &saveDialogInterface)
		if err != nil {
			return err
		}
		defer release(&dialog)
		var custom uintptr
		if checkboxLabel != "" {
			custom, err = addSaveDialogCheckbox(dialog, checkboxLabel, initial)
			if err != nil {
				return err
			}
			defer release(&custom)
		}
		flags := uint32(forceFileSystem | pathMustExist | overwritePrompt | noChangeDirectory)
		if err := configureFileDialog(dialog, options.Title, options.Filters, flags); err != nil {
			return err
		}
		if options.SuggestedName != "" {
			name, err := syscall.UTF16PtrFromString(options.SuggestedName)
			if err != nil {
				return err
			}
			if err := hresult(comCall(dialog, 15, uintptr(unsafe.Pointer(name)))); err != nil {
				return fmt.Errorf("windows: set save file name: %w", err)
			}
		}
		if options.DefaultExtension != "" {
			extension, err := syscall.UTF16PtrFromString(options.DefaultExtension)
			if err != nil {
				return err
			}
			if err := hresult(comCall(dialog, 22, uintptr(unsafe.Pointer(extension)))); err != nil {
				return fmt.Errorf("windows: set save extension: %w", err)
			}
		}
		status := uint32(comCall(dialog, 3, dialogOwner()))
		if status == dialogCanceled {
			canceled = true
			return nil
		}
		if err := hresult(uintptr(status)); err != nil {
			return fmt.Errorf("windows: show save file dialog: %w", err)
		}
		var item uintptr
		if err := hresult(comCall(dialog, 20, uintptr(unsafe.Pointer(&item)))); err != nil {
			return fmt.Errorf("windows: get save file result: %w", err)
		}
		defer release(&item)
		path, err = shellItemPath(item)
		if err != nil {
			return err
		}
		if custom != 0 {
			checked, err = saveDialogCheckboxState(custom)
		}
		return err
	})
	return path, checked, canceled, err
}

func OpenURL(target string) error {
	verb, _ := syscall.UTF16PtrFromString("open")
	value, err := syscall.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	result, _, callErr := shellExecute.Call(dialogOwner(), uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(value)), 0, 0, 1)
	if result <= 32 {
		return windowsCallError(fmt.Sprintf("open external URL (code %d)", result), callErr)
	}
	return nil
}

func PlaySystemSound(sound uint8) error {
	kinds := [...]uintptr{0, 0x40, 0x30, 0x10, 0x20}
	if int(sound) >= len(kinds) {
		return fmt.Errorf("windows: invalid system sound %d", sound)
	}
	played, _, callErr := messageBeep.Call(kinds[sound])
	if played == 0 {
		return windowsCallError("play system sound", callErr)
	}
	return nil
}

// ShowFatalError shows a message box that blocks until it is dismissed. It does
// not need a window or a message loop of its own.
func ShowFatalError(title, message string) {
	titleText, err := syscall.UTF16PtrFromString(title)
	if err != nil {
		titleText, _ = syscall.UTF16PtrFromString("Error")
	}
	messageText, err := syscall.UTF16PtrFromString(message)
	if err != nil {
		return
	}
	const iconError, setForeground, topMost = 0x10, 0x10000, 0x40000
	messageBox.Call(0, uintptr(unsafe.Pointer(messageText)), uintptr(unsafe.Pointer(titleText)), iconError|setForeground|topMost)
}

// SendNotification shows a balloon from the notification area. The icon that
// carries it hangs on a window of the application, any one, or on the hidden
// window its status items use: the application needs a window or a status item
// to be running, not the one window that happened to be used last.
func SendNotification(title, body string) error {
	host := notificationHost()
	if host == 0 {
		return fmt.Errorf("windows: notification requires a running window or status item")
	}
	icon := notificationIcon(host)
	if icon == 0 {
		var callErr error
		icon, _, callErr = loadIcon.Call(0, 32512)
		if icon == 0 {
			return windowsCallError("load notification icon", callErr)
		}
	}
	data := notifyIconData{Size: uint32(unsafe.Sizeof(notifyIconData{})), Window: host, ID: 1, Flags: notifyIcon | notifyTip | notifyInfo, Icon: icon, InfoFlags: notifyInfoIcon}
	copyUTF16(data.Tip[:], "Viva")
	copyUTF16(data.InfoTitle[:], title)
	copyUTF16(data.Info[:], body)
	notificationMu.Lock()
	defer notificationMu.Unlock()
	operation := uintptr(notifyModify)
	if !notificationAdded || notificationWindow != host {
		if notificationAdded {
			removeNotificationIconLocked(notificationWindow)
		}
		operation = notifyAdd
	}
	ok, _, callErr := shellNotifyIcon.Call(operation, uintptr(unsafe.Pointer(&data)))
	if ok == 0 {
		return windowsCallError("send notification", callErr)
	}
	notificationAdded = true
	notificationWindow = host
	return nil
}

func removeNotificationIcon(window uintptr) {
	notificationMu.Lock()
	defer notificationMu.Unlock()
	removeNotificationIconLocked(window)
}

func removeNotificationIconLocked(window uintptr) {
	if !notificationAdded || notificationWindow != window {
		return
	}
	data := notifyIconData{Size: uint32(unsafe.Sizeof(notifyIconData{})), Window: window, ID: 1}
	shellNotifyIcon.Call(notifyDelete, uintptr(unsafe.Pointer(&data)))
	notificationAdded = false
	notificationWindow = 0
}

func cursorHandle(shape uint8) (uintptr, error) {
	if shape == cursorHidden {
		return hiddenCursor()
	}
	identifiers := [...]uintptr{32512, 32513, 32649, 32515, 32514, 32644, 32645, 32642, 32643, 32648}
	if int(shape) >= len(identifiers) {
		return 0, fmt.Errorf("windows: invalid cursor %d", shape)
	}
	handle, _, callErr := loadCursor.Call(0, identifiers[shape])
	if handle == 0 {
		return 0, windowsCallError("load cursor", callErr)
	}
	return handle, nil
}

func withSTA(run func() error) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	status, _, _ := initializeCOM.Call(0, 2)
	code := uint32(status)
	if code != 0 && code != 1 {
		return fmt.Errorf("windows: initialize COM apartment: %w", hresult(status))
	}
	defer uninitializeCOM.Call()
	return run()
}

func createFileDialog(classID, interfaceID *guid) (uintptr, error) {
	var dialog uintptr
	status, _, _ := createCOM.Call(uintptr(unsafe.Pointer(classID)), 0, classInProcess, uintptr(unsafe.Pointer(interfaceID)), uintptr(unsafe.Pointer(&dialog)))
	if err := hresult(status); err != nil {
		return 0, fmt.Errorf("windows: create file dialog: %w", err)
	}
	return dialog, nil
}

func configureFileDialog(dialog uintptr, title string, filters []FileFilter, flags uint32) error {
	var current uint32
	if err := hresult(comCall(dialog, 10, uintptr(unsafe.Pointer(&current)))); err != nil {
		return fmt.Errorf("windows: get file dialog options: %w", err)
	}
	if err := hresult(comCall(dialog, 9, uintptr(current|flags))); err != nil {
		return fmt.Errorf("windows: set file dialog options: %w", err)
	}
	if title != "" {
		value, err := syscall.UTF16PtrFromString(title)
		if err != nil {
			return err
		}
		if err := hresult(comCall(dialog, 17, uintptr(unsafe.Pointer(value)))); err != nil {
			return fmt.Errorf("windows: set file dialog title: %w", err)
		}
	}
	if len(filters) == 0 {
		return nil
	}
	strings := make([][]uint16, len(filters)*2)
	specs := make([]dialogFilter, len(filters))
	for index, filter := range filters {
		name, err := syscall.UTF16FromString(filter.Name)
		if err != nil {
			return err
		}
		pattern, err := syscall.UTF16FromString(filter.Pattern)
		if err != nil {
			return err
		}
		strings[index*2], strings[index*2+1] = name, pattern
		specs[index] = dialogFilter{Name: &name[0], Pattern: &pattern[0]}
	}
	status := comCall(dialog, 4, uintptr(len(specs)), uintptr(unsafe.Pointer(&specs[0])))
	runtime.KeepAlive(strings)
	if err := hresult(status); err != nil {
		return fmt.Errorf("windows: set file dialog filters: %w", err)
	}
	return nil
}

func dialogResults(dialog uintptr) ([]string, error) {
	var array uintptr
	if err := hresult(comCall(dialog, 27, uintptr(unsafe.Pointer(&array)))); err != nil {
		return nil, fmt.Errorf("windows: get open file results: %w", err)
	}
	defer release(&array)
	var count uint32
	if err := hresult(comCall(array, 7, uintptr(unsafe.Pointer(&count)))); err != nil {
		return nil, fmt.Errorf("windows: count open file results: %w", err)
	}
	if count > 1024 {
		return nil, fmt.Errorf("windows: open file result count %d exceeds limit", count)
	}
	paths := make([]string, 0, count)
	for index := uint32(0); index < count; index++ {
		var item uintptr
		if err := hresult(comCall(array, 8, uintptr(index), uintptr(unsafe.Pointer(&item)))); err != nil {
			return nil, fmt.Errorf("windows: get open file result %d: %w", index, err)
		}
		path, err := shellItemPath(item)
		release(&item)
		if err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	return paths, nil
}

func shellItemPath(item uintptr) (string, error) {
	var value uintptr
	if err := hresult(comCall(item, 5, fileSystemPath, uintptr(unsafe.Pointer(&value)))); err != nil {
		return "", fmt.Errorf("windows: get file system path: %w", err)
	}
	defer taskMemoryFree.Call(value)
	length, _, _ := wideStringLength.Call(value)
	if length > 1<<20 {
		return "", fmt.Errorf("windows: file system path exceeds limit")
	}
	return syscall.UTF16ToString(unsafe.Slice((*uint16)(pointer(value)), int(length))), nil
}

var systemParametersInfo = user.NewProc("SystemParametersInfoW")

const spiGetClientAreaAnimation = 0x1042

// ReduceMotion reports whether the user turned off client-area animations
// ("Show animations in Windows"). Failure to query reads as animations on.
func ReduceMotion() bool {
	if systemParametersInfo.Find() != nil {
		return false
	}
	var enabled int32 = 1
	ok, _, _ := systemParametersInfo.Call(spiGetClientAreaAnimation, 0, uintptr(unsafe.Pointer(&enabled)), 0)
	return ok != 0 && enabled == 0
}

var getUserPreferredUILanguages = kernel.NewProc("GetUserPreferredUILanguages")

const muiLanguageName = 0x8

// PreferredLanguages is the user's display-language list, most preferred first,
// as locale names ("zh-CN", "en-US"). It is empty when the query fails.
func PreferredLanguages() []string {
	if getUserPreferredUILanguages.Find() != nil {
		return nil
	}
	var count, size uint32
	if ok, _, _ := getUserPreferredUILanguages.Call(muiLanguageName, uintptr(unsafe.Pointer(&count)), 0, uintptr(unsafe.Pointer(&size))); ok == 0 || size == 0 || size > 1<<14 {
		return nil
	}
	buffer := make([]uint16, size)
	if ok, _, _ := getUserPreferredUILanguages.Call(muiLanguageName, uintptr(unsafe.Pointer(&count)), uintptr(unsafe.Pointer(&buffer[0])), uintptr(unsafe.Pointer(&size))); ok == 0 {
		return nil
	}
	return splitMultiString(buffer[:min(int(size), len(buffer))])
}

// splitMultiString reads a Windows multi-string: UTF-16 names each ended by a
// zero, the list ended by an empty one.
func splitMultiString(buffer []uint16) []string {
	var names []string
	start := 0
	for index, unit := range buffer {
		if unit != 0 {
			continue
		}
		if index == start {
			break
		}
		names = append(names, syscall.UTF16ToString(buffer[start:index]))
		start = index + 1
	}
	return names
}

func dialogOwner() uintptr {
	if state := active.Load(); state != nil {
		return state.window
	}
	return 0
}

func copyUTF16(destination []uint16, value string) {
	encoded := syscall.StringToUTF16(value)
	if len(encoded) > len(destination) {
		encoded = encoded[:len(destination)]
		encoded[len(encoded)-1] = 0
	}
	copy(destination, encoded)
}

func windowsCallError(action string, callErr error) error {
	if callErr == nil || callErr == syscall.Errno(0) {
		return fmt.Errorf("windows: %s failed", action)
	}
	return fmt.Errorf("windows: %s: %w", action, callErr)
}

var regGetValue = advapi.NewProc("RegGetValueW")

const (
	hkeyCurrentUser = 0x80000001
	rrfRTRegDWord   = 0x10
)

// DarkAppearance reports whether the user chose the dark mode for applications
// (Settings, Personalization, Colors). known is false where the setting is not
// there, as on a Windows that has no dark mode.
func DarkAppearance() (dark, known bool) {
	if regGetValue.Find() != nil {
		return false, false
	}
	subkey, err := syscall.UTF16PtrFromString(`Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`)
	if err != nil {
		return false, false
	}
	name, err := syscall.UTF16PtrFromString("AppsUseLightTheme")
	if err != nil {
		return false, false
	}
	var value, size uint32 = 1, 4
	status, _, _ := regGetValue.Call(hkeyCurrentUser, uintptr(unsafe.Pointer(subkey)), uintptr(unsafe.Pointer(name)),
		rrfRTRegDWord, 0, uintptr(unsafe.Pointer(&value)), uintptr(unsafe.Pointer(&size)))
	if status != 0 {
		return false, false
	}
	return value == 0, true
}

// notificationHost is the window a notification icon hangs on: the window last
// used if it still exists, else any window of the application, else the hidden
// window of its status items.
func notificationHost() uintptr {
	if state := active.Load(); state != nil && state.window != 0 {
		return state.window
	}
	var host uintptr
	windowStates.Range(func(_, value any) bool {
		if state := value.(*windowState); state.window != 0 {
			host = state.window
			return false
		}
		return true
	})
	if host != 0 {
		return host
	}
	return statusWindowHandle.Load()
}

// notificationIcon is the picture of the application's window that carries the
// notification, the small one its title bar shows; zero when it has none.
func notificationIcon(host uintptr) uintptr {
	if value, ok := windowStates.Load(host); ok {
		return value.(*windowState).icons[iconSmall]
	}
	return 0
}

// cursorHidden is the shape a surface that draws its own pointer asks for.
const cursorHidden uint8 = 10

var createCursor = user.NewProc("CreateCursor")

// hiddenCursor makes the fully transparent 1x1 cursor once: its masks leave
// every pixel of the screen untouched, so nothing is drawn where the pointer
// is. A real handle keeps the window's cursor bookkeeping unchanged.
var hiddenCursor = sync.OnceValues(func() (uintptr, error) {
	// Monochrome masks are a bit per pixel, each row padded to a DWORD.
	andMask := [...]byte{0xFF, 0, 0, 0} // keep the screen pixel
	xorMask := [...]byte{0, 0, 0, 0}    // XOR in nothing
	handle, _, callErr := createCursor.Call(0, 0, 0, 1, 1,
		uintptr(unsafe.Pointer(&andMask[0])), uintptr(unsafe.Pointer(&xorMask[0])))
	if handle == 0 {
		return 0, windowsCallError("create hidden cursor", callErr)
	}
	return handle, nil
})
