package reloadwire

import (
	"context"
	"errors"
	"io"
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/vivaui/viva/ui"
)

func TestCanvasCodecPreservesRetainedOutput(t *testing.T) {
	var canvas ui.Canvas
	canvas.PushLayer(7)
	canvas.Clip(ui.Rect{X: 2, Y: 3, Width: 90, Height: 40})
	canvas.Fill(ui.Rect{X: 0, Y: 0, Width: 100, Height: 50}, ui.Color{Alpha: 1})
	canvas.Elevate(ui.Rect{X: 1, Y: 2, Width: 80, Height: 30}, 6, ui.ShadowMD)
	canvas.Rounded(ui.Rect{X: 1, Y: 2, Width: 80, Height: 30}, 6, ui.Color{Red: 0.2, Green: 0.3, Blue: 0.4, Alpha: 1})
	canvas.DrawBitmapRegion(ui.Bitmap{
		Pix: []byte{0, 0, 255, 255, 0, 255, 0, 255}, Width: 2, Height: 1, Stride: 8, ID: 55, Version: 2,
	}, ui.Rect{X: 1, Width: 1, Height: 1}, ui.Rect{X: 10, Y: 10, Width: 20, Height: 10})
	canvas.Label("reload", ui.Rect{X: 5, Y: 6, Width: 50, Height: 14}, ui.TextStyle{Size: 12, Weight: 600, Color: ui.Color{Alpha: 1}})
	canvas.Unclip()
	canvas.PopLayer()
	canvas.PushLayer(600)
	canvas.PushFocusScope(ui.Item(44))
	canvas.Block(ui.Rect{X: 2, Y: 3, Width: 70, Height: 40})
	canvas.HitWithOptions(ui.Item(9), ui.Rect{X: 4, Y: 5, Width: 60, Height: 20}, "Enable", ui.HitOptions{Role: ui.RoleCheckbox, Checked: true})
	canvas.HitWithOptions(ui.Item(10), ui.Rect{X: 4, Y: 40, Width: 60, Height: 20}, "Password", ui.HitOptions{Role: ui.RoleTextField, Value: "never", Secure: true})
	canvas.HitWithOptions(ui.Item(11), ui.Rect{X: 4, Y: 70, Width: 120, Height: 30}, "Name", ui.HitOptions{
		Role: ui.RoleTextField, TextBounds: ui.Rect{X: 16, Y: 70, Width: 96, Height: 30},
		TextSize: 12, TextColor: ui.Color{Red: 0.9, Green: 0.8, Blue: 0.7, Alpha: 1},
	})
	canvas.HitWithOptions(ui.Item(12), ui.Rect{X: 4, Y: 110, Width: 120, Height: 30}, "Terminal", ui.HitOptions{Role: ui.RoleTerminal, Autofocus: true})
	canvas.PopFocusScope()
	canvas.PopLayer()
	canvas.TrapFocus(ui.Item(44))
	canvas.SetEscapeTarget(ui.Item(9))
	canvas.SetInitialFocus(ui.Item(9))

	want := CaptureCanvas(&canvas)
	data, err := encodeCanvas(want)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeCanvas(data)
	if err != nil {
		t.Fatal(err)
	}
	var restored ui.Canvas
	ApplyCanvas(&restored, decoded)
	got := CaptureCanvas(&restored)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("restored canvas differs\ngot:  %#v\nwant: %#v", got, want)
	}
}

func TestStateCodecRejectsAmbiguousSlots(t *testing.T) {
	if _, err := encodeState([]StateSlot{{Key: 1}, {Key: 1}}); err == nil {
		t.Fatal("duplicate state keys were accepted")
	}
	if _, err := encodeState([]StateSlot{{Key: 0}}); err == nil {
		t.Fatal("reserved state key was accepted")
	}
}

func TestWorkerProtocolPaintStateAndDeferredClose(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	token := "test-token"
	t.Setenv(AddressEnvironment, listener.Addr().String())
	t.Setenv(TokenEnvironment, token)

	var clicked ui.ID
	state := []StateSlot{{Key: 7, Version: 2, Data: []byte("before")}}
	var allow func()
	serveDone := make(chan error, 1)
	go func() {
		serveDone <- Serve(WorkerConfig{
			Window: WindowConfig{Title: "test", Width: 320, Height: 200, HasClick: true, HasClose: true, HasSnapshot: true, HasRestore: true},
			Paint: func(canvas *ui.Canvas, size ui.Size) {
				canvas.Fill(ui.Rect{Width: size.Width, Height: size.Height}, ui.Color{Alpha: 1})
				if canvas.Clicked(ui.Item(91)) {
					clicked = ui.Item(91)
				}
			},
			CloseRequest: func(value func()) { allow = value },
			Snapshot:     func() ([]StateSlot, error) { return state, nil },
			Restore:      func(slots []StateSlot) error { state = slots; return nil },
		})
	}()
	connection, err := listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	hello, client, err := Accept(connection, token)
	if err != nil {
		t.Fatal(err)
	}
	if hello.Config.Title != "test" || !hello.Config.HasRestore {
		t.Fatalf("unexpected hello: %+v", hello)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	snapshot, err := client.Paint(ctx, PaintRequest{Width: 640, Height: 480})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Rectangles) != 1 || snapshot.Rectangles[0].Bounds.Width != 640 {
		t.Fatalf("unexpected paint response: %+v", snapshot)
	}
	if err := client.Click(ctx, ui.Item(91)); err != nil || clicked != ui.Item(91) {
		t.Fatalf("click = %v, err = %v", clicked, err)
	}
	wantState := []StateSlot{{Key: 8, Version: 3, Data: []byte("after")}}
	if err := client.RestoreState(ctx, wantState); err != nil {
		t.Fatal(err)
	}
	gotState, err := client.SnapshotState(ctx)
	if err != nil || !reflect.DeepEqual(gotState, wantState) {
		t.Fatalf("state = %#v, err = %v", gotState, err)
	}
	closed := make(chan struct{}, 1)
	client.SetCloseAllowed(func() { closed <- struct{}{} })
	if err := client.RequestClose(ctx); err != nil {
		t.Fatal(err)
	}
	if allow == nil {
		t.Fatal("close callback was not retained")
	}
	allow()
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("deferred close notification was not delivered")
	}
	if err := client.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-serveDone; err != nil {
		t.Fatal(err)
	}
}

func TestCanvasCodecPreservesGridLines(t *testing.T) {
	var canvas ui.Canvas
	canvas.Label("plain", ui.Rect{Width: 50, Height: 14}, ui.TextStyle{Size: 12})
	canvas.LabelRuns("ls 世界", ui.Rect{Y: 20, Width: 70, Height: 15}, ui.TextStyle{Size: 13, MonoFace: ui.MonoClassic}, 7.5, []ui.TextRun{
		{Column: 0, Bytes: 2, Cells: 2, Color: ui.Color{Green: 0.8, Alpha: 1}, Weight: 0.4},
		{Column: 3, Bytes: 3, Cells: 2, Color: ui.Color{Red: 1, Alpha: 1}, Italic: true, Underline: true, Strike: true},
	})
	canvas.LabelRuns("x", ui.Rect{Y: 40, Width: 7, Height: 15}, ui.TextStyle{Size: 13}, 7.5, nil)

	want := CaptureCanvas(&canvas)
	data, err := encodeCanvas(want)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeCanvas(data)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded.Runs, want.Runs) || !reflect.DeepEqual(decoded.Texts, want.Texts) {
		t.Fatalf("decoded grid lines differ\ngot:  %#v\nwant: %#v", decoded, want)
	}
	var restored ui.Canvas
	ApplyCanvas(&restored, decoded)
	if got := CaptureCanvas(&restored); !reflect.DeepEqual(got, want) {
		t.Fatalf("restored canvas differs\ngot:  %#v\nwant: %#v", got, want)
	}
}

func TestCanvasCodecRejectsRunRangesOutsideTheArena(t *testing.T) {
	var canvas ui.Canvas
	canvas.LabelRuns("ab", ui.Rect{Width: 14, Height: 15}, ui.TextStyle{Size: 13}, 7, []ui.TextRun{{Bytes: 2, Cells: 2}})
	snapshot := CaptureCanvas(&canvas)
	snapshot.Texts[0].RunCount = 9
	data, err := encodeCanvas(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeCanvas(data); err == nil {
		t.Fatal("a text pointing past the run arena was accepted")
	}
}

// The worker answers a request and exits, which closes the connection right
// behind the answer. The host must read the answer, not the close: losing it
// reported a complete request as io.EOF whenever the two arrived close enough
// together for the scheduler to run the close first.
func TestAnAnswerQueuedBeforeTheCloseIsNotLost(t *testing.T) {
	client := &Client{pending: map[uint32]chan response{}, done: make(chan struct{})}
	result := make(chan response, 1)
	client.pending[7] = result
	result <- response{frame: frame{kind: messageShutdown | responseFlag, id: 7, payload: []byte("answered")}}
	close(client.done)

	payload, err := client.await(context.Background(), 7, messageShutdown, result)
	if err != nil {
		t.Fatalf("a queued answer was reported as %v", err)
	}
	if string(payload) != "answered" {
		t.Fatalf("payload = %q", payload)
	}
}

func TestACloseWithoutAnAnswerIsStillAnEOF(t *testing.T) {
	client := &Client{pending: map[uint32]chan response{}, done: make(chan struct{})}
	result := make(chan response, 1)
	client.pending[7] = result
	close(client.done)

	if _, err := client.await(context.Background(), 7, messageShutdown, result); !errors.Is(err, io.EOF) {
		t.Fatalf("err = %v, want io.EOF", err)
	}
}
