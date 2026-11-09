//go:build linux

package app

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestLinuxAcceptsNativeTextInputConfiguration(t *testing.T) {
	runner = func(Config, *gfx.Canvas) error { return nil }
	defer func() { runner = nil }()
	err := RunCanvas(Config{
		TextInput: func(TextInputContext) *TextInput { return nil },
	}, nil)
	if err != nil {
		t.Fatalf("RunCanvas error = %v", err)
	}
}
