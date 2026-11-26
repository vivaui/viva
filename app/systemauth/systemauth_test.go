package systemauth

import (
	"context"
	"errors"
	"testing"
)

func TestVerifyRejectsInvalidOrCanceledRequestBeforeNativeUI(t *testing.T) {
	for _, reason := range []string{"", "  ", "null\x00injected", "\xff"} {
		if err := Verify(context.Background(), reason); err == nil {
			t.Fatalf("accepted invalid reason %q", reason)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := Verify(ctx, "Unlock application"); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled request: %v", err)
	}
}

func TestVerifyRefusesConcurrentPrompts(t *testing.T) {
	prompt <- struct{}{}
	defer func() { <-prompt }()
	if err := Verify(context.Background(), "Unlock application"); !errors.Is(err, ErrFailed) {
		t.Fatalf("second prompt: %v", err)
	}
}
