//go:build js && wasm

package app

import (
	"errors"
	"testing"
)

func TestWebFileCallbacksAreValidatedAndMissingDOMIsReported(t *testing.T) {
	if err := OpenFilesAsync(OpenFileOptions{}, nil); err == nil {
		t.Fatal("OpenFilesAsync accepted a nil callback")
	}
	if err := SaveFileAsync(SaveFileOptions{}, nil); err == nil {
		t.Fatal("SaveFileAsync accepted a nil callback")
	}
	var openErr error
	startErr := OpenFilesAsync(OpenFileOptions{}, func(_ []FileRef, err error) { openErr = err })
	if !errors.Is(startErr, ErrUnsupported) {
		t.Fatalf("without a document OpenFilesAsync = %v, want ErrUnsupported", startErr)
	}
	if openErr != nil {
		t.Fatalf("done ran for a request that never started: %v", openErr)
	}
}
