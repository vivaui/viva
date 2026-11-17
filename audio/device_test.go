package audio

import (
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
