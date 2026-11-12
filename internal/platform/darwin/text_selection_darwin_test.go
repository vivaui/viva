package darwin

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/internal/platform"
)

func TestNativeTextSelection(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "text-selection")
	// The native command numbers are platform.TextCommand's, read by the Go side
	// as they come; the test program checks its own against these.
	arguments := []string{"clang", "-fno-objc-arc", "-Werror", "-Wno-deprecated-declarations"}
	for name, command := range map[string]platform.TextCommand{
		"MOVE_LINE_START": platform.TextMoveLineStart, "MOVE_LINE_END": platform.TextMoveLineEnd,
		"MOVE_LINE_START_AND_MODIFY_SELECTION": platform.TextMoveLineStartAndModifySelection,
		"MOVE_LINE_END_AND_MODIFY_SELECTION":   platform.TextMoveLineEndAndModifySelection,
		"MOVE_DOCUMENT_START":                  platform.TextMoveDocumentStart, "MOVE_DOCUMENT_END": platform.TextMoveDocumentEnd,
		"MOVE_DOCUMENT_START_AND_MODIFY_SELECTION": platform.TextMoveDocumentStartAndModifySelection,
		"MOVE_DOCUMENT_END_AND_MODIFY_SELECTION":   platform.TextMoveDocumentEndAndModifySelection,
		"MOVE_WORD_LEFT":                           platform.TextMoveWordLeft, "MOVE_WORD_RIGHT": platform.TextMoveWordRight,
		"MOVE_WORD_LEFT_AND_MODIFY_SELECTION":  platform.TextMoveWordLeftAndModifySelection,
		"MOVE_WORD_RIGHT_AND_MODIFY_SELECTION": platform.TextMoveWordRightAndModifySelection,
		"DELETE_WORD_BACKWARD":                 platform.TextDeleteWordBackward, "DELETE_WORD_FORWARD": platform.TextDeleteWordForward,
		"DELETE_TO_LINE_START": platform.TextDeleteToLineStart, "DELETE_TO_LINE_END": platform.TextDeleteToLineEnd,
	} {
		arguments = append(arguments, fmt.Sprintf("-DGO_TEXT_%s=%d", name, command))
	}
	arguments = append(arguments, fmt.Sprintf("-DGO_TEXT_CANCEL_COMPOSITION=%d", platform.TextCancelComposition))
	arguments = append(arguments, "testdata/text_selection.m", "-o", executable,
		"-framework", "Cocoa", "-framework", "CoreText", "-framework", "Metal", "-framework", "QuartzCore")
	command := exec.Command("xcrun", arguments...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("compile native selection tests: %v\n%s", err, output)
	}
	if output, err := exec.Command(executable).CombinedOutput(); err != nil {
		t.Fatalf("native selection tests: %v\n%s", err, output)
	} else {
		t.Logf("%s", output)
	}
}
