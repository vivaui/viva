package components

import (
	"testing"

	"github.com/vivaui/viva/internal/platform/windows"
)

func TestCodeEditorDirectWriteCarriesBidiOverrideAcrossSoftBreak(t *testing.T) {
	engine := windows.TextEngine()
	if engine == nil {
		t.Fatal("DirectWrite unavailable")
	}
	testCodeEditorBidiOverrideAcrossSoftBreak(t, engine)
}
