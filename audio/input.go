package audio

import (
	"fmt"
	"io"
	"runtime"
	"sync"
)

// Input is a stream capturing PCM from the system's default input device. It
// implements io.ReadCloser; each Read returns interleaved 16-bit signed
// little-endian frames in the format given to OpenInput.
//
// While an Input is open it holds one goroutine, a fixed ring buffer, and the
// capture device. If the caller stops reading temporarily, oldest frames are
// discarded so the capture thread never stalls.
type Input struct {
	format Format
	align  int

	mu      sync.Mutex
	cond    *sync.Cond
	ring    []byte
	head    int // read offset in ring
	used    int // unread bytes in ring
	closed  bool
	failed  error
	dev     inputDevice
	openErr error
	done    chan struct{}
	ready   chan struct{}
}

// OpenInput opens the default input device for format. It returns
// ErrInputUnsupported when the platform, the system library, or the system's
// default input device cannot capture.
func OpenInput(format Format) (*Input, error) {
	if err := format.valid(); err != nil {
		return nil, err
	}
	align := format.Channels * 2
	// Keep up to a quarter second of captured audio in the ring.
	ringBytes := max(format.SampleRate/4*align, 4096)
	ringBytes -= ringBytes % align
	if ringBytes < align {
		ringBytes = align
	}
	in := &Input{
		format: format,
		align:  align,
		ring:   make([]byte, ringBytes),
		done:   make(chan struct{}),
		ready:  make(chan struct{}),
	}
	in.cond = sync.NewCond(&in.mu)
	go in.run()
	<-in.ready
	if in.openErr != nil {
		<-in.done
		return nil, in.openErr
	}
	return in, nil
}

// Format returns the format given to OpenInput.
func (in *Input) Format() Format { return in.format }

func (in *Input) run() {
	runtime.LockOSThread()
	defer close(in.done)

	dev, err := loadInputDevice(in.format)
	in.mu.Lock()
	in.openErr = err
	if err != nil {
		in.failed = err
	} else {
		in.dev = dev
	}
	in.mu.Unlock()
	close(in.ready)
	if err != nil {
		return
	}

	scratch := make([]byte, max(in.format.SampleRate/4, 4096)*in.align)
	for {
		n, err := dev.read(scratch)
		if n > 0 {
			in.push(scratch[:n])
		}
		if err != nil {
			in.mu.Lock()
			if !in.closed {
				in.failed = err
			}
			in.cond.Broadcast()
			in.mu.Unlock()
			break
		}
		in.mu.Lock()
		closed := in.closed
		in.mu.Unlock()
		if closed {
			break
		}
	}
	dev.close()
}

func (in *Input) push(pcm []byte) {
	in.mu.Lock()
	defer in.mu.Unlock()
	if in.closed {
		return
	}
	capRing := len(in.ring)
	if len(pcm) >= capRing {
		pcm = pcm[len(pcm)-capRing:]
		in.head = 0
		in.used = 0
	} else if overflow := in.used + len(pcm) - capRing; overflow > 0 {
		in.head = (in.head + overflow) % capRing
		in.used -= overflow
	}
	write := (in.head + in.used) % capRing
	first := min(len(pcm), capRing-write)
	copy(in.ring[write:], pcm[:first])
	if first < len(pcm) {
		copy(in.ring, pcm[first:])
	}
	in.used += len(pcm)
	in.cond.Broadcast()
}

// Read copies captured PCM frames into pcm, blocking until at least one byte
// is available or the stream ends. Returns io.EOF after Close.
func (in *Input) Read(pcm []byte) (int, error) {
	if len(pcm) == 0 {
		return 0, nil
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	for in.used == 0 && !in.closed && in.failed == nil {
		in.cond.Wait()
	}
	if in.closed {
		return 0, io.EOF
	}
	if in.used == 0 && in.failed != nil {
		return 0, in.failed
	}
	take := min(len(pcm), in.used)
	if len(pcm) >= in.align && take >= in.align {
		take -= take % in.align
	}
	capRing := len(in.ring)
	first := min(take, capRing-in.head)
	copy(pcm[:first], in.ring[in.head:in.head+first])
	if first < take {
		copy(pcm[first:take], in.ring[:take-first])
	}
	in.head = (in.head + take) % capRing
	in.used -= take
	return take, nil
}

// Close stops capturing and releases the input device. It is safe to call more
// than once, and unblocks any waiting Read with io.EOF.
func (in *Input) Close() error {
	in.mu.Lock()
	already := in.closed
	in.closed = true
	dev := in.dev
	in.cond.Broadcast()
	in.mu.Unlock()
	if !already && dev != nil {
		dev.stop()
	}
	<-in.done
	return nil
}

func loadInputDevice(format Format) (dev inputDevice, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			dev, err = nil, fmt.Errorf("audio: %v", recovered)
		}
	}()
	return openInputDevice(format)
}

// inputDevice is one platform's capture stream. read and close run on the
// capture goroutine locked to an OS thread; stop may be called from any
// goroutine to unblock read.
type inputDevice interface {
	read(pcm []byte) (int, error)
	stop()
	close()
}

// openInputDevice is the platform's implementation. Tests replace it.
var openInputDevice = platformInputDevice
