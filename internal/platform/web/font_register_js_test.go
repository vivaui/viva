//go:build js && wasm

package web

import (
	"errors"
	"testing"
)

func TestRegisterFontAsyncRejectsDataThatIsNotASfnt(t *testing.T) {
	called := false
	if err := RegisterFontAsync([]byte("not a font"), func(string, func(), error) { called = true }); err == nil {
		t.Fatal("RegisterFontAsync accepted non-font data")
	}
	if called {
		t.Fatal("done ran for a request that never started")
	}
}

func TestRegisterFontAsyncRequiresACallback(t *testing.T) {
	if err := RegisterFontAsync([]byte("x"), nil); err == nil {
		t.Fatal("RegisterFontAsync accepted a nil callback")
	}
}

func TestNormalizedWeightUsesCssHundreds(t *testing.T) {
	for weight, want := range map[int]int{1: 400, 350: 400, 400: 400, 550: 600, 700: 700, 1000: 400} {
		if got := normalizedWeight(weight); got != want {
			t.Errorf("normalizedWeight(%d) = %d, want %d", weight, got, want)
		}
	}
}

func TestSecretStoreWithoutIndexedDBReportsUnavailable(t *testing.T) {
	var got error
	if err := StoreSecretAsync("service", "account", []byte("secret"), func(err error) { got = err }); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(got, ErrUnavailable) {
		t.Fatalf("StoreSecretAsync without indexedDB = %v, want ErrUnavailable", got)
	}
}
