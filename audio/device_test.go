package audio

import (
	"io"
	"os"
	"testing"
	"time"
)

// TestOutputDevice plays silence on the machine's default output. Audio has
// no headless substitute, so it is opt-in: VIVA_AUDIO_TEST=1 runs it.
func TestOutputDevice(t *testing.T) {
	if os.Getenv("VIVA_AUDIO_TEST") == "" {
		t.Skip("set VIVA_AUDIO_TEST=1 to open the default output device")
	}
	for _, format := range []Format{
		{SampleRate: 48000, Channels: 2},
		{SampleRate: 44100, Channels: 2},
		{SampleRate: 48000, Channels: 1},
	} {
		out, err := Open(format)
		if err != nil {
			t.Fatalf("Open(%+v): %v", format, err)
		}
		// 200 ms of silence, then let the device consume it.
		silence := make([]byte, format.SampleRate/5*format.Channels*2)
		if n, err := out.Write(silence); err != nil || n != len(silence) {
			t.Fatalf("Write(%+v) = %d, %v", format, n, err)
		}
		time.Sleep(300 * time.Millisecond)
		if err := out.Close(); err != nil {
			t.Fatalf("Close(%+v): %v", format, err)
		}
	}
}

// TestInputDevice captures a short sample from the machine's default input.
// Like TestOutputDevice, it is opt-in under VIVA_AUDIO_TEST=1.
func TestInputDevice(t *testing.T) {
	if os.Getenv("VIVA_AUDIO_TEST") == "" {
		t.Skip("set VIVA_AUDIO_TEST=1 to open the default input device")
	}
	for _, format := range []Format{
		{SampleRate: 48000, Channels: 1},
		{SampleRate: 44100, Channels: 2},
		{SampleRate: 16000, Channels: 1},
	} {
		in, err := OpenInput(format)
		if err != nil {
			t.Fatalf("OpenInput(%+v): %v", format, err)
		}
		buf := make([]byte, format.SampleRate/20*format.Channels*2)
		if _, err := io.ReadFull(in, buf); err != nil {
			_ = in.Close()
			t.Fatalf("ReadFull(%+v): %v", format, err)
		}
		if err := in.Close(); err != nil {
			t.Fatalf("Close(%+v): %v", format, err)
		}
	}
}
