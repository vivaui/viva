package darwin

import (
	"testing"

	"github.com/vivaui/viva/internal/platform"
)

// AppKit counts text in UTF-16 units and Viva in code points; a selection that
// crosses a character outside the basic plane must come out in AppKit's.
func TestAccessibilityHintCarriesInvalid(t *testing.T) {
	if got := darwinAccessibilityHint(platform.AccessibilityNode{Hint: "Required"}); got != "Required" {
		t.Fatalf("valid hint = %q", got)
	}
	if got := darwinAccessibilityHint(platform.AccessibilityNode{Invalid: true}); got != "Invalid entry" {
		t.Fatalf("invalid hint = %q", got)
	}
	if got := darwinAccessibilityHint(platform.AccessibilityNode{Hint: "Required", Invalid: true}); got != "Required. Invalid entry" {
		t.Fatalf("invalid hint over an existing hint = %q", got)
	}
}

func TestNativeSelectionCountsInUTF16(t *testing.T) {
	node := platform.AccessibilityNode{
		Value:     "a\U0001F468b",
		Selection: platform.AccessibilitySelection{Start: 1, End: 3, Valid: true},
	}
	start, end, length := nativeSelection(node)
	if start != 1 || end != 4 || length != 4 {
		t.Fatalf("selection = %d..%d of %d, want 1..4 of 4", start, end, length)
	}
	if _, _, length := nativeSelection(platform.AccessibilityNode{Value: "x"}); length != 0 {
		t.Fatal("a node with no caret reported a text length")
	}
}
