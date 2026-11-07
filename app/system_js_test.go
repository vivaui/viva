//go:build js && wasm

package app

import (
	"errors"
	"testing"
)

func TestBrowserPromiseOnlySystemAPIsExposeAsyncConstraint(t *testing.T) {
	if _, err := ClipboardText(); !errors.Is(err, ErrAsyncRequired) {
		t.Fatalf("ClipboardText error = %v", err)
	}
	if err := SetClipboardText("viva"); !errors.Is(err, ErrAsyncRequired) {
		t.Fatalf("SetClipboardText error = %v", err)
	}
	if _, err := OpenFiles(OpenFileOptions{}); !errors.Is(err, ErrAsyncRequired) {
		t.Fatalf("OpenFiles error = %v", err)
	}
	if _, err := SaveFile(SaveFileOptions{}); !errors.Is(err, ErrAsyncRequired) {
		t.Fatalf("SaveFile error = %v", err)
	}
}

func TestBrowserSystemSoundIsUnsupported(t *testing.T) {
	if err := PlaySystemSound(SoundDefault); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("PlaySystemSound error = %v", err)
	}
}

func TestBrowserSaveFileWithCheckboxIsUnsupported(t *testing.T) {
	if _, err := SaveFileWithCheckbox(SaveFileOptions{}, SaveFileCheckbox{Label: "Include"}); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("SaveFileWithCheckbox error = %v", err)
	}
}
