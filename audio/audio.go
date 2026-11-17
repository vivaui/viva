// Package audio plays PCM sound on the system's default output device.
//
// It is a small synchronous output: [Open] opens a stream for one format,
// [Output.Write] hands it interleaved frames, and [Output.Close] releases the
// device. The platform backend converts the stream to what the device plays
// (an AudioUnit on macOS, WASAPI shared mode on Windows, ALSA's default plugin
// on Linux), so a caller supplies the format it has — a remote desktop
// server's PCM — not the device's.
//
// An application that never imports this package links no backend and runs
// nothing. While an [Output] is open it holds one goroutine, a fixed ring
// buffer, and the device; an idle application holds none of them.
package audio

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
)

var (
	// ErrUnsupported reports that this build target, or the system, has no
	// output that can play the stream: the package is not implemented for the
	// platform, the system library is missing, or no default device exists.
	ErrUnsupported = errors.New("audio: no output device is available")
	// ErrClosed reports a Write after Close.
	ErrClosed = errors.New("audio: output is closed")
	// ErrFrame reports data that does not hold whole frames.
	ErrFrame = errors.New("audio: the data does not hold whole frames")
	// ErrFormat reports a format this package cannot play.
	ErrFormat = errors.New("audio: unsupported format")
)

// Format is the shape of a stream: signed 16-bit little-endian samples,
// interleaved frame by frame. The backend converts it to the device's own
// format and rate when they differ.
type Format struct {
	// SampleRate is frames per second.
	SampleRate int
	// Channels is the number of interleaved samples in a frame: one or two.
	Channels int
}

const (
	minSampleRate = 8000
	maxSampleRate = 192000
)

func (f Format) valid() error {
	if f.SampleRate < minSampleRate || f.SampleRate > maxSampleRate {
		return fmt.Errorf("%w: %d samples per second", ErrFormat, f.SampleRate)
	}
	if f.Channels < 1 || f.Channels > 2 {
		return fmt.Errorf("%w: %d channels", ErrFormat, f.Channels)
	}
	return nil
}

// Output is a stream playing on the default output device. It implements
// io.Writer; each Write carries whole frames of the format given to Open.
//
// Write blocks while the device already holds enough sound, so a producer
// that reads from a network needs no buffer of its own beyond this one.
type Output struct {
	format Format
	align  int

	mu      sync.Mutex
	cond    *sync.Cond
	ring    []byte
	read    int64 // bytes the device has taken, as an absolute stream offset
	wrote   int64 // bytes accepted from the caller
	closed  bool
	failed  error
	dev     device
	openErr error
	done    chan struct{}
	ready   chan struct{}
}

// ringDurationScale divides a second into the ring's duration: the ring holds
// a fifth of a second of sound when the device falls behind. It absorbs a
// network burst without adding a fixed delay in steady state, where the device
// takes each write as it arrives.
const ringDurationScale = 5

// Open opens the default output device for format. It returns ErrUnsupported
// when the platform, the system library or the system's default device cannot
// play; the caller then has no sound and the rest of its work is unaffected.
func Open(format Format) (*Output, error) {
	if err := format.valid(); err != nil {
		return nil, err
	}
	ringBytes := format.SampleRate / ringDurationScale * format.Channels * 2
	if ringBytes < 4096 {
		ringBytes = 4096
	}
	o := &Output{
		format: format,
		align:  format.Channels * 2,
		ring:   make([]byte, ringBytes),
		done:   make(chan struct{}),
		ready:  make(chan struct{}),
	}
	o.cond = sync.NewCond(&o.mu)
	go o.run()
	<-o.ready
	if o.openErr != nil {
		return nil, o.openErr
	}
	return o, nil
}

// Format returns the format given to Open.
func (o *Output) Format() Format { return o.format }

// run owns the device. Platform device calls happen on this goroutine, whose
// operating-system thread is fixed: WASAPI's COM objects belong to the thread
// that created them, and CoreAudio and ALSA are happiest with one caller.
func (o *Output) run() {
	runtime.LockOSThread()
	defer close(o.done)

	dev, err := loadDevice(o.format)
	o.mu.Lock()
	o.openErr = err
	if err != nil {
		o.failed = err
	} else {
		o.dev = dev
	}
	o.mu.Unlock()
	close(o.ready)
	if err != nil {
		return
	}

	for {
		o.mu.Lock()
		for o.read == o.wrote && !o.closed && o.failed == nil {
			o.cond.Wait()
		}
		if o.closed || o.failed != nil {
			o.mu.Unlock()
			break
		}
		start := o.read % int64(len(o.ring))
		n := min(o.wrote-o.read, int64(len(o.ring))-start)
		chunk := o.ring[start : start+n]
		o.mu.Unlock()

		err := dev.write(chunk)

		o.mu.Lock()
		if err != nil {
			o.failed = err
			o.cond.Broadcast()
			o.mu.Unlock()
			break
		}
		o.read += n
		o.cond.Broadcast()
		o.mu.Unlock()
	}
	dev.close()
}

// Write plays pcm, interleaved frames of the format given to Open, and blocks
// while the ring is full. It reports the whole of pcm or the error that ended
// the stream; a partial count means the device failed or Close interrupted it.
func (o *Output) Write(pcm []byte) (int, error) {
	if len(pcm)%o.align != 0 {
		return 0, ErrFrame
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed {
		return 0, ErrClosed
	}
	if o.failed != nil {
		return 0, o.failed
	}
	written := 0
	for written < len(pcm) {
		for o.wrote-o.read == int64(len(o.ring)) && !o.closed && o.failed == nil {
			o.cond.Wait()
		}
		if o.closed {
			return written, ErrClosed
		}
		if o.failed != nil {
			return written, o.failed
		}
		space := int64(len(o.ring)) - (o.wrote - o.read)
		n := int(min(space, int64(len(pcm)-written)))
		start := o.wrote % int64(len(o.ring))
		if start+int64(n) <= int64(len(o.ring)) {
			copy(o.ring[start:start+int64(n)], pcm[written:written+n])
		} else {
			first := copy(o.ring[start:], pcm[written:written+n])
			copy(o.ring[:n-first], pcm[written+first:written+n])
		}
		o.wrote += int64(n)
		written += n
		o.cond.Broadcast()
	}
	return written, nil
}

// Close stops the stream and releases the device; sound the device has not
// taken yet is discarded. It is safe to call more than once, and unblocks a
// Write waiting for room with ErrClosed.
func (o *Output) Close() error {
	o.mu.Lock()
	if o.closed {
		o.mu.Unlock()
		<-o.done
		return nil
	}
	o.closed = true
	o.cond.Broadcast()
	o.mu.Unlock()
	<-o.done
	return nil
}

// loadDevice opens the platform device, turning a backend panic (a missing
// library symbol, for example) into an ordinary error: sound is optional, and
// an application must not die because its machine has none.
func loadDevice(format Format) (dev device, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			dev, err = nil, fmt.Errorf("audio: %v", recovered)
		}
	}()
	return openDevice(format)
}

// device is one platform's output. write plays interleaved frames of the
// format given to openDevice and must copy pcm before returning: the ring may
// reuse it afterwards. It blocks while the device already holds enough.
type device interface {
	write(pcm []byte) error
	close()
}

// openDevice is the platform's implementation. Tests replace it.
var openDevice = platformDevice
