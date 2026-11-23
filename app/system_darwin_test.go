//go:build darwin && !ios

package app

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	darwinplatform "github.com/vivaui/viva/internal/platform/darwin"
)

func TestDarwinNotificationPermissionMapping(t *testing.T) {
	err := darwinNotificationError(fmt.Errorf("wrapped: %w", darwinplatform.ErrPermissionDenied))
	if !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("notification permission error = %v", err)
	}
}

func TestPreferredLanguagesReportsTheUsersList(t *testing.T) {
	languages := PreferredLanguages()
	if len(languages) == 0 {
		t.Skip("this macOS reports no preferred languages")
	}
	for _, tag := range languages {
		if tag == "" || strings.ContainsAny(tag, "_. ") {
			t.Fatalf("not a BCP 47 tag: %q in %v", tag, languages)
		}
	}
}
