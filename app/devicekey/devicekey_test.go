package devicekey

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestCanceledContextDoesNotStartNativeWork(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	opts := Options{Service: "tests", Account: "slot"}
	if _, err := Enroll(ctx, opts, []byte("key")); !errors.Is(err, context.Canceled) {
		t.Fatalf("Enroll: %v", err)
	}
	if _, err := Unseal(ctx, opts, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("Unseal: %v", err)
	}
	if err := Revoke(ctx, opts, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("Revoke: %v", err)
	}
}

func TestBlobCannotMoveBetweenSlots(t *testing.T) {
	opts := Options{Service: "tests", Account: "one"}
	blob, err := json.Marshal(envelope{Version: 1, Platform: "test", ID: []byte("credential"), Binding: binding(opts)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = parse(opts, blob); err != nil {
		t.Fatal(err)
	}
	opts.Account = "two"
	if _, err = parse(opts, blob); !errors.Is(err, ErrInvalidBlob) {
		t.Fatalf("cross-slot blob: %v", err)
	}
}

func TestMalformedBlobDoesNotReachNativeStore(t *testing.T) {
	for _, blob := range [][]byte{nil, []byte("{}"), []byte("null"), []byte("not json"), make([]byte, 17*1024)} {
		if _, err := Unseal(context.Background(), Options{Service: "tests", Account: "slot"}, blob); !errors.Is(err, ErrInvalidBlob) {
			t.Fatalf("invalid blob: %v", err)
		}
	}
}
