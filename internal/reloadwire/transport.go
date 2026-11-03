package reloadwire

import (
	"context"
	"crypto/subtle"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"

	"github.com/vivaui/viva/ui"
)

var frameMagic = [4]byte{'G', 'O', 'P', 'R'}

type frame struct {
	kind    messageKind
	id      uint32
	payload []byte
}

func writeFrame(writer io.Writer, value frame) error {
	if len(value.payload) > MaxFrameBytes {
		return fmt.Errorf("reload protocol frame exceeds %d bytes", MaxFrameBytes)
	}
	var header [16]byte
	copy(header[:4], frameMagic[:])
	binary.LittleEndian.PutUint16(header[4:6], ProtocolVersion)
	binary.LittleEndian.PutUint16(header[6:8], uint16(value.kind))
	binary.LittleEndian.PutUint32(header[8:12], value.id)
	binary.LittleEndian.PutUint32(header[12:16], uint32(len(value.payload)))
	if err := writeAll(writer, header[:]); err != nil {
		return err
	}
	if len(value.payload) > 0 {
		return writeAll(writer, value.payload)
	}
	return nil
}

func writeAll(writer io.Writer, data []byte) error {
	for len(data) > 0 {
		written, err := writer.Write(data)
		if err != nil {
			return err
		}
		if written <= 0 {
			return io.ErrShortWrite
		}
		data = data[written:]
	}
	return nil
}

func readFrame(reader io.Reader) (frame, error) {
	var header [16]byte
	if _, err := io.ReadFull(reader, header[:]); err != nil {
		return frame{}, err
	}
	if subtle.ConstantTimeCompare(header[:4], frameMagic[:]) != 1 {
		return frame{}, fmt.Errorf("invalid reload protocol magic")
	}
	version := binary.LittleEndian.Uint16(header[4:6])
	if version != ProtocolVersion {
		return frame{}, fmt.Errorf("reload protocol version %d is not supported; want %d", version, ProtocolVersion)
	}
	length := binary.LittleEndian.Uint32(header[12:16])
	if length > MaxFrameBytes {
		return frame{}, fmt.Errorf("reload protocol frame is %d bytes; limit is %d", length, MaxFrameBytes)
	}
	value := frame{
		kind: messageKind(binary.LittleEndian.Uint16(header[6:8])),
		id:   binary.LittleEndian.Uint32(header[8:12]),
	}
	if length > 0 {
		value.payload = make([]byte, length)
		if _, err := io.ReadFull(reader, value.payload); err != nil {
			return frame{}, err
		}
	}
	return value, nil
}

type response struct {
	frame frame
	err   error
}

type Client struct {
	connection net.Conn
	writeMu    sync.Mutex
	pendingMu  sync.Mutex
	pending    map[uint32]chan response
	nextID     atomic.Uint32
	done       chan struct{}
	closeOnce  sync.Once
	notifyMu   sync.RWMutex
	onClose    func()
}

func Accept(connection net.Conn, token string) (Hello, *Client, error) {
	value, err := readFrame(connection)
	if err != nil {
		return Hello{}, nil, err
	}
	if value.kind != messageHello || value.id != 0 {
		return Hello{}, nil, fmt.Errorf("reload worker did not send a valid hello")
	}
	hello, err := decodeHello(value.payload)
	if err != nil {
		return Hello{}, nil, err
	}
	if subtle.ConstantTimeCompare([]byte(hello.Token), []byte(token)) != 1 {
		return Hello{}, nil, fmt.Errorf("reload worker authentication failed")
	}
	if err := writeFrame(connection, frame{kind: messageHelloAck}); err != nil {
		return Hello{}, nil, err
	}
	client := &Client{connection: connection, pending: make(map[uint32]chan response), done: make(chan struct{})}
	go client.readLoop()
	return hello, client, nil
}

func (client *Client) SetCloseAllowed(handler func()) {
	client.notifyMu.Lock()
	client.onClose = handler
	client.notifyMu.Unlock()
}

func (client *Client) Paint(ctx context.Context, request PaintRequest) (CanvasSnapshot, error) {
	payload, err := client.request(ctx, messagePaint, encodePaintRequest(request))
	if err != nil {
		return CanvasSnapshot{}, err
	}
	return decodeCanvas(payload)
}

func (client *Client) Click(ctx context.Context, id ui.ID) error {
	var payload encoder
	payload.id(id)
	_, err := client.request(ctx, messageClick, payload.data)
	return err
}

func (client *Client) Pointer(ctx context.Context, event ui.PointerEvent) (ui.EventResult, error) {
	payload, err := client.request(ctx, messagePointer, encodePointerEvent(event))
	if err != nil {
		return ui.EventResult{}, err
	}
	return decodeEventResult(payload)
}

func (client *Client) Key(ctx context.Context, event ui.KeyEvent) (ui.EventResult, error) {
	payload, err := client.request(ctx, messageKey, encodeKeyEvent(event))
	if err != nil {
		return ui.EventResult{}, err
	}
	return decodeEventResult(payload)
}

func (client *Client) Shortcut(ctx context.Context, event ui.KeyEvent) (ui.EventResult, error) {
	payload, err := client.request(ctx, messageShortcut, encodeKeyEvent(event))
	if err != nil {
		return ui.EventResult{}, err
	}
	return decodeEventResult(payload)
}

func (client *Client) Scroll(ctx context.Context, event ui.ScrollEvent) (ui.EventResult, error) {
	payload, err := client.request(ctx, messageScroll, encodeScrollEvent(event))
	if err != nil {
		return ui.EventResult{}, err
	}
	return decodeEventResult(payload)
}

func (client *Client) SnapshotState(ctx context.Context) ([]StateSlot, error) {
	payload, err := client.request(ctx, messageSnapshotState, nil)
	if err != nil {
		return nil, err
	}
	return decodeState(payload)
}

func (client *Client) RestoreState(ctx context.Context, slots []StateSlot) error {
	payload, err := encodeState(slots)
	if err != nil {
		return err
	}
	_, err = client.request(ctx, messageRestoreState, payload)
	return err
}

func (client *Client) RequestClose(ctx context.Context) error {
	_, err := client.request(ctx, messageCloseRequest, nil)
	return err
}

func (client *Client) Shutdown(ctx context.Context) error {
	_, err := client.request(ctx, messageShutdown, nil)
	return err
}

func (client *Client) Close() error {
	client.fail(io.EOF)
	return client.connection.Close()
}

func (client *Client) Done() <-chan struct{} { return client.done }

func (client *Client) request(ctx context.Context, kind messageKind, payload []byte) ([]byte, error) {
	id := client.nextID.Add(1)
	if id == 0 {
		id = client.nextID.Add(1)
	}
	result := make(chan response, 1)
	client.pendingMu.Lock()
	select {
	case <-client.done:
		client.pendingMu.Unlock()
		return nil, io.EOF
	default:
	}
	client.pending[id] = result
	client.pendingMu.Unlock()

	client.writeMu.Lock()
	err := writeFrame(client.connection, frame{kind: kind, id: id, payload: payload})
	client.writeMu.Unlock()
	if err != nil {
		client.removePending(id)
		client.fail(err)
		return nil, err
	}
	return client.await(ctx, id, kind, result)
}

// await waits for the answer to the request id. readLoop closes done only after
// it has delivered every answer it read, so an answer sitting in result is one
// the worker sent and the caller must get it rather than the close.
func (client *Client) await(ctx context.Context, id uint32, kind messageKind, result chan response) ([]byte, error) {
	select {
	case value := <-result:
		return answer(value, kind)
	case <-ctx.Done():
		client.removePending(id)
		return nil, ctx.Err()
	case <-client.done:
		// The worker answers and then exits, which closes the connection behind
		// the answer. readLoop delivers an answer before it closes done, so an
		// answer already in the channel is one the host did send: taking the
		// close first would report a response that arrived as io.EOF.
		select {
		case value := <-result:
			return answer(value, kind)
		default:
			client.removePending(id)
			return nil, io.EOF
		}
	}
}

// answer turns the frame readLoop delivered into the reply to a request.
func answer(value response, kind messageKind) ([]byte, error) {
	if value.err != nil {
		return nil, value.err
	}
	if value.frame.kind == messageError {
		return nil, errors.New(string(value.frame.payload))
	}
	if value.frame.kind != kind|responseFlag {
		return nil, fmt.Errorf("reload protocol response kind %d does not match request %d", value.frame.kind, kind)
	}
	return value.frame.payload, nil
}

func (client *Client) readLoop() {
	for {
		value, err := readFrame(client.connection)
		if err != nil {
			client.fail(err)
			return
		}
		if value.kind == messageCloseAllowed && value.id == 0 {
			client.notifyMu.RLock()
			handler := client.onClose
			client.notifyMu.RUnlock()
			if handler != nil {
				go handler()
			}
			continue
		}
		client.pendingMu.Lock()
		result := client.pending[value.id]
		delete(client.pending, value.id)
		client.pendingMu.Unlock()
		if result != nil {
			result <- response{frame: value}
		}
	}
}

func (client *Client) removePending(id uint32) {
	client.pendingMu.Lock()
	delete(client.pending, id)
	client.pendingMu.Unlock()
}

func (client *Client) fail(err error) {
	client.closeOnce.Do(func() {
		close(client.done)
		client.pendingMu.Lock()
		for id, result := range client.pending {
			delete(client.pending, id)
			result <- response{err: err}
		}
		client.pendingMu.Unlock()
	})
}
