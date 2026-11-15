//go:build unix

package dbus

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// callTimeout bounds how long Call waits for an answer.
const callTimeout = 5 * time.Second

// errTimedOut is what a blocking read reports when the socket's receive timeout
// passes with nothing to read.
var errTimedOut = errors.New("timed out")

// Conn is a connection to a message bus.
type Conn struct {
	fd      int
	serial  uint32
	unique  string
	in      []byte
	pending []*Message
	// fds are the descriptors the socket delivered for messages still being
	// assembled, in arrival order; each message takes its header's count.
	fds []int
}

// DialSession connects to the session bus named by DBUS_SESSION_BUS_ADDRESS, or
// by the per-user socket systemd and most desktops provide when it is unset.
func DialSession() (*Conn, error) {
	address := os.Getenv("DBUS_SESSION_BUS_ADDRESS")
	if address == "" {
		address = "unix:path=/run/user/" + strconv.Itoa(os.Getuid()) + "/bus"
	}
	return Dial(address)
}

// DialSystem connects to the system bus, where logind and other services that
// outlive one login session live.
func DialSystem() (*Conn, error) {
	address := os.Getenv("DBUS_SYSTEM_BUS_ADDRESS")
	if address == "" {
		// /var/run is a link to /run on every current distribution; try both
		// because a container may keep only one.
		address = "unix:path=/var/run/dbus/system_bus_socket;unix:path=/run/dbus/system_bus_socket"
	}
	return Dial(address)
}

// Dial connects to the bus at a "unix:" address, or the first of several
// separated by semicolons that accepts the connection. Other transports are not
// understood. A daemon with a bus of its own, such as an input method framework,
// is reached this way.
func Dial(address string) (*Conn, error) {
	var last error
	for _, candidate := range strings.Split(address, ";") {
		path, ok := unixPath(candidate)
		if !ok {
			last = fmt.Errorf("dbus: unsupported bus address %q", candidate)
			continue
		}
		fd, err := connect(path)
		if err != nil {
			last = err
			continue
		}
		conn, err := NewConn(fd)
		if err != nil {
			return nil, err
		}
		return conn, nil
	}
	if last == nil {
		last = errors.New("dbus: empty bus address")
	}
	return nil, last
}

// unixPath is the socket path of a "unix:" bus address; an abstract socket's
// name starts with "@".
func unixPath(address string) (string, bool) {
	rest, ok := strings.CutPrefix(address, "unix:")
	if !ok {
		return "", false
	}
	for _, pair := range strings.Split(rest, ",") {
		key, value, _ := strings.Cut(pair, "=")
		switch key {
		case "path":
			return unescape(value), true
		case "abstract":
			return "@" + unescape(value), true
		}
	}
	return "", false
}

// unescape undoes the %xx escapes of a bus address value.
func unescape(value string) string {
	if !strings.Contains(value, "%") {
		return value
	}
	var out []byte
	for index := 0; index < len(value); index++ {
		if value[index] == '%' && index+2 < len(value) {
			if decoded, err := hex.DecodeString(value[index+1 : index+3]); err == nil {
				out = append(out, decoded[0])
				index += 2
				continue
			}
		}
		out = append(out, value[index])
	}
	return string(out)
}

func connect(path string) (int, error) {
	fd, err := syscall.Socket(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		return -1, err
	}
	syscall.CloseOnExec(fd)
	if err := syscall.Connect(fd, &syscall.SockaddrUnix{Name: path}); err != nil {
		syscall.Close(fd)
		return -1, fmt.Errorf("dbus: connect %s: %w", path, err)
	}
	return fd, nil
}

// NewConn authenticates as the current user over an already connected socket
// and says Hello to the bus. It takes ownership of fd, closing it on failure.
func NewConn(fd int) (*Conn, error) {
	conn := &Conn{fd: fd}
	// A call or a write that the bus does not answer must not hang the thread.
	timeout := syscall.NsecToTimeval(callTimeout.Nanoseconds())
	if err := syscall.SetsockoptTimeval(fd, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &timeout); err != nil {
		syscall.Close(fd)
		return nil, err
	}
	if err := syscall.SetsockoptTimeval(fd, syscall.SOL_SOCKET, syscall.SO_SNDTIMEO, &timeout); err != nil {
		syscall.Close(fd)
		return nil, err
	}
	if err := conn.authenticate(); err != nil {
		syscall.Close(fd)
		return nil, err
	}
	reply, err := conn.Call("org.freedesktop.DBus", "/org/freedesktop/DBus", "org.freedesktop.DBus", "Hello", "", nil)
	if err != nil {
		conn.Close()
		return nil, err
	}
	conn.unique = reply.Args().String()
	return conn, nil
}

func (c *Conn) authenticate() error {
	uid := strconv.Itoa(os.Getuid())
	request := "\x00AUTH EXTERNAL " + hex.EncodeToString([]byte(uid)) + "\r\n"
	if err := c.writeAll([]byte(request)); err != nil {
		return err
	}
	line, err := c.readLine()
	if err != nil {
		return err
	}
	if !strings.HasPrefix(line, "OK") {
		return fmt.Errorf("dbus: the bus refused authentication: %s", line)
	}
	return c.writeAll([]byte("BEGIN\r\n"))
}

// readLine reads one CRLF-terminated line of the authentication exchange. It
// stops at the line, so nothing of the message stream after it is consumed.
func (c *Conn) readLine() (string, error) {
	var line []byte
	one := make([]byte, 1)
	for {
		n, err := syscall.Read(c.fd, one)
		if err == syscall.EINTR {
			continue
		}
		if err == syscall.EAGAIN {
			return "", errors.New("dbus: timed out during authentication")
		}
		if err != nil || n == 0 {
			return "", errors.New("dbus: the bus closed during authentication")
		}
		line = append(line, one[0])
		if bytes.HasSuffix(line, []byte("\r\n")) {
			return strings.TrimSpace(string(line)), nil
		}
	}
}

// FD is the socket to wait on for readability.
func (c *Conn) FD() int { return c.fd }

// UniqueName is the name the bus gave this connection.
func (c *Conn) UniqueName() string { return c.unique }

// Close ends the connection and closes every descriptor still attached to a
// message this client has not handed out.
func (c *Conn) Close() {
	for _, fd := range c.fds {
		syscall.Close(fd)
	}
	c.fds = nil
	for _, message := range c.pending {
		message.CloseFDs()
	}
	c.pending = nil
	if c.fd >= 0 {
		syscall.Close(c.fd)
		c.fd = -1
	}
}

func (c *Conn) nextSerial() uint32 {
	c.serial++
	return c.serial
}

// Call sends a method call and waits for its answer. Other messages that arrive
// meanwhile are kept for Receive. An error reply comes back as a *Error. The
// returned message owns any file descriptors it carries; close them when done,
// taking the ones to keep with TakeFD.
func (c *Conn) Call(destination, path, iface, member, signature string, body []byte) (*Message, error) {
	serial := c.nextSerial()
	call := &Message{Type: TypeCall, Serial: serial, Destination: destination, Path: path, Interface: iface, Member: member, Signature: signature, Body: body}
	if err := c.writeAll(call.encode()); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(callTimeout)
	for {
		for index, message := range c.pending {
			if (message.Type == TypeReturn || message.Type == TypeError) && message.ReplySerial == serial {
				c.pending = append(c.pending[:index], c.pending[index+1:]...)
				return answer(message)
			}
		}
		if !time.Now().Before(deadline) {
			return nil, fmt.Errorf("dbus: %s.%s was not answered", iface, member)
		}
		if err := c.fill(true); err != nil {
			return nil, fmt.Errorf("dbus: waiting for %s.%s: %w", iface, member, err)
		}
	}
}

// Send sends a method call without waiting for its answer and returns the
// call's serial. The answer, or an error reply, arrives later through Receive
// with that serial as its ReplySerial, so a caller can keep working, or wait on
// FD, until it does.
func (c *Conn) Send(destination, path, iface, member, signature string, body []byte) (uint32, error) {
	serial := c.nextSerial()
	call := &Message{Type: TypeCall, Serial: serial, Destination: destination, Path: path, Interface: iface, Member: member, Signature: signature, Body: body}
	if err := c.writeAll(call.encode()); err != nil {
		return 0, err
	}
	return serial, nil
}

func answer(message *Message) (*Message, error) {
	if message.Type == TypeError {
		text := ""
		if message.Signature == "s" {
			text = message.Args().String()
		}
		message.CloseFDs()
		return nil, &Error{Name: message.ErrorName, Text: text}
	}
	return message, nil
}

// Error is an error reply from the bus or from the other side of a call.
type Error struct{ Name, Text string }

func (e *Error) Error() string {
	if e.Text == "" {
		return e.Name
	}
	return e.Name + ": " + e.Text
}

// Receive returns the messages that have arrived, without waiting. The caller
// owns the file descriptors they carry.
func (c *Conn) Receive() ([]*Message, error) {
	if err := c.fill(false); err != nil {
		return nil, err
	}
	messages := c.pending
	c.pending = nil
	return messages, nil
}

// fill reads from the socket and parses every complete message into pending.
// Blocking, it waits for the first bytes (at most callTimeout) and reads once;
// otherwise it reads what is there and returns. File descriptors the socket
// delivers are held until the message whose header counts them is complete.
func (c *Conn) fill(block bool) error {
	flags := syscall.MSG_DONTWAIT
	if block {
		flags = 0
	}
	buffer := make([]byte, 16<<10)
	oob := make([]byte, 4096)
	for {
		n, oobn, _, _, err := syscall.Recvmsg(c.fd, buffer, oob, flags)
		if err == syscall.EAGAIN {
			if block {
				return errTimedOut
			}
			break
		}
		if err == syscall.EINTR {
			continue
		}
		if err != nil {
			return err
		}
		if oobn > 0 {
			if err := c.appendControl(oob[:oobn]); err != nil {
				return err
			}
		}
		if n == 0 {
			return errors.New("the bus closed the connection")
		}
		c.in = append(c.in, buffer[:n]...)
		if block || n < len(buffer) {
			break
		}
	}
	for {
		total, err := messageLength(c.in)
		if err != nil {
			return err
		}
		if total == 0 || len(c.in) < total {
			return nil
		}
		message, err := decodeMessage(c.in[:total])
		if err != nil {
			return err
		}
		for index := 0; index < message.unixFDCount; index++ {
			if len(c.fds) == 0 {
				message.CloseFDs()
				return errors.New("dbus: a message arrived without the file descriptors it counts")
			}
			message.FDs = append(message.FDs, c.fds[0])
			c.fds = c.fds[1:]
		}
		// Detach the body from the read buffer, which is about to be reused.
		message.Body = append([]byte(nil), message.Body...)
		c.pending = append(c.pending, message)
		c.in = c.in[total:]
		if len(c.in) == 0 {
			c.in = nil
		}
	}
}

// appendControl takes the file descriptors out of ancillary data set with a
// message, keeping them for the message that counts them.
func (c *Conn) appendControl(oob []byte) error {
	messages, err := syscall.ParseSocketControlMessage(oob)
	if err != nil {
		return err
	}
	for index := range messages {
		message := &messages[index]
		if message.Header.Level != syscall.SOL_SOCKET || message.Header.Type != syscall.SCM_RIGHTS {
			continue
		}
		fds, err := syscall.ParseUnixRights(message)
		if err != nil {
			return err
		}
		c.fds = append(c.fds, fds...)
	}
	return nil
}

// Reply answers a call with the given arguments.
func (c *Conn) Reply(call *Message, signature string, body []byte) error {
	if call.NoReply() {
		return nil
	}
	return c.writeAll((&Message{Type: TypeReturn, Serial: c.nextSerial(), ReplySerial: call.Serial, Destination: call.Sender, Signature: signature, Body: body}).encode())
}

// ReplyError answers a call with an error.
func (c *Conn) ReplyError(call *Message, name, text string) error {
	if call.NoReply() {
		return nil
	}
	var e Encoder
	e.String(text)
	return c.writeAll((&Message{Type: TypeError, Serial: c.nextSerial(), ReplySerial: call.Serial, Destination: call.Sender, ErrorName: name, Signature: "s", Body: e.Bytes()}).encode())
}

// Emit sends a signal to whoever has asked for it.
func (c *Conn) Emit(path, iface, member, signature string, body []byte) error {
	return c.writeAll((&Message{Type: TypeSignal, Flags: flagNoReply, Serial: c.nextSerial(), Path: path, Interface: iface, Member: member, Signature: signature, Body: body}).encode())
}

func (c *Conn) writeAll(data []byte) error {
	for len(data) > 0 {
		n, err := syscall.Write(c.fd, data)
		if err == syscall.EINTR {
			continue
		}
		if err != nil {
			return err
		}
		data = data[n:]
	}
	return nil
}
