// Package instance makes an application run once per user. The first process
// listens on a local socket; a later one connects, hands over its command line
// and exits. No file lock, polling or timer is involved: the first process
// holds one blocked accept, and nothing else runs until a second process starts.
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
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	maxMessage = 1 << 20
	maxArgs    = 4096
	ioDeadline = 3 * time.Second
	// maxSocketPath is below the 104 bytes a Unix socket path may have on macOS.
	maxSocketPath = 100
)

// Instance is the first process's claim on the application.
type Instance struct {
	listener net.Listener
	path     string
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
	path, err := socketPath(id)
	if err != nil {
		return nil, false, err
	}
	for attempt := 0; attempt < 3; attempt++ {
		listener, listenErr := net.Listen("unix", path)
		if listenErr == nil {
			os.Chmod(path, 0o600)
			instance = &Instance{listener: listener, path: path, done: make(chan struct{})}
			go instance.serve(handler)
			return instance, true, nil
		}
		sendErr := send(path, args)
		if sendErr == nil {
			return nil, false, nil
		}
		if !errors.Is(sendErr, errNobody) {
			return nil, false, sendErr
		}
		// Nobody answers: the socket is left from a process that died.
		os.Remove(path)
	}
	return nil, false, fmt.Errorf("instance: cannot claim %s: %w", path, errNobody)
}

// Close stops listening and removes the socket. It is idempotent.
func (instance *Instance) Close() {
	if instance == nil {
		return
	}
	instance.once.Do(func() {
		instance.listener.Close()
		os.Remove(instance.path)
		<-instance.done
	})
}

var errNobody = errors.New("instance: no process is listening")

// socketPath is where the id's socket lives: in a directory only this user can
// enter, short enough for the 104-byte limit of a Unix socket path.
func socketPath(id string) (string, error) {
	sum := sha256.Sum256([]byte(id))
	name := "i-" + hex.EncodeToString(sum[:6]) + ".sock"
	base := os.Getenv("XDG_RUNTIME_DIR")
	if base == "" || runtime.GOOS != "linux" {
		base = os.TempDir()
	}
	dir := filepath.Join(base, fmt.Sprintf("viva-%d", os.Getuid()))
	if runtime.GOOS == "windows" {
		dir = filepath.Join(base, "viva")
	}
	if len(dir)+len(name)+1 > maxSocketPath {
		if runtime.GOOS != "windows" {
			// A very long temporary directory: /tmp is always short.
			dir = filepath.Join("/tmp", fmt.Sprintf("viva-%d", os.Getuid()))
		} else {
			// Windows AF_UNIX paths share the sockaddr limit, and there is
			// no short system directory to fall back to; the 8.3 short form
			// of the temporary directory usually fits.
			if short, ok := shortPath(base); ok {
				dir = filepath.Join(short, "viva")
			}
		}
	}
	if socket := filepath.Join(dir, name); len(socket) > maxSocketPath {
		return "", fmt.Errorf("instance: socket path %s is %d bytes, over the %d-byte limit; point TMPDIR and TEMP at a shorter directory", socket, len(socket), maxSocketPath)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("instance: %s is not a directory", dir)
	}
	if runtime.GOOS != "windows" {
		if info.Mode().Perm()&0o077 != 0 {
			if err := os.Chmod(dir, 0o700); err != nil {
				return "", fmt.Errorf("instance: %s is open to other users: %w", dir, err)
			}
		}
		if !ownedByUser(info) {
			return "", fmt.Errorf("instance: %s belongs to another user", dir)
		}
	}
	return filepath.Join(dir, name), nil
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

// send hands args to the process listening on path.
func send(path string, args []string) error {
	conn, err := net.DialTimeout("unix", path, ioDeadline)
	if err != nil {
		return fmt.Errorf("%w: %v", errNobody, err)
	}
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
