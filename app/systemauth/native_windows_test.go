package systemauth

import (
	"context"
	"crypto/sha1"
	"errors"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestHelloConsentRejectsEveryNonVerifiedResult(t *testing.T) {
	for _, tc := range []struct {
		result int32
		want   error
	}{
		{0, nil}, {1, ErrUnavailable}, {2, ErrUnavailable}, {3, ErrUnavailable},
		{4, ErrFailed}, {5, ErrFailed}, {6, ErrCanceled}, {-1, ErrFailed}, {15, ErrFailed},
	} {
		if err := helloConsentError(tc.result); !errors.Is(err, tc.want) {
			t.Errorf("native result %d returned %v, want %v", tc.result, err, tc.want)
		}
	}
	if err := helloHRESULT("test", uintptr(0x800704c7)); !errors.Is(err, ErrCanceled) {
		t.Fatal(err)
	}
	if err := helloHRESULT("test", uintptr(0x80004005)); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
}

func TestHelloCapabilityWithoutShowingPrompt(t *testing.T) {
	method := Method()
	t.Logf("Windows Hello method: %v", method)
	if method != VerificationUnavailable && method != VerificationWindowsHello {
		t.Fatal("unexpected Windows method")
	}
}

// parameterizedIID derives the IID of a WinRT parameterized interface: a
// name-based (version 5) UUID of its signature under the WinRT namespace.
func parameterizedIID(signature string) windows.GUID {
	namespace := [16]byte{0x11, 0xf4, 0x7a, 0xd5, 0x7b, 0x73, 0x42, 0xc0, 0xab, 0xae, 0x87, 0x8b, 0x1e, 0x16, 0xad, 0xee}
	hash := sha1.New()
	hash.Write(namespace[:])
	hash.Write([]byte(signature))
	sum := hash.Sum(nil)[:16]
	sum[6] = sum[6]&0x0f | 0x50
	sum[8] = sum[8]&0x3f | 0x80
	return windows.GUID{
		Data1: uint32(sum[0])<<24 | uint32(sum[1])<<16 | uint32(sum[2])<<8 | uint32(sum[3]),
		Data2: uint16(sum[4])<<8 | uint16(sum[5]),
		Data3: uint16(sum[6])<<8 | uint16(sum[7]),
		Data4: [8]byte(sum[8:16]),
	}
}

// A handler with a wrong IID is refused by the system at registration or never
// called, so the three IIDs are checked against their signatures.
func TestCompletionIIDsMatchTheirSignatures(t *testing.T) {
	const (
		operation = "{9fc2b0bb-e446-44e2-aa61-9cab8f636af2}"
		handler   = "{fcdcf02c-e5d8-4478-915a-4d90b74b83a5}"
		result    = "enum(Windows.Security.Credentials.UI.UserConsentVerificationResult;i4)"
		available = "enum(Windows.Security.Credentials.UI.UserConsentVerifierAvailability;i4)"
	)
	for name, tc := range map[string]struct {
		got, want windows.GUID
	}{
		"IAsyncOperation<result>":          {consentAsyncResultIID, parameterizedIID("pinterface(" + operation + ";" + result + ")")},
		"completion handler<result>":       {consentResultHandlerIID, parameterizedIID("pinterface(" + handler + ";" + result + ")")},
		"completion handler<availability>": {consentAvailabilityHandlerIID, parameterizedIID("pinterface(" + handler + ";" + available + ")")},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %v, want %v", name, tc.got, tc.want)
		}
	}
}

// startAvailability starts the availability query, which finishes without a
// prompt on any machine, whether or not Windows Hello is set up.
func startAvailability(t *testing.T) (*consentObject, func()) {
	t.Helper()
	factory, err := consentFactory(&consentStaticsIID)
	if err != nil {
		t.Skipf("Windows Hello is not activatable here: %v", err)
	}
	var operation *consentObject
	status, _, _ := syscall.SyscallN(factory.methods[6], uintptr(unsafe.Pointer(factory)), uintptr(unsafe.Pointer(&operation))) // CheckAvailabilityAsync
	if !consentSucceeded(status) || operation == nil {
		factory.release()
		t.Skipf("availability query failed (HRESULT 0x%08X)", uint32(status))
	}
	return operation, func() { operation.release(); factory.release() }
}

func TestWaitIsEndedByTheCompletionHandler(t *testing.T) {
	err := withConsentApartment(func() error {
		operation, done := startAvailability(t)
		defer done()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		var availability int32 = -1
		started := time.Now()
		if err := waitHelloConsent(ctx, operation, &consentAvailabilityHandlerIID, unsafe.Pointer(&availability)); err != nil {
			return err
		}
		if availability < 0 {
			t.Errorf("the wait returned before the result was read (availability %d)", availability)
		}
		t.Logf("availability %d after %v", availability, time.Since(started))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// Whether the context ends the wait before or after the operation does, it ends
// promptly and never reports a success it did not read.
func TestWaitReturnsPromptlyWhenTheContextIsAlreadyDone(t *testing.T) {
	err := withConsentApartment(func() error {
		operation, done := startAvailability(t)
		defer done()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		var availability int32 = -1
		started := time.Now()
		err := waitHelloConsent(ctx, operation, &consentAvailabilityHandlerIID, unsafe.Pointer(&availability))
		if elapsed := time.Since(started); elapsed > consentCancelWait+time.Second {
			t.Errorf("the wait took %v after the context was canceled", elapsed)
		}
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Errorf("wait returned %v", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestVerifyWithoutAnApplicationWindowIsRefused(t *testing.T) {
	if err := nativeVerify(context.Background(), "Unlock"); !errors.Is(err, ErrForegroundRequired) {
		t.Fatalf("verification with no foreground window of this process: %v", err)
	}
}
