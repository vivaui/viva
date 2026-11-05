package reloadwire

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"github.com/vivaui/viva/ui"
)

const (
	AddressEnvironment = "VIVA_RELOAD_ADDRESS"
	TokenEnvironment   = "VIVA_RELOAD_TOKEN"
)

type WorkerConfig struct {
	lastSize     ui.Size // the size of the last paint request, for the event pass
	Window       WindowConfig
	Canvas       *ui.Canvas
	Paint        func(*ui.Canvas, ui.Size)
	Pointer      func(ui.PointerEvent) ui.EventResult
	Key          func(ui.KeyEvent) ui.EventResult
	Shortcut     func(ui.KeyEvent) ui.EventResult
	Scroll       func(ui.ScrollEvent) ui.EventResult
	ModifierTap  func(ui.ModifierTapEvent) ui.EventResult
	CloseRequest func(func())
	Snapshot     func() ([]StateSlot, error)
	Restore      func([]StateSlot) error
}

func Enabled() bool {
	return os.Getenv(AddressEnvironment) != "" || os.Getenv(TokenEnvironment) != ""
}

func Serve(config WorkerConfig) error {
	address := os.Getenv(AddressEnvironment)
	token := os.Getenv(TokenEnvironment)
	if address == "" || token == "" {
		return fmt.Errorf("reload worker requires both %s and %s", AddressEnvironment, TokenEnvironment)
	}
	connection, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		return fmt.Errorf("connect reload host: %w", err)
	}
	defer connection.Close()
	payload, err := encodeHello(Hello{Token: token, Config: config.Window})
	if err != nil {
		return err
	}
	if err := writeFrame(connection, frame{kind: messageHello, payload: payload}); err != nil {
		return err
	}
	ack, err := readFrame(connection)
	if err != nil {
		return err
	}
	if ack.kind != messageHelloAck || ack.id != 0 || len(ack.payload) != 0 {
		return fmt.Errorf("reload host rejected worker handshake")
	}

	canvas := config.Canvas
	if canvas == nil {
		canvas = &ui.Canvas{}
	}
	base := CaptureCanvas(canvas)
	var writeMu sync.Mutex
	respond := func(value frame) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return writeFrame(connection, value)
	}

	for {
		request, readErr := readFrame(connection)
		if readErr != nil {
			if errors.Is(readErr, io.EOF) || errors.Is(readErr, net.ErrClosed) {
				return nil
			}
			return readErr
		}
		if request.id == 0 {
			return fmt.Errorf("reload worker received request without an id")
		}
		responsePayload, handleErr, shutdown := handleRequest(&config, canvas, base, request, respond)
		responseKind := request.kind | responseFlag
		if handleErr != nil {
			responseKind = messageError
			responsePayload = []byte(handleErr.Error())
		}
		if err := respond(frame{kind: responseKind, id: request.id, payload: responsePayload}); err != nil {
			return nil
		}
		if shutdown {
			return nil
		}
	}
}

func handleRequest(config *WorkerConfig, canvas *ui.Canvas, base CanvasSnapshot, request frame, respond func(frame) error) ([]byte, error, bool) {
	switch request.kind {
	case messagePaint:
		value, err := decodePaintRequest(request.payload)
		if err != nil {
			return nil, err, false
		}
		canvas.SetInteraction(value.Hovered, value.Pressed, value.Focused, value.FocusVisible)
		canvas.SetScroll(value.ScrollX, value.ScrollY)
		canvas.SetReduceMotion(value.ReduceMotion)
		if config.Paint != nil {
			canvas.Reset()
			// The host owns the clock: transitions advance on the frame time it sent.
			canvas.SetClock(func() time.Duration { return time.Duration(value.Now) })
			canvas.BeginFrame()
			config.lastSize = ui.Size{Width: value.Width, Height: value.Height}
			config.Paint(canvas, config.lastSize)
			canvas.EndFrame()
		} else {
			ApplyCanvas(canvas, base)
		}
		payload, err := encodeCanvas(CaptureCanvas(canvas))
		return payload, err, false
	case messageClick:
		value := decoder{data: request.payload}
		id := value.id()
		if err := value.finish(); err != nil {
			return nil, err, false
		}
		// A paint that asks Clicked takes the click in its event pass, here in the
		// worker, where the application's state lives.
		if config.Paint != nil && canvas.ListensForClicks() {
			canvas.BeginEventPass(id)
			canvas.Reset()
			canvas.BeginFrame()
			config.Paint(canvas, config.lastSize)
			canvas.EndFrame()
			canvas.EndEventPass()
		}
		return nil, nil, false
	case messagePointer:
		event, err := decodePointerEvent(request.payload)
		if err != nil {
			return nil, err, false
		}
		result := ui.EventResult{}
		if config.Pointer != nil {
			result = config.Pointer(event)
		}
		// A paint that asks PointerOn takes the event in its own pass, here in the
		// worker, where its state lives.
		if !result.Handled && config.Paint != nil && canvas.ListensForPointer() {
			canvas.BeginPointerPass(event)
			canvas.Reset()
			canvas.BeginFrame()
			config.Paint(canvas, config.lastSize)
			canvas.EndFrame()
			if canvas.EndPointerPass() {
				result.Handled, result.Repaint = true, true
			}
		}
		return encodeEventResult(result), nil, false
	case messageKey:
		event, err := decodeKeyEvent(request.payload)
		if err != nil {
			return nil, err, false
		}
		result := ui.EventResult{}
		if config.Key != nil {
			result = config.Key(event)
		}
		return encodeEventResult(result), nil, false
	case messageShortcut:
		event, err := decodeKeyEvent(request.payload)
		if err != nil {
			return nil, err, false
		}
		result := ui.EventResult{}
		if config.Shortcut != nil {
			result = config.Shortcut(event)
		}
		return encodeEventResult(result), nil, false
	case messageScroll:
		event, err := decodeScrollEvent(request.payload)
		if err != nil {
			return nil, err, false
		}
		result := ui.EventResult{}
		if config.Scroll != nil {
			result = config.Scroll(event)
		}
		return encodeEventResult(result), nil, false
	case messageModifierTap:
		event, err := decodeModifierTapEvent(request.payload)
		if err != nil {
			return nil, err, false
		}
		result := ui.EventResult{}
		if config.ModifierTap != nil {
			result = config.ModifierTap(event)
		}
		return encodeEventResult(result), nil, false
	case messageSnapshotState:
		if len(request.payload) != 0 {
			return nil, fmt.Errorf("snapshot-state request must be empty"), false
		}
		if config.Snapshot == nil {
			payload, err := encodeState(nil)
			return payload, err, false
		}
		slots, err := config.Snapshot()
		if err != nil {
			return nil, err, false
		}
		payload, err := encodeState(slots)
		return payload, err, false
	case messageRestoreState:
		slots, err := decodeState(request.payload)
		if err != nil {
			return nil, err, false
		}
		if config.Restore != nil {
			err = config.Restore(slots)
		}
		return nil, err, false
	case messageCloseRequest:
		if len(request.payload) != 0 {
			return nil, fmt.Errorf("close request must be empty"), false
		}
		if config.CloseRequest == nil {
			_ = respond(frame{kind: messageCloseAllowed})
			return nil, nil, false
		}
		var once sync.Once
		config.CloseRequest(func() {
			once.Do(func() { _ = respond(frame{kind: messageCloseAllowed}) })
		})
		return nil, nil, false
	case messageShutdown:
		if len(request.payload) != 0 {
			return nil, fmt.Errorf("shutdown request must be empty"), false
		}
		return nil, nil, true
	default:
		return nil, fmt.Errorf("unsupported reload request kind %d", request.kind), false
	}
}
