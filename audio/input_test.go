package audio

import (
	"bytes"
	"errors"
	"io"
	"sync"
	"testing"
	"time"
)

type fakeInputDevice struct {
	mu     sync.Mutex
	chunks chan []byte
	closed bool
	fail   error
}

func newFakeInputDevice() *fakeInputDevice {
	return &fakeInputDevice{chunks: make(chan []byte, 16)}
}

func (d *fakeInputDevice) push(pcm []byte) {
	d.chunks <- append([]byte(nil), pcm...)
}

func (d *fakeInputDevice) read(dst []byte) (int, error) {
	chunk, ok := <-d.chunks
	if !ok {
		if d.fail != nil {
			return 0, d.fail
		}
		return 0, io.EOF
	}
	return copy(dst, chunk), nil
}

func (d *fakeInputDevice) stop() {
	d.mu.Lock()
	if !d.closed {
		d.closed = true
		close(d.chunks)
	}
	d.mu.Unlock()
}

func (d *fakeInputDevice) close() {
	d.stop()
}

func withInputDevice(t testing.TB, dev inputDevice, err error) {
	t.Helper()
	previous := openInputDevice
	openInputDevice = func(Format) (inputDevice, error) { return dev, err }
	t.Cleanup(func() { openInputDevice = previous })
}

func TestOpenInputRejectsBadFormats(t *testing.T) {
	for _, format := range []Format{
		{SampleRate: 0, Channels: 2},
		{SampleRate: 7999, Channels: 2},
		{SampleRate: 192001, Channels: 1},
		{SampleRate: 8000, Channels: 0},
		{SampleRate: 8000, Channels: 3},
	} {
		if _, err := OpenInput(format); !errors.Is(err, ErrFormat) {
			t.Errorf("OpenInput(%+v) error = %v, want ErrFormat", format, err)
		}
	}
}

func TestOpenInputReportsPlatformError(t *testing.T) {
	withInputDevice(t, nil, ErrInputUnsupported)
	if _, err := OpenInput(testFormat()); !errors.Is(err, ErrInputUnsupported) {
		t.Fatalf("OpenInput error = %v, want ErrInputUnsupported", err)
	}
}

func TestInputStreamsInOrderAndCloseUnblocksRead(t *testing.T) {
	dev := newFakeInputDevice()
	withInputDevice(t, dev, nil)

	in, err := OpenInput(Format{SampleRate: 16000, Channels: 1})
	if err != nil {
		t.Fatal(err)
	}
	if in.Format() != (Format{SampleRate: 16000, Channels: 1}) {
		t.Fatalf("Format() = %+v", in.Format())
	}

	want := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	dev.push(want[:4])
	dev.push(want[4:])

	got := make([]byte, len(want))
	if _, err := io.ReadFull(in, got); err != nil {
		t.Fatalf("ReadFull: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("read %v, want %v", got, want)
	}

	readErr := make(chan error, 1)
	go func() {
		var buf [16]byte
		_, err := in.Read(buf[:])
		readErr <- err
	}()
	time.Sleep(20 * time.Millisecond)
	if err := in.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	select {
	case err := <-readErr:
		if !errors.Is(err, io.EOF) {
			t.Fatalf("Read after Close = %v, want io.EOF", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Read did not unblock after Close")
	}
	if err := in.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestInputRingDropsOldestFramesWhenOverflowed(t *testing.T) {
	dev := newFakeInputDevice()
	withInputDevice(t, dev, nil)

	in, err := OpenInput(Format{SampleRate: 8000, Channels: 2})
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	// Ring size for 8000 Hz stereo is 8000 bytes; push 10000 bytes so the first
	// 2000 bytes are dropped in favor of newer frames.
	chunk := make([]byte, 4000)
	for i := range chunk {
		chunk[i] = 1
	}
	dev.push(chunk)
	chunk2 := make([]byte, 4000)
	for i := range chunk2 {
		chunk2[i] = 2
	}
	dev.push(chunk2)
	chunk3 := make([]byte, 2000)
	for i := range chunk3 {
		chunk3[i] = 3
	}
	dev.push(chunk3)

	// Wait briefly for the capture goroutine to drain all 3 chunks into the ring.
	time.Sleep(30 * time.Millisecond)

	buf := make([]byte, 8000)
	if _, err := io.ReadFull(in, buf); err != nil {
		t.Fatalf("ReadFull: %v", err)
	}
	if buf[0] != 1 || buf[2000] != 2 || buf[6000] != 3 {
		t.Fatalf("ring overwrite unexpected: first=%d mid=%d tail=%d", buf[0], buf[2000], buf[6000])
	}
}
