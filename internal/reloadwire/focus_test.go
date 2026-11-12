package reloadwire

import (
	"bytes"
	"encoding/binary"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/vivaui/viva/gfx"
)

func TestPaintRequestCodecPreservesFocusVisibility(t *testing.T) {
	for _, testCase := range []struct {
		name         string
		focusVisible bool
	}{
		{name: "pointer"},
		{name: "keyboard", focusVisible: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			want := PaintRequest{
				Width: 640, Height: 480, Hovered: gfx.Item(3), Pressed: gfx.Item(5), Focused: gfx.Item(7),
				FocusVisible: testCase.focusVisible, ScrollX: 12.5, ScrollY: -8.25,
			}
			data := encodePaintRequest(want)
			got, err := decodePaintRequest(data)
			if err != nil || got != want {
				t.Fatalf("paint request = %+v, want %+v, err = %v", got, want, err)
			}
		})
	}
}

func TestPaintRequestCodecRejectsMissingOrInvalidFocusVisibility(t *testing.T) {
	request := PaintRequest{Width: 640, Height: 480, Focused: gfx.Item(7), ScrollX: 12.5, ScrollY: -8.25}
	data := encodePaintRequest(request)
	request.FocusVisible = true
	at := byteThatFlips(t, data, encodePaintRequest(request))
	data = encodePaintRequest(request)
	missing := append(append([]byte(nil), data[:at]...), data[at+1:]...)
	invalid := append([]byte(nil), data...)
	invalid[at] = 2
	for _, testCase := range []struct {
		name    string
		payload []byte
	}{
		{name: "version one payload", payload: missing},
		{name: "invalid boolean", payload: invalid},
		{name: "trailing data", payload: append(append([]byte(nil), data...), 0)},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := decodePaintRequest(testCase.payload); err == nil {
				t.Fatalf("invalid paint payload was accepted: %x", testCase.payload)
			}
		})
	}
}

func TestAcceptRejectsVersionOneWorkerBeforeReadingPayload(t *testing.T) {
	payload, err := encodeHello(Hello{Token: "test-token", Config: WindowConfig{Title: "old worker", Width: 320, Height: 200}})
	if err != nil {
		t.Fatal(err)
	}
	var encoded bytes.Buffer
	if err := writeFrame(&encoded, frame{kind: messageHello, payload: payload}); err != nil {
		t.Fatal(err)
	}
	header := encoded.Bytes()[:16]
	binary.LittleEndian.PutUint16(header[4:6], 1)
	host, worker := net.Pipe()
	defer host.Close()
	defer worker.Close()
	deadline := time.Now().Add(time.Second)
	if err := host.SetDeadline(deadline); err != nil {
		t.Fatal(err)
	}
	if err := worker.SetDeadline(deadline); err != nil {
		t.Fatal(err)
	}
	accepted := make(chan error, 1)
	go func() {
		_, client, err := Accept(host, "test-token")
		if client != nil {
			_ = client.Close()
		}
		accepted <- err
	}()
	if _, err := worker.Write(header); err != nil {
		t.Fatal(err)
	}
	if err := <-accepted; err == nil || !strings.Contains(err.Error(), "reload protocol version 1 is not supported; want "+strconv.Itoa(ProtocolVersion)) {
		t.Fatalf("old worker rejection = %v", err)
	}
}

func TestWorkerPaintPreservesRealAndVisibleFocus(t *testing.T) {
	var canvas gfx.Canvas
	for _, testCase := range []struct {
		name         string
		focused      uint32
		focusVisible bool
	}{
		{name: "pointer focus", focused: 7},
		{name: "keyboard focus", focused: 7, focusVisible: true},
		{name: "pointer refocus", focused: 7},
		{name: "blur"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			want := PaintRequest{
				Width: 640, Height: 480, Hovered: gfx.Item(3), Pressed: gfx.Item(5), Focused: gfx.Item(int(testCase.focused)),
				FocusVisible: testCase.focusVisible, ScrollX: 12.5, ScrollY: -8.25,
			}
			var observed PaintRequest
			calls := 0
			config := WorkerConfig{Paint: func(canvas *gfx.Canvas, size gfx.Size) {
				calls++
				observed = PaintRequest{
					Width: size.Width, Height: size.Height, Hovered: canvas.Hovered(), Pressed: canvas.Pressed(),
					Focused: canvas.Focused(), FocusVisible: !canvas.FocusVisible().IsZero(),
					ScrollX: canvas.ScrollX(), ScrollY: canvas.ScrollY(),
				}
			}}
			request := frame{kind: messagePaint, id: 1, payload: encodePaintRequest(want)}
			_, err, shutdown := handleRequest(&config, &canvas, CanvasSnapshot{}, request, nil)
			if err != nil || shutdown {
				t.Fatalf("paint request err = %v, shutdown = %v", err, shutdown)
			}
			if calls != 1 || observed != want {
				t.Fatalf("paint calls = %d, interaction = %+v, want %+v", calls, observed, want)
			}
		})
	}
}

func BenchmarkPaintRequestCodec(b *testing.B) {
	request := PaintRequest{Width: 640, Height: 480, Hovered: gfx.Item(3), Pressed: gfx.Item(5), Focused: gfx.Item(7), FocusVisible: true, ScrollX: 12.5, ScrollY: -8.25}
	b.ReportAllocs()
	b.SetBytes(int64(len(encodePaintRequest(request))))
	b.ResetTimer()
	for range b.N {
		decoded, err := decodePaintRequest(encodePaintRequest(request))
		if err != nil || decoded != request {
			b.Fatalf("paint request = %+v, want %+v, err = %v", decoded, request, err)
		}
	}
}
