//go:build windows

package update

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// Windows installs the NSIS setup program the feed lists. The download's size
// and SHA-256 already match the feed, and the feed's Ed25519 signature is the
// trust anchor; Authenticode adds the publisher check when the installer is
// signed, and a signature that exists but does not verify is refused.

const (
	// detachedProcess starts the helper without a console of its own.
	detachedProcess = 0x00000008
	// powerShellExecutable is the Windows PowerShell host that runs the helper.
	powerShellExecutable = "powershell.exe"
)

// authenticodeState is what WinVerifyTrust found for a file.
type authenticodeState int

const (
	// authenticodeUnknown means the check did not complete.
	authenticodeUnknown authenticodeState = iota
	// authenticodeTrusted means the file carries a signature Windows trusts.
	authenticodeTrusted
	// authenticodeUnsigned means the file carries no Authenticode signature.
	// Install accepts it because the feed's signature already authenticated it.
	authenticodeUnsigned
)

// winTrustFileInfo is WINTRUST_FILE_INFO from wintrust.h. Go inserts the same
// padding the C compiler does when fields are declared in this order.
type winTrustFileInfo struct {
	cbStruct     uint32
	filePath     *uint16
	fileHandle   syscall.Handle
	knownSubject *syscall.GUID
}

// winTrustData is WINTRUST_DATA from wintrust.h. pFile is the union member
// WTD_CHOICE_FILE selects; the other union members are never used here.
type winTrustData struct {
	cbStruct           uint32
	policyCallbackData uintptr
	sipClientData      uintptr
	uiChoice           uint32
	revocationChecks   uint32
	unionChoice        uint32
	pFile              *winTrustFileInfo
	stateAction        uint32
	stateData          syscall.Handle
	urlReference       *uint16
	providerFlags      uint32
	uiContext          uint32
	signatureSettings  uintptr
}

const (
	// winTrustUIChoiceNone asks WinVerifyTrust not to show a user interface.
	winTrustUIChoiceNone = 2
	// winTrustRevocationCheckNone skips revocation checks, which need a network
	// and would make an offline install fail.
	winTrustRevocationCheckNone = 0
	// winTrustUnionChoiceFile verifies the signature embedded in a file.
	winTrustUnionChoiceFile = 1
	// winTrustStateActionVerify asks WinVerifyTrust to verify the file.
	winTrustStateActionVerify = 1
	// winTrustStateActionClose releases the state a verify call allocated.
	winTrustStateActionClose = 2

	// trustENoSignature reports that a file has no signature at all.
	trustENoSignature = 0x800B0100
	// trustESubjectFormUnknown reports a file whose form has no signature.
	trustESubjectFormUnknown = 0x800B0003
	// trustEProviderUnknown reports that no trust provider handled the file.
	trustEProviderUnknown = 0x800B0001
)

// winTrustActionGenericVerifyV2 is WINTRUST_ACTION_GENERIC_VERIFY_V2, the
// action that checks an Authenticode signature.
var winTrustActionGenericVerifyV2 = syscall.GUID{
	Data1: 0x00AAC56B,
	Data2: 0xCD44,
	Data3: 0x11D0,
	Data4: [8]byte{0x8C, 0xC2, 0x00, 0xC0, 0x4F, 0xC2, 0x95, 0xEE},
}

var (
	winTrustDLL        = syscall.NewLazyDLL("wintrust.dll")
	winVerifyTrustProc = winTrustDLL.NewProc("WinVerifyTrust")
)

// install runs the NSIS installer after this process exits and then starts the
// installed program. It returns only after quit, so the helper can wait for
// this process to end before it replaces the running executable.
func install(options InstallOptions, quit func()) error {
	if options.Payload != "" {
		return fmt.Errorf("%w: Windows has no installed payload to patch; download the installer", ErrUnsupported)
	}
	if options.Asset.Format != FormatNSIS {
		return fmt.Errorf("%w: windows needs an NSIS installer, not %q", ErrManifest, options.Asset.Format)
	}
	info, err := os.Stat(options.Artifact)
	if err != nil {
		return fmt.Errorf("update: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%w: %s is not a regular file", ErrManifest, options.Artifact)
	}
	if !strings.EqualFold(filepath.Ext(options.Artifact), ".exe") {
		return fmt.Errorf("%w: %s is not an .exe installer", ErrManifest, options.Artifact)
	}
	installer, err := filepath.Abs(options.Artifact)
	if err != nil {
		return fmt.Errorf("update: resolve %s: %w", options.Artifact, err)
	}
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("update: locate the running program: %w", err)
	}
	if executable, err = filepath.Abs(executable); err != nil {
		return fmt.Errorf("update: resolve %s: %w", executable, err)
	}

	options.report("verifying installer")
	if _, err := verifyAuthenticode(installer); err != nil {
		return fmt.Errorf("%w: %v", ErrUntrusted, err)
	}
	options.report("checking install directory")
	if err := checkWritable(filepath.Dir(executable)); err != nil {
		return err
	}
	options.report("starting installer")
	if err := startHelper(os.Getpid(), installer, executable, options.Args); err != nil {
		return fmt.Errorf("update: start the installer helper: %w", err)
	}
	quit()
	return nil
}

// verifyAuthenticode reports whether path is signed and trusted, unsigned, or
// could not be verified. Only the last case is an error; an unsigned installer
// is allowed because the feed's Ed25519 signature authenticated its bytes.
func verifyAuthenticode(path string) (authenticodeState, error) {
	filePath, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return authenticodeUnknown, err
	}
	file := winTrustFileInfo{
		cbStruct: uint32(unsafe.Sizeof(winTrustFileInfo{})),
		filePath: filePath,
	}
	data := winTrustData{
		cbStruct:         uint32(unsafe.Sizeof(winTrustData{})),
		uiChoice:         winTrustUIChoiceNone,
		revocationChecks: winTrustRevocationCheckNone,
		unionChoice:      winTrustUnionChoiceFile,
		pFile:            &file,
		stateAction:      winTrustStateActionVerify,
	}
	if err := winVerifyTrustProc.Find(); err != nil {
		return authenticodeUnknown, fmt.Errorf("load wintrust.dll: %w", err)
	}
	action := winTrustActionGenericVerifyV2
	result, _, _ := winVerifyTrustProc.Call(
		0,
		uintptr(unsafe.Pointer(&action)),
		uintptr(unsafe.Pointer(&data)),
	)
	// Release the state data the verify call may have allocated.
	data.stateAction = winTrustStateActionClose
	_, _, _ = winVerifyTrustProc.Call(
		0,
		uintptr(unsafe.Pointer(&action)),
		uintptr(unsafe.Pointer(&data)),
	)
	switch uint32(result) {
	case 0:
		return authenticodeTrusted, nil
	case trustENoSignature, trustESubjectFormUnknown, trustEProviderUnknown:
		return authenticodeUnsigned, nil
	default:
		return authenticodeUnknown, fmt.Errorf("WinVerifyTrust returned 0x%08X", uint32(result))
	}
}

// checkWritable proves directory can be written by creating a file in it and
// removing it again. The installer runs after this process has quit, so a
// directory it cannot write must be reported before anything changes.
func checkWritable(directory string) error {
	name := filepath.Join(directory, fmt.Sprintf(".viva-update-%d-%d.tmp", os.Getpid(), time.Now().UnixNano()))
	file, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("%w: %s is not writable: %v", ErrNotWritable, directory, err)
	}
	closeErr := file.Close()
	removeErr := os.Remove(name)
	if closeErr != nil {
		return fmt.Errorf("%w: %s is not writable: %v", ErrNotWritable, directory, closeErr)
	}
	if removeErr != nil {
		return fmt.Errorf("%w: %s is not writable: %v", ErrNotWritable, directory, removeErr)
	}
	return nil
}

// startHelper launches a detached PowerShell script that waits for this
// process to exit, runs the installer silently and starts the new program.
func startHelper(parentPID int, installer, executable string, args []string) error {
	name, arguments := helperCommand(helperScript(parentPID, installer, executable, args))
	command := exec.Command(name, arguments...)
	command.SysProcAttr = helperSysProcAttr()
	if err := command.Start(); err != nil {
		return err
	}
	// The helper is running now. Releasing its handle only frees a handle in
	// this process, so a failure here must not turn a successful launch into a
	// reported failure that leaves the application running.
	_ = command.Process.Release()
	return nil
}

// helperSysProcAttr gives the helper its own process group and no console, so
// it survives the application that is about to quit.
func helperSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		CreationFlags: detachedProcess | syscall.CREATE_NEW_PROCESS_GROUP,
		HideWindow:    true,
	}
}

// helperCommand is the PowerShell invocation that carries script.
func helperCommand(script string) (string, []string) {
	return powerShellExecutable, []string{
		"-NoProfile",
		"-NonInteractive",
		"-WindowStyle", "Hidden",
		"-Command", script,
	}
}

// helperScript builds the script the helper runs. Every path and argument is a
// PowerShell single-quoted literal, so a quote inside one is doubled.
func helperScript(parentPID int, installer, executable string, args []string) string {
	var script strings.Builder
	fmt.Fprintf(&script, "Wait-Process -Id %d -ErrorAction SilentlyContinue; ", parentPID)
	fmt.Fprintf(&script, "Start-Process -FilePath %s -ArgumentList '/S' -Wait; ", powerShellQuote(installer))
	fmt.Fprintf(&script, "Start-Process -FilePath %s", powerShellQuote(executable))
	if len(args) > 0 {
		fmt.Fprintf(&script, " -ArgumentList %s", powerShellArgumentList(args))
	}
	return script.String()
}

// powerShellQuote returns s as a PowerShell single-quoted string. PowerShell
// escapes a single quote by writing it twice; every other character is literal.
func powerShellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// powerShellArgumentList returns the Start-Process -ArgumentList value for
// args. The list happens to be joined with spaces by Start-Process, so each
// argument also gets the double quotes CommandLineToArgvW needs to keep it one
// argument; those quotes survive inside the PowerShell single quotes.
func powerShellArgumentList(args []string) string {
	quoted := make([]string, len(args))
	for index, argument := range args {
		quoted[index] = powerShellQuote(commandLineArgument(argument))
	}
	return "@(" + strings.Join(quoted, ",") + ")"
}

// commandLineArgument quotes one argument the way Windows parses a command
// line. A backslash is literal unless it precedes a double quote, a double
// quote is escaped with a backslash, and backslashes before a closing quote are
// doubled.
func commandLineArgument(argument string) string {
	if argument == "" {
		return `""`
	}
	if !strings.ContainsAny(argument, " \t\n\v\"") {
		return argument
	}
	var quoted strings.Builder
	quoted.WriteByte('"')
	backslashes := 0
	for _, character := range argument {
		switch character {
		case '\\':
			backslashes++
		case '"':
			quoted.WriteString(strings.Repeat(`\`, backslashes*2+1))
			quoted.WriteByte('"')
			backslashes = 0
		default:
			quoted.WriteString(strings.Repeat(`\`, backslashes))
			quoted.WriteRune(character)
			backslashes = 0
		}
	}
	quoted.WriteString(strings.Repeat(`\`, backslashes*2))
	quoted.WriteByte('"')
	return quoted.String()
}
