//go:build linux

package linux

import (
	"errors"
	"fmt"
	"sync"
	"syscall"

	"github.com/vivaui/viva/internal/dbus"
)

// ErrSleepUnavailable reports that this system has no logind to hold an
// inhibitor lock, or that it refused one.
var ErrSleepUnavailable = errors.New("linux: sleep inhibition is unavailable")

// PreventSleep asks logind to keep the machine out of idle sleep until release
// runs. logind answers with a lock held through file descriptor; the connection
// and the descriptor stay open for as long as the lock is wanted, and the lock
// is gone when release closes them.
func PreventSleep(reason string) (release func(), err error) {
	conn, err := dbus.DialSystem()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSleepUnavailable, err)
	}
	var body dbus.Encoder
	body.String("idle")
	body.String(reason)
	body.String(reason)
	body.String("block")
	reply, err := conn.Call(
		"org.freedesktop.login1", "/org/freedesktop/login1",
		"org.freedesktop.login1.Manager", "Inhibit", "ssss", body.Bytes(),
	)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("%w: %v", ErrSleepUnavailable, err)
	}
	lock := reply.TakeFD(reply.Args().UnixFD())
	reply.CloseFDs()
	if lock < 0 {
		conn.Close()
		return nil, fmt.Errorf("%w: logind returned no lock", ErrSleepUnavailable)
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			// Closing the descriptor releases the lock; closing the
			// connection ends the session that held it.
			syscall.Close(lock)
			conn.Close()
		})
	}, nil
}
