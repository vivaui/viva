package audio

import (
	"bytes"
	"errors"
	"sync"
	"testing"
	"time"
)

// fakeDevice records what the output goroutine hands it. wait makes write
// block until the test releases it, which fills the ring.
type fakeDevice struct {
	mu      sync.Mutex
	data    []byte
	closed  bool
	fail    error
	wait    chan struct{}
	discard bool
}

func (d *fakeDevice) write(pcm []byte) error {
	if d.wait != nil {
		<-d.wait
	}
	if d.fail != nil {
		return d.fail
	}
	if !d.discard {
		d.mu.Lock()
		d.data = append(d.data, pcm...)
		d.mu.Unlock()
	}
	return nil
}

func (d *fakeDevice) close() {
	d.mu.Lock()
	d.closed = true
	d.mu.Unlock()
}

func (d *fakeDevice) played() []byte {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]byte(nil), d.data...)
}

func withDevice(t testing.TB, d device, err error) {
	t.Helper()
	previous := openDevice
	openDevice = func(Format) (device, error) { return d, err }
	t.Cleanup(func() { openDevice = previous })
}

func testFormat() Format { return Format{SampleRate: 8000, Channels: 2} }

func TestOpenRejectsBadFormats(t *testing.T) {
	for _, format := range []Format{
		{SampleRate: 0, Channels: 2},
		{SampleRate: 7999, Channels: 2},
		{SampleRate: 8000, Channels: 0},
		{SampleRate: 8000, Channels: 3},
	} {
		if _, err := Open(format); !errors.Is(err, ErrFormat) {
			t.Errorf("Open(%+v) error = %v, want ErrFormat", format, err)
		}
	}
}

func TestOpenReportsPlatformError(t *testing.T) {
	withDevice(t, nil, ErrUnsupported)
	if _, err := Open(testFormat()); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("Open error = %v, want ErrUnsupported", err)
	}
}

func TestWritePlaysInOrderAcrossRing(t *testing.T) {
	dev := &fakeDevice{}
	withDevice(t, dev, nil)
	out, err := Open(testFormat())
	if err != nil {
		t.Fatal(err)
	}
	// More than the 6400-byte ring, so the device drains it more than once.
	var pcm []byte
	for i := 0; i < 5000; i++ {
		pcm = append(pcm, byte(i), byte(i>>8), byte(i>>16), byte(i>>24))
	}
	if n, err := out.Write(pcm); err != nil || n != len(pcm) {
		t.Fatalf("Write = %d, %v", n, err)
	}
	// The device takes the ring asynchronously; let it drain before Close,
	// which stops the stream rather than waiting for the device.
	time.Sleep(50 * time.Millisecond)
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
	if got := dev.played(); !bytes.Equal(got, pcm) {
		t.Fatalf("played %d bytes, want %d identical bytes", len(got), len(pcm))
	}
}

func TestWriteBlocksWhileRingIsFull(t *testing.T) {
	dev := &fakeDevice{wait: make(chan struct{})}
	withDevice(t, dev, nil)
	out, err := Open(testFormat())
	if err != nil {
		t.Fatal(err)
	}
	// The ring is 6400 bytes and the device holds its first write forever, so
	// the second half of this write can only complete once the device takes
	// the first.
	pcm := make([]byte, 12800)
	done := make(chan error, 1)
	go func() {
		_, err := out.Write(pcm)
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("Write returned %v while the device held the ring", err)
	case <-time.After(50 * time.Millisecond):
	}
	close(dev.wait)
	if err := <-done; err != nil {
		t.Fatalf("Write after the device drained: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestCloseUnblocksWrite(t *testing.T) {
	dev := &fakeDevice{wait: make(chan struct{})}
	withDevice(t, dev, nil)
	out, err := Open(testFormat())
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := out.Write(make([]byte, 12800))
		done <- err
	}()
	time.Sleep(50 * time.Millisecond)
	close(dev.wait)
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if !errors.Is(err, ErrClosed) && err != nil {
			t.Fatalf("Write after Close = %v, want ErrClosed", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Write did not return after Close")
	}
}

func TestWriteAfterClose(t *testing.T) {
	withDevice(t, &fakeDevice{}, nil)
	out, err := Open(testFormat())
	if err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	if _, err := out.Write(make([]byte, 4)); !errors.Is(err, ErrClosed) {
		t.Fatalf("Write after Close = %v, want ErrClosed", err)
	}
}

func TestWriteRejectsPartialFrames(t *testing.T) {
	withDevice(t, &fakeDevice{}, nil)
	out, err := Open(testFormat())
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	if _, err := out.Write(make([]byte, 6)); !errors.Is(err, ErrFrame) {
		t.Fatalf("Write of 6 bytes = %v, want ErrFrame", err)
	}
}

func TestDeviceFailureReachesLaterWrites(t *testing.T) {
	broken := errors.New("device gone")
	dev := &fakeDevice{fail: broken}
	withDevice(t, dev, nil)
	out, err := Open(testFormat())
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	// More than the ring holds: the write waits for the device, which fails.
	_, err = out.Write(make([]byte, 12800))
	if !errors.Is(err, broken) {
		t.Fatalf("Write = %v, want the device error", err)
	}
}

func TestNothingIsPlayedWithoutWrite(t *testing.T) {
	dev := &fakeDevice{}
	withDevice(t, dev, nil)
	out, err := Open(testFormat())
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)
	if n := len(dev.played()); n != 0 {
		t.Fatalf("device got %d bytes without a Write", n)
	}
	out.Close()
}

func BenchmarkWrite(b *testing.B) {
	dev := &fakeDevice{discard: true}
	withDevice(b, dev, nil)
	out, err := Open(testFormat())
	if err != nil {
		b.Fatal(err)
	}
	defer out.Close()
	pcm := make([]byte, 640)
	b.SetBytes(int64(len(pcm)))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := out.Write(pcm); err != nil {
			b.Fatal(err)
		}
	}
}
