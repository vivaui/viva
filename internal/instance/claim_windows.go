//go:build windows

package instance

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"os"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
)

// pipePrefix is where the named pipes of the machine live.
const pipePrefix = `\\.\pipe\`

// claimAddress is the pipe the claim lives at. The pipe namespace is
// machine-wide, not per user like the Unix claim directory, so the name carries
// a digest of the user's SID: two users running the same application must not
// find each other's pipe in the way. The pipe itself rejects remote clients and
// admits only the user who made it, so the name is the other half of keeping
// copies apart. Unlike a socket file, the pipe needs no directory, no permission
// check and no clean-up: it exists exactly as long as its process does.
func claimAddress(id string) (string, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return "", fmt.Errorf("instance: the user of this process: %w", err)
	}
	sum := sha256.Sum256([]byte(user.User.Sid.String()))
	return pipePrefix + "viva-" + hex.EncodeToString(sum[:6]) + "-" + claimName(id), nil
}

// listen takes the pipe name. It fails while another copy's pipe has it, which
// is what makes the first process the one that runs.
func listen(address string) (net.Listener, error) {
	return winio.ListenPipe(address, nil)
}

// send hands args to the copy listening on the pipe. A pipe that is not there
// fails with ERROR_FILE_NOT_FOUND, which means no copy is running; any other
// failure means a copy exists and could not be reached, so its claim is not
// taken over.
func send(address string, args []string) error {
	timeout := ioDeadline
	conn, err := winio.DialPipe(address, &timeout)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%w: %v", errNobody, err)
		}
		return fmt.Errorf("instance: the running copy cannot be reached: %w", err)
	}
	return sendArgs(conn, args)
}

// discard has nothing to remove: a named pipe goes away with the process that
// made it, so a claim left by a crash is not there for the next launch to find.
func discard(string) {}
