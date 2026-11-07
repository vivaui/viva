//go:build darwin || windows || linux

package app

import (
	"errors"
	"testing"
)

func TestDriverSaveFileWithCheckboxReturnsSelection(t *testing.T) {
	driver, err := NewDriver(Config{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	options := SaveFileOptions{Title: "Save Session", SuggestedName: "session.xml", DefaultExtension: "xml"}
	checkbox := SaveFileCheckbox{Label: "Save Folder as Workspace", Checked: true}
	if err := driver.QueueSaveFileWithCheckbox("/tmp/session.xml", false, nil); err != nil {
		t.Fatal(err)
	}
	selection, err := SaveFileWithCheckbox(options, checkbox)
	if err != nil || selection.Path != "/tmp/session.xml" || selection.Checked {
		t.Fatalf("selected save file = %+v, %v", selection, err)
	}
	request, checked, ok := driver.LastSaveFileCheckboxRequest()
	if !ok || request.Title != options.Title || checked != checkbox {
		t.Fatalf("checkbox request = %+v, %+v, %v", request, checked, ok)
	}
	if err := driver.QueueSaveFileWithCheckbox("", false, ErrCanceled); err != nil {
		t.Fatal(err)
	}
	if got, err := SaveFileWithCheckbox(options, checkbox); !errors.Is(err, ErrCanceled) || got != (SaveFileSelection{}) {
		t.Fatalf("canceled save file = %+v, %v", got, err)
	}
	if _, err := SaveFileWithCheckbox(options, checkbox); !errors.Is(err, ErrNoDriverResponse) {
		t.Fatalf("unanswered save file = %v", err)
	}
}

func TestSaveFileWithCheckboxValidatesLabelBeforeDialog(t *testing.T) {
	driver, err := NewDriver(Config{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	for _, label := range []string{"", " \t", "bad\nlabel", string([]byte{0xff})} {
		if _, err := SaveFileWithCheckbox(SaveFileOptions{}, SaveFileCheckbox{Label: label}); err == nil {
			t.Fatalf("accepted checkbox label %q", label)
		}
	}
	if _, ok := driver.LastSaveFileRequest(); ok {
		t.Fatal("invalid checkbox reached the file picker")
	}
}

func TestDriverSaveFileWithCheckboxDoesNotConsumeOrdinarySaveResponse(t *testing.T) {
	driver, err := NewDriver(Config{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	options := SaveFileOptions{Title: "Save Session"}
	if err := driver.QueueSaveFile("/tmp/ordinary.xml", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveFileWithCheckbox(options, SaveFileCheckbox{Label: "Save Folder as Workspace"}); !errors.Is(err, ErrNoDriverResponse) {
		t.Fatalf("checkbox request with ordinary response = %v", err)
	}
	path, err := SaveFile(options)
	if err != nil || path != "/tmp/ordinary.xml" {
		t.Fatalf("ordinary save response = %q, %v", path, err)
	}
}
