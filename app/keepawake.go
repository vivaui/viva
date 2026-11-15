package app

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// ErrSleepPreventionUnavailable reports that this system cannot be asked to
// stay awake: it has no power service to hold a request for this process, or
// the one it has refused. The Web target has none, and a Linux session without
// logind has none.
var ErrSleepPreventionUnavailable = errors.New("app: preventing system sleep is unavailable")

// maxSleepReasonLength bounds the reason a platform shows in its power UI.
const maxSleepReasonLength = 256

// PreventSleep asks the operating system to keep the machine out of idle sleep
// while the returned release function has not been called. Use it for work
// whose interruption costs more than the sleep would: a file transfer, a long
// command, a remote desktop session. It does not keep the display on, and it
// does not prevent the user from sleeping the machine deliberately through the
// system's own controls.
//
// reason names the work and is what the system's power diagnostics show: pmset
// -g assertions on macOS, powercfg /requests on Windows, systemd-inhibit --list
// on Linux. It is required, at most 256 characters, and has no control
// characters.
//
// Every call is a separate request. Release each one exactly once; releasing
// twice is harmless, and the request also ends with the process. The call is
// safe from any goroutine, is bounded, and starts no goroutine, timer or
// polling loop.
//
// Where the system has no such service the call returns an error wrapping
// ErrSleepPreventionUnavailable: the Web target, and a Linux session without
// logind. Supports(CapabilityPreventSleep) reports the build target's answer
// before the first call.
func PreventSleep(reason string) (release func(), err error) {
	if strings.TrimSpace(reason) == "" {
		return nil, errors.New("app: a sleep-prevention reason is required")
	}
	if utf8.RuneCountInString(reason) > maxSleepReasonLength {
		return nil, fmt.Errorf("app: a sleep-prevention reason is at most %d characters", maxSleepReasonLength)
	}
	if strings.ContainsAny(reason, "\x00\r\n") {
		return nil, errors.New("app: a sleep-prevention reason has no control characters")
	}
	if release, err, handled := driverPreventSleep(reason); handled {
		return release, err
	}
	return nativePreventSleep(reason)
}
