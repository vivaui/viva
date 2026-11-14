// Package instance makes an application run once per user. The first process
// listens at the claim address; a later one connects, hands over its command
// line and exits. No file lock, polling or timer is involved: the first process
// holds one blocked accept, and nothing else runs until a second process starts.
//
// The address belongs to the user, not to the environment: a copy started from
// a desktop and one started from a terminal must find each other, and their
// TMPDIR and XDG_RUNTIME_DIR can differ (an SSH session, a service or a shell
// profile can leave the first unset or set it to another place). If the claim
// followed the environment, both copies would listen and both would believe they
// are the one running copy, writing the same saved data from two processes.
//
// The platform files say where the address is. macOS and Linux claim at a Unix
// socket in a per-user directory. Windows claims at a per-user named pipe:
// AF_UNIX exists only from Windows 10 1803 and Windows Server 2019, while the
// named pipe is there on every Windows a Go program runs on.
package instance

import (
	"bufio"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"
)

const (
	maxMessage = 1 << 20
	maxArgs    = 4096
	ioDeadline = 3 * time.Second
)

// Instance is the first process's claim on the application.
type Instance struct {
	listener net.Listener
	address  string
	once     sync.Once
	done     chan struct{}
}

// Claim becomes the running instance of id, or hands args to the one that is
// and reports primary false. When primary, handler is called on its own
// goroutine with the arguments of every later launch. The id names the
// application; two applications with different ids never meet.
func Claim(id string, args []string, handler func(args []string)) (instance *Instance, primary bool, err error) {
	if id == "" {
		return nil, false, errors.New("instance: an application id is required")
	}
	address, err := claimAddress(id)
	if err != nil {
		return nil, false, err
	}
	var listenErr error
	for attempt := 0; attempt < 3; attempt++ {
		listener, attemptErr := listen(address)
		if attemptErr == nil {
			instance = &Instance{listener: listener, address: address, done: make(chan struct{})}
			go instance.serve(handler)
			return instance, true, nil
		}
		listenErr = attemptErr
		sendErr := send(address, args)
		if sendErr == nil {
			return nil, false, nil
		}
		if !errors.Is(sendErr, errNobody) {
			return nil, false, sendErr
		}
		// Nobody answers: the address is left from a process that died.
		discard(address)
	}
	return nil, false, claimFailure(address, listenErr)
}

// claimFailure is the error of a claim that could be neither handed to a running
// copy nor made by this process. Both halves are named: the platform's own
// reason for refusing the address is what a report needs, and leaving it out
// made a platform without that address look like an application that was not
// running.
func claimFailure(address string, listenErr error) error {
	return fmt.Errorf("instance: cannot claim %s: %w, and this process cannot listen there: %v", address, errNobody, listenErr)
}

// Close stops listening and removes the claim. It is idempotent.
func (instance *Instance) Close() {
	if instance == nil {
		return
	}
	instance.once.Do(func() {
		instance.listener.Close()
		discard(instance.address)
		<-instance.done
	})
}

var errNobody = errors.New("instance: no process is listening")

// claimName is the part of the address that names the application. A digest
// keeps the address inside the length limit of both a socket path and a pipe
// name whatever the id is.
func claimName(id string) string {
	sum := sha256.Sum256([]byte(id))
	return "i-" + hex.EncodeToString(sum[:6])
}

func (instance *Instance) serve(handler func([]string)) {
	defer close(instance.done)
	for {
		conn, err := instance.listener.Accept()
		if err != nil {
			return
		}
		args, err := readArgs(conn)
		if err == nil {
			handler(args)
			conn.Write([]byte{1})
		}
		conn.Close()
	}
}

// sendArgs hands args to a connected running process and waits for its
// acknowledgement. The connection is closed either way.
func sendArgs(conn net.Conn, args []string) error {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(ioDeadline))
	if err := writeArgs(conn, args); err != nil {
		return err
	}
	var ack [1]byte
	if _, err := io.ReadFull(conn, ack[:]); err != nil {
		return fmt.Errorf("instance: the running process did not answer: %w", err)
	}
	return nil
}

// The wire form is a 4-byte length, then the arguments each ended by NUL.
func writeArgs(w io.Writer, args []string) error {
	var body strings.Builder
	for _, arg := range args {
		if strings.IndexByte(arg, 0) >= 0 {
			return errors.New("instance: an argument contains NUL")
		}
		body.WriteString(arg)
		body.WriteByte(0)
	}
	if body.Len() > maxMessage {
		return errors.New("instance: the arguments are too long")
	}
	var header [4]byte
	binary.BigEndian.PutUint32(header[:], uint32(body.Len()))
	_, err := w.Write(append(header[:], body.String()...))
	return err
}

func readArgs(conn net.Conn) ([]string, error) {
	conn.SetDeadline(time.Now().Add(ioDeadline))
	reader := bufio.NewReader(conn)
	var header [4]byte
	if _, err := io.ReadFull(reader, header[:]); err != nil {
		return nil, err
	}
	size := binary.BigEndian.Uint32(header[:])
	if size > maxMessage {
		return nil, errors.New("instance: message too long")
	}
	body := make([]byte, size)
	if _, err := io.ReadFull(reader, body); err != nil {
		return nil, err
	}
	var args []string
	for len(body) > 0 {
		end := 0
		for end < len(body) && body[end] != 0 {
			end++
		}
		if end == len(body) {
			return nil, errors.New("instance: unterminated argument")
		}
		if len(args) == maxArgs {
			return nil, errors.New("instance: too many arguments")
		}
		args = append(args, string(body[:end]))
		body = body[end+1:]
	}
	return args, nil
}
