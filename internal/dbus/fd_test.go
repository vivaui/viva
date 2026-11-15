//go:build unix

package dbus

import (
	"syscall"
	"testing"
)

// TestReceiveTakesTheFileDescriptorsAMessageCarries is the shape of logind's
// inhibitor answer: a message whose body names a descriptor index and whose
// header counts one descriptor, written in the same sendmsg.
func TestReceiveTakesTheFileDescriptorsAMessageCarries(t *testing.T) {
	pair, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.Close(pair[1])

	var body Encoder
	body.Uint32(0)
	wire := (&Message{Type: TypeReturn, Serial: 1, Signature: "h", Body: body.Bytes(), unixFDCount: 1}).encode()

	fd, err := syscall.Open("/dev/null", syscall.O_RDONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Sendmsg(pair[1], wire, syscall.UnixRights(fd), nil, 0); err != nil {
		syscall.Close(fd)
		t.Fatal(err)
	}
	syscall.Close(fd)

	conn := &Conn{fd: pair[0]}
	defer conn.Close()
	messages, err := conn.Receive()
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 {
		t.Fatalf("received %d messages, want 1", len(messages))
	}
	message := messages[0]
	if len(message.FDs) != 1 {
		t.Fatalf("message carries %d descriptors, want 1", len(message.FDs))
	}
	if got := message.Args().UnixFD(); got != 0 {
		t.Fatalf("fd index = %d, want 0", got)
	}
	received := message.TakeFD(0)
	if received < 0 {
		t.Fatal("TakeFD returned no descriptor")
	}
	if err := syscall.Close(received); err != nil {
		t.Fatalf("closing the received descriptor: %v", err)
	}
	if len(message.FDs) != 0 {
		t.Fatalf("TakeFD left %d descriptors", len(message.FDs))
	}
}

// TestConnCloseClosesDescriptorsItHasNotHandedOut checks the leak that matters
// in a long-lived process: a descriptor still attached to a pending message is
// closed with the connection.
func TestConnCloseClosesDescriptorsItHasNotHandedOut(t *testing.T) {
	pair, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.Close(pair[1])

	wire := (&Message{Type: TypeReturn, Serial: 1, Signature: "h", Body: nil, unixFDCount: 1}).encode()
	fd, err := syscall.Open("/dev/null", syscall.O_RDONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Sendmsg(pair[1], wire, syscall.UnixRights(fd), nil, 0); err != nil {
		syscall.Close(fd)
		t.Fatal(err)
	}
	syscall.Close(fd)

	conn := &Conn{fd: pair[0]}
	if err := conn.fill(false); err != nil {
		t.Fatal(err)
	}
	if len(conn.pending) != 1 || len(conn.pending[0].FDs) != 1 {
		t.Fatalf("parsed %d messages", len(conn.pending))
	}
	received := conn.pending[0].FDs[0]
	conn.Close()
	if err := syscall.Close(received); err != syscall.EBADF {
		if err == nil {
			t.Fatal("the connection left a descriptor open")
		}
	}
}
