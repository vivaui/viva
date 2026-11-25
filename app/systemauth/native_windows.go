package systemauth

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// IUserConsentVerifierStatics, IUserConsentVerifierInterop and IAsyncInfo from
// the Windows SDK. The verifier presents the system Hello prompt; a
// verifier does not release key material.
var (
	consentStaticsIID = windows.GUID{
		Data1: 0xaf4f3f91, Data2: 0x564c, Data3: 0x4ddc,
		Data4: [8]byte{0xb8, 0xb5, 0x97, 0x34, 0x47, 0x62, 0x7c, 0x65},
	}
	consentAsyncInfoIID = windows.GUID{
		Data1: 0x00000036,
		Data4: [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46},
	}
	consentVerifierInteropIID = windows.GUID{
		Data1: 0x39e050c3, Data2: 0x4e74, Data3: 0x441a,
		Data4: [8]byte{0x8d, 0xc0, 0xb8, 0x11, 0x04, 0xdf, 0x94, 0x9c},
	}
	consentAsyncResultIID = windows.GUID{
		Data1: 0xfd596ffd, Data2: 0x2318, Data3: 0x558f,
		Data4: [8]byte{0x9d, 0xbe, 0xd2, 0x1d, 0xf4, 0x37, 0x64, 0xa5},
	}

	consentCOMBase      = windows.NewLazySystemDLL("combase.dll")
	consentInitialize   = consentCOMBase.NewProc("RoInitialize")
	consentUninitialize = consentCOMBase.NewProc("RoUninitialize")
	consentCreateString = consentCOMBase.NewProc("WindowsCreateString")
	consentDeleteString = consentCOMBase.NewProc("WindowsDeleteString")
	consentGetFactory   = consentCOMBase.NewProc("RoGetActivationFactory")
	consentUser32       = windows.NewLazySystemDLL("user32.dll")
	consentForeground   = consentUser32.NewProc("GetForegroundWindow")
	consentWindowPID    = consentUser32.NewProc("GetWindowThreadProcessId")
)

const (
	consentClass     = "Windows.Security.Credentials.UI.UserConsentVerifier"
	consentPoll      = 10 * time.Millisecond
	consentMTA       = 1
	consentPending   = 0
	consentCompleted = 1

	consentVerified         = 0
	consentDeviceNotPresent = 1
	consentNotConfigured    = 2
	consentDisabledByPolicy = 3
	consentDeviceBusy       = 4
	consentRetriesExhausted = 5
	consentCanceled         = 6

	consentChangedMode = 0x80010106 // RPC_E_CHANGED_MODE
)

type consentObject struct {
	methods *[10]uintptr
}

func (object *consentObject) release() {
	syscall.SyscallN(object.methods[2], uintptr(unsafe.Pointer(object))) // IUnknown.Release
}

func consentSucceeded(status uintptr) bool { return int32(status) >= 0 }

var errNoWindowInterop = errors.New("window-owned Hello API is unavailable")

func foregroundWindow() (uintptr, error) {
	owner, _, _ := consentForeground.Call()
	var pid uint32
	if owner != 0 {
		consentWindowPID.Call(owner, uintptr(unsafe.Pointer(&pid)))
	}
	if pid != windows.GetCurrentProcessId() {
		return 0, ErrForegroundRequired
	}
	return owner, nil
}

func confirmForegroundOwner(owner uintptr) error {
	current, err := foregroundWindow()
	if err != nil || current != owner {
		return ErrForegroundRequired
	}
	return nil
}

func helloHRESULT(operation string, status uintptr) error {
	if consentSucceeded(status) {
		return nil
	}
	if uint32(status) == 0x800704c7 || uint32(status) == 0x80090036 {
		return ErrCanceled
	}
	return fmt.Errorf("%w: Windows Hello %s (HRESULT 0x%08X)", ErrUnavailable, operation, uint32(status))
}

func nativeMethod() VerificationMethod {
	ctx, cancel := context.WithTimeout(context.Background(), 750*time.Millisecond)
	defer cancel()
	result := make(chan VerificationMethod, 1)
	go func() { result <- configuredMethod(ctx) }()
	select {
	case method := <-result:
		return method
	case <-ctx.Done():
		return VerificationUnavailable
	}
}

func configuredMethod(ctx context.Context) VerificationMethod {
	ready := VerificationUnavailable
	_ = withConsentApartment(func() error {
		factory, err := consentFactory(&consentStaticsIID)
		if err != nil {
			return err
		}
		defer factory.release()
		var operation *consentObject
		status, _, _ := syscall.SyscallN(factory.methods[6], uintptr(unsafe.Pointer(factory)), uintptr(unsafe.Pointer(&operation))) // CheckAvailabilityAsync
		if !consentSucceeded(status) || operation == nil {
			return ErrUnavailable
		}
		defer operation.release()
		var availability int32 = -1
		if err := waitHelloConsent(ctx, operation, unsafe.Pointer(&availability)); err != nil {
			return err
		}
		if availability == 0 && ctx.Err() == nil {
			ready = VerificationWindowsHello
		}
		return nil
	})
	return ready
}
func nativeVerify(ctx context.Context, reason string) error {
	owner, err := foregroundWindow()
	if err != nil {
		return err
	}
	return withConsentApartment(func() error {
		message, err := consentHString(reason)
		if err != nil {
			return err
		}
		defer consentDeleteString.Call(message)
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := confirmForegroundOwner(owner); err != nil {
			return err
		}
		operation, err := startHelloConsent(owner, message)
		if err != nil {
			return err
		}
		defer operation.release()
		var result int32 = -1
		if err := waitHelloConsent(ctx, operation, unsafe.Pointer(&result)); err != nil {
			return err
		}
		return helloConsentError(result)
	})
}

func withConsentApartment(run func() error) error {
	for _, proc := range []*windows.LazyProc{consentInitialize, consentUninitialize, consentCreateString, consentDeleteString, consentGetFactory} {
		if proc.Find() != nil {
			return ErrUnavailable
		}
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	status, _, _ := consentInitialize.Call(consentMTA)
	switch {
	case consentSucceeded(status):
		defer consentUninitialize.Call()
	case uint32(status) == consentChangedMode:
		// Another part of this thread already joined an apartment. The
		// verifier can still be activated, as GoIM does on the same path.
	default:
		return fmt.Errorf("%w: Windows Hello initialization failed (HRESULT 0x%08X)", ErrUnavailable, uint32(status))
	}
	return run()
}

func consentHString(value string) (uintptr, error) {
	units, err := windows.UTF16FromString(value)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	var name uintptr
	status, _, _ := consentCreateString.Call(uintptr(unsafe.Pointer(&units[0])), uintptr(len(units)-1), uintptr(unsafe.Pointer(&name)))
	if !consentSucceeded(status) || name == 0 {
		return 0, fmt.Errorf("%w: Windows Hello message failed (HRESULT 0x%08X)", ErrUnavailable, uint32(status))
	}
	return name, nil
}

func consentFactory(iid *windows.GUID) (*consentObject, error) {
	class, err := consentHString(consentClass)
	if err != nil {
		return nil, err
	}
	defer consentDeleteString.Call(class)
	var factory *consentObject
	status, _, _ := consentGetFactory.Call(class, uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&factory)))
	runtime.KeepAlive(iid)
	if uint32(status) == 0x80004002 && iid == &consentVerifierInteropIID { // E_NOINTERFACE
		return nil, errNoWindowInterop
	}
	if !consentSucceeded(status) || factory == nil {
		return nil, fmt.Errorf("%w: Windows Hello activation failed (HRESULT 0x%08X)", ErrUnavailable, uint32(status))
	}
	return factory, nil
}

func startHelloConsent(owner uintptr, message uintptr) (*consentObject, error) {
	// Fall back to the older API only if the HWND interop is absent.
	factory, err := consentFactory(&consentVerifierInteropIID)
	if err == nil {
		defer factory.release()
		var operation *consentObject
		status, _, _ := syscall.SyscallN(factory.methods[6], uintptr(unsafe.Pointer(factory)), owner, message,
			uintptr(unsafe.Pointer(&consentAsyncResultIID)), uintptr(unsafe.Pointer(&operation)))
		if ctxErr := helloHRESULT("request verification for window", status); ctxErr != nil {
			return nil, ctxErr
		}
		if operation == nil {
			return nil, ErrUnavailable
		}
		return operation, nil
	}
	if !errors.Is(err, errNoWindowInterop) {
		return nil, err
	}
	factory, err = consentFactory(&consentStaticsIID)
	if err != nil {
		return nil, err
	}
	defer factory.release()
	var operation *consentObject
	// The static WinRT method has no REFIID argument. Its result type
	// is fixed in the SDK; only the desktop interop entry takes a REFIID.
	status, _, _ := syscall.SyscallN(factory.methods[7], uintptr(unsafe.Pointer(factory)), message,
		uintptr(unsafe.Pointer(&operation)))
	if err := helloHRESULT("request verification", status); err != nil {
		return nil, err
	}
	if operation == nil {
		return nil, ErrUnavailable
	}
	return operation, nil
}

func waitHelloConsent(ctx context.Context, operation *consentObject, result unsafe.Pointer) error {
	var info *consentObject
	status, _, _ := syscall.SyscallN(operation.methods[0], uintptr(unsafe.Pointer(operation)),
		uintptr(unsafe.Pointer(&consentAsyncInfoIID)), uintptr(unsafe.Pointer(&info)))
	runtime.KeepAlive(&consentAsyncInfoIID)
	if !consentSucceeded(status) || info == nil {
		return fmt.Errorf("%w: Windows Hello result is unavailable (HRESULT 0x%08X)", ErrUnavailable, uint32(status))
	}
	defer info.release()
	poll := time.NewTicker(consentPoll)
	defer poll.Stop()
	for {
		var state int32
		status, _, _ = syscall.SyscallN(info.methods[7], uintptr(unsafe.Pointer(info)), uintptr(unsafe.Pointer(&state)))
		if !consentSucceeded(status) {
			return fmt.Errorf("%w: Windows Hello status failed (HRESULT 0x%08X)", ErrUnavailable, uint32(status))
		}
		switch state {
		case consentPending:
			select {
			case <-poll.C:
			case <-ctx.Done():
				syscall.SyscallN(info.methods[9], uintptr(unsafe.Pointer(info))) // IAsyncInfo.Cancel
				return ctx.Err()
			}
		case consentCompleted:
			status, _, _ = syscall.SyscallN(operation.methods[8], uintptr(unsafe.Pointer(operation)), uintptr(result))
			if !consentSucceeded(status) {
				return helloHRESULT("verification result", status)
			}
			return nil
		case 2:
			return ErrCanceled
		default:
			return fmt.Errorf("%w: Windows Hello verification ended in state %d", ErrUnavailable, state)
		}
	}
}

func helloConsentError(result int32) error {
	switch result {
	case consentVerified:
		return nil
	case consentCanceled:
		return ErrCanceled
	case consentDeviceNotPresent, consentNotConfigured:
		return fmt.Errorf("%w: Windows Hello is not configured for this account", ErrUnavailable)
	case consentDisabledByPolicy:
		return fmt.Errorf("%w: Windows Hello is disabled by policy", ErrUnavailable)
	case consentDeviceBusy:
		return fmt.Errorf("%w: Windows Hello is busy", ErrFailed)
	case consentRetriesExhausted:
		return fmt.Errorf("%w: Windows Hello retries were exhausted", ErrFailed)
	default:
		return fmt.Errorf("%w: unrecognized Windows Hello result %d", ErrFailed, result)
	}
}
