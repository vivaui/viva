//go:build windows

package windows

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

// HRESULT_FROM_WIN32(ERROR_FILE_NOT_FOUND): the protocol has no key at all, as
// hresultNoAssociation means it has no command.
const hresultFileNotFound = 0x80070002

// OpenInBrowser opens the file at path in the default browser by starting the
// command Windows registers for http links with the file's URL, without a
// shell, as the shell itself fills the command in; the browser is reaped in the
// background once it exits. Without a default browser it returns
// platform.ErrNoBrowser.
func OpenInBrowser(path string) error {
	command, err := browserCommand()
	if err != nil {
		return err
	}
	program, commandLine, err := platform.BrowserCommandLine(command, platform.FileURL(path, true), regularFile)
	if err != nil {
		return fmt.Errorf("windows: open in browser: %w", err)
	}
	browser := exec.Command(program)
	browser.SysProcAttr = &syscall.SysProcAttr{CmdLine: commandLine}
	if err := platform.StartReaped(browser); err != nil {
		return fmt.Errorf("windows: launch %s: %w", filepath.Base(program), err)
	}
	return nil
}

// browserCommand is the command line Windows starts http links with, the
// default browser's.
func browserCommand() (string, error) {
	protocol, _ := syscall.UTF16PtrFromString("http")
	var length uint32
	status, _, _ := assocQueryString.Call(assocfIsProtocol, assocStrCommand, uintptr(unsafe.Pointer(protocol)), 0, 0, uintptr(unsafe.Pointer(&length)))
	if code := uint32(status); code == hresultNoAssociation || code == hresultFileNotFound {
		return "", fmt.Errorf("windows: open in browser: %w", platform.ErrNoBrowser)
	}
	if err := hresult(status); err != nil {
		return "", fmt.Errorf("windows: look up the default browser: %w", err)
	}
	if length == 0 {
		return "", fmt.Errorf("windows: open in browser: %w", platform.ErrNoBrowser)
	}
	buffer := make([]uint16, length)
	status, _, _ = assocQueryString.Call(assocfIsProtocol, assocStrCommand, uintptr(unsafe.Pointer(protocol)), 0, uintptr(unsafe.Pointer(&buffer[0])), uintptr(unsafe.Pointer(&length)))
	if err := hresult(status); err != nil {
		return "", fmt.Errorf("windows: look up the default browser: %w", err)
	}
	command := strings.TrimSpace(syscall.UTF16ToString(buffer))
	if command == "" {
		return "", fmt.Errorf("windows: open in browser: %w", platform.ErrNoBrowser)
	}
	return command, nil
}

func regularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
