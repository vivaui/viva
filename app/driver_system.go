//go:build darwin || windows || linux || (js && wasm)

package app

import (
	"fmt"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/vivaui/viva/internal/platform"
)

const driverSaveFileQueueSize = 8
const driverOpenFilesQueueSize = 8
const driverOpenDirectoryQueueSize = 8

type driverSystem struct {
	driver         *Driver
	clipboardImage []byte   // PNG bytes; the clipboard holds this or text, never both
	clipboardFiles []string // paths of copied files, held beside the text a file manager adds
	primary        string   // the primary selection (Linux only), private like the clipboard
	schemes        []string // the URL schemes SetSchemeHandlers declared as registered
}

type driverSaveFileResponse struct {
	path         string
	checked      bool
	withCheckbox bool
	err          error
}

type driverOpenFilesResponse struct {
	paths []string
	err   error
}

type driverOpenDirectoryResponse struct {
	path string
	err  error
}

var activeDriverSystem atomic.Pointer[driverSystem]

// A worker may finish after a driver closes, or before a tool's first driver
// opens. Once a process has installed a driver or a replacement runner, its
// RevealPath calls must never fall through to a real file manager.
var nonNativeSystemUsed atomic.Bool

func markNonNativeSystemUsed() { nonNativeSystemUsed.Store(true) }

var revealedPaths struct {
	sync.Mutex
	path string
	ok   bool
}
var driverSystems struct {
	sync.Mutex
	values []*driverSystem
}

func driverClipboardText() (string, error, bool) {
	system := activeDriverSystem.Load()
	if system == nil {
		return "", nil, false
	}
	return system.driver.clipboard, nil, true
}

func driverSetClipboardText(value string) (error, bool) {
	system := activeDriverSystem.Load()
	if system == nil {
		return nil, false
	}
	system.driver.clipboard, system.driver.system.clipboardImage, system.driver.system.clipboardFiles = value, nil, nil
	return nil, true
}

func driverSetClipboardFiles(paths []string) (error, bool) {
	system := activeDriverSystem.Load()
	if system == nil {
		return nil, false
	}
	system.driver.SetClipboardFiles(paths)
	return nil, true
}

// driverSetPrimarySelection and driverPrimarySelection are the primary selection of a driver-backed
// run, which exists where the real one does, on Linux.
func driverSetPrimarySelection(value string) (error, bool) {
	system := activeDriverSystem.Load()
	if system == nil {
		return nil, false
	}
	if runtime.GOOS != "linux" {
		return ErrUnsupported, true
	}
	system.primary = value
	return nil, true
}

func driverPrimarySelection() (string, error, bool) {
	system := activeDriverSystem.Load()
	if system == nil {
		return "", nil, false
	}
	if runtime.GOOS != "linux" {
		return "", ErrUnsupported, true
	}
	return system.primary, nil, true
}

// driverSecrets is the secret store of every driver-backed run: in memory, shared,
// and gone with the process, so a headless run never touches the user's keychain.
var driverSecrets struct {
	sync.Mutex
	values map[[2]string][]byte
}

func driverStoreSecret(service, account string, secret []byte) (error, bool) {
	if activeDriverSystem.Load() == nil {
		return nil, false
	}
	driverSecrets.Lock()
	defer driverSecrets.Unlock()
	if driverSecrets.values == nil {
		driverSecrets.values = make(map[[2]string][]byte)
	}
	driverSecrets.values[[2]string{service, account}] = append([]byte(nil), secret...)
	return nil, true
}

func driverLoadSecret(service, account string) ([]byte, error, bool) {
	if activeDriverSystem.Load() == nil {
		return nil, nil, false
	}
	driverSecrets.Lock()
	defer driverSecrets.Unlock()
	secret, ok := driverSecrets.values[[2]string{service, account}]
	if !ok {
		return nil, ErrSecretNotFound, true
	}
	return append([]byte(nil), secret...), nil, true
}

func driverDeleteSecret(service, account string) (error, bool) {
	if activeDriverSystem.Load() == nil {
		return nil, false
	}
	driverSecrets.Lock()
	defer driverSecrets.Unlock()
	delete(driverSecrets.values, [2]string{service, account})
	return nil, true
}

func driverOpenURL(target string) (error, bool) {
	system := activeDriverSystem.Load()
	if system == nil {
		return nil, false
	}
	system.driver.openedURL = target
	system.driver.hasOpenedURL = true
	return nil, true
}

// SetSchemeHandlers declares which custom URL schemes have a registered
// application for the rest of this driver's life, replacing the previous
// declaration; the default is none. SchemeHandlerAvailable answers from it, so a
// run does not depend on what the machine under test has installed.
func (driver *Driver) SetSchemeHandlers(schemes ...string) {
	driver.system.schemes = append(driver.system.schemes[:0], schemes...)
}

func driverSchemeHandler(scheme string) (available, handled bool) {
	system := activeDriverSystem.Load()
	if system == nil {
		return false, false
	}
	for _, registered := range system.schemes {
		if registered == scheme {
			return true, true
		}
	}
	return false, true
}

func driverOpenPath(path string) (error, bool) {
	system := activeDriverSystem.Load()
	if system == nil {
		return nil, false
	}
	system.driver.openedPath = path
	system.driver.hasOpenedPath = true
	return nil, true
}

func driverRevealPath(path string) (error, bool) {
	if activeDriverSystem.Load() == nil {
		if nonNativeSystemUsed.Load() {
			return fmt.Errorf("app: reveal needs an active headless driver: %w", ErrUnsupported), true
		}
		return nil, false
	}
	revealedPaths.Lock()
	revealedPaths.path, revealedPaths.ok = path, true
	revealedPaths.Unlock()
	return nil, true
}

// LastRevealedPath returns the most recent system-wide RevealPath request in
// this driver-backed run. The native file manager is a process-wide action, so
// all windows of the run observe the same request. Background calls are safe.
func (driver *Driver) LastRevealedPath() (string, bool) {
	revealedPaths.Lock()
	defer revealedPaths.Unlock()
	return revealedPaths.path, revealedPaths.ok
}

func driverOpenWith(executable, path string) (error, bool) {
	system := activeDriverSystem.Load()
	if system == nil {
		return nil, false
	}
	system.driver.openedWith = [2]string{executable, path}
	system.driver.hasOpenedWith = true
	return nil, true
}

func driverOpenFiles(options OpenFileOptions) ([]string, error, bool) {
	system := activeDriverSystem.Load()
	if system == nil {
		return nil, nil, false
	}
	paths, err := system.driver.openFiles(options)
	return paths, err, true
}

func driverOpenDirectory(options OpenDirectoryOptions) (string, error, bool) {
	system := activeDriverSystem.Load()
	if system == nil {
		return "", nil, false
	}
	path, err := system.driver.openDirectory(options)
	return path, err, true
}

func driverSaveFile(options SaveFileOptions) (string, error, bool) {
	system := activeDriverSystem.Load()
	if system == nil {
		return "", nil, false
	}
	path, err := system.driver.saveFile(options)
	return path, err, true
}

func driverSaveFileWithCheckbox(options SaveFileOptions, checkbox SaveFileCheckbox) (SaveFileSelection, error, bool) {
	system := activeDriverSystem.Load()
	if system == nil {
		return SaveFileSelection{}, nil, false
	}
	result, err := system.driver.saveFileWithCheckbox(options, checkbox)
	return result, err, true
}

func (driver *Driver) registerSystem() error {
	driver.system.driver = driver
	nonNativeSystemUsed.Store(true)
	driverSystems.Lock()
	if len(driverSystems.values) == 0 {
		revealedPaths.Lock()
		revealedPaths.path, revealedPaths.ok = "", false
		revealedPaths.Unlock()
	}
	driverSystems.values = append(driverSystems.values, &driver.system)
	driverSystems.Unlock()
	activeDriverSystem.Store(&driver.system)
	return nil
}

func (driver *Driver) unregisterSystem() {
	driverSystems.Lock()
	for index, system := range driverSystems.values {
		if system == &driver.system {
			copy(driverSystems.values[index:], driverSystems.values[index+1:])
			driverSystems.values[len(driverSystems.values)-1] = nil
			driverSystems.values = driverSystems.values[:len(driverSystems.values)-1]
			break
		}
	}
	if activeDriverSystem.Load() == &driver.system {
		if len(driverSystems.values) == 0 {
			activeDriverSystem.Store(nil)
		} else {
			activeDriverSystem.Store(driverSystems.values[len(driverSystems.values)-1])
		}
	}
	driverSystems.Unlock()
}

func (driver *Driver) activateSystem() {
	activeDriverSystem.Store(&driver.system)
}

// QueueOpenFiles queues the result returned by the next OpenFiles call made
// while driver is active. Paths are copied so the queued response does not
// depend on caller-owned slice storage.
func (driver *Driver) QueueOpenFiles(paths []string, err error) error {
	if err == nil && len(paths) == 0 {
		return fmt.Errorf("app: queued open-file paths are empty")
	}
	for _, path := range paths {
		if path == "" {
			return fmt.Errorf("app: queued open-file path is empty")
		}
	}
	if driver.openFilesCount == len(driver.openFilesResponses) {
		return fmt.Errorf("app: open-files response queue is full")
	}
	index := (driver.openFilesHead + driver.openFilesCount) % len(driver.openFilesResponses)
	driver.openFilesResponses[index] = driverOpenFilesResponse{paths: append([]string(nil), paths...), err: err}
	driver.openFilesCount++
	return nil
}

// LastOpenFilesRequest returns the most recent OpenFiles options observed by
// driver. The returned Filters slice is borrowed from the application call.
func (driver *Driver) LastOpenFilesRequest() (OpenFileOptions, bool) {
	return driver.lastOpenFilesRequest, driver.hasOpenFilesRequest
}

func (driver *Driver) openFiles(options OpenFileOptions) ([]string, error) {
	driver.lastOpenFilesRequest = options
	driver.hasOpenFilesRequest = true
	if driver.openFilesCount == 0 {
		return nil, ErrNoDriverResponse
	}
	response := driver.openFilesResponses[driver.openFilesHead]
	driver.openFilesResponses[driver.openFilesHead] = driverOpenFilesResponse{}
	driver.openFilesHead = (driver.openFilesHead + 1) % len(driver.openFilesResponses)
	driver.openFilesCount--
	return response.paths, response.err
}

// QueueOpenDirectory queues the result returned by the next OpenDirectory
// call made while driver is active.
func (driver *Driver) QueueOpenDirectory(path string, err error) error {
	if err == nil && path == "" {
		return fmt.Errorf("app: queued open-directory path is empty")
	}
	if driver.openDirectoryCount == len(driver.openDirectoryResponses) {
		return fmt.Errorf("app: open-directory response queue is full")
	}
	index := (driver.openDirectoryHead + driver.openDirectoryCount) % len(driver.openDirectoryResponses)
	driver.openDirectoryResponses[index] = driverOpenDirectoryResponse{path: path, err: err}
	driver.openDirectoryCount++
	return nil
}

// LastOpenDirectoryRequest returns the most recent OpenDirectory options
// observed by driver.
func (driver *Driver) LastOpenDirectoryRequest() (OpenDirectoryOptions, bool) {
	return driver.lastOpenDirectoryRequest, driver.hasOpenDirectoryRequest
}

func (driver *Driver) openDirectory(options OpenDirectoryOptions) (string, error) {
	driver.lastOpenDirectoryRequest = options
	driver.hasOpenDirectoryRequest = true
	if driver.openDirectoryCount == 0 {
		return "", ErrNoDriverResponse
	}
	response := driver.openDirectoryResponses[driver.openDirectoryHead]
	driver.openDirectoryResponses[driver.openDirectoryHead] = driverOpenDirectoryResponse{}
	driver.openDirectoryHead = (driver.openDirectoryHead + 1) % len(driver.openDirectoryResponses)
	driver.openDirectoryCount--
	return response.path, response.err
}

// QueueSaveFile queues the result returned by the next SaveFile call made
// while driver is active. It lets headless tools exercise native file-save
// workflows without opening an operating-system dialog.
func (driver *Driver) QueueSaveFile(path string, err error) error {
	if err == nil && path == "" {
		return fmt.Errorf("app: queued save path is empty")
	}
	if driver.saveFileCount == len(driver.saveFileResponses) {
		return fmt.Errorf("app: save-file response queue is full")
	}
	index := (driver.saveFileHead + driver.saveFileCount) % len(driver.saveFileResponses)
	driver.saveFileResponses[index] = driverSaveFileResponse{path: path, err: err}
	driver.saveFileCount++
	return nil
}

// QueueSaveFileWithCheckbox queues the response to the next SaveFileWithCheckbox
// call, including the final checkbox state.
func (driver *Driver) QueueSaveFileWithCheckbox(path string, checked bool, err error) error {
	if err == nil && path == "" {
		return fmt.Errorf("app: queued save path is empty")
	}
	if driver.saveFileCount == len(driver.saveFileResponses) {
		return fmt.Errorf("app: save-file response queue is full")
	}
	index := (driver.saveFileHead + driver.saveFileCount) % len(driver.saveFileResponses)
	driver.saveFileResponses[index] = driverSaveFileResponse{path: path, checked: checked, withCheckbox: true, err: err}
	driver.saveFileCount++
	return nil
}

// LastSaveFileCheckboxRequest reports the most recent SaveFileWithCheckbox
// request; the bool is false when none has been made.
func (driver *Driver) LastSaveFileCheckboxRequest() (SaveFileOptions, SaveFileCheckbox, bool) {
	return driver.lastSaveFileRequest, driver.lastSaveFileCheckbox, driver.hasSaveFileCheckbox
}

func (driver *Driver) saveFileWithCheckbox(options SaveFileOptions, checkbox SaveFileCheckbox) (SaveFileSelection, error) {
	driver.lastSaveFileCheckbox, driver.hasSaveFileCheckbox = checkbox, true
	if driver.saveFileCount == 0 || !driver.saveFileResponses[driver.saveFileHead].withCheckbox {
		driver.lastSaveFileRequest, driver.hasSaveFileRequest = options, true
		return SaveFileSelection{}, ErrNoDriverResponse
	}
	response := driver.takeSaveFileResponse(options)
	if response.err != nil {
		return SaveFileSelection{}, response.err
	}
	return SaveFileSelection{Path: response.path, Checked: response.checked}, nil
}

// LastSaveFileRequest returns the most recent SaveFile options observed by
// driver. The returned Filters slice is borrowed from the application call.
func (driver *Driver) LastSaveFileRequest() (SaveFileOptions, bool) {
	return driver.lastSaveFileRequest, driver.hasSaveFileRequest
}

func (driver *Driver) saveFile(options SaveFileOptions) (string, error) {
	if driver.saveFileCount > 0 && driver.saveFileResponses[driver.saveFileHead].withCheckbox {
		driver.lastSaveFileRequest, driver.hasSaveFileRequest = options, true
		return "", ErrNoDriverResponse
	}
	response := driver.takeSaveFileResponse(options)
	return response.path, response.err
}

func (driver *Driver) takeSaveFileResponse(options SaveFileOptions) driverSaveFileResponse {
	driver.lastSaveFileRequest = options
	driver.hasSaveFileRequest = true
	if driver.saveFileCount == 0 {
		return driverSaveFileResponse{err: ErrNoDriverResponse}
	}
	response := driver.saveFileResponses[driver.saveFileHead]
	driver.saveFileResponses[driver.saveFileHead] = driverSaveFileResponse{}
	driver.saveFileHead = (driver.saveFileHead + 1) % len(driver.saveFileResponses)
	driver.saveFileCount--
	return response
}

// driverClipboardImage answers the clipboard-image reads of a driver-backed
// run from the driver's private clipboard.
func driverClipboardImage() (png []byte, err error, handled bool) {
	system := activeDriverSystem.Load()
	if system == nil {
		return nil, nil, false
	}
	if len(system.clipboardImage) == 0 {
		return nil, ErrNoImage, true
	}
	return system.clipboardImage, nil, true
}

// SetClipboardImage replaces the driver's private clipboard with a PNG image,
// as another application copying a picture would. The text it held is gone, as
// on a real clipboard. The bytes are copied. A nil or empty png empties the
// clipboard of images. The image must be a PNG within MaxClipboardImageBytes.
func (driver *Driver) SetClipboardImage(png []byte) error {
	if len(png) == 0 {
		driver.system.clipboardImage = nil
		return nil
	}
	if err := platform.CheckClipboardPNG(png); err != nil {
		return err
	}
	driver.system.clipboardImage = append([]byte(nil), png...)
	driver.system.clipboardFiles = nil
	driver.clipboard = ""
	return nil
}

// ClipboardImage returns the PNG the driver's private clipboard holds, or nil.
// The slice is the driver's own; do not modify it.
func (driver *Driver) ClipboardImage() []byte { return driver.system.clipboardImage }

// driverClipboardFiles answers the clipboard-file reads of a driver-backed run
// from the driver's private clipboard.
func driverClipboardFiles() (paths []string, err error, handled bool) {
	system := activeDriverSystem.Load()
	if system == nil {
		return nil, nil, false
	}
	if len(system.clipboardFiles) == 0 {
		return nil, ErrNoFiles, true
	}
	return append([]string(nil), system.clipboardFiles...), nil, true
}

// SetClipboardFiles replaces the driver's private clipboard with copied files,
// as a file manager would. Like Finder and Explorer it keeps the files' names
// as the clipboard's text, so a reader that asks for text first sees why files
// are asked for first; the image it held is gone. The paths are copied and cut
// at MaxClipboardFiles. No paths empties the clipboard of files.
func (driver *Driver) SetClipboardFiles(paths []string) {
	if len(paths) == 0 {
		driver.system.clipboardFiles = nil
		return
	}
	paths = paths[:min(len(paths), MaxClipboardFiles)]
	driver.system.clipboardFiles = append([]string(nil), paths...)
	driver.system.clipboardImage = nil
	names := make([]byte, 0, 64)
	for index, path := range paths {
		if index > 0 {
			names = append(names, '\n')
		}
		names = append(names, filepath.Base(path)...)
	}
	driver.clipboard = string(names)
}
