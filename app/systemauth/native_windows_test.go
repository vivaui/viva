package systemauth

import (
	"errors"
	"testing"
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
