package bsdiff

import (
	"bytes"
	"errors"
	"math/rand"
	"testing"

	"github.com/gabstv/go-bsdiff/pkg/bsdiff"
)

func TestPatchRoundTrip(t *testing.T) {
	cases := map[string][2][]byte{
		"identical":  {bytes.Repeat([]byte("same"), 1000), bytes.Repeat([]byte("same"), 1000)},
		"empty old":  {nil, []byte("brand new content")},
		"both empty": {nil, nil},
		"insertion":  {bytes.Repeat([]byte("abcdefgh"), 5000), append(bytes.Repeat([]byte("abcdefgh"), 2500), append([]byte("middle"), bytes.Repeat([]byte("abcdefgh"), 2500)...)...)},
		"deletion":   {append(bytes.Repeat([]byte("abcdefgh"), 2500), append([]byte("gone"), bytes.Repeat([]byte("abcdefgh"), 2500)...)...), bytes.Repeat([]byte("abcdefgh"), 5000)},
	}
	for name, data := range cases {
		patch, err := bsdiff.Bytes(data[0], data[1])
		if err != nil {
			t.Fatalf("%s: diff: %v", name, err)
		}
		got, err := Patch(data[0], patch)
		if err != nil {
			t.Fatalf("%s: patch: %v", name, err)
		}
		if !bytes.Equal(got, data[1]) {
			t.Errorf("%s: patched data differs from the new data", name)
		}
	}
}

func TestPatchRandomData(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	old := make([]byte, 64<<10)
	rng.Read(old)
	fresh := make([]byte, len(old))
	copy(fresh, old)
	copy(fresh[1000:], []byte("a changed stretch of bytes"))
	patch, err := bsdiff.Bytes(old, fresh)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Patch(old, patch)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, fresh) {
		t.Fatal("patched data differs from the new data")
	}
}

func TestPatchRefusesBadInput(t *testing.T) {
	old := []byte("hello")
	patch, err := bsdiff.Bytes(old, []byte("hello world"))
	if err != nil {
		t.Fatal(err)
	}
	tests := map[string]struct {
		old, patch []byte
	}{
		"not a patch":    {old, []byte("not a bsdiff patch at all")},
		"truncated":      {old, patch[:len(patch)/2]},
		"truncated head": {old, patch[:20]},
	}
	for name, test := range tests {
		if _, err := Patch(test.old, test.patch); err == nil {
			t.Errorf("%s: patch accepted", name)
		}
	}
}

func TestPatchRefusesAHostileSize(t *testing.T) {
	patch := make([]byte, 32)
	copy(patch, "BSDIFF40")
	// newSize is the third off_t: seven little-endian bytes and a sign.
	patch[24+0] = 0xff
	patch[24+1] = 0xff
	patch[24+2] = 0xff
	patch[24+3] = 0xff
	patch[24+4] = 0x7f
	if _, err := Patch(nil, patch); !errors.Is(err, ErrPatch) {
		t.Fatalf("err = %v, want ErrPatch", err)
	}
}
