package app

import (
	"os"

	"github.com/vivaui/viva/internal/autostart"
)

// ErrAutostartUnsupported means this system has no login-start mechanism Viva
// drives.
var ErrAutostartUnsupported = autostart.ErrUnsupported

// Autostart describes how an application starts when the user logs in.
type Autostart struct {
	// ID is a stable name for the entry: letters, digits, dot, dash and
	// underscore. The application's identifier will do.
	ID string
	// Name is the program's display name for the system's startup list. Empty
	// means the ID.
	Name string
	// Args are passed to the program at login, for instance to start hidden.
	Args []string
}

// SetAutostart turns starting this program at login on or off for the current
// user: a launch agent on macOS, an autostart entry on Linux, a Run value on
// Windows. Nothing runs until the next login. It records the path of the
// running executable, so a program that moves must call it again.
func SetAutostart(entry Autostart, enabled bool) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	return autostart.Set(autostart.Entry{ID: entry.ID, Name: entry.Name, Executable: executable, Args: entry.Args}, enabled)
}

// AutostartEnabled reports whether an entry with the id is installed.
func AutostartEnabled(id string) (bool, error) { return autostart.Enabled(id) }

// openURLHost is what a Host implements to receive links the system opens the
// application with.
type openURLHost interface {
	SetOpenURLHandler(handler func(url string))
}

// openFileHost is what a Host implements to receive documents the system opens
// the application with.
type openFileHost interface {
	SetOpenFileHandler(handler func(path string))
}

// OnOpenURL sets who receives a link the system opens the application with: one
// whose scheme the application registered (viva.toml url_schemes). It is called
// on the event-loop thread, also for links that arrived while the application
// was starting. Call it from the startup callback.
//
// macOS delivers such links to the running program. Windows and Linux start a
// program with the link as an argument instead; package app/singleinstance carries
// it to the running copy, and the program reads it from its arguments at launch. Where the
// system delivers nothing, the handler is never called.
func (application *Application) OnOpenURL(handler func(url string)) {
	if application == nil || application.host == nil {
		return
	}
	if host, ok := application.host.(openURLHost); ok {
		host.SetOpenURLHandler(handler)
	}
}

// OnOpenFile sets who receives a document the system opens the application
// with: one of the types the application registered in viva.toml. It is called
// on the event-loop thread, also for documents that arrived while the
// application was starting. Call it from the startup callback.
//
// macOS delivers documents to the running program. Windows and Linux start the
// program with the path as an argument instead; package app/singleinstance
// carries it to the running copy, and the program reads it from its arguments
// at launch. Where the system delivers nothing, the handler is never called.
func (application *Application) OnOpenFile(handler func(path string)) {
	if application == nil || application.host == nil {
		return
	}
	if host, ok := application.host.(openFileHost); ok {
		host.SetOpenFileHandler(handler)
	}
}
