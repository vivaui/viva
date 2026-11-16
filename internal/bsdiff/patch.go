// Package bsdiff applies bsdiff 4 patches. It reads the format
// github.com/gabstv/go-bsdiff (and Colin Percival's original bsdiff) writes:
// a "BSDIFF40" header and three bzip2 streams (control, diff and extra), with
// the standard library's bzip2 reader.
//
// The update package applies patches on the user's machine, so this file is
// deliberately the applier only: generation needs a suffix array and belongs
// in the release command (cmd/viva), which uses go-bsdiff for it.
package bsdiff

import (
	"bytes"
	"compress/bzip2"
	"errors"
	"fmt"
	"io"
)

// ErrPatch reports a patch that is malformed or does not fit the old data.
var ErrPatch = errors.New("bsdiff: invalid patch")

// MaxOutput caps the size a patch may produce, so a corrupt or hostile header
// cannot make the caller allocate an arbitrary amount. Callers that know the
// size check it themselves too; this is the backstop.
const MaxOutput = 512 << 20

// Patch applies patch to old and returns the new data.
func Patch(old, patch []byte) ([]byte, error) {
	if len(patch) < 32 || string(patch[:8]) != "BSDIFF40" {
		return nil, fmt.Errorf("%w: not a BSDIFF40 patch", ErrPatch)
	}
	ctrlLen, err := offTin(patch[8:16])
	if err != nil {
		return nil, err
	}
	diffLen, err := offTin(patch[16:24])
	if err != nil {
		return nil, err
	}
	newSize, err := offTin(patch[24:32])
	if err != nil {
		return nil, err
	}
	if ctrlLen < 0 || diffLen < 0 || newSize < 0 {
		return nil, fmt.Errorf("%w: negative length", ErrPatch)
	}
	if newSize > MaxOutput {
		return nil, fmt.Errorf("%w: output %d exceeds %d bytes", ErrPatch, newSize, MaxOutput)
	}
	endCtrl := 32 + int(ctrlLen)
	endDiff := endCtrl + int(diffLen)
	if int64(endDiff) > int64(len(patch)) || endCtrl < 32 || endDiff < endCtrl {
		return nil, fmt.Errorf("%w: truncated streams", ErrPatch)
	}
	ctrl := bzip2.NewReader(bytes.NewReader(patch[32:endCtrl]))
	diff := bzip2.NewReader(bytes.NewReader(patch[endCtrl:endDiff]))
	extra := bzip2.NewReader(bytes.NewReader(patch[endDiff:]))

	out := make([]byte, newSize)
	var oldPos, newPos int64
	var header [24]byte
	for newPos < newSize {
		if _, err := io.ReadFull(ctrl, header[:]); err != nil {
			return nil, fmt.Errorf("%w: control stream: %v", ErrPatch, err)
		}
		x, err := offTin(header[0:8])
		if err != nil {
			return nil, err
		}
		y, err := offTin(header[8:16])
		if err != nil {
			return nil, err
		}
		z, err := offTin(header[16:24])
		if err != nil {
			return nil, err
		}
		if x < 0 || y < 0 || x > newSize-newPos || y > newSize-newPos-x {
			return nil, fmt.Errorf("%w: control triple out of range", ErrPatch)
		}
		if x == 0 && y == 0 && z == 0 {
			return nil, fmt.Errorf("%w: control triple makes no progress", ErrPatch)
		}
		// The diff stream is added to the old data, byte by byte. Old
		// positions outside the old file read as zero.
		scratch := make([]byte, x)
		if _, err := io.ReadFull(diff, scratch); err != nil {
			return nil, fmt.Errorf("%w: diff stream: %v", ErrPatch, err)
		}
		for i := int64(0); i < x; i++ {
			var base byte
			if at := oldPos + i; at >= 0 && at < int64(len(old)) {
				base = old[at]
			}
			out[newPos+i] = base + scratch[i]
		}
		newPos += x
		oldPos += x
		if y > 0 {
			if _, err := io.ReadFull(extra, out[newPos:newPos+y]); err != nil {
				return nil, fmt.Errorf("%w: extra stream: %v", ErrPatch, err)
			}
			newPos += y
		}
		oldPos += z
	}
	return out, nil
}

// offTin reads a signed 64-bit value the way bsdiff writes it: seven
// little-endian bytes, the sign in the high bit of the eighth.
func offTin(b []byte) (int64, error) {
	if len(b) < 8 {
		return 0, fmt.Errorf("%w: short integer", ErrPatch)
	}
	value := int64(b[7] & 0x7f)
	for i := 6; i >= 0; i-- {
		value = value<<8 | int64(b[i])
	}
	if b[7]&0x80 != 0 {
		value = -value
	}
	return value, nil
}
